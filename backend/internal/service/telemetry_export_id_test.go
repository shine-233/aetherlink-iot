package service

import (
	"regexp"
	"sync"
	"testing"
)

func TestNewTelemetryExportIDPreservesPrefixAndFileSafeFormat(t *testing.T) {
	id := newTelemetryExportID("csv")
	if matched := regexp.MustCompile(`^csv[0-9a-f]+$`).MatchString(id); !matched {
		t.Fatalf("newTelemetryExportID() = %q, want csv prefix followed by lowercase hexadecimal characters", id)
	}
}

func TestNewTelemetryExportIDIsUniqueUnderConcurrency(t *testing.T) {
	const workers = 256

	ids := make(chan string, workers)
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			ids <- newTelemetryExportID("excel")
		}()
	}
	group.Wait()
	close(ids)

	seen := make(map[string]struct{}, workers)
	for id := range ids {
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate telemetry export ID generated: %q", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != workers {
		t.Fatalf("generated %d unique IDs, want %d", len(seen), workers)
	}
}
