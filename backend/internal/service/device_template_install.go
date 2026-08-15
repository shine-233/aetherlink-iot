// 文件用途：维护市场物模型安装和多表落库流程。
// 核心逻辑：把市场物模型包转换为本地物模型、配置、遥测、属性、事件和命令模型，并按事务保存。
// 关键注意事项：安装流程涉及多表写入，任何阶段失败都必须回滚并避免半安装状态。
// 重构建议：将安装计划构造和事务执行分离，补齐重复安装、部分失败、权限和回滚测试。
// device_template_install.go installs thing models from marketplace data.
//
// It imports template definitions, telemetry/attribute/command/event metadata,
// and related records in transactional stages. Rollback and idempotency behavior
// should be documented before refactoring install phases.
package service

import (
	"context"
	"encoding/json"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type marketInstallClient interface {
	DownloadTemplate(ctx context.Context, token string, templateID string, version string) (*model.MarketTemplateFullData, error)
	ExtractUserIDFromMarketToken(token string) (string, error)
	InstallTemplate(ctx context.Context, token string, templateID string, versionID string, userID string, orgID string) error
}

var (
	newMarketInstallClient = func() marketInstallClient {
		return NewMarketClient()
	}
	marketInstallBeginTx = func() *gorm.DB {
		return global.DB.Begin()
	}
	marketInstallFindExistingTemplate = func(name string, tenantID string) (*model.DeviceTemplate, error) {
		return query.DeviceTemplate.WithContext(context.Background()).
			Where(query.DeviceTemplate.Name.Eq(name), query.DeviceTemplate.TenantID.Eq(tenantID)).
			First()
	}
	marketInstallUUID                = uuid.New
	marketInstallNow                 = func() time.Time { return time.Now().UTC() }
	marketInstallCheckMissingPlugins = checkMissingPlugins
	marketInstallSaveTemplate        = saveInstalledTemplate
	marketInstallDeviceModels        = installTemplateDeviceModels
	marketInstallCreateDeviceConfig  = func(tx *gorm.DB, dc *model.DeviceConfig) error { return tx.Create(dc).Error }
	marketInstallCommitTx            = func(tx *gorm.DB) error { return tx.Commit().Error }
	marketInstallRollbackTx          = func(tx *gorm.DB) { tx.Rollback() }
	marketInstallGetDeviceTemplate   = dal.GetDeviceTemplateById
	marketInstallGetDeviceConfig     = dal.GetDeviceConfigByID
	marketInstallNotifyInstalled     = notifyMarketTemplateInstalled
	marketInstallRunAsync            = func(fn func()) { go fn() }
	marketInstallGetServicePlugin    = dal.GetServicePluginByServiceIdentifier
)

type marketTemplateInstallPlan struct {
	templateID  string
	deviceCfgID string
	template    *model.DeviceTemplate
	deviceCfg   *model.DeviceConfig
	tplDef      *model.TemplateDefinitionPayload
	now         time.Time
	isUpdate    bool
}

// InstallFromMarket downloads a thing model from the market and creates it locally:
// 1. DeviceTemplate (thing model + dashboard config)
// 2. DeviceConfig (credential/protocol config referencing the DeviceTemplate)
func (*DeviceTemplate) InstallFromMarket(req model.InstallFromMarketReq, claims *utils.UserClaims) (*model.InstallFromMarketRsp, error) {
	client := newMarketInstallClient()
	fullData, err := client.DownloadTemplate(context.Background(), req.MarketToken, req.MarketTemplateID, req.Version)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": "Failed to download thing model from market: " + err.Error(),
		})
	}

	missingPlugins := marketInstallCheckMissingPlugins(fullData.PluginDependencies)

	tx := marketInstallBeginTx()
	if tx.Error != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": "Failed to begin transaction: " + tx.Error.Error(),
		})
	}

	defer func() {
		if r := recover(); r != nil {
			marketInstallRollbackTx(tx)
		}
	}()

	plan, err := buildMarketTemplateInstallPlan(fullData, claims)
	if err != nil {
		marketInstallRollbackTx(tx)
		return nil, marketInstallDBError("Failed to check existing template: ", err)
	}
	if err := marketInstallSaveTemplate(tx, plan); err != nil {
		marketInstallRollbackTx(tx)
		return nil, err
	}
	if err := marketInstallDeviceModels(tx, plan); err != nil {
		marketInstallRollbackTx(tx)
		return nil, err
	}
	if err := marketInstallCreateDeviceConfig(tx, plan.deviceCfg); err != nil {
		marketInstallRollbackTx(tx)
		return nil, marketInstallDBError("Failed to create device config: ", err)
	}

	if err := marketInstallCommitTx(tx); err != nil {
		return nil, marketInstallDBError("Failed to commit transaction: ", err)
	}

	marketInstallNotifyInstalled(client, req, fullData, claims)

	createdTpl, _ := marketInstallGetDeviceTemplate(plan.templateID)
	createdDC, _ := marketInstallGetDeviceConfig(plan.deviceCfgID)

	return &model.InstallFromMarketRsp{
		DeviceTemplate: createdTpl,
		DeviceConfig:   createdDC,
		MissingPlugins: missingPlugins,
	}, nil
}

