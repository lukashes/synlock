package synlock

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/go-redis/redis"
)

// lua scripts
var (
	redisDelScript = redis.NewScript(
		`if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("del", KEYS[1])
else
	return 0
end`)

	redisExpireScript = redis.NewScript(
		`if redis.call("get", KEYS[1]) == ARGV[1] then
	return redis.call("expire", KEYS[1], ARGV[2])
else
	return 0
end`)
)

var (
	ErrRedisInvalidAddr     = errors.New("invalid redis address")
	ErrRedisKeyDoesNotExist = errors.New("key does not exist")
	ErrRedisInvalidPrefix   = errors.New("invalid redis key prefix")
)

var DefRedisOpts = RedisOpts{
	Host:   "127.0.0.1",
	Port:   "6379",
	DB:     0,
	Prefix: "synlock",
}

type RedisOpts struct {
	Host   string
	Port   string
	DB     int
	Prefix string
}

type Redis struct {
	client *redis.Client
	prefix string
}

func NewRedis(conf RedisOpts) (*Redis, error) {
	if conf.Host == "" || conf.Port == "" {
		return nil, ErrRedisInvalidAddr
	}

	if conf.Prefix == "" {
		return nil, ErrRedisInvalidPrefix
	}

	return &Redis{
		client: redis.NewClient(&redis.Options{
			Addr:       net.JoinHostPort(conf.Host, conf.Port),
			DB:         conf.DB,
			MaxRetries: 3,
		}),
		prefix: conf.Prefix,
	}, nil
}

func (r *Redis) NewMutex(key int64) (Mutex, error) {
	return &RedisMutex{
		client: r.client,
		key:    fmt.Sprintf("%s:%d", r.prefix, key),
	}, nil
}

type RedisMutex struct {
	client  *redis.Client
	key     string
	monitor chan struct{}
	mu      sync.Mutex
	tok     string
}

func (s *RedisMutex) Lock() error {
	s.mu.Lock()
	return s.lock()
}

func (s *RedisMutex) Unlock() error {
	defer s.mu.Unlock()
	return s.unlock()
}

func (s *RedisMutex) lock() error {
	var (
		ok     bool
		err    error
		jitter time.Duration
	)

	token := make([]byte, 20)
	if _, err = rand.Read(token); err != nil {
		return err
	}

	s.tok = hex.EncodeToString(token)

	for !ok {
		if jitter > 0 {
			time.Sleep(jitter)
		}

		ok, err = s.client.SetNX(s.key, s.tok, time.Minute).Result()
		if err != nil {
			return err
		}

		switch {
		case jitter == 0:
			jitter = 10 * time.Millisecond
		case jitter > time.Second:
			jitter = time.Second
		default:
			jitter *= 2
		}
	}

	s.monitor = make(chan struct{})

	go func() {
		var t = time.NewTicker(time.Second * 10)
		defer t.Stop()

		for {
			select {
			case <-t.C:
			case <-s.monitor:
				return
			}

			val, err := redisExpireScript.Run(s.client, []string{s.key}, s.tok, int(time.Minute/time.Second)).Result()

			select {
			case <-s.monitor:
				return
			default:
			}

			if err != nil || castToInt(val) != 1 {
				log.Printf("Can't reset session expiration: %s", err)
			}
		}
	}()

	return nil
}

func (s *RedisMutex) unlock() error {
	close(s.monitor)

	val, err := redisDelScript.Run(s.client, []string{s.key}, s.tok).Result()
	if err != nil {
		return err
	}

	if castToInt(val) != 1 {
		return ErrRedisKeyDoesNotExist
	}

	return nil
}

func castToInt(any interface{}) int {
	switch t := any.(type) {
	case int:
		return t
	case int16:
		return int(t)
	case int32:
		return int(t)
	case int64:
		return int(t)
	case uint:
		return int(t)
	case uint16:
		return int(t)
	case uint32:
		return int(t)
	case uint64:
		return int(t)
	}

	return 0
}
