// 文件用途：验证市场物模型安装时的物模型、配置和模型转换规则。
// 核心逻辑：构造市场返回数据，断言物模型、设备配置、图表配置和模型列表的落库前转换。
// 关键注意事项：安装流程是多表事务入口，测试需防止字段默认值、租户归属和定义过滤发生回归。
// 重构建议：拆分安装计划构造和事务保存测试，补齐部分表写入失败和重复安装幂等边界。
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

type stubMarketInstallClient struct {
	fullData         *model.MarketTemplateFullData
	downloadErr      error
	extractUserID    string
	extractErr       error
	installErr       error
	downloadToken    string
	downloadID       string
	downloadVersion  string
	installToken     string
	installTemplate  string
	installVersionID string
	installUserID    string
	installOrgID     string
}

func (s *stubMarketInstallClient) DownloadTemplate(_ context.Context, token string, templateID string, version string) (*model.MarketTemplateFullData, error) {
	s.downloadToken = token
	s.downloadID = templateID
	s.downloadVersion = version
	return s.fullData, s.downloadErr
}

func (s *stubMarketInstallClient) ExtractUserIDFromMarketToken(string) (string, error) {
	return s.extractUserID, s.extractErr
}

func (s *stubMarketInstallClient) InstallTemplate(_ context.Context, token string, templateID string, versionID string, userID string, orgID string) error {
	s.installToken = token
	s.installTemplate = templateID
	s.installVersionID = versionID
	s.installUserID = userID
	s.installOrgID = orgID
	return s.installErr
}

func TestBuildInstalledDeviceConfigDefaultsWithoutMarketConfig(t *testing.T) {
	now := time.Date(2026, 7, 3, 1, 2, 3, 0, time.UTC)
	fullData := &model.MarketTemplateFullData{Name: "market template"}

	got := buildInstalledDeviceConfig("dc-1", "tpl-1", fullData, "tenant-1", now)

	if got.ID != "dc-1" || got.Name != "market template" || got.TenantID != "tenant-1" {
		t.Fatalf("device config identity = (%q, %q, %q), want dc/name/tenant", got.ID, got.Name, got.TenantID)
	}
	if got.DeviceTemplateID == nil || *got.DeviceTemplateID != "tpl-1" {
		t.Fatalf("thing model id = %v, want tpl-1", got.DeviceTemplateID)
	}
	if got.DeviceType != "1" {
		t.Fatalf("device type = %q, want default 1", got.DeviceType)
	}
	if got.ProtocolType != nil || got.ProtocolConfig != nil || got.OtherConfig != nil || got.AdditionalInfo != nil {
		t.Fatalf("empty market config should not populate protocol/config pointer fields")
	}
	if !got.CreatedAt.Equal(now) || !got.UpdatedAt.Equal(now) {
		t.Fatalf("timestamps = (%v, %v), want %v", got.CreatedAt, got.UpdatedAt, now)
	}
}

func TestBuildInstalledDeviceConfigUsesMarketPayload(t *testing.T) {
	fullData := &model.MarketTemplateFullData{
		Name: "template fallback",
		DeviceConfig: &model.DeviceConfigPayload{
			Name:           "market config",
			ProtocolType:   "mqtt",
			VoucherType:    "access_token",
			ProtocolConfig: map[string]interface{}{"host": "broker"},
			DeviceConnType: "A",
			OtherConfig:    map[string]interface{}{"qos": float64(1)},
			AdditionalInfo: map[string]interface{}{"source": "market"},
			AutoRegister:   1,
		},
	}

	got := buildInstalledDeviceConfig("dc-1", "tpl-1", fullData, "tenant-1", time.Time{})

	if got.Name != "market config" {
		t.Fatalf("name = %q, want market config", got.Name)
	}
	if got.DeviceType != "1" {
		t.Fatalf("blank market device type should default to 1, got %q", got.DeviceType)
	}
	assertStringPtrValue(t, got.ProtocolType, "mqtt", "protocol type")
	assertStringPtrValue(t, got.VoucherType, "access_token", "voucher type")
	assertStringPtrValue(t, got.DeviceConnType, "A", "device connection type")
	assertStringPtrValue(t, got.ProtocolConfig, `{"host":"broker"}`, "protocol config")
	assertStringPtrValue(t, got.OtherConfig, `{"qos":1}`, "other config")
	assertStringPtrValue(t, got.AdditionalInfo, `{"source":"market"}`, "additional info")
	if got.AutoRegister != 1 {
		t.Fatalf("auto register = %d, want 1", got.AutoRegister)
	}
}

