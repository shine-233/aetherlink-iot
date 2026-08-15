package service

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

func TestBuildMarketDeviceConfigPayloadMapsPointersAndDropsInvalidJSON(t *testing.T) {
	dc := &model.DeviceConfig{
		Name:           "pump-config",
		DeviceType:     "2",
		ProtocolType:   pureHelperStringPtr("mqtt"),
		VoucherType:    pureHelperStringPtr("access_token"),
		ProtocolConfig: pureHelperStringPtr(`{"host":"broker.local","port":1883}`),
		DeviceConnType: pureHelperStringPtr("A"),
		OtherConfig:    pureHelperStringPtr(`{`),
		AdditionalInfo: pureHelperStringPtr(`{"source":"market","enabled":true}`),
		AutoRegister:   1,
	}

	got := buildMarketDeviceConfigPayload(dc)

	if got.Name != "pump-config" || got.DeviceType != "2" {
		t.Fatalf("device config identity = %#v, want mapped name/device type", got)
	}
	if got.ProtocolType != "mqtt" || got.VoucherType != "access_token" || got.DeviceConnType != "A" {
		t.Fatalf("device config string fields = %#v, want mapped protocol/voucher/conn type", got)
	}
	if got.ProtocolConfig["host"] != "broker.local" || got.ProtocolConfig["port"] != float64(1883) {
		t.Fatalf("protocol config = %#v, want parsed JSON object", got.ProtocolConfig)
	}
	if got.OtherConfig != nil {
		t.Fatalf("invalid other_config JSON should be dropped, got %#v", got.OtherConfig)
	}
	if got.AdditionalInfo["source"] != "market" || got.AdditionalInfo["enabled"] != true {
		t.Fatalf("additional info = %#v, want parsed JSON object", got.AdditionalInfo)
	}
	if got.AutoRegister != 1 {
		t.Fatalf("auto register = %d, want 1", got.AutoRegister)
	}
}

func TestBuildMarketDeviceConfigPayloadHandlesNilPointerFields(t *testing.T) {
	got := buildMarketDeviceConfigPayload(&model.DeviceConfig{
		Name:       "minimal-config",
		DeviceType: "1",
	})

	if got.ProtocolType != "" || got.VoucherType != "" || got.DeviceConnType != "" {
		t.Fatalf("nil string pointers should become empty strings, got %#v", got)
	}
	if got.ProtocolConfig != nil || got.OtherConfig != nil || got.AdditionalInfo != nil {
		t.Fatalf("missing JSON pointers should stay nil maps, got %#v", got)
	}
}

