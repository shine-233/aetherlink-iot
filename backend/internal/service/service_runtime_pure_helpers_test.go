// service_runtime_pure_helpers_test.go covers offline service guards for scripts, plugins, UI policy, device debug, and telemetry simulation.
package service

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

func TestDataScriptListRejectsMissingDeviceConfigBeforeDAL(t *testing.T) {
	_, err := (&DataScript{}).GetDataScriptListByPage(&model.GetDataScriptListByPageReq{}, &utils.UserClaims{
		TenantID:  "tenant-1",
		Authority: constant.TENANT_ADMIN,
	})
	if err == nil {
		t.Fatal("missing device_config_id should be rejected before data script list DAL query")
	}
	assertDataScriptServiceError(t, err, "missing data script device config id", errcode.CodeParamError, "device_config_id is required")
}

func TestDataScriptQuizRejectsNilClaimsBeforeExecutingScript(t *testing.T) {
	_, err := (&DataScript{}).QuizDataScript(&model.QuizDataScriptReq{
		Content:     "return payload;",
		AnalogInput: "payload",
		Topic:       "telemetry",
	}, nil)
	if err == nil {
		t.Fatal("nil claims should not run data script quiz")
	}
	assertDataScriptServiceError(t, err, "nil claims data script quiz", errcode.CodeNoPermission, "no permission to run data script quiz")
}

func TestDataScriptQuizRejectsInvalidHexInputBeforeScriptExecution(t *testing.T) {
	_, err := (&DataScript{}).QuizDataScript(&model.QuizDataScriptReq{
		Content:     "return payload;",
		AnalogInput: "0xnot-hex",
		Topic:       "telemetry",
	}, &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN})
	if err == nil {
		t.Fatal("invalid hex analog input should be rejected before script execution")
	}
	apiErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("invalid hex error type = %T, want *errcode.Error", err)
	}
	if apiErr.Code != errcode.CodeParamError {
		t.Fatalf("invalid hex error code = %d, want %d", apiErr.Code, errcode.CodeParamError)
	}
	if apiErr.Variables["error"] != "hex decode error" || apiErr.Variables["input"] != "0xnot-hex" {
		t.Fatalf("invalid hex error vars = %#v, want error hex decode error and input 0xnot-hex", apiErr.Variables)
	}
}

func TestDataScriptRunScriptIsTelemetryNoop(t *testing.T) {
	// RunScript 是 telemetry 上报 cron 的占位入口，已被 TelemetryService 接管；
	// 它必须 (1) 不 panic、(2) 不发起任何外部调用、(3) 仅输出一条 debug 日志说明用途。
	// 用 logrus hook 捕获标准 logger 的输出做行为断言，避免"调一下就过"的空测试。
	standardLogger := logrus.StandardLogger()

	hook := &dataScriptRunScriptLogHook{messages: make([]string, 0)}
	standardLogger.AddHook(hook)
	defer standardLogger.ReplaceHooks(make(logrus.LevelHooks))

	prevLevel := standardLogger.Level
	standardLogger.SetLevel(logrus.DebugLevel)
	defer standardLogger.SetLevel(prevLevel)

	(&DataScript{}).RunScript()

	hook.mu.Lock()
	defer hook.mu.Unlock()
	if len(hook.messages) == 0 {
		t.Fatalf("RunScript() emitted no debug log; expected telemetry-noop announcement")
	}
	if !strings.Contains(hook.messages[len(hook.messages)-1], "RunScript cron executed") {
		t.Fatalf("RunScript() last log = %q, want substring %q",
			hook.messages[len(hook.messages)-1], "RunScript cron executed")
	}
}

type dataScriptRunScriptLogHook struct {
	mu       sync.Mutex
	messages []string
}

func (h *dataScriptRunScriptLogHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *dataScriptRunScriptLogHook) Fire(entry *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, entry.Message)
	return nil
}

func TestProtocolPluginDeviceConfigRejectsNilClaimsBeforeDAL(t *testing.T) {
	_, err := (&ProtocolPlugin{}).GetDeviceConfig(model.GetDeviceConfigReq{
		DeviceId: "device-1",
	}, nil)
	if err == nil {
		t.Fatal("nil api-key claims should be rejected before device lookup")
	}
	assertPluginDeviceConfigAccessError(t, err, "nil claims get protocol plugin device config", "plugin device config requires api key")
}