func TestBuildInstalledDeviceTemplateExtractsChartConfigs(t *testing.T) {
	tplDef := model.TemplateDefinitionPayload{
		"web_chart_config": map[string]interface{}{"layout": "grid"},
		"app_chart_config": []interface{}{"mobile"},
	}
	fullData := &model.MarketTemplateFullData{
		Name:               "template",
		Brand:              "brand",
		ModelNumber:        "model",
		Author:             "author",
		Version:            "1.0.0",
		Description:        "description",
		TemplateDefinition: &tplDef,
	}

	got := buildInstalledDeviceTemplate("tpl-1", fullData, "tenant-1", time.Time{})

	if got.ID != "tpl-1" || got.Name != "template" || got.TenantID != "tenant-1" {
		t.Fatalf("template identity = (%q, %q, %q), want tpl/name/tenant", got.ID, got.Name, got.TenantID)
	}
	if got.Flag == nil || *got.Flag != 1 {
		t.Fatalf("flag = %v, want private flag 1", got.Flag)
	}
	assertStringPtrValue(t, got.Brand, "brand", "brand")
	assertStringPtrValue(t, got.ModelNumber, "model", "model number")
	assertStringPtrValue(t, got.Author, "author", "author")
	assertStringPtrValue(t, got.Version, "1.0.0", "version")
	assertStringPtrValue(t, got.Description, "description", "description")
	assertStringPtrValue(t, got.WebChartConfig, `{"layout":"grid"}`, "web chart config")
	assertStringPtrValue(t, got.AppChartConfig, `["mobile"]`, "app chart config")
}

func TestTemplateDefinitionListFiltersNonMapEntries(t *testing.T) {
	tplDef := model.TemplateDefinitionPayload{
		"telemetry": []interface{}{
			map[string]interface{}{"data_identifier": "temp"},
			"ignore-me",
			map[string]interface{}{"data_identifier": "humidity"},
		},
		"commands": "not-a-list",
	}

	got := templateDefinitionList(&tplDef, "telemetry")
	if len(got) != 2 {
		t.Fatalf("telemetry item count = %d, want 2", len(got))
	}
	if got[0]["data_identifier"] != "temp" || got[1]["data_identifier"] != "humidity" {
		t.Fatalf("telemetry identifiers = %#v, want temp/humidity", got)
	}
	if got := templateDefinitionList(&tplDef, "commands"); got != nil {
		t.Fatalf("non-list definition = %#v, want nil", got)
	}
	if got := templateDefinitionList(nil, "telemetry"); got != nil {
		t.Fatalf("nil definition = %#v, want nil", got)
	}
}

func TestBuildMarketTemplateInstallPlanUsesExistingTemplateAndStableClock(t *testing.T) {
	origFindExisting := marketInstallFindExistingTemplate
	origUUID := marketInstallUUID
	origNow := marketInstallNow
	t.Cleanup(func() {
		marketInstallFindExistingTemplate = origFindExisting
		marketInstallUUID = origUUID
		marketInstallNow = origNow
	})

	now := time.Date(2026, 7, 4, 2, 3, 4, 0, time.UTC)
	uuidValues := []string{"new-template-id", "device-config-id"}
	uuidIndex := 0
	marketInstallUUID = func() string {
		value := uuidValues[uuidIndex]
		uuidIndex++
		return value
	}
	marketInstallNow = func() time.Time { return now }
	marketInstallFindExistingTemplate = func(name string, tenantID string) (*model.DeviceTemplate, error) {
		if name != "market-template" || tenantID != "tenant-1" {
			t.Fatalf("lookup = (%q, %q), want market-template/tenant-1", name, tenantID)
		}
		return &model.DeviceTemplate{ID: "existing-template-id", Name: "market-template"}, nil
	}

	fullData := &model.MarketTemplateFullData{
		Name:      "market-template",
		Brand:     "brand",
		Version:   "1.0.0",
		Author:    "author",
		VersionID: "ver-1",
	}
	claims := &utils.UserClaims{TenantID: "tenant-1"}

	plan, err := buildMarketTemplateInstallPlan(fullData, claims)
	if err != nil {
		t.Fatalf("buildMarketTemplateInstallPlan() error = %v", err)
	}
	if !plan.isUpdate {
		t.Fatal("plan.isUpdate = false, want true for existing template")
	}
	if plan.templateID != "existing-template-id" {
		t.Fatalf("templateID = %q, want existing-template-id", plan.templateID)
	}
	if plan.deviceCfgID != "device-config-id" {
		t.Fatalf("deviceCfgID = %q, want device-config-id", plan.deviceCfgID)
	}
	if !plan.now.Equal(now) {
		t.Fatalf("plan.now = %v, want %v", plan.now, now)
	}
	if plan.template == nil || plan.template.ID != "existing-template-id" || plan.template.TenantID != "tenant-1" {
		t.Fatalf("template plan = %#v, want template for existing id and tenant", plan.template)
	}
	if plan.deviceCfg == nil || plan.deviceCfg.ID != "device-config-id" || plan.deviceCfg.TenantID != "tenant-1" {
		t.Fatalf("device config plan = %#v, want generated config for tenant", plan.deviceCfg)
	}
}

