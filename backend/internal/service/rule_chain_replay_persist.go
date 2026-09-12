// File purpose: P1.2 persistence wiring for rule chain replay snapshots.
// Core logic: when replay retention is explicitly enabled, install a recorder that writes
// each node's input snapshot; otherwise leave the recorder nil (bypass, zero hot-path cost).
// Key notes: the recorder is deliberately opt-in. Replay must retain raw input, which is a
// second copy of payload data and a different commitment from the minimal trace. Turning it
// on by default would silently start persisting payloads nobody asked for.
//
// Retention is also best-effort by design: a failed write must never break rule chain
// execution. Replay is a diagnostic aid, not part of the delivery path.

package service

import (
	"context"
	"encoding/json"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ruleChainReplayRetentionKey 回放留存开关。
// 未显式开启即不接线：宁可没有回放数据，也不能悄悄留存第二份载荷。
const ruleChainReplayRetentionKey = "rule_chain.replay.retention_enabled"

// InstallRuleChainReplayPersistence 按配置决定是否接入回放留存。
// 关闭时确保 recorder 为 nil（旁路），开启时安装落库 recorder。
func InstallRuleChainReplayPersistence() {
	if !ruleChainReplayRetentionEnabled() {
		ruleChainReplayRecorder = nil
		return
	}
	ruleChainReplayRecorder = persistRuleChainReplayRecord
}

func ruleChainReplayRetentionEnabled() bool {
	return viper.IsSet(ruleChainReplayRetentionKey) && viper.GetBool(ruleChainReplayRetentionKey)
}

// RuleChainReplayRetentionEnabled 暴露留存开关状态，供启动装配打日志提示。
func RuleChainReplayRetentionEnabled() bool {
	return ruleChainReplayRetentionEnabled()
}

// persistRuleChainReplayRecord 把一条快照落库。失败只记日志，绝不打断规则链执行。
func persistRuleChainReplayRecord(rec RuleChainReplayRecord) {
	if rec.ExecID == "" || rec.NodeID == "" {
		return
	}
	payload, err := marshalRuleChainReplayJSON(rec.Payload)
	if err != nil {
		logrus.WithError(err).Warn("rule chain replay: skip record with unencodable payload")
		return
	}
	metadata, err := marshalRuleChainReplayJSON(rec.Metadata)
	if err != nil {
		logrus.WithError(err).Warn("rule chain replay: skip record with unencodable metadata")
		return
	}
	recordedAt := rec.At
	if recordedAt.IsZero() {
		recordedAt = time.Now().UTC()
	}
	if err := dal.SaveRuleChainReplayRecords(context.Background(), []model.RuleChainReplayRecordRow{{
		TenantID:    rec.TenantID,
		ChainID:     rec.ChainID,
		ExecutionID: rec.ExecID,
		NodeID:      rec.NodeID,
		NodeType:    rec.NodeType,
		Payload:     payload,
		Metadata:    metadata,
		Pass:        rec.Pass,
		Error:       rec.Error,
		RecordedAt:  recordedAt,
	}}); err != nil {
		logrus.WithError(err).Warn("rule chain replay record not persisted")
	}
}

func marshalRuleChainReplayJSON(value map[string]any) (string, error) {
	if value == nil {
		return "{}", nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// LoadRuleChainReplayRecords 读取一次执行的回放快照并转成回放函数所需的形态。
// 顺序由 DAL 保证（recorded_at 升序），这里不重排。
func LoadRuleChainReplayRecords(ctx context.Context, tenantID, executionID string) ([]RuleChainReplayRecord, error) {
	rows, err := dal.ListRuleChainReplayRecords(ctx, tenantID, executionID)
	if err != nil {
		return nil, err
	}
	records := make([]RuleChainReplayRecord, 0, len(rows))
	for _, row := range rows {
		payload, err := unmarshalRuleChainReplayJSON(row.Payload)
		if err != nil {
			return nil, err
		}
		metadata, err := unmarshalRuleChainReplayJSON(row.Metadata)
		if err != nil {
			return nil, err
		}
		records = append(records, RuleChainReplayRecord{
			ExecID:   row.ExecutionID,
			ChainID:  row.ChainID,
			NodeID:   row.NodeID,
			NodeType: row.NodeType,
			TenantID: row.TenantID,
			Payload:  payload,
			Metadata: metadata,
			Pass:     row.Pass,
			Error:    row.Error,
			At:       row.RecordedAt,
		})
	}
	return records, nil
}

func unmarshalRuleChainReplayJSON(raw string) (map[string]any, error) {
	if raw == "" {
		return map[string]any{}, nil
	}
	decoded := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}
