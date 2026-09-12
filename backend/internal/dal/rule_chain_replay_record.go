// File purpose: persistence for P1.2 rule chain replay input snapshots.
// Core logic: upsert one snapshot per (execution, node), and read them back per execution.
// Key notes:
//   - Every statement is tenant-scoped: replay records hold raw payloads, so leaking them
//     across tenants would leak telemetry-level data.
//   - Upsert on (execution_id, node_id) keeps replay idempotent: re-running an execution
//     refreshes the snapshot instead of creating a second one.

package dal

import (
	"context"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// SaveRuleChainReplayRecords 写入（或刷新）一批回放快照。
// 采用 (execution_id, node_id) 冲突即更新：重跑同一次执行应刷新快照，
// 而不是产生第二条记录——否则回放会返回重复节点，下游被重放多次。
func SaveRuleChainReplayRecords(ctx context.Context, rows []model.RuleChainReplayRecordRow) error {
	if len(rows) == 0 || global.DB == nil {
		return nil
	}
	return global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, row := range rows {
			err := tx.Exec(`
				INSERT INTO rule_chain_replay_records
					(tenant_id, chain_id, execution_id, node_id, node_type,
					 payload, metadata, pass, error, recorded_at)
				VALUES (?, ?, ?, ?, ?, ?::jsonb, ?::jsonb, ?, ?, ?)
				ON CONFLICT (execution_id, node_id) DO UPDATE SET
					payload = EXCLUDED.payload,
					metadata = EXCLUDED.metadata,
					pass = EXCLUDED.pass,
					error = EXCLUDED.error,
					recorded_at = EXCLUDED.recorded_at`,
				row.TenantID, row.ChainID, row.ExecutionID, row.NodeID, row.NodeType,
				row.Payload, row.Metadata, row.Pass, nullIfEmpty(row.Error), row.RecordedAt,
			).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// ListRuleChainReplayRecords 读取一次执行的全部节点快照，按记录时间升序。
// 顺序即回放顺序，不能交给调用方排序——顺序错了就是把上游当下游重放。
func ListRuleChainReplayRecords(ctx context.Context, tenantID, executionID string) ([]model.RuleChainReplayRecordRow, error) {
	rows := make([]model.RuleChainReplayRecordRow, 0)
	if global.DB == nil {
		return rows, nil
	}
	type scanRow struct {
		ID          string    `gorm:"column:id"`
		TenantID    string    `gorm:"column:tenant_id"`
		ChainID     string    `gorm:"column:chain_id"`
		ExecutionID string    `gorm:"column:execution_id"`
		NodeID      string    `gorm:"column:node_id"`
		NodeType    string    `gorm:"column:node_type"`
		Payload     string    `gorm:"column:payload"`
		Metadata    string    `gorm:"column:metadata"`
		Pass        bool      `gorm:"column:pass"`
		Error       string    `gorm:"column:error"`
		RecordedAt  time.Time `gorm:"column:recorded_at"`
	}
	var scanned []scanRow
	err := global.DB.WithContext(ctx).
		Table("rule_chain_replay_records").
		Select("id, tenant_id, chain_id, execution_id, node_id, node_type, payload, metadata, pass, error, recorded_at").
		Where("tenant_id = ?", tenantID).
		Where("execution_id = ?", executionID).
		Order("recorded_at ASC, node_id ASC").
		Scan(&scanned).Error
	if err != nil {
		return nil, err
	}
	for _, item := range scanned {
		rows = append(rows, model.RuleChainReplayRecordRow{
			ID:          item.ID,
			TenantID:    item.TenantID,
			ChainID:     item.ChainID,
			ExecutionID: item.ExecutionID,
			NodeID:      item.NodeID,
			NodeType:    item.NodeType,
			Payload:     item.Payload,
			Metadata:    item.Metadata,
			Pass:        item.Pass,
			Error:       item.Error,
			RecordedAt:  item.RecordedAt,
		})
	}
	return rows, nil
}

// nullIfEmpty 让"没有错误"与"空字符串错误"在库里都表现为 NULL。
func nullIfEmpty(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}
