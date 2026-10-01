// 文件用途：Integration 上行转换管线单测（TB-45）——绑定命中/未绑定/失败丢弃/计数/fail-closed。
// 核心逻辑：以注入式 loader + executor 驱动 IntegrationResolver.Hook 与 applyUplinkTransform，
//
//	零 DB 依赖验证转换语义与计数；OPC UA 采集器存量路径（nil 钩子）不回归一并锁定。
//
// 关键注意事项：不测真实 SQL（绑定查询语义由 loadEnabledUplinkConverterFromDB 的
// 租户过滤约定保证）；真实转换引擎行为由 service 层单测覆盖。
// 重构建议：接入真实 sqlite 测试库后可补 loadEnabledUplinkConverterFromDB 的 SQL 路径用例。
package collector

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
)

// mustResolver 构造注入式 Resolver。
func mustResolver(t *testing.T, loader func(tenantID, deviceID string) (*model.DataConverter, error), executor func(conv *model.DataConverter, values map[string]interface{}, metadata map[string]string) (map[string]interface{}, error)) *IntegrationResolver {
	t.Helper()
	r := NewIntegrationResolver(nil, model.IntegrationConnectorOpcua, nil)
	r.loadEnabledUplinkConverter = loader
	r.executeUplink = executor
	return r
}

func TestIntegrationHookUnboundDevicePassthrough(t *testing.T) {
	loaderCalled := false
	r := mustResolver(t, func(tenantID, deviceID string) (*model.DataConverter, error) {
		loaderCalled = true
		return nil, nil
	}, nil)

	values := map[string]interface{}{"k": 1.0}
	got, ok := r.Hook(context.Background(), deviceTarget{DeviceID: "d1", TenantID: "t1"}, values)
	if !ok || got["k"] != 1.0 {
		t.Fatalf("未绑定设备必须旁路: ok=%v got=%v", ok, got)
	}
	if !loaderCalled {
		t.Fatal("loader 应被调用一次（建立负缓存）")
	}
	// 第二次走负缓存，loader 不再被调。
	if _, ok := r.Hook(context.Background(), deviceTarget{DeviceID: "d1", TenantID: "t1"}, values); !ok {
		t.Fatal("负缓存命中仍应旁路")
	}
	if stats := r.Stats(); stats.Transformed != 0 || stats.Dropped != 0 {
		t.Fatalf("旁路不应计数: %+v", stats)
	}
}

func TestIntegrationHookTenantFailClosed(t *testing.T) {
	loaderCalled := false
	r := mustResolver(t, func(tenantID, deviceID string) (*model.DataConverter, error) {
		loaderCalled = true
		return nil, nil
	}, nil)

	// 租户 ID 为空（发现数据异常）：直接旁路，绝不触发绑定解析。
	values := map[string]interface{}{"k": 1.0}
	if _, ok := r.Hook(context.Background(), deviceTarget{DeviceID: "d1"}, values); !ok {
		t.Fatal("空租户必须旁路")
	}
	if loaderCalled {
		t.Fatal("空租户不得触发绑定解析（fail-closed）")
	}
}

func TestIntegrationHookTransformSuccess(t *testing.T) {
	conv := &model.DataConverter{ID: "c1", ConverterMode: "JSON_PATH"}
	r := mustResolver(t,
		func(tenantID, deviceID string) (*model.DataConverter, error) {
			return conv, nil
		},
		func(c *model.DataConverter, values map[string]interface{}, metadata map[string]string) (map[string]interface{}, error) {
			if c != conv {
				t.Fatalf("应传入缓存命中的转换器: %+v", c)
			}
			if metadata["device_id"] != "d1" {
				t.Fatalf("metadata 应携带 device_id: %v", metadata)
			}
			return map[string]interface{}{"temperature": values["raw"]}, nil
		})

	got, ok := r.Hook(context.Background(), deviceTarget{DeviceID: "d1", TenantID: "t1", DeviceNumber: "DN1"}, map[string]interface{}{"raw": 3.5})
	if !ok || got["temperature"] != 3.5 {
		t.Fatalf("转换结果不符: ok=%v got=%v", ok, got)
	}
	if stats := r.Stats(); stats.Transformed != 1 || stats.Dropped != 0 {
		t.Fatalf("成功计数不符: %+v", stats)
	}
}

func TestIntegrationHookTransformFailureDropsAndCounts(t *testing.T) {
	r := mustResolver(t,
		func(tenantID, deviceID string) (*model.DataConverter, error) {
			return &model.DataConverter{ID: "c1", ConverterMode: "SCRIPT"}, nil
		},
		func(c *model.DataConverter, values map[string]interface{}, metadata map[string]string) (map[string]interface{}, error) {
			return nil, errors.New("boom")
		})

	got, ok := r.Hook(context.Background(), deviceTarget{DeviceID: "d1", TenantID: "t1"}, map[string]interface{}{"k": 1.0})
	if ok || got != nil {
		t.Fatalf("转换失败必须丢弃: ok=%v got=%v", ok, got)
	}
	if stats := r.Stats(); stats.Dropped != 1 || stats.Transformed != 0 {
		t.Fatalf("丢弃计数不符: %+v", stats)
	}
}

