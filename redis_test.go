package synlock

import (
	"context"
	"errors"
	"fmt"
	"runtime"
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

	resChan := make(chan error, 1)
	go func() {
		defer close(resChan)
		if critical {
			resChan <- errors.New("invalid critical section value")
			return
		}

		if err := mu2.lock(context.Background()); err != nil {
			resChan <- fmt.Errorf("acquiring lock error: %s", err)
			return
		}
		if !critical {
			resChan <- errors.New("unexpected access to the critical section")
			return
		}
		if err := mu2.unlock(); err != nil {
			resChan <- fmt.Errorf("releasinglock error: %s", err)
			return
		}
	}()

	runtime.Gosched()
	critical = true

	time.Sleep(time.Second)

	if err := mu1.unlock(); err != nil {
		t.Fatalf("releasing lock error: %s", err)
	}

	for err := range resChan {
		t.Fatal(err.Error())
	}
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
	//nolint
	defer mu1.Unlock()

	res := make(chan error)
	go func() {
		defer close(res)

		ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
		defer cancel()

		err = mu2.LockContext(ctx)
		if err != ctx.Err() {
			res <- fmt.Errorf("unexpected aqcuiring context lock error: %s", err)
			return
		}
	}()

	select {
	case err, ok := <-res:
		if ok {
			t.Fatal(err.Error())
		}
	case <-time.After(time.Second):
		t.Fatalf("Checking blocking context timeout")
	}
}
