package service

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// telemetryExportIDSequence prevents collisions when multiple exports start in
// the same nanosecond. Random entropy keeps names distinct across processes.
var telemetryExportIDSequence atomic.Uint64

// newTelemetryExportID returns a file-name-safe identifier while preserving the
// historical contract that the export type is the identifier prefix.
func newTelemetryExportID(prefix string) string {
	sequence := telemetryExportIDSequence.Add(1)
	entropy := make([]byte, 6)
	if _, err := rand.Read(entropy); err != nil {
		// Time plus the process-local atomic sequence remains unique if the OS
		// entropy source is temporarily unavailable.
		return fmt.Sprintf("%s%x%08x", prefix, time.Now().UnixNano(), sequence)
	}
	return fmt.Sprintf("%s%x%08x%s", prefix, time.Now().UnixNano(), sequence, hex.EncodeToString(entropy))
}