func TestProtocolPluginDeviceConfigRequiresAtLeastOneDeviceIdentifier(t *testing.T) {
	_, err := (&ProtocolPlugin{}).GetDeviceConfig(model.GetDeviceConfigReq{}, &utils.UserClaims{
		TenantID:  "tenant-1",
		Authority: constant.TENANT_ADMIN,
	})
	if err == nil {
		t.Fatal("empty device id, voucher, and device_number should be rejected before DAL")
	}
	assertProtocolPluginServiceError(t, err, "missing protocol plugin device identifier", errcode.CodeParamError, "", map[string]interface{}{
		"error": "device id and voucher and device_number must have one",
	}, nil)
}

func TestProtocolPluginDeviceListRejectsNilClaimsBeforeDAL(t *testing.T) {
	_, err := (&ProtocolPlugin{}).GetDevicesByProtocolPlugin(model.GetDevicesByProtocolPluginReq{
		DeviceType:        "1",
		ServiceIdentifier: "MQTT",
	}, nil)
	if err == nil {
		t.Fatal("nil api-key claims should not list devices by protocol plugin")
	}
	assertProtocolPluginServiceError(t, err, "nil claims list protocol plugin devices", errcode.CodeNoPermission, "plugin device list requires api key", nil, nil)
}

func TestProtocolPluginDeviceListRejectsUnsupportedDeviceTypeBeforeDAL(t *testing.T) {
	_, err := (&ProtocolPlugin{}).GetDevicesByProtocolPlugin(model.GetDevicesByProtocolPluginReq{
		DeviceType:        "gateway",
		ServiceIdentifier: "MQTT",
	}, &utils.UserClaims{TenantID: "tenant-1", Authority: constant.TENANT_ADMIN})
	assertProtocolPluginServiceError(t, err, "unsupported protocol plugin device type", errcode.CodeParamError, "protocol plugin device list only supports direct devices", nil, nil)
}

func TestDataPolicyMethodsRejectNonAdminBeforeDAL(t *testing.T) {
	service := &DataPolicy{}
	tenantAdmin := &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}
	tenantUser := &utils.UserClaims{Authority: constant.TENANT_USER, TenantID: "tenant-1"}

	err := service.UpdateDataPolicy(&model.UpdateDataPolicyReq{
		Id:            "policy-1",
		RetentionDays: 30,
		Enabled:       "1",
		Remark:        pureHelperStringPtr("keep one month"),
	}, tenantAdmin)
	assertNoPermissionToManageDataPolicy(t, err, "tenant admin update data policy")

	_, err = service.GetDataPolicyListByPage(&model.GetDataPolicyListByPageReq{}, tenantUser)
	assertNoPermissionToManageDataPolicy(t, err, "tenant user query data policy list")
	_, err = service.GetDataPolicyListByPage(&model.GetDataPolicyListByPageReq{}, nil)
	assertNoPermissionToManageDataPolicy(t, err, "nil claims query data policy list")
}

