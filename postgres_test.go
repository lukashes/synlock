package synlock

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestPostgresMutex(t *testing.T) {
	p, err := NewPostgres(DefPostgresOpts)
	if err != nil {
		t.Fatalf(	"init postgres object error: %s", err)
	}

	m1, err := p.NewMutex(123)
	if err != nil {
		t.Fatalf("making new mutex error: %s", err)
	}
	mu1 := m1.(*PostgresMutex)

	m2, err := p.NewMutex(123)
	if err != nil {
		t.Fatalf("making new mutex error: %s", err)
	}
	mu2 := m2.(*PostgresMutex)

	var critical = false
	if err = mu1.lock(); err != nil {
		t.Fatalf("acquiring lock error: %s", err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if critical {
			t.Fatalf("invalid critical section value")
		}

		if err := mu2.lock(); err != nil {
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
