package service

import (
	"testing"

	"github.com/spf13/viper"
)

func TestTotpCipherRoundTrip(t *testing.T) {
	viper.Set("jwt.key", "test-secret-for-totp-cipher-roundtrip-0123456789")
	defer viper.Reset()
	cipherText, err := aesGCMEncrypt("JBSWY3DPEHPK3PXP")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if cipherText == "JBSWY3DPEHPK3PXP" {
		t.Fatal("cipher must not equal plaintext")
	}
	plain, err := aesGCMDecrypt(cipherText)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if plain != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("roundtrip mismatch: %q", plain)
	}
	if _, err := aesGCMDecrypt("broken!!"); err == nil {
		t.Fatal("decrypt of garbage must fail")
	}
}

func TestTotpChallengeTicketRoundTrip(t *testing.T) {
	viper.Set("jwt.key", "test-secret-for-totp-ticket-roundtrip-0123456789")
	defer viper.Reset()
	ticket, err := (&UserTotp{}).IssueChallenge("user-42")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	uid, err := parseChallenge(ticket)
	if err != nil || uid != "user-42" {
		t.Fatalf("parse uid=%q err=%v", uid, err)
	}
	if _, err := parseChallenge("not-a-jwt"); err == nil {
		t.Fatal("garbage ticket must be rejected")
	}
}

func TestTotpRecoveryHashDeterministic(t *testing.T) {
	a := hashRecoveryCode("ABCDE-12345")
	b := hashRecoveryCode("ABCDE-12345")
	c := hashRecoveryCode("ABCDE-12346")
	if a != b {
		t.Fatal("hash must be deterministic")
	}
	if a == c {
		t.Fatal("distinct codes must differ")
	}
}
