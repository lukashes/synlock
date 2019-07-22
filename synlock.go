package synlock

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type (
	Mutex interface {
		Lock() error
		Unlock() error
	}

	Synlock interface {
		NewMutex(key int64) (Mutex, error)
	}
)

func New(conn string) (Synlock, error) {
	var u, err = url.Parse(conn)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "redis":
		var opts = DefRedisOpts

		if u.Path != "" {
			opts.DB, err = strconv.Atoi(strings.TrimPrefix(u.Path, "/"))
			if err != nil {
				return nil, err
			}
		}

		opts.Host = u.Host
		if p := u.Port(); p != "" {
			opts.Port = u.Port()
		}

		return NewRedis(opts)
	default:
		return nil, fmt.Errorf("unexpected scheme: %s", u.Scheme)
	}
}
