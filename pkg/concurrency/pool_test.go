package concurrency

import (
	"sync"
	"testing"
	"time"
)

func TestWorkerPoolExecutesSubmittedTasks(t *testing.T) {
	pool := NewWorkerPool(2, 4)
	defer pool.Stop()

	counter := &SafeCounter{}
	done := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		ok := pool.Submit(func() {
			defer wg.Done()
			counter.Inc()
		})
		if !ok {
			t.Fatal("Submit returned false before queue was full")
		}
	}
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tasks did not complete in time")
	}
	if got := counter.Get(); got != 4 {
		t.Fatalf("counter = %d, want 4", got)
	}
}

func TestSafeMapSetGetDelete(t *testing.T) {
	m := NewSafeMap()
	m.Set("token", int64(42))
	got, ok := m.Get("token")
	if !ok || got.(int64) != 42 {
		t.Fatalf("Get(token) = %v, %v; want 42, true", got, ok)
	}

	m.Delete("token")
	if _, ok := m.Get("token"); ok {
		t.Fatal("expected token to be deleted")
	}
}
