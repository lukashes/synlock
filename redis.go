package synlock

import (
	"fmt"
	"github.com/pkg/errors"
	"net"
	"time"

	"github.com/go-redis/redis"
)

var DefRedisOpts = RedisOpts{
	Host: "127.0.0.1",
	Port: "6379",
	DB: 0,
	Prefix: "synlock",
}

type RedisOpts struct {
	Host string
	Port string
	DB   int
	Prefix string
}

type Redis struct {
	client *redis.Client
	prefix string
}

func NewRedis(conf RedisOpts) (*Redis, error) {
	if conf.Host == "" || conf.Port == "" {
		return nil, errors.New("invalid redis addr")
	}

	if conf.Prefix == "" {
		return nil, errors.New("invalid redis key prefix")
	}

	return &Redis{
		client:redis.NewClient(&redis.Options{
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
	client *redis.Client
	key    string
}

func (s *RedisMutex) Lock() error {
	var (
		ok     bool
		err    error
		jitter time.Duration
	)
	for !ok {
		if jitter > 0 {
			time.Sleep(jitter)
		}

		ok, err = s.client.SetNX(s.key, true, time.Minute).Result()
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

	return nil
}

func (s *RedisMutex) Unlock() error {
	return s.client.Del(s.key).Err()
}
