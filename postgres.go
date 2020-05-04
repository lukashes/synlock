package synlock

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx"
)

var (
	ErrPostgresInvalidAddr = errors.New("invalid postgres address")
)

var DefPostgresOpts = PostgresOpts{
	Host: "127.0.0.1",
	Port: "5432",
	DB:   "postgres",
	User: "postgres",
}

type PostgresOpts struct {
	Host           string
	Port           string
	DB             string
	User           string
	Pass           string
	MaxConnections int
}

type Postgres struct {
	client *pgx.ConnPool
}

func NewPostgres(conf PostgresOpts) (*Postgres, error) {
	if conf.Host == "" || conf.Port == "" {
		return nil, ErrPostgresInvalidAddr
	}

	var auth string
	if conf.User != "" {
		auth += conf.User
		auth += ":" + conf.Pass
		auth += "@"
	}

	var (
		connString      = fmt.Sprintf("postgres://%s%s:%s/%s", auth, conf.Host, conf.Port, conf.DB)
		connConfig, err = pgx.ParseConnectionString(connString)
	)
	if err != nil {
		return nil, err
	}

	var connPoolConfig = pgx.ConnPoolConfig{
		ConnConfig:     connConfig,
		MaxConnections: conf.MaxConnections,
	}

	conn, err := pgx.NewConnPool(connPoolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %v", err)
	}

	return &Postgres{
		client: conn,
	}, nil
}

func (r *Postgres) NewMutex(key int64) (Mutex, error) {
	return &PostgresMutex{
		client: r.client,
		key:    key,
	}, nil
}

type PostgresMutex struct {
	client  *pgx.ConnPool
	key     int64
	monitor chan struct{}
	mu      sync.Mutex
	tok     string
	tx      *pgx.Tx
}

func (s *PostgresMutex) Lock() error {
	s.mu.Lock()
	return s.lock()
}

func (s *PostgresMutex) Unlock() error {
	defer s.mu.Unlock()
	return s.unlock()
}

func (s *PostgresMutex) lock() error {
	var (
		err    error
		ok     bool
		jitter time.Duration
	)

	for {
		if jitter > 0 {
			time.Sleep(jitter)
		}

		if s.tx, err = s.client.Begin(); err != nil {
			return err
		}

		err = s.tx.QueryRow("SELECT pg_try_advisory_xact_lock($1)", s.key).Scan(&ok)
		if err != nil {
			return err
		}

		if ok {
			return nil
		}

		if err = s.tx.Rollback(); err != nil {
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
}

func (s *PostgresMutex) unlock() error {
	return s.tx.Rollback()
}