func TestSysFunctionUpdateRejectsNonAdminBeforeDAL(t *testing.T) {
	service := &SysFunction{}
	for _, claims := range []*utils.UserClaims{
		nil,
		{Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"},
		{Authority: constant.TENANT_USER, TenantID: "tenant-1"},
		{Authority: ""},
	} {
		err := service.UpdateSysFuncion("fn-dashboard", claims)
		if err == nil {
			t.Fatalf("claims %#v should not update system function switch", claims)
		}
		appErr, ok := err.(*errcode.Error)
		if !ok {
			t.Fatalf("claims %#v error type = %T, want *errcode.Error", claims, err)
		}
		if appErr.Code != errcode.CodeNoPermission || appErr.CustomMsg != "no permission to update system function" {
			t.Fatalf("claims %#v error = code %d message %q, want code %d no-permission message", claims, appErr.Code, appErr.CustomMsg, errcode.CodeNoPermission)
		}
	}
}

func TestUiElementsAdminMethodsRejectNonSystemAdminBeforeDAL(t *testing.T) {
	service := &UiElements{}
	tenantAdmin := &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}
	tenantUser := &utils.UserClaims{Authority: constant.TENANT_USER, TenantID: "tenant-1"}

	err := service.CreateUiElements(&model.CreateUiElementsReq{
		ParentID:    "root",
		ElementCode: "dashboard",
		Authority:   constant.TENANT_ADMIN,
	}, tenantAdmin)
	assertNoPermissionToManageUIElements(t, err, "tenant admin create ui elements")

	err = service.UpdateUiElements(&model.UpdateUiElementsReq{
		Id:          "ui-1",
		ParentID:    pureHelperStringPtr("root"),
		ElementCode: pureHelperStringPtr("dashboard"),
		Authority:   pureHelperStringPtr(constant.TENANT_ADMIN),
	}, tenantUser)
	assertNoPermissionToManageUIElements(t, err, "tenant user update ui elements")

	err = service.DeleteUiElements("ui-1", nil)
	assertNoPermissionToManageUIElements(t, err, "nil claims delete ui elements")

	_, err = service.ServeUiElementsListByPage(&model.ServeUiElementsListByPageReq{}, tenantAdmin)
	assertNoPermissionToManageUIElements(t, err, "tenant admin query sys ui elements admin list")
}

func TestUiElementsTenantViewerMethodsRejectUnsupportedClaimsBeforeDAL(t *testing.T) {
	service := &UiElements{}
	tenantUser := &utils.UserClaims{Authority: constant.TENANT_USER, TenantID: "tenant-1"}

	_, err := service.ServeUiElementsListByAuthority(nil)
	assertNoPermissionToQueryUIElements(t, err, "nil claims query authority ui elements")

	_, err = service.GetTenantUiElementsList(nil)
	assertNoPermissionToQueryUIElements(t, err, "nil claims query tenant ui elements")

	_, err = service.GetTenantUiElementsList(tenantUser)
	assertNoPermissionToQueryUIElements(t, err, "tenant user query tenant ui elements management tree")
}

func TestMessagePushConfigMethodsRejectNonAdminBeforeDAL(t *testing.T) {
	service := &MessagePush{}
	tenantAdmin := &utils.UserClaims{Authority: constant.TENANT_ADMIN, TenantID: "tenant-1"}
	tenantUser := &utils.UserClaims{Authority: constant.TENANT_USER, TenantID: "tenant-1"}

	_, err := service.GetMessagePushConfig(tenantAdmin)
	assertNoPermissionToManageMessagePushConfig(t, err, "tenant admin query message push config")

	err = service.SetMessagePushConfig(&model.MessagePushConfigReq{Url: "https://push.example.com"}, tenantUser)
	assertNoPermissionToManageMessagePushConfig(t, err, "tenant user update message push config")

	err = service.SetMessagePushConfig(&model.MessagePushConfigReq{Url: "https://push.example.com"}, nil)
	assertNoPermissionToManageMessagePushConfig(t, err, "nil claims update message push config")
}

func TestMessagePushConfigRejectsInvalidURLBeforeDAL(t *testing.T) {
	service := &MessagePush{}
	admin := &utils.UserClaims{Authority: constant.SYS_ADMIN}

	cases := []struct {
		rawURL      string
		wantMessage string
	}{
		{rawURL: "", wantMessage: "message push url is required"},
		{rawURL: "   ", wantMessage: "message push url is required"},
		{rawURL: "not a url", wantMessage: "message push url is invalid"},
		{rawURL: "ftp://push.example.com/hook", wantMessage: "message push url must use http or https"},
		{rawURL: "ws://push.example.com/socket", wantMessage: "message push url must use http or https"},
		{rawURL: "wss://push.example.com/socket", wantMessage: "message push url must use http or https"},
	}

	for _, tc := range cases {
		req := &model.MessagePushConfigReq{Url: tc.rawURL}
		err, panicked := callMessagePushConfigAndRecoverPanic(service, req, admin)
		if panicked != nil {
			t.Fatalf("message push config url %s reached DAL instead of rejecting invalid URL: %v", tc.rawURL, panicked)
		}
		assertErrcodeError(t, err, "message push config url "+tc.rawURL, errcode.CodeParamError, tc.wantMessage)
	}
}

