package middleware

import (
	"errors"
	"testing"
	"time"
)

func TestCircuitBreakerOpensAfterConsecutiveFailures(t *testing.T) {
	cb := NewCircuitBreaker()
	failure := errors.New("upstream failed")

	for i := 0; i < 5; i++ {
		if err := cb.Execute(func() error { return failure }); !errors.Is(err, failure) {
			t.Fatalf("failure %d returned %v, want upstream failure", i+1, err)
		}
	}

	if err := cb.Execute(func() error { return nil }); !errors.Is(err, ErrOpenState) {
		t.Fatalf("expected open-state error, got %v", err)
	}
}

func TestCircuitBreakerClosesAfterHalfOpenSuccesses(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.timeout = time.Millisecond
	failure := errors.New("upstream failed")

	for i := 0; i < 5; i++ {
		_ = cb.Execute(func() error { return failure })
	}
	time.Sleep(2 * time.Millisecond)

	for i := 0; i < 3; i++ {
		if err := cb.Execute(func() error { return nil }); err != nil {
			t.Fatalf("half-open success %d returned %v", i+1, err)
		}
	}
	if cb.state != StateClosed {
		t.Fatalf("state = %v, want StateClosed", cb.state)
	}
}
