// 文件用途：验证属性命令网关和属性写入服务边界。
// 核心逻辑：覆盖属性 payload 解析、下行总线参数和服务侧写权限的关键分支。
// 关键注意事项：属性写入可能触发设备下行，测试必须确保无效设备或无权限请求不会产生外部副作用。
// 重构建议：沉淀下行总线 mock，补齐协议差异、空 payload 和权限失败时的无副作用断言。
package service

import (
	"encoding/json"
	"reflect"
	"testing"

	"aetherlink-iot/backend/internal/downlink"
	"aetherlink-iot/backend/internal/model"
)

func gatewayTestStringPtr(value string) *string {
	return &value
}

func gatewayTestDecodeJSONMap(t *testing.T, raw string) map[string]interface{} {
	t.Helper()

	var got map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	return got
}

func gatewayTestRequireDeepEqual(t *testing.T, got, want map[string]interface{}) {
	t.Helper()

	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.Marshal(got)
		wantJSON, _ := json.Marshal(want)
		t.Fatalf("unexpected data\n got: %s\nwant: %s", gotJSON, wantJSON)
	}
}

func gatewayTestRequireError(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error %q, got nil", want)
	}
	if err.Error() != want {
		t.Fatalf("gateway error = %q, want %q", err.Error(), want)
	}
}

func TestTransformCommandDataForMultiLevelGatewayDirectDeviceLeavesPayloadUnwrapped(t *testing.T) {
	payload := map[string]interface{}{
		"method": "set_mode",
		"params": map[string]interface{}{"mode": "auto"},
	}

	got, err := transformCommandDataForMultiLevelGateway(payload, &model.Device{ID: "direct-device"}, "1")
	if err != nil {
		t.Fatalf("transform command data: %v", err)
	}

	if !reflect.DeepEqual(got, payload) {
		t.Fatalf("direct device should keep original payload: got %#v want %#v", got, payload)
	}
}

func TestTransformCommandDataForMultiLevelGatewayTopGatewayWrapsGatewayData(t *testing.T) {
	payload := map[string]interface{}{"method": "restart"}

	got, err := transformCommandDataForMultiLevelGateway(payload, &model.Device{ID: "gateway"}, "2")
	if err != nil {
		t.Fatalf("transform command data: %v", err)
	}

	want := map[string]interface{}{
		"gateway_data": payload,
	}
	gatewayTestRequireDeepEqual(t, got, want)
}

func TestTransformCommandDataForMultiLevelGatewaySubGatewayWrapsSubGatewayData(t *testing.T) {
	payload := map[string]interface{}{"method": "sync_time"}
	device := &model.Device{
		ID:            "sub-gateway",
		ParentID:      gatewayTestStringPtr("top-gateway"),
		SubDeviceAddr: gatewayTestStringPtr("gw-01"),
	}

	got, err := transformCommandDataForMultiLevelGateway(payload, device, "2")
	if err != nil {
		t.Fatalf("transform command data: %v", err)
	}

	want := map[string]interface{}{
		"sub_gateway_data": map[string]interface{}{
			"gw-01": map[string]interface{}{
				"gateway_data": payload,
			},
		},
	}
	gatewayTestRequireDeepEqual(t, got, want)
}

func TestTransformCommandDataForMultiLevelGatewaySubGatewayRequiresAddress(t *testing.T) {
	payload := map[string]interface{}{"method": "restart"}
	device := &model.Device{
		ID:       "sub-gateway",
		ParentID: gatewayTestStringPtr("top-gateway"),
	}

	_, err := transformCommandDataForMultiLevelGateway(payload, device, "2")
	gatewayTestRequireError(t, err, "子网关的SubDeviceAddr为空")
}

func TestTransformCommandDataForMultiLevelGatewaySubDeviceRequiresAddressBeforeCacheLookup(t *testing.T) {
	payload := map[string]interface{}{"method": "set_value"}
	device := &model.Device{
		ID:       "sub-device",
		ParentID: gatewayTestStringPtr("sub-gateway"),
	}

	_, err := transformCommandDataForMultiLevelGateway(payload, device, "3")
	gatewayTestRequireError(t, err, "子设备的SubDeviceAddr为空")
}

