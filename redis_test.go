package synlock

import (
	"context"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestRedisMutex(t *testing.T) {
	r, err := NewRedis(DefRedisOpts)
	if err != nil {
		t.Fatalf("init redis object error: %s", err)
	}

	m1, err := r.NewMutex(123)
	if err != nil {
		t.Fatalf("making new mutex error: %s", err)
	}
	mu1 := MustRedisMutex(m1)

	m2, err := r.NewMutex(123)
	if err != nil {
		t.Fatalf("making new mutex error: %s", err)
	}
	mu2 := MustRedisMutex(m2)

	var critical = false
	if err = mu1.lock(context.Background()); err != nil {
		t.Fatalf("acquiring lock error: %s", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if critical {
			t.Fatalf("invalid critical section value")
		}

		if err := mu2.lock(context.Background()); err != nil {
			t.Fatalf("acquiring lock error: %s", err)
		}
		if !critical {
			t.Fatalf("unexpected access to the critical section")
		}
		if err := mu2.unlock(); err != nil {
			t.Fatalf("releasinglock error: %s", err)
		}
	}()

	runtime.Gosched()
	critical = true

	time.Sleep(time.Second)

	if err := mu1.unlock(); err != nil {
		t.Fatalf("releasing lock error: %s", err)
	}

	wg.Wait()
}

func TestRedisMutexContextCancellation(t *testing.T) {
	r, err := NewRedis(DefRedisOpts)
	if err != nil {
		t.Fatalf("init redis object error: %s", err)
	}

	m1, err := r.NewMutex(123)
	if err != nil {
		t.Fatalf("making new mutex error: %s", err)
	}
	mu1 := MustRedisMutex(m1)

	m2, err := r.NewMutex(123)
	if err != nil {
		t.Fatalf("making new mutex error: %s", err)
	}
	mu2 := MustRedisMutex(m2)

	err = mu1.LockContext(context.Background())
	if err != nil {
		t.Fatalf("acquiring lock error: %s", err)
	}
	defer mu1.Unlock()

	wait := make(chan struct{})
	go func() {
		defer close(wait)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
		defer cancel()

		err = mu2.LockContext(ctx)
		if err != ctx.Err() {
			t.Fatalf("unexpected aqcuiring context lock error: %s", err)
		}
	}()

	select {
	case <-wait:
	case <-time.After(time.Second):
		t.Fatalf("Checking blocking context timeout")
	}
}
