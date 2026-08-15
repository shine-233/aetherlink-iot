package storage

import "gorm.io/gorm/clause"

// TelemetryCurrentUpsertClause keeps the current-value table monotonic by
// accepting an incoming row only when it is at least as new as the persisted
// row for the same device/key pair.
func TelemetryCurrentUpsertClause() clause.OnConflict {
	return clause.OnConflict{
		Columns: []clause.Column{{Name: "device_id"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"ts", "bool_v", "number_v", "string_v", "tenant_id",
		}),
		Where: clause.Where{Exprs: []clause.Expression{
			clause.Expr{SQL: "EXCLUDED.ts >= telemetry_current_datas.ts"},
		}},
	}
}