func TestRoleCreateRejectsUnsupportedClaimsBeforeDAL(t *testing.T) {
	service := &Role{}
	for _, claims := range []*utils.UserClaims{
		nil,
		{Authority: constant.TENANT_USER, TenantID: "tenant-1"},
		{Authority: ""},
	} {
		err := service.CreateRole(&model.CreateRoleReq{Name: "operator"}, claims)
		assertNoPermissionToManageRoles(t, err, "CreateRole")
	}
}

type structMapTestReq struct {
	Name           string            `json:"name"`
	Count          int               `json:"count"`
	Optional       *string           `json:"optional,omitempty"`
	AdditionalInfo *string           `json:"additional_info,omitempty"`
	Tags           []string          `json:"tags,omitempty"`
	Meta           map[string]string `json:"meta,omitempty"`
	Ignored        string            `json:"-"`
	Untagged       string
}

func TestStructToMapAndVerifyJsonFiltersFieldsAndValidatesConfiguredJSONTags(t *testing.T) {
	additional := `{"firmware":"1.0.0","threshold":12}`
	req := structMapTestReq{
		Name:           "rdi-device",
		Count:          2,
		AdditionalInfo: &additional,
		Tags:           []string{"rdi", "alarm"},
		Meta:           map[string]string{"tenant": "tenant-1"},
		Ignored:        "hidden",
		Untagged:       "hidden",
	}

	got, err := StructToMapAndVerifyJson(req, "additional_info")
	if err != nil {
		t.Fatalf("StructToMapAndVerifyJson returned error: %v", err)
	}
	if got["name"] != "rdi-device" || got["count"] != 2 {
		t.Fatalf("core fields mismatch: %#v", got)
	}
	if got["additional_info"] != &additional {
		t.Fatalf("additional_info pointer mismatch: %#v", got["additional_info"])
	}
	if _, ok := got["optional"]; ok {
		t.Fatalf("nil optional pointer should be filtered out: %#v", got)
	}
	if _, ok := got["-"]; ok {
		t.Fatalf("ignored field leaked into map: %#v", got)
	}
	if _, ok := got["Untagged"]; ok {
		t.Fatalf("untagged field leaked into map: %#v", got)
	}

	invalidJSON := "{"
	_, err = StructToMapAndVerifyJson(structMapTestReq{AdditionalInfo: &invalidJSON}, "additional_info")
	if err == nil || err.Error() != "additional_info is not valid JSON" {
		t.Fatalf("invalid JSON tag error = %v, want additional_info is not valid JSON", err)
	}

	if _, err := StructToMapAndVerifyJson("not-a-struct"); err == nil || err.Error() != "input is not a struct" {
		t.Fatalf("StructToMapAndVerifyJson non-struct error = %v, want input is not a struct", err)
	}
}

func TestStructToMapHandlesPointersCollectionsAndNonStructInputs(t *testing.T) {
	optional := "visible"
	req := &structMapTestReq{
		Name:     "device",
		Count:    0,
		Optional: &optional,
		Tags:     []string{},
		Meta:     nil,
	}

	got := StructToMap(req)
	if got["name"] != "device" || got["count"] != 0 {
		t.Fatalf("StructToMap core fields mismatch: %#v", got)
	}
	if got["optional"] != &optional {
		t.Fatalf("StructToMap optional pointer mismatch: %#v", got["optional"])
	}
	if tags, ok := got["tags"].([]string); !ok || len(tags) != 0 {
		t.Fatalf("StructToMap should include non-nil empty slice, got %#v", got["tags"])
	}
	if _, ok := got["meta"]; ok {
		t.Fatalf("StructToMap should omit nil map: %#v", got)
	}
	if empty := StructToMap(123); len(empty) != 0 {
		t.Fatalf("StructToMap non-struct = %#v, want empty map", empty)
	}
}