func TestMarketPublishFieldFallbacksPreferRequestThenTemplateThenDefaults(t *testing.T) {
	brand := "template-brand"
	modelNumber := "template-model"
	version := "1.0.0"
	author := "template-author"
	description := "template-description"
	tpl := &model.DeviceTemplate{
		Name:        "template-name",
		Brand:       &brand,
		ModelNumber: &modelNumber,
		Version:     &version,
		Author:      &author,
		Description: &description,
	}

	reqOverrides := model.PublishToMarketReq{
		MarketName:  "req-name",
		Brand:       "req-brand",
		Model:       "req-model",
		Category:    "req-category",
		Version:     "2.0.0",
		Author:      "req-author",
		Description: "req-description",
	}
	if got := marketPublishName(reqOverrides, tpl); got != "req-name" {
		t.Fatalf("marketPublishName override = %q, want req-name", got)
	}
	if got := marketPublishBrand(reqOverrides, tpl); got != "req-brand" {
		t.Fatalf("marketPublishBrand override = %q, want req-brand", got)
	}
	if got := marketPublishModel(reqOverrides, tpl); got != "req-model" {
		t.Fatalf("marketPublishModel override = %q, want req-model", got)
	}
	if got := marketPublishCategory(reqOverrides); got != "req-category" {
		t.Fatalf("marketPublishCategory override = %q, want req-category", got)
	}
	if got := marketPublishVersion(reqOverrides, tpl); got != "2.0.0" {
		t.Fatalf("marketPublishVersion override = %q, want 2.0.0", got)
	}
	if got := marketPublishAuthor(reqOverrides, tpl); got != "req-author" {
		t.Fatalf("marketPublishAuthor override = %q, want req-author", got)
	}
	if got := marketPublishDescription(reqOverrides, tpl); got != "req-description" {
		t.Fatalf("marketPublishDescription override = %q, want req-description", got)
	}

	emptyReq := model.PublishToMarketReq{}
	if got := marketPublishName(emptyReq, tpl); got != "template-name" {
		t.Fatalf("marketPublishName template fallback = %q, want template-name", got)
	}
	if got := marketPublishBrand(emptyReq, tpl); got != "template-brand" {
		t.Fatalf("marketPublishBrand template fallback = %q, want template-brand", got)
	}
	if got := marketPublishModel(emptyReq, tpl); got != "template-model" {
		t.Fatalf("marketPublishModel template fallback = %q, want template-model", got)
	}
	if got := marketPublishCategory(emptyReq); got != "default" {
		t.Fatalf("marketPublishCategory default = %q, want default", got)
	}
	if got := marketPublishVersion(emptyReq, tpl); got != "1.0.0" {
		t.Fatalf("marketPublishVersion template fallback = %q, want 1.0.0", got)
	}
	if got := marketPublishAuthor(emptyReq, tpl); got != "template-author" {
		t.Fatalf("marketPublishAuthor template fallback = %q, want template-author", got)
	}
	if got := marketPublishDescription(emptyReq, tpl); got != "template-description" {
		t.Fatalf("marketPublishDescription template fallback = %q, want template-description", got)
	}

	tplWithoutBrandOrModel := &model.DeviceTemplate{Name: "fallback-only"}
	if got := marketPublishBrand(emptyReq, tplWithoutBrandOrModel); got != "AetherLink" {
		t.Fatalf("marketPublishBrand default = %q, want AetherLink", got)
	}
	if got := marketPublishModel(emptyReq, tplWithoutBrandOrModel); got != "IoT-Device" {
		t.Fatalf("marketPublishModel default = %q, want IoT-Device", got)
	}
}

func TestExtractMarketUserIDReturnsJWTSubjectOrBlank(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"market-user-42"}`))
	token := header + "." + payload + "."

	if got := extractMarketUserID(token); got != "market-user-42" {
		t.Fatalf("extractMarketUserID subject = %q, want market-user-42", got)
	}
	if got := extractMarketUserID("not-a-token"); got != "" {
		t.Fatalf("extractMarketUserID malformed token = %q, want empty string", got)
	}
	if got := extractMarketUserID(header + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"email":"fixture@example.com"}`)) + "."); got != "" {
		t.Fatalf("extractMarketUserID token without sub = %q, want empty string", got)
	}
}

func TestValidateMarketPublishResponseClassifiesSuccessConflictAndGenericFailure(t *testing.T) {
	success := &model.MarketPublishApiResponse{Code: 0, Message: "ok", Data: map[string]interface{}{"id": "tpl-1"}}
	got, err := validateMarketPublishResponse(success)
	if err != nil {
		t.Fatalf("success response returned error: %v", err)
	}
	if got != success {
		t.Fatalf("success response pointer changed: got %#v want %#v", got, success)
	}

	conflict := &model.MarketPublishApiResponse{Code: 4009, Message: "duplicate version"}
	got, err = validateMarketPublishResponse(conflict)
	if got != conflict {
		t.Fatalf("conflict response pointer changed: got %#v want %#v", got, conflict)
	}
	assertErrcodeDataError(t, err, "validate market publish conflict", errcode.CodeSystemError, "Version conflict: this thing model version already exists in the market")

	failed := &model.MarketPublishApiResponse{Code: 5001, Message: "market rejected payload"}
	got, err = validateMarketPublishResponse(failed)
	if got != failed {
		t.Fatalf("failed response pointer changed: got %#v want %#v", got, failed)
	}
	assertErrcodeDataError(t, err, "validate market publish generic failure", errcode.CodeSystemError, "market rejected payload")
}

