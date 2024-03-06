package synlock

import (
	"errors"
	"fmt"
	"runtime"
	"testing"
	"time"
)

func TestPostgresMutex(t *testing.T) {
	p, err := NewPostgres(DefPostgresOpts)
	if err != nil {
		t.Fatalf("init postgres object error: %s", err)
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

	resChan := make(chan error, 1)
	go func() {
		defer close(resChan)
		if critical {
			resChan <- errors.New("invalid critical section value")
			return
		}

		if err := mu2.lock(); err != nil {
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

	for m := range resChan {
		t.Fatal(m.Error())
	}
}