func TestNormalizeDeviceDebugLogEntryKeepsPreviousRedisLogsReadable(t *testing.T) {
	item := normalizeDeviceDebugLogEntry(model.DeviceDebugLogEntry{
		DeviceID:  "device-a",
		Direction: "up",
		Event:     "publish",
		Result:    "denied",
		Extra: map[string]interface{}{
			"topic": "devices/telemetry/1",
		},
	})

	if item.Action != "publish" || item.Outcome != "deny" {
		t.Fatalf("normalized action/outcome = %q/%q, want publish/deny", item.Action, item.Outcome)
	}
	if item.Meta["topic"] != "devices/telemetry/1" {
		t.Fatalf("previous extra was not moved to meta: %#v", item.Meta)
	}
	if item.Event != "" || item.Result != "" || item.Extra != nil {
		t.Fatalf("previous fields leaked after normalization: %#v", item)
	}
}

func TestNormalizeDeviceDebugLogEntryPreservesCurrentFields(t *testing.T) {
	item := normalizeDeviceDebugLogEntry(model.DeviceDebugLogEntry{
		Action:  "publish",
		Outcome: "ok",
		Result:  "error",
		Meta: map[string]interface{}{
			"current": true,
		},
		Extra: map[string]interface{}{
			"previous": true,
		},
	})

	if item.Action != "publish" || item.Outcome != "ok" {
		t.Fatalf("current action/outcome changed: %#v", item)
	}
	if item.Meta["current"] != true {
		t.Fatalf("current meta was not preserved: %#v", item.Meta)
	}
	if _, ok := item.Meta["previous"]; ok {
		t.Fatalf("previous extra should not override current meta: %#v", item.Meta)
	}
	if item.Result != "" || item.Extra != nil {
		t.Fatalf("previous fields leaked after preserving current fields: %#v", item)
	}
}

func TestInvalidDeviceDebugLogEntryDoesNotEchoRawPayload(t *testing.T) {
	raw := `{"password":"secret","payload":"raw"`
	entry := invalidDeviceDebugLogEntry(raw)

	if entry.Meta["raw"] != nil {
		t.Fatalf("invalid debug log should not expose raw payload: %#v", entry.Meta)
	}
	if entry.Meta["raw_size"] != len(raw) {
		t.Fatalf("raw_size = %#v, want %d", entry.Meta["raw_size"], len(raw))
	}
}

func TestNormalizeDeviceDebugLogRowsSanitizesInvalidAndHistoricalEntries(t *testing.T) {
	rows := []string{
		`{"direction":"up","event":"publish","result":"denied","extra":{"topic":"devices/1"}}`,
		`{"password":"secret"`,
	}

	list := normalizeDeviceDebugLogRows(rows)
	if len(list) != 2 {
		t.Fatalf("normalized rows len = %d, want 2", len(list))
	}
	if list[0].Action != "publish" || list[0].Outcome != "deny" {
		t.Fatalf("historical debug row normalized to %#v, want publish/deny", list[0])
	}
	if !reflect.DeepEqual(list[0].Meta, map[string]interface{}{"topic": "devices/1"}) {
		t.Fatalf("historical debug row meta = %#v", list[0].Meta)
	}
	if list[1].Action != "error" || list[1].Outcome != "error" || list[1].Error != "invalid log json" {
		t.Fatalf("invalid debug row normalized to %#v", list[1])
	}
	if list[1].Meta["raw"] != nil {
		t.Fatalf("invalid debug row should not echo raw payload: %#v", list[1].Meta)
	}
}