func TestBuildNestedSubGatewayDataForCommandTopGatewayBuildsSubDeviceData(t *testing.T) {
	payload := map[string]interface{}{"method": "set_level"}
	gateway := &model.Device{ID: "top-gateway"}

	got, err := buildNestedSubGatewayDataForCommand(gateway, "dev-01", payload)
	if err != nil {
		t.Fatalf("buildNestedSubGatewayDataForCommand returned error: %v", err)
	}

	want := map[string]interface{}{
		"sub_device_data": map[string]interface{}{
			"dev-01": payload,
		},
	}
	gatewayTestRequireDeepEqual(t, got, want)
}

func TestBuildNestedSubGatewayDataForCommandFallsBackWhenParentCacheMisses(t *testing.T) {
	payload := map[string]interface{}{"method": "set_level"}
	gateway := &model.Device{
		ID:            "sub-gateway",
		ParentID:      gatewayTestStringPtr("missing-parent"),
		SubDeviceAddr: gatewayTestStringPtr("gw-01"),
	}

	got, err := buildNestedSubGatewayDataForCommand(gateway, "dev-01", payload)
	if err != nil {
		t.Fatalf("buildNestedSubGatewayDataForCommand returned error: %v", err)
	}

	want := map[string]interface{}{
		"sub_gateway_data": map[string]interface{}{
			"gw-01": map[string]interface{}{
				"sub_device_data": map[string]interface{}{
					"dev-01": payload,
				},
			},
		},
	}
	gatewayTestRequireDeepEqual(t, got, want)
}

func TestTransformAttributeDataForMultiLevelGatewayDirectDeviceLeavesJSONUnwrapped(t *testing.T) {
	param := &model.AttributePutMessage{
		Value: `{"mode":"auto","enabled":true}`,
	}

	if err := transformAttributeDataForMultiLevelGateway(param, &model.Device{ID: "direct-device"}, "1"); err != nil {
		t.Fatalf("transform attribute data: %v", err)
	}

	got := gatewayTestDecodeJSONMap(t, param.Value)
	want := gatewayTestDecodeJSONMap(t, `{"mode":"auto","enabled":true}`)
	gatewayTestRequireDeepEqual(t, got, want)
}

func TestTransformAttributeDataForMultiLevelGatewayTopGatewayWrapsGatewayData(t *testing.T) {
	param := &model.AttributePutMessage{
		Value: `{"mode":"manual"}`,
	}

	if err := transformAttributeDataForMultiLevelGateway(param, &model.Device{ID: "gateway"}, "2"); err != nil {
		t.Fatalf("transform attribute data: %v", err)
	}

	got := gatewayTestDecodeJSONMap(t, param.Value)
	want := gatewayTestDecodeJSONMap(t, `{"gateway_data":{"mode":"manual"}}`)
	gatewayTestRequireDeepEqual(t, got, want)
}

func TestTransformAttributeDataForMultiLevelGatewaySubGatewayWrapsSubGatewayData(t *testing.T) {
	param := &model.AttributePutMessage{
		Value: `{"interval":30}`,
	}
	device := &model.Device{
		ID:            "sub-gateway",
		ParentID:      gatewayTestStringPtr("top-gateway"),
		SubDeviceAddr: gatewayTestStringPtr("gw-01"),
	}

	if err := transformAttributeDataForMultiLevelGateway(param, device, "2"); err != nil {
		t.Fatalf("transform attribute data: %v", err)
	}

	got := gatewayTestDecodeJSONMap(t, param.Value)
	want := gatewayTestDecodeJSONMap(t, `{"sub_gateway_data":{"gw-01":{"gateway_data":{"interval":30}}}}`)
	gatewayTestRequireDeepEqual(t, got, want)
}

