package safehttp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type staticResolver map[string][]netip.Addr

func (r staticResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	addresses, ok := r[host]
	if !ok {
		return nil, fmt.Errorf("no fixture for %s", host)
	}
	return addresses, nil
}

func TestValidateWebhookURLRejectsUnsafeTargets(t *testing.T) {
	resolver := staticResolver{
		"private.test": {netip.MustParseAddr("10.0.0.5")},
		"mixed.test":   {netip.MustParseAddr("93.184.216.34"), netip.MustParseAddr("169.254.169.254")},
	}
	tests := []string{
		"",
		"/relative",
		"ftp://example.com/hook",
		"http://user:password@example.com/hook",
		"https://example.com/hook#fragment",
		"http://localhost/hook",
		"http://127.0.0.1/hook",
		"http://[::1]/hook",
		"http://169.254.169.254/latest/meta-data",
		"http://100.64.0.1/hook",
		"http://private.test/hook",
		"http://mixed.test/hook",
	}
	for _, target := range tests {
		t.Run(target, func(t *testing.T) {
			_, _, err := ValidateWebhookURL(context.Background(), target, resolver)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrUnsafeWebhookURL)
		})
	}
}

func TestValidateWebhookURLAcceptsOnlyPublicResolution(t *testing.T) {
	resolver := staticResolver{"hooks.example.com": {netip.MustParseAddr("93.184.216.34")}}
	endpoint, addresses, err := ValidateWebhookURL(context.Background(), "https://hooks.example.com/v1", resolver)
	require.NoError(t, err)
	require.Equal(t, "https://hooks.example.com/v1", endpoint.String())
	require.Equal(t, []netip.Addr{netip.MustParseAddr("93.184.216.34")}, addresses)
}

func TestWebhookClientPinsValidatedAddressAtDialTime(t *testing.T) {
	resolver := staticResolver{"hooks.example.com": {netip.MustParseAddr("93.184.216.34")}}
	var dialed string
	client := NewWebhookClient(WebhookClientOptions{
		Resolver: resolver,
		DialContext: func(_ context.Context, _, address string) (net.Conn, error) {
			dialed = address
			return nil, errors.New("stop after observing address")
		},
	})
	req, err := http.NewRequest(http.MethodPost, "http://hooks.example.com:8080/hook", strings.NewReader(`{}`))
	require.NoError(t, err)
	_, err = client.Do(req)
	require.Error(t, err)
	require.Equal(t, "93.184.216.34:8080", dialed)
}

func TestWebhookClientRejectsPrivateRedirectBeforeFollowing(t *testing.T) {
	privateHits := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "http://metadata.test/latest", http.StatusTemporaryRedirect)
			return
		}
		privateHits++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	serverAddress := strings.TrimPrefix(server.URL, "http://")

	resolver := staticResolver{
		"public.test":   {netip.MustParseAddr("93.184.216.34")},
		"metadata.test": {netip.MustParseAddr("169.254.169.254")},
	}
	client := NewWebhookClient(WebhookClientOptions{
		Resolver: resolver,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, serverAddress)
		},
	})
	req, err := http.NewRequest(http.MethodPost, "http://public.test/start", strings.NewReader(`{}`))
	require.NoError(t, err)
	_, err = client.Do(req)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrUnsafeWebhookURL)
	require.Zero(t, privateHits)
}