func TestDecodeDeviceDebugConfigAndStatusFromConfig(t *testing.T) {
	now := int64(1_700_000_000)

	cfg, err := decodeDeviceDebugConfig(`{"enabled":true,"expire_at":1700000060,"max_items":12,"payload_max_bytes":0}`)
	if err != nil {
		t.Fatalf("decodeDeviceDebugConfig returned err: %v", err)
	}
	if cfg != (model.DeviceDebugConfig{
		Enabled:         true,
		ExpireAt:        now + 60,
		MaxItems:        12,
		PayloadMaxBytes: 0,
	}) {
		t.Fatalf("decoded debug config = %#v", cfg)
	}

	status := deviceDebugStatusFromConfig(cfg, now, true)
	if !status.Enabled || status.ExpireAt != cfg.ExpireAt || status.RemainingSeconds != 60 {
		t.Fatalf("active debug status = %#v", status)
	}

	expired := deviceDebugStatusFromConfig(model.DeviceDebugConfig{
		Enabled:  true,
		ExpireAt: now - 1,
	}, now, true)
	if expired.Enabled || expired.ExpireAt != now-1 || expired.RemainingSeconds != 0 {
		t.Fatalf("expired debug status = %#v", expired)
	}

	missing := deviceDebugStatusFromConfig(defaultDeviceDebugConfig(), now, false)
	if missing.Enabled || missing.ExpireAt != 0 || missing.RemainingSeconds != 0 {
		t.Fatalf("missing debug status = %#v", missing)
	}
	if missing.Config != defaultDeviceDebugConfig() {
		t.Fatalf("missing debug status config = %#v", missing.Config)
	}
}

func TestTelemetrySimulationRejectsNilRequestsBeforeDAL(t *testing.T) {
	service := &TelemetryData{}

	_, err := service.ServeEchoData(nil, "", &utils.UserClaims{TenantID: "tenant-1"})
	assertErrcodeError(t, err, "nil serve echo telemetry request", errcode.CodeParamError, "请求不能为空")

	err = service.SimulationSend(nil, &utils.UserClaims{TenantID: "tenant-1"})
	assertErrcodeError(t, err, "nil simulation send request", errcode.CodeParamError, "请求不能为空")
}

func TestValidateSimulationPublishTargetRejectsUnavailableMQTT(t *testing.T) {
	cases := []struct {
		name    string
		enabled bool
		topic   string
		want    string
	}{
		{name: "disabled", enabled: false, topic: "devices/telemetry", want: "mqtt.enabled must be true"},
		{name: "uninitialized topic", enabled: true, topic: " ", want: "MQTT telemetry topic is not initialized"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSimulationPublishTarget(tc.enabled, tc.topic)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("validateSimulationPublishTarget(%v, %q) = %v, want error containing %q", tc.enabled, tc.topic, err, tc.want)
			}
		})
	}

	if err := validateSimulationPublishTarget(true, "devices/telemetry"); err != nil {
		t.Fatalf("configured MQTT simulation target rejected: %v", err)
	}
}

func TestTelemetrySimulationLogFieldsDoNotEchoSecretsOrPayload(t *testing.T) {
	fields := telemetryPublishLogFields(&utils.MQTTParams{
		Host:     "mqtt.example.com",
		Port:     "1883",
		Username: "device-user",
		Password: "secret-password",
		Topic:    "telemetry/device",
		Payload:  `{"temperature":25.5}`,
		ClientId: "client-1",
	})

	if _, ok := fields["password"]; ok {
		t.Fatalf("telemetry publish log fields leaked password: %#v", fields)
	}
	if _, ok := fields["payload"]; ok {
		t.Fatalf("telemetry publish log fields leaked payload: %#v", fields)
	}
	if fields["payload_size"] != len(`{"temperature":25.5}`) {
		t.Fatalf("telemetry publish payload_size = %#v", fields["payload_size"])
	}
	if fields["host"] != "mqtt.example.com" || fields["topic"] != "telemetry/device" || fields["client_id"] != "client-1" {
		t.Fatalf("telemetry publish log fields lost routing context: %#v", fields)
	}

	sendFields := simulationSendLogFields("mqtt.example.com", "1883", "telemetry/device", "client-2", `{"password":"raw"}`)
	if _, ok := sendFields["payload"]; ok {
		t.Fatalf("simulation send log fields leaked payload: %#v", sendFields)
	}
	if sendFields["payload_size"] != len(`{"password":"raw"}`) {
		t.Fatalf("simulation send payload_size = %#v", sendFields["payload_size"])
	}
}

func TestSimulationVoucherCredentialsRejectsMissingUsername(t *testing.T) {
	_, _, err := simulationVoucherCredentials(map[string]interface{}{"password": "secret"})
	assertErrcodeError(t, err, "simulation voucher missing username", errcode.CodeParamError, "设备凭证中缺少 username")
}

