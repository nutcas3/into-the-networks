package esl

import (
	"testing"
	"time"
)

func TestNewCircuitBreaker(t *testing.T) {
	cb := NewCircuitBreaker(5, 30*time.Second)

	if cb.threshold != 5 {
		t.Errorf("Expected threshold 5, got %d", cb.threshold)
	}

	if cb.timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", cb.timeout)
	}

	if cb.state != StateClosed {
		t.Errorf("Expected initial state StateClosed, got %v", cb.state)
	}
}

func TestCircuitBreakerInitialState(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)

	if cb.IsOpen() {
		t.Error("Circuit should be closed initially")
	}

	if cb.GetState() != StateClosed {
		t.Errorf("Expected StateClosed, got %v", cb.GetState())
	}

	if cb.GetFailures() != 0 {
		t.Errorf("Expected 0 failures, got %d", cb.GetFailures())
	}
}

func TestCircuitBreakerSuccess(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)

	cb.RecordSuccess()

	if cb.IsOpen() {
		t.Error("Circuit should remain closed after success")
	}

	if cb.GetFailures() != 0 {
		t.Errorf("Expected failures to reset to 0, got %d", cb.GetFailures())
	}
}

func TestCircuitBreakerFailureBelowThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 1*time.Second)

	cb.RecordFailure()

	if cb.IsOpen() {
		t.Error("Circuit should remain closed below threshold")
	}

	if cb.GetFailures() != 1 {
		t.Errorf("Expected 1 failure, got %d", cb.GetFailures())
	}
}

func TestCircuitBreakerFailureAtThreshold(t *testing.T) {
	cb := NewCircuitBreaker(2, 100*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()

	if !cb.IsOpen() {
		t.Error("Circuit should open at threshold")
	}

	if cb.GetState() != StateOpen {
		t.Errorf("Expected StateOpen, got %v", cb.GetState())
	}
}

func TestCircuitBreakerTimeout(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure()

	if !cb.IsOpen() {
		t.Error("Circuit should open immediately")
	}

	time.Sleep(60 * time.Millisecond)

	if cb.IsOpen() {
		t.Error("Circuit should be half-open after timeout")
	}

	if cb.GetState() != StateHalfOpen {
		t.Errorf("Expected StateHalfOpen, got %v", cb.GetState())
	}
}

func TestCircuitBreakerHalfOpenToClosed(t *testing.T) {
	cb := NewCircuitBreaker(1, 50*time.Millisecond)

	cb.RecordFailure()
	time.Sleep(60 * time.Millisecond)

	cb.RecordSuccess()

	if cb.IsOpen() {
		t.Error("Circuit should close after success in half-open state")
	}

	if cb.GetState() != StateClosed {
		t.Errorf("Expected StateClosed, got %v", cb.GetState())
	}

	if cb.GetFailures() != 0 {
		t.Errorf("Expected failures to reset, got %d", cb.GetFailures())
	}
}

func TestCircuitBreakerConcurrency(t *testing.T) {
	cb := NewCircuitBreaker(10, 100*time.Millisecond)

	done := make(chan bool, 20)

	for range 10 {
		go func() {
			cb.RecordFailure()
			done <- true
		}()
	}

	for range 10 {
		go func() {
			cb.RecordSuccess()
			done <- true
		}()
	}

	for range 20 {
		<-done
	}

	state := cb.GetState()
	if state != StateClosed && state != StateOpen && state != StateHalfOpen {
		t.Errorf("Unexpected state: %v", state)
	}
}
