// telemetry_writer_replay.go replays the independent telemetry file spool
// back into PostgreSQL with first-writer-wins history semantics.

package storage

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (w *telemetryWriter) replayTelemetryFileSpool(parent context.Context) {
	if w == nil || w.spool == nil {
		return
	}
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, w.config.TelemetrySpoolReplayTimeout)
	defer cancel()
	result, err := w.spool.replay(
		ctx,
		w.config.TelemetrySpoolReplayBatchSize,
		w.replayTelemetryFileSpoolRow,
	)
	if w.metrics != nil {
		w.metrics.addTelemetrySpoolReplayed(int64(result.Replayed))
		w.metrics.addTelemetrySpoolCorrupt(int64(result.Corrupt))
		w.metrics.setTelemetrySpoolUsage(result.Usage)
	}
	if result.Replayed > 0 && w.logger != nil {
		w.logger.Infof(
			"telemetry file spool replayed: replayed=%d attempted=%d backlog=%d bytes=%d quarantine_records=%d quarantine_bytes=%d",
			result.Replayed,
			result.Attempted,
			result.Usage.Records,
			result.Usage.Bytes,
			result.Usage.QuarantinedRecords,
			result.Usage.QuarantinedBytes,
		)
	}
	if err != nil && w.logger != nil {
		w.logger.Warnf(
			"telemetry file spool replay incomplete: attempted=%d replayed=%d corrupt=%d backlog=%d quarantine_records=%d quarantine_bytes=%d err=%v",
			result.Attempted,
			result.Replayed,
			result.Corrupt,
			result.Usage.Records,
			result.Usage.QuarantinedRecords,
			result.Usage.QuarantinedBytes,
			err,
		)
	}
}

func (w *telemetryWriter) replayTelemetryFileSpoolRow(ctx context.Context, history TelemetryData) error {
	if w == nil || w.db == nil {
		return fmt.Errorf("telemetry database is unavailable")
	}
	return w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		insert := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "device_id"}, {Name: "key"}, {Name: "ts"}},
			DoNothing: true,
		}).Create(&history)
		if insert.Error != nil {
			return fmt.Errorf("insert replayed history data failed: %w", insert.Error)
		}

		authoritative := history
		if insert.RowsAffected == 0 {
			// Another path already persisted this unique history identity. Read
			// that first-writer-wins value before touching current data; otherwise
			// an equal-timestamp spool replay could leave history=B/current=A.
			if err := tx.Where(
				"device_id = ? AND key = ? AND ts = ?",
				history.DeviceID,
				history.Key,
				history.TS,
			).Take(&authoritative).Error; err != nil {
				return fmt.Errorf("load authoritative replay history data failed: %w", err)
			}
		}

		current := telemetryCurrentFromHistory(authoritative)
		if err := tx.Clauses(TelemetryCurrentUpsertClause()).Create(&current).Error; err != nil {
			return fmt.Errorf("upsert replayed current data failed: %w", err)
		}
		return nil
	})
}

func telemetryCurrentFromHistory(history TelemetryData) TelemetryCurrentData {
	return TelemetryCurrentData{
		DeviceID: history.DeviceID,
		Key:      history.Key,
		TS:       time.UnixMilli(history.TS),
		BoolV:    history.BoolV,
		NumberV:  history.NumberV,
		StringV:  history.StringV,
		TenantID: history.TenantID,
	}
}
