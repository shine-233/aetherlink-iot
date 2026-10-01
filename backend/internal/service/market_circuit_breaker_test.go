package service

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type marketRoundTripFunc func(*http.Request) (*http.Response, error)

func (f marketRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func marketTestResponse(status int) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}
}

func TestMarketCircuitBreakerOpensAndRecoversThroughSingleProbe(t *testing.T) {
	var mu sync.Mutex
	calls := 0
	status := http.StatusServiceUnavailable
	now := time.Unix(1_700_000_000, 0)

	breaker := newMarketCircuitBreakerTransport(marketRoundTripFunc(func(*http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		return marketTestResponse(status), nil
	}))
	breaker.failureThreshold = 2
	breaker.openDuration = time.Minute
	breaker.now = func() time.Time { return now }

	req, err := http.NewRequest(http.MethodGet, "https://market.test/templates", nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		resp, roundTripErr := breaker.RoundTrip(req)
		if roundTripErr != nil {
			t.Fatalf("failure %d unexpectedly returned transport error: %v", i+1, roundTripErr)
		}
		_ = resp.Body.Close()
	}

	if _, err := breaker.RoundTrip(req); !errors.Is(err, ErrMarketCircuitOpen) {
		t.Fatalf("open breaker error = %v, want ErrMarketCircuitOpen", err)
	}
	if calls != 2 {
		t.Fatalf("underlying calls = %d, want 2 while open", calls)
	}

	now = now.Add(time.Minute)
	status = http.StatusOK
	resp, err := breaker.RoundTrip(req)
	if err != nil {
		t.Fatalf("half-open probe error = %v", err)
	}
	_ = resp.Body.Close()

	resp, err = breaker.RoundTrip(req)
	if err != nil {
		t.Fatalf("request after successful probe error = %v", err)
	}
	_ = resp.Body.Close()
	if calls != 4 {
		t.Fatalf("underlying calls = %d, want 4 after recovery", calls)
	}
}

func TestMarketCircuitBreakerCountsOnlyAvailabilityFailures(t *testing.T) {
	statuses := []int{
		http.StatusServiceUnavailable,
		http.StatusBadRequest,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
	}
	calls := 0
	breaker := newMarketCircuitBreakerTransport(marketRoundTripFunc(func(*http.Request) (*http.Response, error) {
		status := statuses[calls]
		calls++
		return marketTestResponse(status), nil
	}))
	breaker.failureThreshold = 2

	req, err := http.NewRequest(http.MethodGet, "https://market.test/templates", nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(statuses); i++ {
		resp, roundTripErr := breaker.RoundTrip(req)
		if roundTripErr != nil {
			t.Fatalf("status %d unexpectedly short-circuited: %v", statuses[i], roundTripErr)
		}
		_ = resp.Body.Close()
	}

	if _, err := breaker.RoundTrip(req); !errors.Is(err, ErrMarketCircuitOpen) {
		t.Fatalf("error after consecutive 429/500 = %v, want ErrMarketCircuitOpen", err)
	}
}

func TestNewMarketClientUsesBoundedCircuitBreakingHTTPClient(t *testing.T) {
	client := NewMarketClient()
	if client.httpClient.Timeout != 10*time.Second {
		t.Fatalf("HTTP timeout = %v, want 10s", client.httpClient.Timeout)
	}
	if _, ok := client.httpClient.Transport.(*marketCircuitBreakerTransport); !ok {
		t.Fatalf("HTTP transport = %T, want *marketCircuitBreakerTransport", client.httpClient.Transport)
	}
}

// TestNewMarketClientSharesCircuitBreakerAcrossInstances guards the actual
// bug: NewMarketClient() used to wrap a brand-new breaker on every call, so
// consecutiveFailures reset to 0 each time and the breaker could never
// trip. Every client returned by NewMarketClient() must share one
// process-wide breaker instance so failures accumulate across requests and
// across client instances (e.g. the 3 NewMarketClient() calls per request
// flow in device_templates.go).
func TestNewMarketClientSharesCircuitBreakerAcrossInstances(t *testing.T) {
	clientA := NewMarketClient()
	clientB := NewMarketClient()

	transportA, ok := clientA.httpClient.Transport.(*marketCircuitBreakerTransport)
	if !ok {
		t.Fatalf("clientA transport = %T, want *marketCircuitBreakerTransport", clientA.httpClient.Transport)
	}
	transportB, ok := clientB.httpClient.Transport.(*marketCircuitBreakerTransport)
	if !ok {
		t.Fatalf("clientB transport = %T, want *marketCircuitBreakerTransport", clientB.httpClient.Transport)
	}
	if transportA != transportB {
		t.Fatalf("NewMarketClient() returned distinct breaker transports (%p vs %p), want the shared singleton", transportA, transportB)
	}
	if transportA != sharedMarketCircuitBreakerTransport() {
		t.Fatalf("client transport is not the process-wide shared breaker returned by sharedMarketCircuitBreakerTransport()")
	}

	// Swap in a deterministic failing upstream so this test never touches
	// the real network, then drive the shared breaker toward its failure
	// threshold using clientA's transport, and confirm clientB (a fresh
	// NewMarketClient() call simulating a new request) observes the same
	// accumulated state instead of starting over at 0 failures.
	t.Cleanup(func() {
		transportA.mu.Lock()
		transportA.next = http.DefaultTransport
		transportA.consecutiveFailures = 0
		transportA.openUntil = time.Time{}
		transportA.halfOpenProbe = false
		transportA.mu.Unlock()
	})
	transportA.mu.Lock()
	transportA.next = marketRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return marketTestResponse(http.StatusServiceUnavailable), nil
	})
	transportA.consecutiveFailures = transportA.failureThreshold - 1
	transportA.mu.Unlock()

	req, err := http.NewRequest(http.MethodGet, "https://market.test/templates", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, roundTripErr := transportB.RoundTrip(req)
	if roundTripErr != nil {
		t.Fatalf("RoundTrip() unexpected error = %v, want the final pre-threshold failure to pass through", roundTripErr)
	}
	_ = resp.Body.Close()

	transportB.mu.Lock()
	openUntilAfter := transportB.openUntil
	transportB.mu.Unlock()

	if openUntilAfter.IsZero() {
		t.Fatalf("breaker did not open after clientB's call pushed the shared failure count past threshold — breaker state is not shared across NewMarketClient() instances")
	}
}