func buildMarketTemplateInstallPlan(fullData *model.MarketTemplateFullData, claims *utils.UserClaims) (*marketTemplateInstallPlan, error) {
	existingTpl, err := marketInstallFindExistingTemplate(fullData.Name, claims.TenantID)
	if err != nil && err.Error() != "record not found" {
		return nil, err
	}

	templateID := marketInstallUUID()
	isUpdate := false
	if existingTpl != nil {
		templateID = existingTpl.ID
		isUpdate = true
	}

	now := marketInstallNow()
	deviceCfgID := marketInstallUUID()
	return &marketTemplateInstallPlan{
		templateID:  templateID,
		deviceCfgID: deviceCfgID,
		template:    buildInstalledDeviceTemplate(templateID, fullData, claims.TenantID, now),
		deviceCfg:   buildInstalledDeviceConfig(deviceCfgID, templateID, fullData, claims.TenantID, now),
		tplDef:      fullData.TemplateDefinition,
		now:         now,
		isUpdate:    isUpdate,
	}, nil
}

func buildInstalledDeviceTemplate(templateID string, fullData *model.MarketTemplateFullData, tenantID string, now time.Time) *model.DeviceTemplate {
	flag := int16(1)
	webChartConfig, appChartConfig := extractTemplateChartConfigs(fullData.TemplateDefinition)

	return &model.DeviceTemplate{
		ID:             templateID,
		Name:           fullData.Name,
		TenantID:       tenantID,
		Brand:          ptrStrP(fullData.Brand),
		ModelNumber:    ptrStrP(fullData.ModelNumber),
		Author:         ptrStrP(fullData.Author),
		Version:        ptrStrP(fullData.Version),
		Description:    ptrStrP(fullData.Description),
		Flag:           &flag,
		WebChartConfig: ptrStrP(webChartConfig),
		AppChartConfig: ptrStrP(appChartConfig),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func extractTemplateChartConfigs(tplDef *model.TemplateDefinitionPayload) (string, string) {
	if tplDef == nil {
		return "", ""
	}
	return marshalTemplateDefinitionField(tplDef, "web_chart_config"), marshalTemplateDefinitionField(tplDef, "app_chart_config")
}

func marshalTemplateDefinitionField(tplDef *model.TemplateDefinitionPayload, key string) string {
	v, ok := (*tplDef)[key]
	if !ok || v == nil {
		return ""
	}
	bytes, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(bytes)
}

func buildInstalledDeviceConfig(dcID string, templateID string, fullData *model.MarketTemplateFullData, tenantID string, now time.Time) *model.DeviceConfig {
	dcName := fullData.Name
	if fullData.DeviceConfig != nil && fullData.DeviceConfig.Name != "" {
		dcName = fullData.DeviceConfig.Name
	}

	newDC := &model.DeviceConfig{
		ID:               dcID,
		Name:             dcName,
		DeviceTemplateID: &templateID,
		DeviceType:       "1",
		TenantID:         tenantID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if fullData.DeviceConfig == nil {
		return newDC
	}

	newDC.DeviceType = fullData.DeviceConfig.DeviceType
	if fullData.DeviceConfig.DeviceType == "" {
		newDC.DeviceType = "1"
	}
	newDC.ProtocolType = ptrStrP(fullData.DeviceConfig.ProtocolType)
	newDC.VoucherType = ptrStrP(fullData.DeviceConfig.VoucherType)
	newDC.ProtocolConfig = ptrJSONString(fullData.DeviceConfig.ProtocolConfig)
	newDC.DeviceConnType = ptrStrP(fullData.DeviceConfig.DeviceConnType)
	newDC.OtherConfig = ptrJSONString(fullData.DeviceConfig.OtherConfig)
	newDC.AdditionalInfo = ptrJSONString(fullData.DeviceConfig.AdditionalInfo)
	newDC.AutoRegister = fullData.DeviceConfig.AutoRegister
	return newDC
}

func ptrJSONString(value map[string]interface{}) *string {
	if value == nil {
		return nil
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	s := string(bytes)
	return &s
}

func saveInstalledTemplate(tx *gorm.DB, plan *marketTemplateInstallPlan) error {
	if !plan.isUpdate {
		if err := tx.Create(plan.template).Error; err != nil {
			return marketInstallDBError("Failed to create template: ", err)
		}
		return nil
	}

	tx.Where("device_template_id = ?", plan.templateID).Delete(&model.DeviceModelTelemetry{})
	tx.Where("device_template_id = ?", plan.templateID).Delete(&model.DeviceModelAttribute{})
	tx.Where("device_template_id = ?", plan.templateID).Delete(&model.DeviceModelEvent{})
	tx.Where("device_template_id = ?", plan.templateID).Delete(&model.DeviceModelCommand{})

	if err := tx.Save(plan.template).Error; err != nil {
		return marketInstallDBError("Failed to update template: ", err)
	}
	return nil
}

func installTemplateDeviceModels(tx *gorm.DB, plan *marketTemplateInstallPlan) error {
	if plan.tplDef == nil {
		return nil
	}
	if err := installTelemetryModels(tx, plan); err != nil {
		return err
	}
	if err := installAttributeModels(tx, plan); err != nil {
		return err
	}
	if err := installEventModels(tx, plan); err != nil {
		return err
	}
	return installCommandModels(tx, plan)
}

func installTelemetryModels(tx *gorm.DB, plan *marketTemplateInstallPlan) error {
	for _, m := range templateDefinitionList(plan.tplDef, "telemetry") {
		created := model.DeviceModelTelemetry{
			ID:               uuid.New(),
			DeviceTemplateID: plan.templateID,
			TenantID:         plan.template.TenantID,
			DataName:         getStrP(m, "data_name"),
			DataIdentifier:   getStr(m, "data_identifier"),
			ReadWriteFlag:    getStrP(m, "read_write_flag"),
			DataType:         getStrP(m, "data_type"),
			Unit:             getStrP(m, "unit"),
			Description:      getStrP(m, "description"),
			AdditionalInfo:   getStrP(m, "additional_info"),
			CreatedAt:        plan.now,
			UpdatedAt:        plan.now,
		}
		if err := tx.Create(&created).Error; err != nil {
			return marketInstallDBError("Failed to create telemetry: ", err)
		}
	}
	return nil
}

func installAttributeModels(tx *gorm.DB, plan *marketTemplateInstallPlan) error {
	for _, m := range templateDefinitionList(plan.tplDef, "attributes") {
		created := model.DeviceModelAttribute{
			ID:               uuid.New(),
			DeviceTemplateID: plan.templateID,
			TenantID:         plan.template.TenantID,
			DataName:         getStrP(m, "data_name"),
			DataIdentifier:   getStr(m, "data_identifier"),
			ReadWriteFlag:    getStrP(m, "read_write_flag"),
			DataType:         getStrP(m, "data_type"),
			Unit:             getStrP(m, "unit"),
			Description:      getStrP(m, "description"),
			AdditionalInfo:   getStrP(m, "additional_info"),
			CreatedAt:        plan.now,
			UpdatedAt:        plan.now,
		}
		if err := tx.Create(&created).Error; err != nil {
			return marketInstallDBError("Failed to create attribute: ", err)
		}
	}
	return nil
}

func installEventModels(tx *gorm.DB, plan *marketTemplateInstallPlan) error {
	for _, m := range templateDefinitionList(plan.tplDef, "events") {
		created := model.DeviceModelEvent{
			ID:               uuid.New(),
			DeviceTemplateID: plan.templateID,
			TenantID:         plan.template.TenantID,
			DataName:         getStrP(m, "data_name"),
			DataIdentifier:   getStr(m, "data_identifier"),
			Param:            getStrP(m, "params"),
			Description:      getStrP(m, "description"),
			AdditionalInfo:   getStrP(m, "additional_info"),
			CreatedAt:        plan.now,
			UpdatedAt:        plan.now,
		}
		if err := tx.Create(&created).Error; err != nil {
			return marketInstallDBError("Failed to create event: ", err)
		}
	}
	return nil
}

func installCommandModels(tx *gorm.DB, plan *marketTemplateInstallPlan) error {
	for _, m := range templateDefinitionList(plan.tplDef, "commands") {
		created := model.DeviceModelCommand{
			ID:               uuid.New(),
			DeviceTemplateID: plan.templateID,
			TenantID:         plan.template.TenantID,
			DataName:         getStrP(m, "data_name"),
			DataIdentifier:   getStr(m, "data_identifier"),
			Param:            getStrP(m, "params"),
			Description:      getStrP(m, "description"),
			AdditionalInfo:   getStrP(m, "additional_info"),
			CreatedAt:        plan.now,
			UpdatedAt:        plan.now,
		}
		if err := tx.Create(&created).Error; err != nil {
			return marketInstallDBError("Failed to create command: ", err)
		}
	}
	return nil
}

func templateDefinitionList(tplDef *model.TemplateDefinitionPayload, key string) []map[string]interface{} {
	if tplDef == nil {
		return nil
	}
	v, ok := (*tplDef)[key]
	if !ok {
		return nil
	}
	items, ok := v.([]interface{})
	if !ok {
		return nil
	}

	result := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result
}

func notifyMarketTemplateInstalled(client marketInstallClient, req model.InstallFromMarketReq, fullData *model.MarketTemplateFullData, claims *utils.UserClaims) {
	marketUserID, err := client.ExtractUserIDFromMarketToken(req.MarketToken)
	if err != nil {
		logrus.Warnf("Could not extract market user_id from token, install notification may fail: %v", err)
		marketUserID = ""
	}
	if marketUserID == "" {
		marketUserID = claims.ID
	}
	versionID := ""
	if fullData.VersionID != "" {
		versionID = fullData.VersionID
	}
	logrus.Infof("Notifying market of installation: TemplateID=%s, VersionID=%s, MarketUserID=%s, OrgID=%s",
		req.MarketTemplateID, versionID, marketUserID, claims.TenantID)
	marketInstallRunAsync(func() {
		err := client.InstallTemplate(context.Background(), req.MarketToken, req.MarketTemplateID, versionID, marketUserID, claims.TenantID)
		if err != nil {
			logrus.Errorf("Failed to notify market service of installation: %v", err)
		} else {
			logrus.Infof("Successfully notified market service of installation for template %s", req.MarketTemplateID)
		}
	})
}

func marketInstallDBError(prefix string, err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		"error": prefix + err.Error(),
	})
}

// checkMissingPlugins checks which plugin dependencies are not installed locally
func checkMissingPlugins(deps []model.PluginDependency) []model.PluginDependency {
	if len(deps) == 0 {
		return nil
	}

	var missing []model.PluginDependency
	for _, dep := range deps {
		p, err := marketInstallGetServicePlugin(dep.PluginName)
		if err != nil || p == nil {
			missing = append(missing, dep)
		}
	}
	return missing
}

// ptrStrP returns a pointer to a string (nil-safe)
func ptrStrP(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// getStr safely extracts a String field from a map
func getStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// getStrP safely extracts a string and returns a pointer (nil if empty)
func getStrP(m map[string]interface{}, key string) *string {
	s := getStr(m, key)
	if s == "" {
		return nil
	}
	return &s
}
