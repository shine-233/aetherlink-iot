package service

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/safehttp"
	"github.com/stretchr/testify/require"
)

type aiStaticResolver map[string][]netip.Addr

func (r aiStaticResolver) LookupNetIP(_ context.Context, _ string, host string) ([]netip.Addr, error) {
	addresses, ok := r[host]
	if !ok {
		return nil, errors.New("missing DNS fixture")
	}
	return addresses, nil
}

func TestValidateAILLMBaseURLRequiresPublicHTTPS(t *testing.T) {
	previous := aiLLMResolver
	aiLLMResolver = aiStaticResolver{
		"public.test": {netip.MustParseAddr("93.184.216.34")},
		"mixed.test": {
			netip.MustParseAddr("93.184.216.34"),
			netip.MustParseAddr("169.254.169.254"),
		},
	}
	t.Cleanup(func() { aiLLMResolver = previous })

	for _, target := range []string{
		"http://public.test/v1",
		"https://localhost/v1",
		"https://169.254.169.254/latest",
		"https://mixed.test/v1",
		"https://user:pass@public.test/v1",
		"https://public.test/v1?tenant=secret",
	} {
		t.Run(target, func(t *testing.T) {
			_, err := validateAILLMBaseURL(context.Background(), target)
			require.Error(t, err)
			require.ErrorIs(t, err, errAILLMInvalidConfiguration)
		})
	}

	endpoint, err := validateAILLMBaseURL(context.Background(), "https://public.test/v1/")
	require.NoError(t, err)
	require.Equal(t, "https://public.test/v1/", endpoint.String())
}

func TestAIChatCompletionPinsAddressIgnoresProxyAndSendsCredential(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
	}))
	defer server.Close()

	resolver := aiStaticResolver{"example.com": {netip.MustParseAddr("93.184.216.34")}}
	previousResolver := aiLLMResolver
	aiLLMResolver = resolver
	t.Cleanup(func() { aiLLMResolver = previousResolver })
	endpoint, err := aiLLMChatCompletionsURL(context.Background(), "https://example.com/v1")
	require.NoError(t, err)

	serverAddress := strings.TrimPrefix(server.URL, "https://")
	var dialed string
	base := safehttp.NewWebhookClient(safehttp.WebhookClientOptions{
		Resolver: resolver,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			dialed = address
			return (&net.Dialer{}).DialContext(ctx, network, serverAddress)
		},
	}).Transport.(*http.Transport)
	base.TLSClientConfig = server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	client := &http.Client{Transport: bindAILLMTransport(endpoint, "test-key", base)}

	previousClient := newAILLMHTTPClient
	newAILLMHTTPClient = func(*url.URL, string) *http.Client { return client }
	t.Cleanup(func() { newAILLMHTTPClient = previousClient })
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")

	reply, err := aiChatCompletion(context.Background(), "https://example.com/v1", "test-key", "model", []aiLLMChatMessage{{Role: "user", Content: "hello"}}, 0, nil)
	require.NoError(t, err)
	require.Equal(t, "ok", reply)
	require.Equal(t, "93.184.216.34:443", dialed)
}

func TestAIChatCompletionRejectsRedirectWithoutCredentialForwarding(t *testing.T) {
	redirected := false
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusTemporaryRedirect,
				Header:     http.Header{"Location": []string{"https://other.test/steal"}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		}),
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			redirected = true
			require.Empty(t, req.Header.Get("Authorization"))
			return errAILLMOriginMismatch
		},
	}
	previousResolver := aiLLMResolver
	aiLLMResolver = aiStaticResolver{"provider.test": {netip.MustParseAddr("93.184.216.34")}}
	previousClient := newAILLMHTTPClient
	newAILLMHTTPClient = func(*url.URL, string) *http.Client { return client }
	t.Cleanup(func() {
		aiLLMResolver = previousResolver
		newAILLMHTTPClient = previousClient
	})

	_, err := aiChatCompletion(context.Background(), "https://provider.test/v1", "secret-key", "model", []aiLLMChatMessage{{Role: "user", Content: "hello"}}, 0, nil)
	require.Error(t, err)
	require.True(t, redirected)
	require.NotContains(t, err.Error(), "other.test")
	require.NotContains(t, err.Error(), "secret-key")
}

func TestAIChatCompletionRejectsOversizedAndSanitizesProviderErrors(t *testing.T) {
	previousResolver := aiLLMResolver
	aiLLMResolver = aiStaticResolver{"provider.test": {netip.MustParseAddr("93.184.216.34")}}
	previousClient := newAILLMHTTPClient
	t.Cleanup(func() {
		aiLLMResolver = previousResolver
		newAILLMHTTPClient = previousClient
	})

	cases := []struct {
		name       string
		response   *http.Response
		want       string
		notContain string
	}{
		{
			name: "oversized",
			response: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
				Body: io.NopCloser(strings.NewReader(strings.Repeat("x", aiModelMaxReplyChars+1)))},
			want: errAILLMResponseTooLarge.Error(),
		},
		{
			name: "provider error",
			response: &http.Response{StatusCode: http.StatusBadGateway, Header: make(http.Header),
				Body: io.NopCloser(strings.NewReader("internal provider token=secret"))},
			want:       "AI provider returned HTTP 502",
			notContain: "secret",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			newAILLMHTTPClient = func(*url.URL, string) *http.Client {
				return &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					tt.response.Request = req
					return tt.response, nil
				})}
			}
			_, err := aiChatCompletion(context.Background(), "https://provider.test/v1", "key", "model", nil, 0, nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.want)
			if tt.notContain != "" {
				require.NotContains(t, err.Error(), tt.notContain)
			}
		})
	}
}

func TestAIChatCompletionPropagatesContextAndSanitizesNetworkErrors(t *testing.T) {
	previousResolver := aiLLMResolver
	aiLLMResolver = aiStaticResolver{"provider.test": {netip.MustParseAddr("93.184.216.34")}}
	previousClient := newAILLMHTTPClient
	newAILLMHTTPClient = func(*url.URL, string) *http.Client {
		return &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			<-req.Context().Done()
			return nil, errors.New("dial tcp 10.0.0.1: leaked detail")
		})}
	}
	t.Cleanup(func() {
		aiLLMResolver = previousResolver
		newAILLMHTTPClient = previousClient
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := aiChatCompletion(ctx, "https://provider.test/v1", "key", "model", nil, 0, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), errAILLMRequestFailed.Error())
	require.NotContains(t, err.Error(), "10.0.0.1")
	var appErr *errcode.Error
	require.ErrorAs(t, err, &appErr)
}
