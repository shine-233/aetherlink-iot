package utils

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// SanitizeLogValue keeps log fields single-line and removes remaining control
// characters so request values cannot forge or corrupt plain-text log entries.
func SanitizeLogValue(value string) string {
	value = strings.NewReplacer(
		"\r", `\r`,
		"\n", `\n`,
		"\t", `\t`,
	).Replace(value)

	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
}

// SanitizeLogError returns an error message that is safe to place in a log
// field without preserving an attacker-controlled line break.
func SanitizeLogError(err error) string {
	if err == nil {
		return ""
	}
	return SanitizeLogValue(err.Error())
}

// SanitizeLogFields removes common credential fields and sanitizes string
// values in structured log fields. A fresh map is returned so callers do not
// mutate data that is still used by the request or business operation.
func SanitizeLogFields(fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return nil
	}

	safe := make(map[string]interface{}, len(fields))
	for key, value := range fields {
		if sensitiveLogField(key) {
			continue
		}
		safe[key] = sanitizeLogFieldValue(value)
	}
	return safe
}

// FormatLogFields returns a deterministic, single-line representation of
// structured fields. Keys are sanitized as well as values, and sensitive keys
// are omitted before formatting so the result is safe for both console and
// file formatters.
func FormatLogFields(fields map[string]interface{}) string {
	if len(fields) == 0 {
		return ""
	}

	safe := SanitizeLogFields(fields)
	keys := make([]string, 0, len(safe))
	for key := range safe {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var builder strings.Builder
	for _, key := range keys {
		if builder.Len() > 0 {
			builder.WriteByte(' ')
		}
		builder.WriteString(SanitizeLogValue(key))
		builder.WriteByte('=')
		builder.WriteString(SanitizeLogValue(fmt.Sprint(safe[key])))
	}
	return builder.String()
}

func sensitiveLogField(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", " ", "").Replace(key))
	for _, marker := range []string{
		"password",
		"passwd",
		"pwd",
		"token",
		"secret",
		"apikey",
		"authorization",
		"voucher",
		"privatekey",
		"credential",
		"clientsecret",
	} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func sanitizeLogFieldValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case string:
		return SanitizeLogValue(typed)
	case error:
		return SanitizeLogError(typed)
	case fmt.Stringer:
		return SanitizeLogValue(typed.String())
	case []string:
		safe := make([]string, len(typed))
		for i, item := range typed {
			safe[i] = SanitizeLogValue(item)
		}
		return safe
	case []interface{}:
		safe := make([]interface{}, len(typed))
		for i, item := range typed {
			safe[i] = sanitizeLogFieldValue(item)
		}
		return safe
	default:
		return SanitizeLogValue(fmt.Sprint(value))
	}
}