func TestSimulationVoucherCredentialsTrimsUsernameAndKeepsOptionalPassword(t *testing.T) {
	username, password, err := simulationVoucherCredentials(map[string]interface{}{
		"username": "  device-user  ",
		"password": "secret",
	})
	if err != nil {
		t.Fatalf("simulationVoucherCredentials returned error: %v", err)
	}
	if username != "device-user" || password != "secret" {
		t.Fatalf("simulationVoucherCredentials = %q/%q, want device-user/secret", username, password)
	}
}

func TestParseMQTTAccessAddressRejectsMalformedConfig(t *testing.T) {
	cases := []struct {
		name        string
		raw         string
		wantMessage string
	}{
		{name: "blank", raw: " ", wantMessage: "未配置 MQTT access_address"},
		{name: "missing port", raw: "mqtt.example.com", wantMessage: "MQTT access_address 必须是 host:port"},
		{name: "blank host", raw: ":1883", wantMessage: "MQTT access_address 必须是 host:port"},
		{name: "blank port", raw: "mqtt.example.com:", wantMessage: "MQTT access_address 必须是 host:port"},
		{name: "non numeric port", raw: "mqtt.example.com:abc", wantMessage: "MQTT access_address 端口无效"},
		{name: "out of range port", raw: "mqtt.example.com:70000", wantMessage: "MQTT access_address 端口无效"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := parseMQTTAccessAddress(tc.raw)
			assertErrcodeError(t, err, "mqtt access address "+tc.name, errcode.CodeParamError, tc.wantMessage)
		})
	}
}

func TestParseMQTTAccessAddressNormalizesHostAndPort(t *testing.T) {
	host, port, err := parseMQTTAccessAddress(" mqtt.example.com:01883 ")
	if err != nil {
		t.Fatalf("parseMQTTAccessAddress returned error: %v", err)
	}
	if host != "mqtt.example.com" || port != "1883" {
		t.Fatalf("parseMQTTAccessAddress normalized to %q:%q, want mqtt.example.com:1883", host, port)
	}
}

func TestResolveSimulationSendTargetAppliesTrimmedOverrides(t *testing.T) {
	overridePort := 2883
	host, port, topic, err := resolveSimulationSendTarget(&model.SimulationSendReq{
		Server: " mqtt.override.local ",
		Port:   &overridePort,
		Topic:  " telemetry/custom ",
	}, "mqtt.example.com", "1883", "telemetry/default")
	if err != nil {
		t.Fatalf("resolveSimulationSendTarget returned error: %v", err)
	}
	if host != "mqtt.override.local" || port != "2883" || topic != "telemetry/custom" {
		t.Fatalf("resolveSimulationSendTarget = %q/%q/%q", host, port, topic)
	}
}

func TestResolveSimulationSendTargetRejectsInvalidOverridePort(t *testing.T) {
	overridePort := 70000
	_, _, _, err := resolveSimulationSendTarget(&model.SimulationSendReq{Port: &overridePort}, "mqtt.example.com", "1883", "telemetry/default")
	assertErrcodeError(t, err, "simulation send invalid override port", errcode.CodeParamError, "MQTT 端口无效")
}

func TestDeviceDebugRedisKeysTrimDeviceIDForBrokerCompatibility(t *testing.T) {
	if got := devDebugCfgKey("  device-a  "); got != "tp:devdebug:cfg:device-a" {
		t.Fatalf("devDebugCfgKey trimmed key = %q", got)
	}
	if got := devDebugLogsKey("\tdevice-a\n"); got != "tp:devdebug:logs:device-a" {
		t.Fatalf("devDebugLogsKey trimmed key = %q", got)
	}
}

