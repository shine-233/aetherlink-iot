package service

import (
	"reflect"
	"testing"
)

func TestResolveExecuteEmailRecipientsPrefersConfiguredRecipients(t *testing.T) {
	got := resolveExecuteEmailRecipients("ops@example.com, admin@example.com", []string{"registered@example.com"})
	want := []string{"ops@example.com", "admin@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveExecuteEmailRecipients = %#v, want %#v", got, want)
	}
}

func TestResolveExecuteEmailRecipientsFallsBackToDefaultRecipients(t *testing.T) {
	got := resolveExecuteEmailRecipients("  ", []string{"registered@example.com"})
	want := []string{"registered@example.com"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("resolveExecuteEmailRecipients fallback = %#v, want %#v", got, want)
	}
}