func TestCheckMissingPluginsUsesInjectedLookup(t *testing.T) {
	origGetServicePlugin := marketInstallGetServicePlugin
	t.Cleanup(func() {
		marketInstallGetServicePlugin = origGetServicePlugin
	})

	marketInstallGetServicePlugin = func(serviceIdentifier string) (*model.ServicePlugin, error) {
		if serviceIdentifier == "mqtt" {
			return &model.ServicePlugin{ServiceIdentifier: "mqtt"}, nil
		}
		if serviceIdentifier == "opcua" {
			return nil, errors.New("not installed")
		}
		t.Fatalf("unexpected plugin lookup: %q", serviceIdentifier)
		return nil, nil
	}

	missing := checkMissingPlugins([]model.PluginDependency{
		{PluginName: "mqtt"},
		{PluginName: "opcua"},
	})
	if len(missing) != 1 || missing[0].PluginName != "opcua" {
		t.Fatalf("missing plugins = %#v, want only opcua", missing)
	}
}

func TestNotifyMarketTemplateInstalledFallsBackToClaimsUserAndRunsInstall(t *testing.T) {
	origRunAsync := marketInstallRunAsync
	t.Cleanup(func() {
		marketInstallRunAsync = origRunAsync
	})

	marketInstallRunAsync = func(fn func()) { fn() }
	client := &stubMarketInstallClient{
		extractErr: errors.New("bad token"),
	}
	req := model.InstallFromMarketReq{
		MarketTemplateID: "market-template-1",
		MarketToken:      "market-token",
	}
	fullData := &model.MarketTemplateFullData{VersionID: "ver-9"}
	claims := &utils.UserClaims{ID: "fallback-user", TenantID: "tenant-1"}

	notifyMarketTemplateInstalled(client, req, fullData, claims)

	if client.installToken != "market-token" || client.installTemplate != "market-template-1" || client.installVersionID != "ver-9" {
		t.Fatalf("install notification args = (%q, %q, %q), want token/template/version", client.installToken, client.installTemplate, client.installVersionID)
	}
	if client.installUserID != "fallback-user" || client.installOrgID != "tenant-1" {
		t.Fatalf("install notification user/org = (%q, %q), want fallback-user/tenant-1", client.installUserID, client.installOrgID)
	}
}

func TestInstallFromMarketClassifiesDownloadFailure(t *testing.T) {
	origClient := newMarketInstallClient
	t.Cleanup(func() {
		newMarketInstallClient = origClient
	})

	newMarketInstallClient = func() marketInstallClient {
		return &stubMarketInstallClient{downloadErr: errors.New("market unavailable")}
	}

	_, err := (&DeviceTemplate{}).InstallFromMarket(model.InstallFromMarketReq{
		MarketTemplateID: "market-template-1",
		MarketToken:      "market-token",
		Version:          "1.0.0",
	}, &utils.UserClaims{ID: "user-1", TenantID: "tenant-1"})
	assertErrcodeDataError(t, err, "InstallFromMarket download failure", errcode.CodeSystemError, "Failed to download thing model from market: market unavailable")
}