func TestNormalizeDeviceDebugLogPageDefaultsAndCapsLimit(t *testing.T) {
	offset, limit := normalizeDeviceDebugLogPage(nil)
	if offset != 0 || limit != defaultDebugLogsLimit {
		t.Fatalf("nil debug log page = offset %d limit %d, want 0/%d", offset, limit, defaultDebugLogsLimit)
	}

	offset, limit = normalizeDeviceDebugLogPage(&model.GetDeviceDebugLogsReq{Offset: -10, Limit: -1})
	if offset != 0 || limit != defaultDebugLogsLimit {
		t.Fatalf("negative debug log page = offset %d limit %d, want defaults", offset, limit)
	}

	offset, limit = normalizeDeviceDebugLogPage(&model.GetDeviceDebugLogsReq{Offset: 25, Limit: maxDebugLogsLimit + 100})
	if offset != 25 || limit != maxDebugLogsLimit {
		t.Fatalf("capped debug log page = offset %d limit %d, want 25/%d", offset, limit, maxDebugLogsLimit)
	}
}

func TestNormalizeDeviceDebugConfigDefaultsAndBounds(t *testing.T) {
	now := int64(1_700_000_000)

	cfg, ttl, enabled := normalizeDeviceDebugConfig(nil, now)
	if !enabled {
		t.Fatal("nil request should enable debug with defaults")
	}
	if cfg != (model.DeviceDebugConfig{
		Enabled:         true,
		ExpireAt:        now + defaultDebugDurationSeconds,
		MaxItems:        defaultDebugMaxItems,
		PayloadMaxBytes: defaultDebugPayloadMaxBytes,
	}) {
		t.Fatalf("default debug config = %#v", cfg)
	}
	if ttl != defaultDebugDurationSeconds+debugTTLExtendSeconds {
		t.Fatalf("default ttl = %d, want %d", ttl, defaultDebugDurationSeconds+debugTTLExtendSeconds)
	}

	enabledFlag := false
	if _, _, enabled := normalizeDeviceDebugConfig(&model.SetDeviceDebugReq{Enabled: &enabledFlag}, now); enabled {
		t.Fatal("explicit disabled request should not produce an enabled config")
	}

	zeroDuration := int64(0)
	if _, _, enabled := normalizeDeviceDebugConfig(&model.SetDeviceDebugReq{Duration: &zeroDuration}, now); enabled {
		t.Fatal("zero duration should disable debug")
	}

	expireAt := now + 60
	maxItems := 12
	payloadMaxBytes := 0
	cfg, ttl, enabled = normalizeDeviceDebugConfig(&model.SetDeviceDebugReq{
		ExpireAt:        &expireAt,
		MaxItems:        &maxItems,
		PayloadMaxBytes: &payloadMaxBytes,
	}, now)
	if !enabled {
		t.Fatal("future expire_at should enable debug")
	}
	if cfg.ExpireAt != expireAt || cfg.MaxItems != maxItems || cfg.PayloadMaxBytes != payloadMaxBytes {
		t.Fatalf("explicit debug config was not preserved: %#v", cfg)
	}
	if ttl != 60+debugTTLExtendSeconds {
		t.Fatalf("explicit ttl = %d, want %d", ttl, 60+debugTTLExtendSeconds)
	}
}

func TestDeviceDebugCacheErrorPreservesCodeAndRawMessage(t *testing.T) {
	err := deviceDebugCacheError(errors.New("redis pipeline failed"))
	appErr, ok := err.(*errcode.Error)
	if !ok {
		t.Fatalf("deviceDebugCacheError type = %T, want *errcode.Error", err)
	}
	if appErr.Code != errcode.CodeCacheError {
		t.Fatalf("deviceDebugCacheError code = %d, want %d", appErr.Code, errcode.CodeCacheError)
	}
	data, ok := appErr.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("deviceDebugCacheError data type = %T, want map[string]interface{}", appErr.Data)
	}
	if got := data["cache_error"]; got != "redis pipeline failed" {
		t.Fatalf("deviceDebugCacheError data = %#v", data)
	}
}

func TestOperationLogEmptyPageUsesStableEmptyArray(t *testing.T) {
	var nilRows []model.GetOperationLogListByPageRsp
	responseList := normalizeOperationLogList(nilRows)
	rows, ok := responseList.([]model.GetOperationLogListByPageRsp)
	if !ok {
		t.Fatalf("normalized operation log list type = %T", responseList)
	}
	if rows == nil {
		t.Fatal("normalized operation log list must be a non-nil empty slice")
	}
	if len(rows) != 0 {
		t.Fatalf("normalized operation log list length = %d, want 0", len(rows))
	}
}
