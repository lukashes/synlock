package synlock

import "testing"

func TestGoodNew(t *testing.T) {
	type suite struct {
		desc string
		conn string
	}

	var suites = []suite{{
		desc: "good redis connection with database",
		conn: "redis://127.0.0.1:6379/0",
	}, {
		desc: "good redis connection default database",
		conn: "redis://127.0.0.1",
	}, {
		desc: "good host connection",
		conn: "redis://localhost",
	},}

	for _, s := range suites {
		t.Run(s.desc, func(t *testing.T) {
			var sl, err = New(s.conn)
			if err != nil {
				t.Fatal("unexpected error", err)
			}

			if sl == nil {
				t.Fatal("empty constructor result")
			}
		})
	}
}

func TestBrokenNew(t *testing.T) {
	type suite struct {
		desc string
		conn string
	}

	var suites = []suite{{
		desc: "invalid scheme",
		conn: "invalid://localhost/0",
	}, {
		desc: "invalid redis database",
		conn: "redis://localhost/invalid",
	}}

	for _, s := range suites {
		t.Run(s.desc, func(t *testing.T) {
			var _, err = New(s.conn)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