func TestInstallFromMarketClassifiesBeginTransactionFailure(t *testing.T) {
	origClient := newMarketInstallClient
	origBeginTx := marketInstallBeginTx
	t.Cleanup(func() {
		newMarketInstallClient = origClient
		marketInstallBeginTx = origBeginTx
	})

	newMarketInstallClient = func() marketInstallClient {
		return &stubMarketInstallClient{fullData: &model.MarketTemplateFullData{Name: "market-template"}}
	}
	marketInstallBeginTx = func() *gorm.DB {
		return &gorm.DB{Error: errors.New("db offline")}
	}

	_, err := (&DeviceTemplate{}).InstallFromMarket(model.InstallFromMarketReq{
		MarketTemplateID: "market-template-1",
		MarketToken:      "market-token",
	}, &utils.UserClaims{ID: "user-1", TenantID: "tenant-1"})
	assertErrcodeDataError(t, err, "InstallFromMarket begin tx failure", errcode.CodeDBError, "Failed to begin transaction: db offline")
}

func TestInstallFromMarketBuildsPlanPersistsAndReturnsInstalledRecords(t *testing.T) {
	origClient := newMarketInstallClient
	origCheckMissing := marketInstallCheckMissingPlugins
	origBeginTx := marketInstallBeginTx
	origFindExisting := marketInstallFindExistingTemplate
	origUUID := marketInstallUUID
	origNow := marketInstallNow
	origSaveTemplate := marketInstallSaveTemplate
	origDeviceModels := marketInstallDeviceModels
	origCreateDeviceConfig := marketInstallCreateDeviceConfig
	origCommitTx := marketInstallCommitTx
	origRollbackTx := marketInstallRollbackTx
	origGetTemplate := marketInstallGetDeviceTemplate
	origGetConfig := marketInstallGetDeviceConfig
	origNotify := marketInstallNotifyInstalled
	t.Cleanup(func() {
		newMarketInstallClient = origClient
		marketInstallCheckMissingPlugins = origCheckMissing
		marketInstallBeginTx = origBeginTx
		marketInstallFindExistingTemplate = origFindExisting
		marketInstallUUID = origUUID
		marketInstallNow = origNow
		marketInstallSaveTemplate = origSaveTemplate
		marketInstallDeviceModels = origDeviceModels
		marketInstallCreateDeviceConfig = origCreateDeviceConfig
		marketInstallCommitTx = origCommitTx
		marketInstallRollbackTx = origRollbackTx
		marketInstallGetDeviceTemplate = origGetTemplate
		marketInstallGetDeviceConfig = origGetConfig
		marketInstallNotifyInstalled = origNotify
	})

	installClient := &stubMarketInstallClient{
		fullData: &model.MarketTemplateFullData{
			Name:      "market-template",
			VersionID: "ver-1",
			DeviceConfig: &model.DeviceConfigPayload{
				Name:         "cfg-name",
				ProtocolType: "mqtt",
			},
			TemplateDefinition: &model.TemplateDefinitionPayload{
				"telemetry": []interface{}{map[string]interface{}{"data_identifier": "temperature"}},
			},
		},
	}
	newMarketInstallClient = func() marketInstallClient { return installClient }
	marketInstallCheckMissingPlugins = func(deps []model.PluginDependency) []model.PluginDependency {
		if len(deps) != 0 {
			t.Fatalf("plugin deps = %#v, want empty from fixture", deps)
		}
		return []model.PluginDependency{{PluginName: "opcua"}}
	}
	marketInstallBeginTx = func() *gorm.DB { return &gorm.DB{} }
	marketInstallFindExistingTemplate = func(string, string) (*model.DeviceTemplate, error) { return nil, nil }
	uuidValues := []string{"template-new", "config-new"}
	uuidIndex := 0
	marketInstallUUID = func() string {
		value := uuidValues[uuidIndex]
		uuidIndex++
		return value
	}
	now := time.Date(2026, 7, 4, 3, 4, 5, 0, time.UTC)
	marketInstallNow = func() time.Time { return now }
	saved := false
	installedModels := false
	createdConfig := false
	committed := false
	notified := false
	rolledBack := false
	marketInstallSaveTemplate = func(_ *gorm.DB, plan *marketTemplateInstallPlan) error {
		saved = true
		if plan.templateID != "template-new" || plan.deviceCfgID != "config-new" {
			t.Fatalf("save plan ids = (%q, %q), want generated ids", plan.templateID, plan.deviceCfgID)
		}
		return nil
	}
	marketInstallDeviceModels = func(_ *gorm.DB, plan *marketTemplateInstallPlan) error {
		installedModels = true
		if plan.tplDef == nil {
			t.Fatal("tplDef = nil, want template definition for model install")
		}
		return nil
	}
	marketInstallCreateDeviceConfig = func(_ *gorm.DB, dc *model.DeviceConfig) error {
		createdConfig = true
		if dc.ID != "config-new" || dc.Name != "cfg-name" || dc.DeviceTemplateID == nil || *dc.DeviceTemplateID != "template-new" {
			t.Fatalf("created device config = %#v, want generated linked config", dc)
		}
		return nil
	}
	marketInstallCommitTx = func(*gorm.DB) error {
		committed = true
		return nil
	}
	marketInstallRollbackTx = func(*gorm.DB) { rolledBack = true }
	marketInstallGetDeviceTemplate = func(id string) (*model.DeviceTemplate, error) {
		return &model.DeviceTemplate{ID: id, Name: "persisted-template"}, nil
	}
	marketInstallGetDeviceConfig = func(id string) (*model.DeviceConfig, error) {
		return &model.DeviceConfig{ID: id, Name: "persisted-config"}, nil
	}
	marketInstallNotifyInstalled = func(client marketInstallClient, req model.InstallFromMarketReq, fullData *model.MarketTemplateFullData, claims *utils.UserClaims) {
		notified = true
		if client != installClient {
			t.Fatal("notify received unexpected client instance")
		}
		if req.MarketTemplateID != "market-template-1" || fullData.Name != "market-template" || claims.TenantID != "tenant-1" {
			t.Fatalf("notify args mismatch: req=%#v fullData=%#v claims=%#v", req, fullData, claims)
		}
	}

	got, err := (&DeviceTemplate{}).InstallFromMarket(model.InstallFromMarketReq{
		MarketTemplateID: "market-template-1",
		MarketToken:      "market-token",
	}, &utils.UserClaims{ID: "user-1", TenantID: "tenant-1"})
	if err != nil {
		t.Fatalf("InstallFromMarket() error = %v", err)
	}
	if !saved || !installedModels || !createdConfig || !committed || !notified {
		t.Fatalf("execution flags = saved:%v models:%v create:%v commit:%v notify:%v, want all true", saved, installedModels, createdConfig, committed, notified)
	}
	if rolledBack {
		t.Fatal("rollback should not run on success")
	}
	if got == nil || got.DeviceTemplate == nil || got.DeviceConfig == nil {
		t.Fatalf("response = %#v, want installed template and device config", got)
	}
	if got.DeviceTemplate.ID != "template-new" || got.DeviceConfig.ID != "config-new" {
		t.Fatalf("response ids = (%q, %q), want template-new/config-new", got.DeviceTemplate.ID, got.DeviceConfig.ID)
	}
	if len(got.MissingPlugins) != 1 || got.MissingPlugins[0].PluginName != "opcua" {
		t.Fatalf("missing plugins = %#v, want opcua", got.MissingPlugins)
	}
}

