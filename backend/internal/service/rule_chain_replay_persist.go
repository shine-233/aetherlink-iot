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
	"fmt"
	"os"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// ruleChainReplayRetentionKey 回放留存开关。
// 未显式开启即不接线：宁可没有回放数据，也不能悄悄留存第二份载荷。
const ruleChainReplayRetentionKey = "rule_chain.replay.retention_enabled"

var replayRetentionExplicitOverride *bool

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
	if replayRetentionExplicitOverride != nil {
		return *replayRetentionExplicitOverride
	}
	if viper.IsSet(ruleChainReplayRetentionKey) && viper.GetBool(ruleChainReplayRetentionKey) {
		return true
	}
	if viper.IsSet("rule-chain.replay.retention-enabled") && viper.GetBool("rule-chain.replay.retention-enabled") {
		return true
	}
	val := os.Getenv("AETHERLINK_RULE_CHAIN_REPLAY_RETENTION")
	return val == "true" || val == "1"
}

// SetRuleChainReplayRetentionEnabled 动态调整回放留存开关（供测试与运维控制）。
func SetRuleChainReplayRetentionEnabled(enabled bool) {
	replayRetentionExplicitOverride = &enabled
	InstallRuleChainReplayPersistence()
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

// ReplayExecution 校验租户并执行单次批次输入回放，严格施加副作用闸门保护。
func (*RuleChain) ReplayExecution(ctx context.Context, chainID, execID string, confirmSideEffects bool, claims *utils.UserClaims) (map[string]any, error) {
	tenantID, err := normalizeRuleChainTenant("", claims)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(chainID) == "" || strings.TrimSpace(execID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "chain_id and exec_id are required")
	}
	chain, err := dal.GetRuleChainByID(chainID, tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if chain == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, "rule chain not found")
	}
	graph, err := ParseRuleChainGraph(string(chain.Graph))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeParamError, map[string]interface{}{"error": fmt.Sprintf("invalid chain graph: %v", err)})
	}
	graph.ChainID = chain.ID

	records, err := LoadRuleChainReplayRecords(ctx, tenantID, strings.TrimSpace(execID))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
	}
	if len(records) == 0 {
		return nil, errcode.NewWithMessage(errcode.CodeNotFound, fmt.Sprintf("no replay records found for execution %s", execID))
	}

	rcc := &RuleChainContext{
		TenantID:  tenantID,
		Timestamp: time.Now().UnixMilli(),
	}
	nodeErrs, gateErr := ReplayRuleChainExecution(ctx, graph, records, rcc, RuleChainReplayOptions{
		ReplayOf:           strings.TrimSpace(execID),
		ConfirmSideEffects: confirmSideEffects,
	})
	if gateErr != nil {
		return nil, errcode.NewWithMessage(errcode.CodeOpDenied, gateErr.Error())
	}

	errStrings := make([]string, 0, len(nodeErrs))
	for _, e := range nodeErrs {
		if e != nil {
			errStrings = append(errStrings, e.Error())
		}
	}

	return map[string]any{
		"execution_id":   strings.TrimSpace(execID),
		"chain_id":       chainID,
		"replayed_nodes": len(records),
		"errors":         errStrings,
	}, nil
}

// GetReplayRecords 查询某次执行的回放快照原始记录（租户隔离）。
func (*RuleChain) GetReplayRecords(ctx context.Context, chainID, execID string, claims *utils.UserClaims) ([]model.RuleChainReplayRecordRow, error) {
	tenantID, err := normalizeRuleChainTenant("", claims)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(execID) == "" {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "execId is required")
	}
	if strings.TrimSpace(chainID) != "" {
		chain, err := dal.GetRuleChainByID(chainID, tenantID)
		if err != nil {
			return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"error": err.Error()})
		}
		if chain == nil {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "rule chain not found")
		}
	}
	return dal.ListRuleChainReplayRecords(ctx, tenantID, strings.TrimSpace(execID))
}

