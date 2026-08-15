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
