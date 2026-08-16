package auth

import (
	"strings"
	"testing"
)

func TestDefaultConfigUsesBcrypt(t *testing.T) {
	if DefaultConfig.Hash != Bcrypt {
		t.Fatalf("default hash = %q, want %q", DefaultConfig.Hash, Bcrypt)
	}
	if err := DefaultConfig.Validate(); err != nil {
		t.Fatalf("default auth config should validate: %v", err)
	}
}

func TestConfigRejectsLegacyHashTypes(t *testing.T) {
	for _, hash := range []string{Plain, MD5, SHA256} {
		t.Run(hash, func(t *testing.T) {
			cfg := DefaultConfig
			cfg.Hash = hash
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("legacy hash %q should be rejected", hash)
			}
			if !strings.Contains(err.Error(), "bcrypt") {
				t.Fatalf("error = %q, want bcrypt migration guidance", err.Error())
			}
		})
	}
}
