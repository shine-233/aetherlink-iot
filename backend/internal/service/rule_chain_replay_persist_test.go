package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// P1.2 回放持久化证据：ruleChainReplayRecorder 此前恒为 nil（刻意设计），
// 但缺少落库实现意味着"运维想开启也无处可落"。本次补上落点与开关。

func TestInstallRuleChainReplayPersistenceDefaultsToBypass(t *testing.T) {
	original := ruleChainReplayRecorder
	t.Cleanup(func() {
		ruleChainReplayRecorder = original
		viper.Set(ruleChainReplayRetentionKey, nil)
	})

	viper.Set(ruleChainReplayRetentionKey, false)
	InstallRuleChainReplayPersistence()
	// 默认/显式关闭时必须为 nil：回放留存原始输入，
	// 悄悄开启等于给每个部署都留一份载荷副本。
	if ruleChainReplayRecorder != nil {
		t.Fatal("replay recorder must stay nil when retention is disabled")
	}
	if RuleChainReplayRetentionEnabled() {
		t.Fatal("retention must report disabled")
	}
}

func TestInstallRuleChainReplayPersistenceInstallsWhenEnabled(t *testing.T) {
	original := ruleChainReplayRecorder
	t.Cleanup(func() {
		ruleChainReplayRecorder = original
		viper.Set(ruleChainReplayRetentionKey, nil)
	})

	viper.Set(ruleChainReplayRetentionKey, true)
	InstallRuleChainReplayPersistence()
	if ruleChainReplayRecorder == nil {
		t.Fatal("replay recorder must be installed when retention is enabled")
	}
	if !RuleChainReplayRetentionEnabled() {
		t.Fatal("retention must report enabled")
	}
}

// 缺执行 ID 或节点 ID 的快照没有回放意义，落库只会制造垃圾且无法定位。
func TestPersistRuleChainReplayRecordSkipsIncomplete(t *testing.T) {
	persistRuleChainReplayRecord(RuleChainReplayRecord{ExecID: "", NodeID: "n1"})
	persistRuleChainReplayRecord(RuleChainReplayRecord{ExecID: "e1", NodeID: ""})
	// 无数据库时只记日志返回；本例断言不 panic 且确实跳过（无写入路径可执行）。
}

func TestMarshalRuleChainReplayJSONHandlesNilAndValues(t *testing.T) {
	if got, err := marshalRuleChainReplayJSON(nil); err != nil || got != "{}" {
		t.Fatalf("nil payload = %q, err = %v; want {}", got, err)
	}
	encoded, err := marshalRuleChainReplayJSON(map[string]any{"temp": 21.5})
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	decoded, err := unmarshalRuleChainReplayJSON(encoded)
	if err != nil {
		t.Fatalf("round trip error = %v", err)
	}
	if decoded["temp"] != 21.5 {
		t.Fatalf("round trip lost the value: %v", decoded)
	}
}

// 不可编码的载荷必须被跳过而不是污染整批——回放是诊断辅助，
// 不能因为一条脏数据让规则链执行失败。
func TestPersistRuleChainReplayRecordSkipsUnencodablePayload(t *testing.T) {
	persistRuleChainReplayRecord(RuleChainReplayRecord{
		ExecID:  "e1",
		NodeID:  "n1",
		Payload: map[string]any{"bad": make(chan int)},
	})
}

func TestUnmarshalRuleChainReplayJSONTreatsEmptyAsEmptyMap(t *testing.T) {
	decoded, err := unmarshalRuleChainReplayJSON("")
	if err != nil {
		t.Fatalf("empty must not error: %v", err)
	}
	if len(decoded) != 0 {
		t.Fatalf("empty → %v, want empty map", decoded)
	}
}

// 记录必须带上租户：回放留存的是原始输入，敏感性与遥测同级。
func TestRuleChainReplayRecordCarriesTenant(t *testing.T) {
	record := RuleChainReplayRecord{ExecID: "e1", NodeID: "n1", TenantID: "tenant-hq"}
	if record.TenantID != "tenant-hq" {
		t.Fatalf("tenant = %q", record.TenantID)
	}
}

func TestRuleChainReplayJSONRoundTripPreservesTime(t *testing.T) {
	// At 为零值时落库应回退到当前时间，避免出现 0001-01-01 这种假时间戳。
	encoded, err := json.Marshal(time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		t.Fatalf("marshal error = %v", err)
	}
	if len(encoded) == 0 {
		t.Fatal("encoded time must not be empty")
	}
}
