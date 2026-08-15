package service

import (
	"errors"
	"net/http"
	"sync"
	"time"
)

const (
	marketCircuitFailureThreshold = 5
	marketCircuitOpenDuration     = 30 * time.Second
)

// ErrMarketCircuitOpen indicates that recent Market failures caused outbound
// calls to fail fast while the optional external service recovers.
var ErrMarketCircuitOpen = errors.New("market circuit breaker open")

type marketCircuitBreakerTransport struct {
	next             http.RoundTripper
	failureThreshold int
	openDuration     time.Duration
	now              func() time.Time

	mu                  sync.Mutex
	consecutiveFailures int
	openUntil           time.Time
	halfOpenProbe       bool
}

func newMarketCircuitBreakerTransport(next http.RoundTripper) *marketCircuitBreakerTransport {
	if next == nil {
		next = http.DefaultTransport
	}
	return &marketCircuitBreakerTransport{
		next:             next,
		failureThreshold: marketCircuitFailureThreshold,
		openDuration:     marketCircuitOpenDuration,
		now:              time.Now,
	}
}

func (t *marketCircuitBreakerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !t.allowRequest() {
		return nil, ErrMarketCircuitOpen
	}

	resp, err := t.next.RoundTrip(req)
	failed := err != nil || (resp != nil && (resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError))
	t.recordResult(failed)
	return resp, err
}

func (t *marketCircuitBreakerTransport) allowRequest() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.openUntil.IsZero() {
		return true
	}
	if t.now().Before(t.openUntil) || t.halfOpenProbe {
		return false
	}

	// After the cool-down, permit one half-open probe and fail concurrent calls fast.
	t.halfOpenProbe = true
	return true
}

func (t *marketCircuitBreakerTransport) recordResult(failed bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.halfOpenProbe {
		t.halfOpenProbe = false
		if failed {
			t.openUntil = t.now().Add(t.openDuration)
			return
		}
		t.openUntil = time.Time{}
		t.consecutiveFailures = 0
		return
	}

	if !t.openUntil.IsZero() {
		return
	}
	if !failed {
		t.consecutiveFailures = 0
		return
	}

	t.consecutiveFailures++
	if t.consecutiveFailures >= t.failureThreshold {
		t.openUntil = t.now().Add(t.openDuration)
	}
}