type stubMarketPublishClient struct {
	resp    *model.MarketPublishApiResponse
	err     error
	token   string
	userID  string
	request *model.PublishTemplateReq
}

func (s *stubMarketPublishClient) PublishTemplate(_ context.Context, token string, userID string, req *model.PublishTemplateReq) (*model.MarketPublishApiResponse, error) {
	s.token = token
	s.userID = userID
	s.request = req
	return s.resp, s.err
}

func TestLoadMarketPublishSourceClassifiesLookupFailuresAndSuccess(t *testing.T) {
	origGetConfig := marketPublishGetDeviceConfigByID
	origGetTemplate := marketPublishGetDeviceTemplateByID
	t.Cleanup(func() {
		marketPublishGetDeviceConfigByID = origGetConfig
		marketPublishGetDeviceTemplateByID = origGetTemplate
	})

	t.Run("device config lookup failure", func(t *testing.T) {
		marketPublishGetDeviceConfigByID = func(deviceConfigID, tenantID string) (*model.DeviceConfig, error) {
			if deviceConfigID != "dc-1" || tenantID != "tenant-a" {
				t.Fatalf("scope = (%q, %q), want dc-1/tenant-a", deviceConfigID, tenantID)
			}
			return nil, errors.New("config missing")
		}
		marketPublishGetDeviceTemplateByID = func(string, string) (*model.DeviceTemplate, error) {
			t.Fatal("thing model lookup should not run when device config lookup fails")
			return nil, nil
		}

		_, _, _, err := loadMarketPublishSource("dc-1", "tenant-a")
		assertErrcodeDataError(t, err, "loadMarketPublishSource config failure", errcode.CodeDBError, "failed to find device config: config missing")
	})

	t.Run("missing associated template id", func(t *testing.T) {
		marketPublishGetDeviceConfigByID = func(string, string) (*model.DeviceConfig, error) {
			return &model.DeviceConfig{ID: "dc-1", Name: "fixture-config", DeviceType: "1"}, nil
		}
		marketPublishGetDeviceTemplateByID = func(string, string) (*model.DeviceTemplate, error) {
			t.Fatal("thing model lookup should not run when template id is missing")
			return nil, nil
		}

		_, _, _, err := loadMarketPublishSource("dc-1", "tenant-a")
		assertErrcodeDataError(t, err, "loadMarketPublishSource missing thing model id", errcode.CodeParamError, "device config is not bound to a thing model")
	})

	t.Run("thing model lookup failure", func(t *testing.T) {
		templateID := "tpl-1"
		marketPublishGetDeviceConfigByID = func(string, string) (*model.DeviceConfig, error) {
			return &model.DeviceConfig{ID: "dc-1", Name: "fixture-config", DeviceType: "1", DeviceTemplateID: &templateID}, nil
		}
		marketPublishGetDeviceTemplateByID = func(id, tenantID string) (*model.DeviceTemplate, error) {
			if id != "tpl-1" || tenantID != "tenant-a" {
				t.Fatalf("scope = (%q, %q), want tpl-1/tenant-a", id, tenantID)
			}
			return nil, errors.New("template missing")
		}

		_, _, _, err := loadMarketPublishSource("dc-1", "tenant-a")
		assertErrcodeDataError(t, err, "loadMarketPublishSource thing model failure", errcode.CodeDBError, "failed to find thing model: template missing")
	})

	t.Run("success", func(t *testing.T) {
		templateID := "tpl-1"
		wantConfig := &model.DeviceConfig{ID: "dc-1", Name: "fixture-config", DeviceType: "1", DeviceTemplateID: &templateID}
		wantTemplate := &model.DeviceTemplate{ID: "tpl-1", Name: "template"}
		marketPublishGetDeviceConfigByID = func(string, string) (*model.DeviceConfig, error) { return wantConfig, nil }
		marketPublishGetDeviceTemplateByID = func(string, string) (*model.DeviceTemplate, error) { return wantTemplate, nil }

		gotConfig, gotTemplate, gotTemplateID, err := loadMarketPublishSource("dc-1", "tenant-a")
		if err != nil {
			t.Fatalf("loadMarketPublishSource() error = %v", err)
		}
		if gotConfig != wantConfig || gotTemplate != wantTemplate || gotTemplateID != "tpl-1" {
			t.Fatalf("loadMarketPublishSource() = (%#v, %#v, %q), want original config/template/tpl-1", gotConfig, gotTemplate, gotTemplateID)
		}
	})
}

