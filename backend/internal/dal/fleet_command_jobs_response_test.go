package dal

import (
	"testing"
	"time"
)

func TestCommandJobDetailResponseIsStale(t *testing.T) {
	current := time.Date(2026, 7, 7, 12, 0, 0, 0, time.UTC)
	older := current.Add(-time.Second)
	same := current
	newer := current.Add(time.Second)

	if !commandJobDetailResponseIsStale(&current, older) {
		t.Fatalf("expected older response to be stale")
	}
	if commandJobDetailResponseIsStale(&current, same) {
		t.Fatalf("expected same-time response to be accepted for idempotent retry")
	}
	if commandJobDetailResponseIsStale(&current, newer) {
		t.Fatalf("expected newer response to be accepted")
	}
	if commandJobDetailResponseIsStale(nil, newer) {
		t.Fatalf("expected first response to be accepted")
	}
}

func TestCommandJobDetailResponseMatchIsAmbiguous(t *testing.T) {
	if commandJobDetailResponseMatchIsAmbiguous(0) {
		t.Fatalf("expected no match to stay unambiguous")
	}
	if commandJobDetailResponseMatchIsAmbiguous(1) {
		t.Fatalf("expected one match to stay unambiguous")
	}
	if !commandJobDetailResponseMatchIsAmbiguous(2) {
		t.Fatalf("expected duplicate message candidates to be ambiguous")
	}
}
