package middleware

import (
	"errors"
	"sync"
	"time"
)

type CircuitBreaker struct {
	maxRequests  uint32
	interval     time.Duration
	timeout      time.Duration
	state        State
	counts       Counts
	expiry       time.Time
	mu           sync.Mutex
}

type State int

const (
	StateClosed State = iota
	StateHalfOpen
	StateOpen
)

type Counts struct {
	Requests             uint32
	TotalSuccesses       uint32
	TotalFailures        uint32
	ConsecutiveSuccesses uint32
	ConsecutiveFailures  uint32
}

var ErrOpenState = errors.New("circuit breaker is open")

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		maxRequests: 10,
		interval:    time.Minute,
		timeout:     time.Second * 30,
		state:       StateClosed,
	}
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
	cb.mu.Lock()
	
	if cb.state == StateOpen {
		if time.Now().After(cb.expiry) {
			cb.state = StateHalfOpen
			cb.counts = Counts{}
		} else {
			cb.mu.Unlock()
			return ErrOpenState
		}
	}
	
	cb.counts.Requests++
	cb.mu.Unlock()
	
	err := fn()
	
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	if err != nil {
		cb.counts.TotalFailures++
		cb.counts.ConsecutiveFailures++
		cb.counts.ConsecutiveSuccesses = 0
		
		if cb.counts.ConsecutiveFailures >= 5 {
			cb.state = StateOpen
			cb.expiry = time.Now().Add(cb.timeout)
		}
	} else {
		cb.counts.TotalSuccesses++
		cb.counts.ConsecutiveSuccesses++
		cb.counts.ConsecutiveFailures = 0
		
		if cb.state == StateHalfOpen && cb.counts.ConsecutiveSuccesses >= 3 {
			cb.state = StateClosed
			cb.counts = Counts{}
		}
	}
	
	return err
}