func TestPublishToMarketRejectsMissingTenantClaimsBeforeDAL(t *testing.T) {
	origGetConfig := marketPublishGetDeviceConfigByID
	t.Cleanup(func() { marketPublishGetDeviceConfigByID = origGetConfig })

	marketPublishGetDeviceConfigByID = func(string, string) (*model.DeviceConfig, error) {
		t.Fatal("device config lookup must not run without a tenant claim")
		return nil, nil
	}

	for _, claims := range []*utils.UserClaims{nil, {TenantID: ""}} {
		_, err := (&DeviceTemplate{}).PublishToMarket(model.PublishToMarketReq{DeviceConfigID: "dc-1"}, claims)
		assertErrcodeDataError(t, err, "PublishToMarket missing tenant claims", errcode.CodeParamError, "Market publishing requires a non-empty tenant claim")
	}
}

func TestPublishToMarketBuildsPayloadFromConfigTemplateAndDeviceModel(t *testing.T) {
	origGetConfig := marketPublishGetDeviceConfigByID
	origGetTemplate := marketPublishGetDeviceTemplateByID
	origGetTelemetry := marketPublishGetTelemetryDataList
	origGetAttributes := marketPublishGetAttributeDataList
	origGetEvents := marketPublishGetEventDataList
	origGetCommands := marketPublishGetCommandDataList
	origGetProtocolVersion := marketPublishGetProtocolPluginVersion
	origNewClient := newMarketPublishClient
	t.Cleanup(func() {
		marketPublishGetDeviceConfigByID = origGetConfig
		marketPublishGetDeviceTemplateByID = origGetTemplate
		marketPublishGetTelemetryDataList = origGetTelemetry
		marketPublishGetAttributeDataList = origGetAttributes
		marketPublishGetEventDataList = origGetEvents
		marketPublishGetCommandDataList = origGetCommands
		marketPublishGetProtocolPluginVersion = origGetProtocolVersion
		newMarketPublishClient = origNewClient
	})

	templateID := "tpl-1"
	token := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`)) + "." +
		base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"market-user-99"}`)) + "."
	dc := &model.DeviceConfig{
		ID:               "dc-1",
		Name:             "pump-config",
		DeviceType:       "2",
		DeviceTemplateID: &templateID,
		ProtocolType:     pureHelperStringPtr("mqtt"),
		VoucherType:      pureHelperStringPtr("access_token"),
		ProtocolConfig:   pureHelperStringPtr(`{"host":"broker.local","port":1883}`),
		DeviceConnType:   pureHelperStringPtr("A"),
		AdditionalInfo:   pureHelperStringPtr(`{"source":"market"}`),
		AutoRegister:     1,
	}
	tpl := &model.DeviceTemplate{
		ID:             "tpl-1",
		Name:           "template-name",
		Brand:          pureHelperStringPtr("tpl-brand"),
		ModelNumber:    pureHelperStringPtr("tpl-model"),
		Version:        pureHelperStringPtr("1.0.0"),
		Author:         pureHelperStringPtr("tpl-author"),
		Description:    pureHelperStringPtr("tpl-description"),
		WebChartConfig: pureHelperStringPtr(`{"layout":"grid"}`),
		AppChartConfig: pureHelperStringPtr(`["mobile"]`),
	}
	stubClient := &stubMarketPublishClient{
		resp: &model.MarketPublishApiResponse{
			Code:    0,
			Message: "ok",
			Data:    map[string]interface{}{"id": "market-template-1"},
		},
	}

	marketPublishGetDeviceConfigByID = func(id, tenantID string) (*model.DeviceConfig, error) {
		if id != "dc-1" || tenantID != "tenant-a" {
			t.Fatalf("device config scope = (%q, %q), want dc-1/tenant-a", id, tenantID)
		}
		return dc, nil
	}
	marketPublishGetDeviceTemplateByID = func(id, tenantID string) (*model.DeviceTemplate, error) {
		if id != "tpl-1" || tenantID != "tenant-a" {
			t.Fatalf("template scope = (%q, %q), want tpl-1/tenant-a", id, tenantID)
		}
		return tpl, nil
	}
	marketPublishGetTelemetryDataList = func(id, tenantID string) ([]*model.DeviceModelTelemetry, error) {
		if id != "tpl-1" || tenantID != "tenant-a" {
			t.Fatalf("telemetry scope = (%q, %q), want tpl-1/tenant-a", id, tenantID)
		}
		return []*model.DeviceModelTelemetry{{DataIdentifier: "temperature"}}, nil
	}
	marketPublishGetAttributeDataList = func(string, string) ([]*model.DeviceModelAttribute, error) {
		return []*model.DeviceModelAttribute{{DataIdentifier: "serialNumber"}}, nil
	}
	marketPublishGetEventDataList = func(string, string) ([]*model.DeviceModelEvent, error) {
		return []*model.DeviceModelEvent{{DataIdentifier: "alarmRaised"}}, nil
	}
	marketPublishGetCommandDataList = func(string, string) ([]*model.DeviceModelCommand, error) {
		return []*model.DeviceModelCommand{{DataIdentifier: "restart"}}, nil
	}
	marketPublishGetProtocolPluginVersion = func(serviceIdentifier string) string {
		if serviceIdentifier != "mqtt" {
			t.Fatalf("serviceIdentifier = %q, want mqtt", serviceIdentifier)
		}
		return "2.3.4"
	}
	newMarketPublishClient = func() marketPublishClient {
		return stubClient
	}

	got, err := (&DeviceTemplate{}).PublishToMarket(model.PublishToMarketReq{
		DeviceConfigID: "dc-1",
		MarketToken:    token,
		MarketName:     "req-name",
		Category:       "industrial-sensor",
	}, &utils.UserClaims{TenantID: "tenant-a"})
	if err != nil {
		t.Fatalf("PublishToMarket() error = %v", err)
	}
	if got != stubClient.resp {
		t.Fatalf("PublishToMarket() response = %#v, want stub response %#v", got, stubClient.resp)
	}
	if stubClient.token != token || stubClient.userID != "market-user-99" {
		t.Fatalf("market client auth = (%q, %q), want token + extracted user id", stubClient.token, stubClient.userID)
	}
	if stubClient.request == nil {
		t.Fatal("PublishToMarket() should send a market publish request")
	}
	if stubClient.request.Name != "req-name" || stubClient.request.Brand != "tpl-brand" || stubClient.request.Model != "tpl-model" {
		t.Fatalf("publish identity fields = %#v, want request/template fallback mapping", stubClient.request)
	}
	if stubClient.request.Category != "industrial-sensor" || stubClient.request.Author != "tpl-author" || stubClient.request.Version != "1.0.0" {
		t.Fatalf("publish metadata fields = %#v, want category/author/version mapped", stubClient.request)
	}
	if stubClient.request.DeviceConfig == nil || stubClient.request.DeviceConfig.ProtocolType != "mqtt" || stubClient.request.DeviceConfig.AutoRegister != 1 {
		t.Fatalf("device config payload = %#v, want mapped device config", stubClient.request.DeviceConfig)
	}
	if stubClient.request.DeviceConfig.ProtocolConfig["host"] != "broker.local" {
		t.Fatalf("protocol config = %#v, want parsed broker host", stubClient.request.DeviceConfig.ProtocolConfig)
	}
	if len(stubClient.request.PluginDependencies) != 1 {
		t.Fatalf("plugin dependencies = %#v, want one protocol dependency", stubClient.request.PluginDependencies)
	}
	dependency := stubClient.request.PluginDependencies[0]
	if dependency.PluginName != "mqtt" || dependency.PluginType != "protocol" || dependency.MinVersion != "2.3.4" || !dependency.Required {
		t.Fatalf("plugin dependency = %#v, want mqtt protocol dependency with version", dependency)
	}

	templateDefinition := stubClient.request.TemplateDefinition
	if templateDefinition["web_chart_config"].(map[string]interface{})["layout"] != "grid" {
		t.Fatalf("web chart config = %#v, want parsed layout", templateDefinition["web_chart_config"])
	}
	if telemetry, ok := templateDefinition["telemetry"].([]*model.DeviceModelTelemetry); !ok || len(telemetry) != 1 || telemetry[0].DataIdentifier != "temperature" {
		t.Fatalf("telemetry definition = %#v, want collected telemetry", templateDefinition["telemetry"])
	}
	if attributes, ok := templateDefinition["attributes"].([]*model.DeviceModelAttribute); !ok || len(attributes) != 1 || attributes[0].DataIdentifier != "serialNumber" {
		t.Fatalf("attribute definition = %#v, want collected attributes", templateDefinition["attributes"])
	}
	if events, ok := templateDefinition["events"].([]*model.DeviceModelEvent); !ok || len(events) != 1 || events[0].DataIdentifier != "alarmRaised" {
		t.Fatalf("event definition = %#v, want collected events", templateDefinition["events"])
	}
	if commands, ok := templateDefinition["commands"].([]*model.DeviceModelCommand); !ok || len(commands) != 1 || commands[0].DataIdentifier != "restart" {
		t.Fatalf("command definition = %#v, want collected commands", templateDefinition["commands"])
	}
}

