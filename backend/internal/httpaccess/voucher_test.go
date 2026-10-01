package httpaccess

import (
	"errors"
	"strings"
	"testing"
)

func TestParseVoucher(t *testing.T) {
	voucher, err := ParseVoucher(`{"accessToken":"  token-1  ","downlinkHost":"  https://device.example  ","auth_type":" auto ","extra":"ignored"}`)
	if err != nil {
		t.Fatalf("ParseVoucher returned error: %v", err)
	}
	if voucher.AccessToken != "token-1" {
		t.Fatalf("AccessToken = %q", voucher.AccessToken)
	}
	if voucher.DownlinkHost != "https://device.example" {
		t.Fatalf("DownlinkHost = %q", voucher.DownlinkHost)
	}
	if voucher.AuthType != "auto" {
		t.Fatalf("AuthType = %q", voucher.AuthType)
	}
}

func TestParseVoucherRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr error
	}{
		{name: "empty", raw: "", wantErr: ErrInvalidVoucher},
		{name: "whitespace", raw: "   ", wantErr: ErrInvalidVoucher},
		{name: "malformed", raw: `{`, wantErr: ErrInvalidVoucher},
		{name: "null", raw: `null`, wantErr: ErrInvalidVoucher},
		{name: "array", raw: `[]`, wantErr: ErrInvalidVoucher},
		{name: "missing token", raw: `{}`, wantErr: ErrMissingAccessToken},
		{name: "empty token", raw: `{"accessToken":"  "}`, wantErr: ErrMissingAccessToken},
		{name: "null token", raw: `{"accessToken":null}`, wantErr: ErrMissingAccessToken},
		{name: "numeric token", raw: `{"accessToken":123}`, wantErr: ErrInvalidVoucher},
		{name: "object token", raw: `{"accessToken":{}}`, wantErr: ErrInvalidVoucher},
		{name: "invalid downlink host type", raw: `{"accessToken":"secret","downlinkHost":123}`, wantErr: ErrInvalidVoucher},
		{name: "invalid auth type", raw: `{"accessToken":"secret","auth_type":true}`, wantErr: ErrInvalidVoucher},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseVoucher(test.raw)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
		})
	}
}

func TestParseVoucherErrorsDoNotLeakSecrets(t *testing.T) {
	const secret = "never-log-this-token"
	_, err := ParseVoucher(`{"accessToken":"` + secret + `","downlinkHost":123}`)
	if err == nil {
		t.Fatal("ParseVoucher accepted invalid input")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error leaked access token: %v", err)
	}
}
