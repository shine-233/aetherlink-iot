// Package safehttp provides fail-closed HTTP delivery for tenant-controlled webhook URLs.
package safehttp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

var ErrUnsafeWebhookURL = errors.New("unsafe webhook URL")

type Resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

type WebhookClientOptions struct {
	Resolver    Resolver
	DialContext DialContextFunc
}

var blockedWebhookPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("64:ff9b:1::/48"),
	netip.MustParsePrefix("100::/64"),
	netip.MustParsePrefix("2001:2::/48"),
	netip.MustParsePrefix("2001:db8::/32"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
}

func ValidateWebhookURL(ctx context.Context, rawURL string, resolver Resolver) (*url.URL, []netip.Addr, error) {
	endpoint, err := ParseWebhookURL(rawURL)
	if err != nil {
		return nil, nil, err
	}
	addresses, err := resolvePublicWebhookAddresses(ctx, endpoint.Hostname(), resolver)
	if err != nil {
		return nil, nil, err
	}
	return endpoint, addresses, nil
}

func ParseWebhookURL(rawURL string) (*url.URL, error) {
	endpoint, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" || endpoint.Opaque != "" {
		return nil, fmt.Errorf("%w: endpoint must be an absolute URL", ErrUnsafeWebhookURL)
	}
	endpoint.Scheme = strings.ToLower(endpoint.Scheme)
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, fmt.Errorf("%w: only HTTP(S) is allowed", ErrUnsafeWebhookURL)
	}
	if endpoint.User != nil {
		return nil, fmt.Errorf("%w: URL credentials are not allowed", ErrUnsafeWebhookURL)
	}
	if endpoint.Fragment != "" {
		return nil, fmt.Errorf("%w: URL fragments are not allowed", ErrUnsafeWebhookURL)
	}
	host := strings.TrimSuffix(strings.TrimSpace(endpoint.Hostname()), ".")
	if host == "" || strings.EqualFold(host, "localhost") {
		return nil, fmt.Errorf("%w: host is not public", ErrUnsafeWebhookURL)
	}
	if literal, err := netip.ParseAddr(host); err == nil && isBlockedWebhookAddress(literal.Unmap()) {
		return nil, fmt.Errorf("%w: host is not public", ErrUnsafeWebhookURL)
	}
	return endpoint, nil
}

func NewWebhookClient(options WebhookClientOptions) *http.Client {
	resolver := options.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	dial := options.DialContext
	if dial == nil {
		dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
		dial = dialer.DialContext
	}

	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, fmt.Errorf("%w: invalid dial address", ErrUnsafeWebhookURL)
			}
			addresses, err := resolvePublicWebhookAddresses(ctx, host, resolver)
			if err != nil {
				return nil, err
			}
			var lastErr error
			for _, resolvedAddress := range addresses {
				connection, dialErr := dial(ctx, network, net.JoinHostPort(resolvedAddress.String(), port))
				if dialErr == nil {
					return connection, nil
				}
				lastErr = dialErr
			}
			return nil, lastErr
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          20,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 5 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("%w: too many redirects", ErrUnsafeWebhookURL)
			}
			if _, _, err := ValidateWebhookURL(req.Context(), req.URL.String(), resolver); err != nil {
				return err
			}
			return fmt.Errorf("%w: redirects are not allowed", ErrUnsafeWebhookURL)
		},
	}
}

func PostWebhookJSON(ctx context.Context, rawURL string, body []byte) (*http.Response, error) {
	endpoint, _, err := ValidateWebhookURL(ctx, rawURL, net.DefaultResolver)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return NewWebhookClient(WebhookClientOptions{}).Do(req)
}

func resolvePublicWebhookAddresses(ctx context.Context, host string, resolver Resolver) ([]netip.Addr, error) {
	normalizedHost := strings.TrimSuffix(strings.TrimSpace(host), ".")
	if normalizedHost == "" || strings.EqualFold(normalizedHost, "localhost") {
		return nil, fmt.Errorf("%w: host is not public", ErrUnsafeWebhookURL)
	}
	if resolver == nil {
		resolver = net.DefaultResolver
	}

	addresses := make([]netip.Addr, 0, 2)
	if literal, err := netip.ParseAddr(normalizedHost); err == nil {
		addresses = append(addresses, literal)
	} else {
		resolved, err := resolver.LookupNetIP(ctx, "ip", normalizedHost)
		if err != nil {
			return nil, fmt.Errorf("%w: host resolution failed", ErrUnsafeWebhookURL)
		}
		addresses = append(addresses, resolved...)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("%w: host resolved to no addresses", ErrUnsafeWebhookURL)
	}
	for i := range addresses {
		addresses[i] = addresses[i].Unmap()
		if addresses[i].Is6() {
			addresses[i] = addresses[i].WithZone("")
		}
		if isBlockedWebhookAddress(addresses[i]) {
			return nil, fmt.Errorf("%w: host resolved to a non-public address", ErrUnsafeWebhookURL)
		}
	}
	return addresses, nil
}

func isBlockedWebhookAddress(address netip.Addr) bool {
	if !address.IsValid() || address.IsUnspecified() || address.IsLoopback() || address.IsPrivate() ||
		address.IsLinkLocalUnicast() || address.IsLinkLocalMulticast() || address.IsMulticast() {
		return true
	}
	for _, prefix := range blockedWebhookPrefixes {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func DrainAndClose(response *http.Response) {
	if response == nil || response.Body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 32*1024))
	_ = response.Body.Close()
}
