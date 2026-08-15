// Package httpaccess contains the protocol-independent core for the built-in
// HTTP device access path.
package httpaccess

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"strings"
)

var (
	ErrInvalidVoucher     = errors.New("invalid HTTP access voucher")
	ErrMissingAccessToken = errors.New("HTTP access voucher requires an access token")
)

// Voucher is the credential and optional downlink configuration stored in
// service_access.voucher for the built-in HTTP service.
type Voucher struct {
	AccessToken  string `json:"accessToken"`
	DownlinkHost string `json:"downlinkHost,omitempty"`
	AuthType     string `json:"auth_type,omitempty"`
}

// ParseVoucher validates the persisted HTTP voucher without including its
// sensitive contents in returned errors.
func ParseVoucher(raw string) (Voucher, error) {
	if strings.TrimSpace(raw) == "" {
		return Voucher{}, ErrInvalidVoucher
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &fields); err != nil || fields == nil {
		return Voucher{}, ErrInvalidVoucher
	}

	var voucher Voucher
	if err := decodeOptionalString(fields, "accessToken", &voucher.AccessToken); err != nil {
		return Voucher{}, ErrInvalidVoucher
	}
	if strings.TrimSpace(voucher.AccessToken) == "" {
		return Voucher{}, ErrMissingAccessToken
	}
	voucher.AccessToken = strings.TrimSpace(voucher.AccessToken)

	if err := decodeOptionalString(fields, "downlinkHost", &voucher.DownlinkHost); err != nil {
		return Voucher{}, ErrInvalidVoucher
	}
	voucher.DownlinkHost = strings.TrimSpace(voucher.DownlinkHost)

	if err := decodeOptionalString(fields, "auth_type", &voucher.AuthType); err != nil {
		return Voucher{}, ErrInvalidVoucher
	}
	voucher.AuthType = strings.TrimSpace(voucher.AuthType)
	return voucher, nil
}

func decodeOptionalString(fields map[string]json.RawMessage, key string, target *string) error {
	raw, exists := fields[key]
	if !exists {
		return nil
	}
	return json.Unmarshal(raw, target)
}

// MatchesAccessToken compares token digests so the comparison work does not
// reveal whether the raw values had equal lengths.
func MatchesAccessToken(voucher Voucher, presented string) bool {
	if voucher.AccessToken == "" || presented == "" {
		return false
	}
	expectedDigest := sha256.Sum256([]byte(voucher.AccessToken))
	presentedDigest := sha256.Sum256([]byte(presented))
	return subtle.ConstantTimeCompare(expectedDigest[:], presentedDigest[:]) == 1
}
