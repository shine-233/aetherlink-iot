package dal

import (
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestCommandJobRetryAfterUpdateUsesPostgresTimestampType(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{DryRun: true})
	if err != nil {
		t.Fatalf("open dry-run database: %v", err)
	}

	nextRetryAfter := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	result := db.Model(&model.CommandJobDetail{}).
		Where("command_job_id = ?", "job-1").
		Updates(map[string]interface{}{
			"next_retry_after": commandJobNextRetryAfterExpression(3, nextRetryAfter),
		})
	if result.Error != nil {
		t.Fatalf("build retry-after update: %v", result.Error)
	}

	sql := result.Statement.SQL.String()
	if !strings.Contains(strings.ToLower(sql), "cast") || !strings.Contains(strings.ToLower(sql), "timestamptz") {
		t.Fatalf("retry-after SQL = %q, want an explicit timestamptz cast", sql)
	}
	if len(result.Statement.Vars) < 3 {
		t.Fatalf("retry-after SQL vars = %#v, want max attempts, timestamp, and job id", result.Statement.Vars)
	}
	if got, ok := result.Statement.Vars[1].(time.Time); !ok || !got.Equal(nextRetryAfter) {
		t.Fatalf("retry-after timestamp var = %#v, want %s", result.Statement.Vars[1], nextRetryAfter)
	}
	if result.Statement.Vars[len(result.Statement.Vars)-1] != "job-1" {
		t.Fatalf("retry-after predicate var = %#v, want job-1", result.Statement.Vars[len(result.Statement.Vars)-1])
	}
}
