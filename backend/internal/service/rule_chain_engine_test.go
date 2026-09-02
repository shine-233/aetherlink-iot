// 文件用途：规则链执行引擎回归测试（ROADMAP B2）。
// 核心逻辑：阈值过滤剪枝、字段映射、webhook httptest 闭环、命令动作注入桩。
// 关键注意事项：ruleChainCommandSender 为测试可替换注入点，用例结束后必须恢复。
package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"github.com/stretchr/testify/require"
)

func TestExecuteRuleChainThresholdBlocksBranch(t *testing.T) {
	var webhookHits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		webhookHits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	graphJSON := `{"nodes":[
		{"id":"t","type":"trigger.telemetry"},
		{"id":"f","type":"filter.threshold","config":{"key":"temperature","op":">","value":30}},
		{"id":"w","type":"action.webhook","config":{"url":"` + server.URL + `"}}
	],"edges":[{"from":"t","to":"f"},{"from":"f","to":"w"}]}`
	graph, err := ParseRuleChainGraph(graphJSON)
	require.NoError(t, err)

	rcc := &RuleChainContext{DeviceID: "dev-1", TenantID: "tenant-1"}

	// 阈值不满足 → webhook 不触发
	errs := ExecuteRuleChainGraph(context.Background(), graph, rcc, map[string]any{"temperature": float64(25)})
	require.Empty(t, errs)
	require.Equal(t, int32(0), webhookHits.Load())

	// 阈值满足 → webhook 触发一次
	errs = ExecuteRuleChainGraph(context.Background(), graph, rcc, map[string]any{"temperature": float64(35.5)})
	require.Empty(t, errs)
	require.Equal(t, int32(1), webhookHits.Load())

	// 键缺失 → 同样不触发
	errs = ExecuteRuleChainGraph(context.Background(), graph, rcc, map[string]any{})
	require.Empty(t, errs)
	require.Equal(t, int32(1), webhookHits.Load())
}

func TestExecuteRuleChainMappingTransformsPayload(t *testing.T) {
	var lastBody atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		lastBody.Store(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	graphJSON := `{"nodes":[
		{"id":"t","type":"trigger.telemetry"},
		{"id":"m","type":"transform.mapping","config":{"fields":{"temperature":"temp_c"}}},
		{"id":"w","type":"action.webhook","config":{"url":"` + server.URL + `"}}
	],"edges":[{"from":"t","to":"m"},{"from":"m","to":"w"}]}`
	graph, err := ParseRuleChainGraph(graphJSON)
	require.NoError(t, err)

	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{DeviceID: "dev-2", DeviceNumber: "num-2", TenantID: "tenant-1"},
		map[string]any{"temperature": float64(21.5), "humidity": float64(60)})
	require.Empty(t, errs)

	body := lastBody.Load().(map[string]any)
	values := body["values"].(map[string]any)
	require.Equal(t, 21.5, values["temp_c"])
	require.NotContains(t, values, "temperature", "mapping 输出应只含映射后的键")
	require.Equal(t, "dev-2", body["device_id"])
}

func TestExecuteRuleChainCommandActionUsesSender(t *testing.T) {
	oldSender := ruleChainCommandSender
	defer func() { ruleChainCommandSender = oldSender }()

	var gotIdentify string
	var gotParams string
	ruleChainCommandSender = func(_ context.Context, deviceID, identify, paramsJSON string) error {
		gotIdentify = identify
		gotParams = paramsJSON
		return nil
	}

	graphJSON := `{"nodes":[
		{"id":"t","type":"trigger.device_online"},
		{"id":"c","type":"action.command","config":{"identify":"set","params":{"power":1}}}
	],"edges":[{"from":"t","to":"c"}]}`
	graph, err := ParseRuleChainGraph(graphJSON)
	require.NoError(t, err)

	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{DeviceID: "dev-3", TenantID: "tenant-1"},
		map[string]any{"status": float64(1)})
	require.Empty(t, errs)
	require.Equal(t, "set", gotIdentify)
	require.JSONEq(t, `{"power":1}`, gotParams)
}

func TestExecuteRuleChainNodeErrorIsReported(t *testing.T) {
	graphJSON := `{"nodes":[
		{"id":"t","type":"trigger.telemetry"},
		{"id":"w","type":"action.webhook","config":{}}
	],"edges":[{"from":"t","to":"w"}]}`
	graph, err := ParseRuleChainGraph(graphJSON)
	require.NoError(t, err)

	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{DeviceID: "dev-4"}, map[string]any{"k": float64(1)})
	require.Len(t, errs, 1)
	require.True(t, strings.Contains(errs[0].Error(), "url"), errs[0].Error())
}

func TestExecuteRuleChainAlarmActionCreatesHistory(t *testing.T) {
	oldCreator := ruleChainAlarmCreator
	defer func() { ruleChainAlarmCreator = oldCreator }()
	var got *model.AlarmHistory
	ruleChainAlarmCreator = func(_ context.Context, h *model.AlarmHistory) error {
		got = h
		return nil
	}
	graphJSON := `{"nodes":[
		{"id":"t","type":"trigger.telemetry"},
		{"id":"a","type":"action.alarm","config":{"name":"高温告警","severity":"H","description":"温度超过阈值"}}
	],"edges":[{"from":"t","to":"a"}]}`
	graph, err := ParseRuleChainGraph(graphJSON)
	require.NoError(t, err)
	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{DeviceID: "dev-5", TenantID: "tenant-1"},
		map[string]any{"temperature": float64(41)})
	require.Empty(t, errs)
	require.NotNil(t, got)
	require.Equal(t, "高温告警", got.Name)
	require.Equal(t, "H", got.AlarmStatus)
	require.Equal(t, "tenant-1", got.TenantID)
	require.Equal(t, `["dev-5"]`, got.AlarmDeviceList)
	require.NotNil(t, got.Content)
}

func TestExecuteRuleChainAlarmActionValidation(t *testing.T) {
	oldCreator := ruleChainAlarmCreator
	defer func() { ruleChainAlarmCreator = oldCreator }()
	called := false
	ruleChainAlarmCreator = func(_ context.Context, _ *model.AlarmHistory) error {
		called = true
		return nil
	}
	// 缺少 name → 报错且不落库
	graphJSON := `{"nodes":[
		{"id":"t","type":"trigger.telemetry"},
		{"id":"a","type":"action.alarm","config":{"severity":"M"}}
	],"edges":[{"from":"t","to":"a"}]}`
	graph, err := ParseRuleChainGraph(graphJSON)
	require.NoError(t, err)
	errs := ExecuteRuleChainGraph(context.Background(), graph,
		&RuleChainContext{DeviceID: "dev-6", TenantID: "tenant-1"}, map[string]any{"k": float64(1)})
	require.Len(t, errs, 1)
	require.Contains(t, errs[0].Error(), "name")
	require.False(t, called, "missing name must not create alarm")
	// 非法 severity → 报错
	graphJSON2 := `{"nodes":[
		{"id":"t","type":"trigger.telemetry"},
		{"id":"a","type":"action.alarm","config":{"name":"x","severity":"X"}}
	],"edges":[{"from":"t","to":"a"}]}`
	graph2, _ := ParseRuleChainGraph(graphJSON2)
	errs2 := ExecuteRuleChainGraph(context.Background(), graph2,
		&RuleChainContext{DeviceID: "dev-6", TenantID: "tenant-1"}, map[string]any{"k": float64(1)})
	require.Len(t, errs2, 1)
	require.Contains(t, errs2[0].Error(), "severity")
}