func TestInstallFromMarketRollsBackWhenSaveTemplateFails(t *testing.T) {
	origClient := newMarketInstallClient
	origCheckMissing := marketInstallCheckMissingPlugins
	origBeginTx := marketInstallBeginTx
	origFindExisting := marketInstallFindExistingTemplate
	origUUID := marketInstallUUID
	origNow := marketInstallNow
	origSaveTemplate := marketInstallSaveTemplate
	origRollbackTx := marketInstallRollbackTx
	t.Cleanup(func() {
		newMarketInstallClient = origClient
		marketInstallCheckMissingPlugins = origCheckMissing
		marketInstallBeginTx = origBeginTx
		marketInstallFindExistingTemplate = origFindExisting
		marketInstallUUID = origUUID
		marketInstallNow = origNow
		marketInstallSaveTemplate = origSaveTemplate
		marketInstallRollbackTx = origRollbackTx
	})

	newMarketInstallClient = func() marketInstallClient {
		return &stubMarketInstallClient{fullData: &model.MarketTemplateFullData{Name: "market-template"}}
	}
	marketInstallCheckMissingPlugins = func([]model.PluginDependency) []model.PluginDependency { return nil }
	marketInstallBeginTx = func() *gorm.DB { return &gorm.DB{} }
	marketInstallFindExistingTemplate = func(string, string) (*model.DeviceTemplate, error) { return nil, nil }
	marketInstallUUID = func() string { return "generated-id" }
	marketInstallNow = func() time.Time { return time.Date(2026, 7, 4, 5, 0, 0, 0, time.UTC) }
	rolledBack := false
	marketInstallSaveTemplate = func(*gorm.DB, *marketTemplateInstallPlan) error {
		return marketInstallDBError("Failed to create template: ", errors.New("insert failed"))
	}
	marketInstallRollbackTx = func(*gorm.DB) { rolledBack = true }

	_, err := (&DeviceTemplate{}).InstallFromMarket(model.InstallFromMarketReq{
		MarketTemplateID: "market-template-1",
		MarketToken:      "market-token",
	}, &utils.UserClaims{ID: "user-1", TenantID: "tenant-1"})
	assertErrcodeDataError(t, err, "InstallFromMarket save template failure", errcode.CodeDBError, "Failed to create template: insert failed")
	if !rolledBack {
		t.Fatal("rollback should run when saveInstalledTemplate fails")
	}
}