func TestIntegrationHookExecutorNilOutputCountsAsDrop(t *testing.T) {
	r := mustResolver(t,
		func(tenantID, deviceID string) (*model.DataConverter, error) {
			return &model.DataConverter{ID: "c1"}, nil
		},
		func(c *model.DataConverter, values map[string]interface{}, metadata map[string]string) (map[string]interface{}, error) {
			return nil, nil // 引擎异常返回 nil 无错误：同样按丢弃处理
		})

	if _, ok := r.Hook(context.Background(), deviceTarget{DeviceID: "d1", TenantID: "t1"}, map[string]interface{}{"k": 1.0}); ok {
		t.Fatal("nil 输出必须按丢弃处理")
	}
	if r.Stats().Dropped != 1 {
		t.Fatalf("丢弃计数不符: %+v", r.Stats())
	}
}

func TestApplyUplinkTransformNilHookKeepsLegacyBehavior(t *testing.T) {
	tgt := deviceTarget{DeviceID: "d1", TenantID: "t1"}
	values := map[string]interface{}{"k": 1.0}

	// 存量语义：nil 钩子原样返回（OPC UA 采集器不回归的根）。
	got, ok := applyUplinkTransform(context.Background(), nil, tgt, values)
	if !ok || got["k"] != 1.0 {
		t.Fatalf("nil 钩子必须旁路: ok=%v got=%v", ok, got)
	}

	// 空遥测直接旁路，不触发钩子。
	hookCalled := false
	got, ok = applyUplinkTransform(context.Background(), func(_ context.Context, _ deviceTarget, _ map[string]interface{}) (map[string]interface{}, bool) {
		hookCalled = true
		return nil, false
	}, tgt, map[string]interface{}{})
	if !ok || len(got) != 0 || hookCalled {
		t.Fatalf("空遥测必须旁路: ok=%v got=%v hookCalled=%v", ok, got, hookCalled)
	}

	// 新建采集器默认未接线。
	if NewOpcuaPoller(nil).Transform != nil {
		t.Fatal("NewOpcuaPoller 默认 Transform 必须为 nil（存量行为不回归）")
	}
}

func TestDeviceBoundParsesConfigDeviceIDs(t *testing.T) {
	if !deviceBound(`{"device_ids":["a","b"]}`, "b") {
		t.Fatal("命中 device_ids 应返回 true")
	}
	if deviceBound(`{"device_ids":["a"]}`, "c") {
		t.Fatal("未命中应返回 false")
	}
	if deviceBound(`{bad json`, "a") || deviceBound(`{}`, "a") {
		t.Fatal("坏 JSON/空配置应视为未绑定（fail-closed）")
	}
	if got := model.ParseDeviceIDs(`{"device_ids":["x"]}`); len(got) != 1 || got[0] != "x" {
		t.Fatalf("ParseDeviceIDs 解析不符: %v", got)
	}
}

// TestDefaultExecutorRealEngine 用真实转换引擎（service.ExecuteUplinkDataConverter，即
// NewIntegrationResolver 的默认执行器）验证管线端到端语义；service 包测试二进制在低内存
// 环境无法链接，故真实引擎用例随 collector 二进制执行（collector 本就依赖 service）。
func TestDefaultExecutorRealEngine(t *testing.T) {
	r := NewIntegrationResolver(nil, model.IntegrationConnectorOpcua, nil)

	// JSON_PATH：真实引擎命中映射。
	conv := &model.DataConverter{
		ID:            "c-real",
		ConverterMode: "JSON_PATH",
		Configuration: `{"telemetry":{"temperature":"temp"}}`,
	}
	out, err := r.executeUplink(conv, map[string]interface{}{"temp": 26.5}, map[string]string{"device_id": "d1"})
	if err != nil || out["temperature"] != 26.5 {
		t.Fatalf("真实引擎 JSON_PATH 转换不符: out=%v err=%v", out, err)
	}

	// 坏模式：报错路径。
	_, err = r.executeUplink(&model.DataConverter{ID: "c2", ConverterMode: "BOGUS"}, map[string]interface{}{"k": 1.0}, nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported converter mode") {
		t.Fatalf("坏模式应报错: %v", err)
	}

	// 无输出（路径不命中）：报错 → Hook 走丢弃计数。
	empty := &model.DataConverter{ID: "c3", ConverterMode: "JSON_PATH", Configuration: `{"telemetry":{"x":"missing"}}`}
	r2 := mustResolver(t,
		func(tenantID, deviceID string) (*model.DataConverter, error) { return empty, nil },
		service.ExecuteUplinkDataConverter)
	if _, ok := r2.Hook(context.Background(), deviceTarget{DeviceID: "d1", TenantID: "t1"}, map[string]interface{}{"k": 1.0}); ok {
		t.Fatal("无输出必须按丢弃处理")
	}
	if r2.Stats().Dropped != 1 {
		t.Fatalf("丢弃计数不符: %+v", r2.Stats())
	}
}