func TestTransformAttributeDataForMultiLevelGatewayRejectsInvalidJSON(t *testing.T) {
	param := &model.AttributePutMessage{
		Value: `{"mode":`,
	}

	err := transformAttributeDataForMultiLevelGateway(param, &model.Device{ID: "direct-device"}, "1")
	if err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestTransformAttributeDataForMultiLevelGatewaySubGatewayRequiresAddress(t *testing.T) {
	param := &model.AttributePutMessage{
		Value: `{"mode":"auto"}`,
	}
	device := &model.Device{
		ID:       "sub-gateway",
		ParentID: gatewayTestStringPtr("top-gateway"),
	}

	err := transformAttributeDataForMultiLevelGateway(param, device, "2")
	gatewayTestRequireError(t, err, "子网关的SubDeviceAddr为空")
}

func TestTransformAttributeDataForMultiLevelGatewaySubDeviceRequiresAddressBeforeCacheLookup(t *testing.T) {
	param := &model.AttributePutMessage{
		Value: `{"mode":"auto"}`,
	}
	device := &model.Device{
		ID:       "sub-device",
		ParentID: gatewayTestStringPtr("sub-gateway"),
	}

	err := transformAttributeDataForMultiLevelGateway(param, device, "3")
	gatewayTestRequireError(t, err, "子设备的SubDeviceAddr为空")
}

func TestBuildNestedSubGatewayDataForAttributeTopGatewayBuildsSubDeviceData(t *testing.T) {
	inputData := map[string]interface{}{"mode": "auto"}
	gateway := &model.Device{ID: "top-gateway"}

	got := buildNestedSubGatewayDataForAttribute(gateway, "dev-01", inputData)

	want := map[string]interface{}{
		"sub_device_data": map[string]interface{}{
			"dev-01": inputData,
		},
	}
	gatewayTestRequireDeepEqual(t, got, want)
}

func TestBuildNestedSubGatewayDataForAttributeFallsBackWhenParentCacheMisses(t *testing.T) {
	inputData := map[string]interface{}{"mode": "auto"}
	gateway := &model.Device{
		ID:            "sub-gateway",
		ParentID:      gatewayTestStringPtr("missing-parent"),
		SubDeviceAddr: gatewayTestStringPtr("gw-01"),
	}

	got := buildNestedSubGatewayDataForAttribute(gateway, "dev-01", inputData)

	want := map[string]interface{}{
		"sub_gateway_data": map[string]interface{}{
			"gw-01": map[string]interface{}{
				"sub_device_data": map[string]interface{}{
					"dev-01": inputData,
				},
			},
		},
	}
	gatewayTestRequireDeepEqual(t, got, want)
}

func TestCommandDataGetDeviceConfigIDReturnsConfigOrEmpty(t *testing.T) {
	service := &CommandData{}

	if got := service.getDeviceConfigID(&model.Device{}); got != "" {
		t.Fatalf("nil config should return empty string, got %q", got)
	}
	if got := service.getDeviceConfigID(&model.Device{DeviceConfigID: gatewayTestStringPtr("config-1")}); got != "config-1" {
		t.Fatalf("unexpected config ID: %q", got)
	}
}

func TestAttributeDataGetDeviceConfigIDReturnsConfigOrEmpty(t *testing.T) {
	service := &AttributeData{}

	if got := service.getDeviceConfigID(&model.Device{}); got != "" {
		t.Fatalf("nil config should return empty string, got %q", got)
	}
	if got := service.getDeviceConfigID(&model.Device{DeviceConfigID: gatewayTestStringPtr("config-1")}); got != "config-1" {
		t.Fatalf("unexpected config ID: %q", got)
	}
}

func TestCommandDataSetDownlinkBusStoresInjectedBus(t *testing.T) {
	service := &CommandData{}
	bus := downlink.NewBus(1)
	defer bus.Close()

	service.SetDownlinkBus(bus)
	if service.downlinkBus != bus {
		t.Fatal("CommandData.SetDownlinkBus should store the injected command downlink bus")
	}

	service.SetDownlinkBus(nil)
	if service.downlinkBus != nil {
		t.Fatal("command downlinkBus should be nil after SetDownlinkBus(nil)")
	}
}

func TestAttributeDataSetDownlinkBusStoresInjectedBus(t *testing.T) {
	service := &AttributeData{}
	bus := downlink.NewBus(1)
	defer bus.Close()

	service.SetDownlinkBus(bus)
	if service.downlinkBus != bus {
		t.Fatal("AttributeData.SetDownlinkBus should store the injected attribute downlink bus")
	}

	service.SetDownlinkBus(nil)
	if service.downlinkBus != nil {
		t.Fatal("attribute downlinkBus should be nil after SetDownlinkBus(nil)")
	}
}