func TestPublishToMarketWrapsMarketClientFailure(t *testing.T) {
	origGetConfig := marketPublishGetDeviceConfigByID
	origGetTemplate := marketPublishGetDeviceTemplateByID
	origGetTelemetry := marketPublishGetTelemetryDataList
	origGetAttributes := marketPublishGetAttributeDataList
	origGetEvents := marketPublishGetEventDataList
	origGetCommands := marketPublishGetCommandDataList
	origNewClient := newMarketPublishClient
	t.Cleanup(func() {
		marketPublishGetDeviceConfigByID = origGetConfig
		marketPublishGetDeviceTemplateByID = origGetTemplate
		marketPublishGetTelemetryDataList = origGetTelemetry
		marketPublishGetAttributeDataList = origGetAttributes
		marketPublishGetEventDataList = origGetEvents
		marketPublishGetCommandDataList = origGetCommands
		newMarketPublishClient = origNewClient
	})

	templateID := "tpl-1"
	marketPublishGetDeviceConfigByID = func(string, string) (*model.DeviceConfig, error) {
		return &model.DeviceConfig{
			ID:               "dc-1",
			Name:             "pump-config",
			DeviceType:       "1",
			DeviceTemplateID: &templateID,
		}, nil
	}
	marketPublishGetDeviceTemplateByID = func(string, string) (*model.DeviceTemplate, error) {
		return &model.DeviceTemplate{Name: "template-name"}, nil
	}
	marketPublishGetTelemetryDataList = func(string, string) ([]*model.DeviceModelTelemetry, error) { return nil, nil }
	marketPublishGetAttributeDataList = func(string, string) ([]*model.DeviceModelAttribute, error) { return nil, nil }
	marketPublishGetEventDataList = func(string, string) ([]*model.DeviceModelEvent, error) { return nil, nil }
	marketPublishGetCommandDataList = func(string, string) ([]*model.DeviceModelCommand, error) { return nil, nil }
	newMarketPublishClient = func() marketPublishClient {
		return &stubMarketPublishClient{err: errors.New("market offline")}
	}

	_, err := (&DeviceTemplate{}).PublishToMarket(model.PublishToMarketReq{
		DeviceConfigID: "dc-1",
		MarketToken:    "not-a-token",
	}, &utils.UserClaims{TenantID: "tenant-a"})
	assertErrcodeDataError(t, err, "PublishToMarket market failure", errcode.CodeSystemError, "Market service unreachable or request failed: market offline")
}
