package utils

import (
	"errors"
	"strings"
	"testing"
)

func TestSanitizeLogValueRemovesLogControlCharacters(t *testing.T) {
	got := SanitizeLogValue("device\r\nforged\tentry\x00")
	if got != `device\r\nforged\tentry` {
		t.Fatalf("SanitizeLogValue() = %q, want escaped single-line value", got)
	}
	if strings.ContainsAny(got, "\r\n\x00") {
		t.Fatalf("SanitizeLogValue() retained a log control character: %q", got)
	}
}

func TestSanitizeLogFieldsDropsSensitiveValues(t *testing.T) {
	fields := SanitizeLogFields(map[string]interface{}{
		"device_id":     "device\n1",
		"apiKey":        "key-should-not-be-logged",
		"authorization": "Bearer should-not-be-logged",
		"keys":          []string{"temperature\r", "humidity"},
	})

	if _, ok := fields["apiKey"]; ok {
		t.Fatal("SanitizeLogFields retained apiKey")
	}
	if _, ok := fields["authorization"]; ok {
		t.Fatal("SanitizeLogFields retained authorization")
	}
	if got := fields["device_id"]; got != `device\n1` {
		t.Fatalf("sanitized device_id = %#v, want escaped value", got)
	}
	if got := fields["keys"].([]string); got[0] != `temperature\r` {
		t.Fatalf("sanitized keys = %#v, want escaped values", got)
	}
}

func TestSanitizeLogErrorHandlesNilAndControlCharacters(t *testing.T) {
	if got := SanitizeLogError(nil); got != "" {
		t.Fatalf("SanitizeLogError(nil) = %q, want empty string", got)
	}
	if got := SanitizeLogError(errors.New("bad\ninput")); got != `bad\ninput` {
		t.Fatalf("SanitizeLogError() = %q, want escaped value", got)
	}
}
