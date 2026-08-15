package service

import (
	"errors"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
)

func TestResolveEmailProviderConfigRequiresExplicitOpenStatus(t *testing.T) {
	configJSON := `{"host":"smtp.example.com","port":465,"from_email":"sender@example.com","from_password":"secret","ssl":true}`

	tests := []struct {
		name       string
		status     string
		configJSON string
		wantErr    string
	}{
		{name: "open provider is allowed", status: "OPEN", configJSON: configJSON},
		{name: "closed provider is rejected before parsing", status: "CLOSE", configJSON: `{invalid`, wantErr: "is disabled"},
		{name: "unknown provider status fails closed", status: "PAUSED", configJSON: configJSON, wantErr: "is not enabled"},
		{name: "empty provider status fails closed", status: "", configJSON: configJSON, wantErr: "is not enabled"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.configJSON
			got, err := resolveEmailProviderConfig(&model.NotificationServicesConfig{
				Status: tt.status,
				Config: &config,
			})

			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("resolveEmailProviderConfig error = %v, want containing %q", err, tt.wantErr)
				}
				if !errors.Is(err, ErrEmailProviderUnavailable) {
					t.Fatalf("resolveEmailProviderConfig error = %v, want ErrEmailProviderUnavailable", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolveEmailProviderConfig returned unexpected error: %v", err)
			}
			if got.Host != "smtp.example.com" || got.Port != 465 || got.FromEmail != "sender@example.com" {
				t.Fatalf("resolveEmailProviderConfig changed provider fields: %#v", got)
			}
		})
	}
}

func TestNewEmailProviderDialerUsesSavedSSLValue(t *testing.T) {
	falseValue := false
	trueValue := true

	tests := []struct {
		name    string
		ssl     *bool
		wantSSL bool
	}{
		{name: "missing SSL is false", ssl: nil, wantSSL: false},
		{name: "saved false stays false", ssl: &falseValue, wantSSL: false},
		{name: "saved true enables SSL", ssl: &trueValue, wantSSL: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialer := newEmailProviderDialer(model.EmailConfig{
				Host:         "smtp.example.com",
				Port:         465,
				FromEmail:    "sender@example.com",
				FromPassword: "secret",
				SSL:          tt.ssl,
			})

			if dialer.SSL != tt.wantSSL {
				t.Fatalf("dialer.SSL = %v, want %v", dialer.SSL, tt.wantSSL)
			}
			if dialer.Host != "smtp.example.com" || dialer.Port != 465 || dialer.Username != "sender@example.com" || dialer.Password != "secret" {
				t.Fatalf("newEmailProviderDialer changed connection fields: %#v", dialer)
			}
		})
	}
}