func TestInstallFromMarketRollsBackWhenCreateDeviceConfigFails(t *testing.T) {
	origClient := newMarketInstallClient
	origCheckMissing := marketInstallCheckMissingPlugins
	origBeginTx := marketInstallBeginTx
	origFindExisting := marketInstallFindExistingTemplate
	origUUID := marketInstallUUID
	origNow := marketInstallNow
	origSaveTemplate := marketInstallSaveTemplate
	origDeviceModels := marketInstallDeviceModels
	origCreateDeviceConfig := marketInstallCreateDeviceConfig
	origRollbackTx := marketInstallRollbackTx
	t.Cleanup(func() {
		newMarketInstallClient = origClient
		marketInstallCheckMissingPlugins = origCheckMissing
		marketInstallBeginTx = origBeginTx
		marketInstallFindExistingTemplate = origFindExisting
		marketInstallUUID = origUUID
		marketInstallNow = origNow
		marketInstallSaveTemplate = origSaveTemplate
		marketInstallDeviceModels = origDeviceModels
		marketInstallCreateDeviceConfig = origCreateDeviceConfig
		marketInstallRollbackTx = origRollbackTx
	})

	newMarketInstallClient = func() marketInstallClient {
		return &stubMarketInstallClient{
			fullData: &model.MarketTemplateFullData{
				Name:         "market-template",
				DeviceConfig: &model.DeviceConfigPayload{Name: "cfg-name"},
			},
		}
	}
	marketInstallCheckMissingPlugins = func([]model.PluginDependency) []model.PluginDependency { return nil }
	marketInstallBeginTx = func() *gorm.DB { return &gorm.DB{} }
	marketInstallFindExistingTemplate = func(string, string) (*model.DeviceTemplate, error) { return nil, nil }
	uuidValues := []string{"template-new", "config-new"}
	uuidIndex := 0
	marketInstallUUID = func() string {
		value := uuidValues[uuidIndex]
		uuidIndex++
		return value
	}
	marketInstallNow = func() time.Time { return time.Date(2026, 7, 4, 6, 0, 0, 0, time.UTC) }
	marketInstallSaveTemplate = func(*gorm.DB, *marketTemplateInstallPlan) error { return nil }
	marketInstallDeviceModels = func(*gorm.DB, *marketTemplateInstallPlan) error { return nil }
	rolledBack := false
	marketInstallCreateDeviceConfig = func(*gorm.DB, *model.DeviceConfig) error {
		return errors.New("device config insert failed")
	}
	marketInstallRollbackTx = func(*gorm.DB) { rolledBack = true }

	_, err := (&DeviceTemplate{}).InstallFromMarket(model.InstallFromMarketReq{
		MarketTemplateID: "market-template-1",
		MarketToken:      "market-token",
	}, &utils.UserClaims{ID: "user-1", TenantID: "tenant-1"})
	assertErrcodeDataError(t, err, "InstallFromMarket create device config failure", errcode.CodeDBError, "Failed to create device config: device config insert failed")
	if !rolledBack {
		t.Fatal("rollback should run when device config creation fails")
	}
}

func assertStringPtrValue(t *testing.T, got *string, want string, field string) {
	t.Helper()
	if got == nil {
		t.Fatalf("%s = nil, want %q", field, want)
	}
	if *got != want {
		t.Fatalf("%s = %q, want %q", field, *got, want)
	}
}
