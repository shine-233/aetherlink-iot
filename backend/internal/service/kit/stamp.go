package kit

import (
	"time"

	"github.com/go-basic/uuid"
)

// NowUTC is time.Now().UTC().
func NowUTC() time.Time { return time.Now().UTC() }

// NowUTCPtr returns a pointer to a fresh UTC timestamp, for models whose
// CreatedAt/UpdatedAt are *time.Time.
func NowUTCPtr() *time.Time {
	t := NowUTC()
	return &t
}

// NewID returns a new UUID string (github.com/go-basic/uuid, as services use).
func NewID() string { return uuid.New() }
