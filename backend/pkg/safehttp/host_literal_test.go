package safehttp

import "testing"

func TestIsBlockedHostLiteral(t *testing.T) {
	blocked := []string{
		"", "localhost", "LOCALHOST.", "127.0.0.1", "10.1.2.3", "172.16.0.9", "192.168.1.1",
		"169.254.169.254", "100.64.0.1", "0.0.0.0", "::1", "fe80::1", "fd00::1", "::ffff:127.0.0.1",
	}
	for _, host := range blocked {
		if !IsBlockedHostLiteral(host) {
			t.Errorf("IsBlockedHostLiteral(%q) = false, want true", host)
		}
	}
	allowed := []string{"93.184.216.34", "2606:4700::1111", "hooks.example.com", "oapi.dingtalk.com"}
	for _, host := range allowed {
		if IsBlockedHostLiteral(host) {
			t.Errorf("IsBlockedHostLiteral(%q) = true, want false", host)
		}
	}
}
