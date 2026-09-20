package service

import (
	"context"
	"testing"

	"aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/require"
)

func TestRuleChainDeadLetterPersistenceInstallation(t *testing.T) {
	origSink := ruleChainDeadLetterSink
	defer func() { ruleChainDeadLetterSink = origSink }()

	ruleChainDeadLetterSink = nil
	require.Nil(t, ruleChainDeadLetterSink)

	InstallRuleChainDeadLetterPersistence()
	require.NotNil(t, ruleChainDeadLetterSink)

	// Incomplete record should be skipped safely without panic
	persistRuleChainDeadLetter(RuleChainDeadLetter{ExecID: "", ChainID: "c1", NodeID: "n1"})
	persistRuleChainDeadLetter(RuleChainDeadLetter{ExecID: "e1", ChainID: "", NodeID: "n1"})
	persistRuleChainDeadLetter(RuleChainDeadLetter{ExecID: "e1", ChainID: "c1", NodeID: ""})
}

func TestShouldRecordNodeTrace(t *testing.T) {
	origEnabled := ruleChainTraceEnabled
	defer func() { ruleChainTraceEnabled = origEnabled }()

	SetRuleChainTraceEnabled(false)

	nodeNil := (*RuleChainNode)(nil)
	require.False(t, shouldRecordNodeTrace(nodeNil))

	nodeEmpty := &RuleChainNode{ID: "n1", Type: "transform.mapping"}
	require.False(t, shouldRecordNodeTrace(nodeEmpty))

	nodeDebug := &RuleChainNode{ID: "n2", Type: "transform.mapping", Config: map[string]any{"debug": true}}
	require.True(t, shouldRecordNodeTrace(nodeDebug))

	nodeTrace := &RuleChainNode{ID: "n3", Type: "transform.mapping", Config: map[string]any{"trace": true}}
	require.True(t, shouldRecordNodeTrace(nodeTrace))

	SetRuleChainTraceEnabled(true)
	require.True(t, shouldRecordNodeTrace(nodeEmpty))
}

func TestReplayRetentionToggle(t *testing.T) {
	// 恢复必须还原 override 指针本身而不是再调一次 setter：
	// SetRuleChainReplayRetentionEnabled 会把包级 override 永久钉在显式值上，
	// 若这里用 defer Set...(orig)，orig=false 时后续依赖 viper/env 默认路径的
	// 用例（如 TestInstallRuleChainReplayPersistenceInstallsWhenEnabled）会被短路。
	origOverride := replayRetentionExplicitOverride
	t.Cleanup(func() {
		replayRetentionExplicitOverride = origOverride
		InstallRuleChainReplayPersistence()
	})

	SetRuleChainReplayRetentionEnabled(true)
	require.True(t, RuleChainReplayRetentionEnabled())

	SetRuleChainReplayRetentionEnabled(false)
	require.False(t, RuleChainReplayRetentionEnabled())
}

func TestReplayExecutionValidation(t *testing.T) {
	svc := &RuleChain{}
	claims := &utils.UserClaims{TenantID: "tenant-1"}

	// Missing chainID or execID
	_, err := svc.ReplayExecution(context.Background(), "", "", false, claims)
	require.Error(t, err)

	_, err = svc.ReplayExecution(context.Background(), "c1", "", false, claims)
	require.Error(t, err)

	// Missing execId in GetReplayRecords
	_, err = svc.GetReplayRecords(context.Background(), "c1", "", claims)
	require.Error(t, err)

	// Missing execId in GetExecutionTraces
	_, err = svc.GetExecutionTraces("c1", "", claims)
	require.Error(t, err)
}
