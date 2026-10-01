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
	"strings"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// ruleChainReplayRecordBatchSize 单条多行 INSERT 的行数上限。
// 每行 10 个绑定参数，500 行 = 5000 参数，远低于 PostgreSQL 的 65535 上限。
const ruleChainReplayRecordBatchSize = 500

// SaveRuleChainReplayRecords 写入（或刷新）一批回放快照。
// 采用 (execution_id, node_id) 冲突即更新：重跑同一次执行应刷新快照，
// 而不是产生第二条记录——否则回放会返回重复节点，下游被重放多次。
//
// 用一条多行 VALUES 的 INSERT ... ON CONFLICT 替代逐行 Exec：语句数从 N 降到 ceil(N/500)，
// 且整批仍在同一个事务里，语义（原子 + 幂等刷新）与逐行完全一致。
func SaveRuleChainReplayRecords(ctx context.Context, rows []model.RuleChainReplayRecordRow) error {
	if len(rows) == 0 || global.DB == nil {
		return nil
	}
	return global.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for start := 0; start < len(rows); start += ruleChainReplayRecordBatchSize {
			end := start + ruleChainReplayRecordBatchSize
			if end > len(rows) {
				end = len(rows)
			}
			batch := rows[start:end]
			var sb strings.Builder
			sb.WriteString(`
				INSERT INTO rule_chain_replay_records
					(tenant_id, chain_id, execution_id, node_id, node_type,
					 payload, metadata, pass, error, recorded_at)
				VALUES `)
			args := make([]interface{}, 0, len(batch)*10)
			for i, row := range batch {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString("(?, ?, ?, ?, ?, ?::jsonb, ?::jsonb, ?, ?, ?)")
				args = append(args, row.TenantID, row.ChainID, row.ExecutionID, row.NodeID, row.NodeType,
					row.Payload, row.Metadata, row.Pass, nullIfEmpty(row.Error), row.RecordedAt)
			}
			sb.WriteString(`
				ON CONFLICT (execution_id, node_id) DO UPDATE SET
					payload = EXCLUDED.payload,
					metadata = EXCLUDED.metadata,
					pass = EXCLUDED.pass,
					error = EXCLUDED.error,
					recorded_at = EXCLUDED.recorded_at`)
			if err := tx.Exec(sb.String(), args...).Error; err != nil {
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
