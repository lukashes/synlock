package synlock

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/go-redis/redis/v8"
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
	Host, Port         string
	DB                 int
	Username, Password string
	TLSConfig          *tls.Config
	Prefix             string
}

type Redis struct {
	client *redis.Client
	prefix string
	opts   *options
}

func MustRedisMutex(mu Mutex) *RedisMutex {
	switch t := mu.(type) {
	case *RedisMutex:
		return t
	}

	return nil
}

func NewRedis(conf RedisOpts, opts ...Option) (*Redis, error) {
	if conf.Host == "" || conf.Port == "" {
		return nil, ErrRedisInvalidAddr
	}

	if conf.Prefix == "" {
		return nil, ErrRedisInvalidPrefix
	}

	o := new(options)

	o.errorHandler = func(key string, err error) {
		log.Printf("Can't reset session expiration for key %s: %s", key, err)
	}

	for _, opt := range opts {
		opt(o)
	}

	return &Redis{
		client: redis.NewClient(&redis.Options{
			Addr:       net.JoinHostPort(conf.Host, conf.Port),
			DB:         conf.DB,
			Username:   conf.Username,
			Password:   conf.Password,
			TLSConfig:  conf.TLSConfig,
			MaxRetries: 3,
		}),
		prefix: conf.Prefix,
		opts:   o,
	}, nil
}

func (r *Redis) NewMutex(key int64) (Mutex, error) {
	return &RedisMutex{
		client: r.client,
		key:    fmt.Sprintf("%s:%d", r.prefix, key),
		mu:     make(chan struct{}, 1),
		opts:   r.opts,
	}, nil
}

type RedisMutex struct {
	client  *redis.Client
	key     string
	monitor chan struct{}
	mu      chan struct{}
	tok     string
	opts    *options
}

func (s *RedisMutex) LockContext(ctx context.Context) (err error) {
	s.mu <- struct{}{}
	defer func() {
		if err != nil {
			<-s.mu
		}
	}()

	return s.lock(ctx)
}

func (s *RedisMutex) Lock() (err error) {
	s.mu <- struct{}{}
	defer func() {
		if err != nil {
			<-s.mu
		}
	}()

	return s.lock(context.Background())
}

func (s *RedisMutex) Unlock() error {
	defer func() {
		select {
		case <-s.mu:
		default:
		}
	}()

	return s.unlock()
}

func (s *RedisMutex) lock(ctx context.Context) error {
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
		select {
		case <-time.After(jitter):
		case <-ctx.Done():
			return ctx.Err()
		}

		ok, err = s.client.SetNX(ctx, s.key, s.tok, time.Minute).Result()
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

			val, err := redisExpireScript.Run(context.Background(), s.client, []string{s.key}, s.tok, int(time.Minute/time.Second)).Result()

			select {
			case <-s.monitor:
				return
			default:
			}

			if err != nil || castToInt(val) != 1 {
				s.opts.errorHandler(s.key, err)
				select {
				case <-s.mu:
				default:
				}
				return
			}
		}
	}()

	return nil
}

func (s *RedisMutex) unlock() error {
	if s.monitor == nil {
		return errors.New("mutex is not locked")
	}

	close(s.monitor)

	val, err := redisDelScript.Run(context.Background(), s.client, []string{s.key}, s.tok).Result()
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
