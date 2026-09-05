// 文件用途：维护物模型 CRUD、查询和模型关联服务。
// 核心逻辑：按租户处理物模型列表、详情、更新和模型关系，供设备配置和市场流程复用。
// 关键注意事项：物模型字段会影响设备创建与自动化来源，跨租户读取和级联删除需谨慎。
// 重构建议：拆分物模型仓储和模型关联服务，补齐事务、权限、级联副作用和兼容字段测试。
package service

import (
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
)

type DeviceTemplate struct{}

// tenantIDInScopes 纯成员判断：resourceTenant 是否落在自上而下可读租户作用域内（供测试注入）。
func tenantIDInScopes(resourceTenant string, scopes []string) bool {
	for _, s := range scopes {
		if s == resourceTenant {
			return true
		}
	}
	return false
}

func ensureDeviceTemplateReadAccess(templateID string, claims *utils.UserClaims) (*model.DeviceTemplate, error) {
	if claims == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query thing model")
	}
	t, err := dal.GetDeviceTemplateById(templateID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if t.Flag != nil && *t.Flag == dal.DEVICE_TEMPLATE_PUBLIC {
		return t, nil
	}
	// 自上而下（self∪子孙）：总部/父级管理员可读取子租户模板；系统管理员全量；叶子租户退化为仅自身。
	if claims.Authority != dal.SYS_ADMIN && !tenantIDInScopes(t.TenantID, expandTenantIDScope(claims.TenantID)) {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query thing model")
	}
	return t, nil
}

func ensureDeviceTemplateWriteAccess(templateID string, claims *utils.UserClaims) (*model.DeviceTemplate, error) {
	t, err := ensureDeviceTemplateReadAccess(templateID, claims)
	if err != nil {
		return nil, err
	}
	if t.Flag != nil && *t.Flag == dal.DEVICE_TEMPLATE_PUBLIC && claims.Authority == dal.TENANT_USER {
		return nil, errcode.New(errcode.CodeOpDenied)
	}
	if claims.Authority != dal.SYS_ADMIN && t.TenantID != claims.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify thing model")
	}
	return t, nil
}

func (*DeviceTemplate) CreateDeviceTemplate(req model.CreateDeviceTemplateReq, claims *utils.UserClaims) (*model.DeviceTemplate, error) {
	if err := ensureTenantScopedWriteClaims(claims, "create thing model"); err != nil {
		return nil, err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errcode.WithVars(100005, map[string]interface{}{
			"field": "name",
		})
	}

	var deviceTemplate = model.DeviceTemplate{}

	deviceTemplate.ID = uuid.New()
	deviceTemplate.Name = name
	deviceTemplate.Author = req.Author
	deviceTemplate.Version = req.Version
	deviceTemplate.Description = req.Description
	deviceTemplate.TenantID = claims.TenantID

	deviceTemplate.Path = req.Path
	deviceTemplate.Label = req.Label
	deviceTemplate.Brand = req.Brand
	deviceTemplate.ModelNumber = req.ModelNumber
	deviceTemplate.TypeKey = req.TypeKey

	t := time.Now().UTC()

	deviceTemplate.CreatedAt = t
	deviceTemplate.UpdatedAt = t

	data, err := dal.CreateDeviceTemplate(&deviceTemplate)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, err
}

func (*DeviceTemplate) UpdateDeviceTemplate(req model.UpdateDeviceTemplateReq, claims *utils.UserClaims) (*model.DeviceTemplate, error) {
	// 根据ID 获取物模型
	t, err := ensureDeviceTemplateWriteAccess(req.Id, claims)
	if err != nil {
		return nil, err
	}
	t.ID = req.Id
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, errcode.WithVars(100005, map[string]interface{}{
				"field": "name",
			})
		}
		t.Name = name
	}
	if req.Author != nil {
		t.Author = req.Author
	}
	if req.Version != nil {
		t.Version = req.Version
	}
	if req.Description != nil {
		t.Description = req.Description
	}
	if req.Path != nil {
		t.Path = req.Path
	}
	if req.Label != nil {
		t.Label = req.Label
	}
	if req.Remark != nil {
		t.Remark = req.Remark
	}
	if req.Brand != nil {
		t.Brand = req.Brand
	}
	if req.ModelNumber != nil {
		t.ModelNumber = req.ModelNumber
	}
	if req.TypeKey != nil {
		t.TypeKey = req.TypeKey
	}
	if req.WebChartConfig != nil {
		if !IsJSON(*req.WebChartConfig) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "web_chart_config is not a valid JSON")
		}
		t.WebChartConfig = req.WebChartConfig
	}

	if req.AppChartConfig != nil {
		if !IsJSON(*req.AppChartConfig) {
			return nil, errcode.NewWithMessage(errcode.CodeParamError, "app_chart_config is not a valid JSON")
		}
		t.AppChartConfig = req.AppChartConfig
	}
	t.UpdatedAt = time.Now().UTC()
	data, err := dal.UpdateDeviceTemplate(t)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, err
}

func (*DeviceTemplate) GetDeviceTemplate(id string) (*model.DeviceTemplate, error) {
	// 根据ID 获取物模型
	t, err := dal.GetDeviceTemplateById(id)
	if err != nil {
		return t, err
	}

	return t, nil
}

func (*DeviceTemplate) GetDeviceTemplateById(id string, claims *utils.UserClaims) (*model.DeviceTemplate, error) {
	return ensureDeviceTemplateReadAccess(id, claims)
}

// GetDeviceTemplateByDeviceId 根据设备ID获取物模型
func (*DeviceTemplate) GetDeviceTemplateByDeviceId(deviceId string, claims *utils.UserClaims) (any, error) {
	if _, err := ensureTelemetryDeviceReadAccess(deviceId, claims); err != nil {
		return nil, err
	}
	// 根据ID 获取物模型
	t, err := dal.GetDeviceTemplateByDeviceId(deviceId)
	if err != nil {
		return t, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return t, nil
}

func (*DeviceTemplate) DeleteDeviceTemplate(id string, claims *utils.UserClaims) error {
	// 根据ID 获取物模型
	t, err := ensureDeviceTemplateWriteAccess(id, claims)
	if err != nil {
		return err
	}
	// 根据功能物模型ID查询关联的配置数量
	count, err := dal.GetDeviceConfigCountByFuncTemplateId(t.ID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
			"msg":       "get device config count by func template id error",
		})
	}
	if count > 0 {
		return errcode.WithVars(200050, map[string]interface{}{
			"count": count,
		})
	}

	err = dal.DeleteDeviceTemplate(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return err
}

func (*DeviceTemplate) GetDeviceTemplateListByPage(req model.GetDeviceTemplateListByPageReq, claims *utils.UserClaims) (interface{}, error) {

	total, list, err := dal.GetDeviceTemplateListByPage(&req, expandTenantIDScope(claims.TenantID))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	deviceTemplateMap := make(map[string]interface{})
	deviceTemplateMap["total"] = total
	deviceTemplateMap["list"] = list

	return deviceTemplateMap, nil
}

// 获取物模型下拉菜单
func (*DeviceTemplate) GetDeviceTemplateMenu(req model.GetDeviceTemplateMenuReq, claims *utils.UserClaims) (interface{}, error) {

	data, err := dal.GetDeviceTemplateMenu(&req, expandTenantIDScope(claims.TenantID))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, nil
}

// GetDeviceTemplateStats 获取设备物模型统计信息（统计范围 = claims 自上而下作用域，供总部下钻子模板）
func (*DeviceTemplate) GetDeviceTemplateStats(req model.GetDeviceTemplateStatsReq, claims *utils.UserClaims) (*model.GetDeviceTemplateStatsRsp, error) {
	data, err := dal.GetDeviceTemplateStats(req.DeviceTemplateID, expandTenantIDScope(claims.TenantID))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, nil
}

// GetDeviceTemplateSelector 获取设备物模型选择器列表
func (*DeviceTemplate) GetDeviceTemplateSelector(req model.GetDeviceTemplateSelectorReq, claims *utils.UserClaims) ([]*model.GetDeviceTemplateSelectorRsp, error) {
	data, err := dal.GetDeviceTemplateSelector(&req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, nil
}

// ExportDeviceTemplate 模板市场导出：按读权限导出可移植模板描述符（不含 id/tenant_id）。
func (*DeviceTemplate) ExportDeviceTemplate(id string, claims *utils.UserClaims) (*model.DeviceTemplateExport, error) {
	t, err := ensureDeviceTemplateReadAccess(id, claims)
	if err != nil {
		return nil, err
	}
	return &model.DeviceTemplateExport{
		Kind:           "aetherlink-device-template",
		Name:           t.Name,
		Author:         t.Author,
		Version:        t.Version,
		Description:    t.Description,
		Remark:         t.Remark,
		Path:           t.Path,
		Label:          t.Label,
		Brand:          t.Brand,
		ModelNumber:    t.ModelNumber,
		TypeKey:        t.TypeKey,
		WebChartConfig: t.WebChartConfig,
		AppChartConfig: t.AppChartConfig,
		ExportedAt:     time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// ProvisionIdentify 实体下发命令的 identify（云端把导出载荷经命令通道推给边端）。
const ProvisionIdentify = "aetherlink/template/import"

// ImportDeviceTemplate 模板市场导入：把导出载荷创建为调用者租户下的新模板。
// 幂等语义：同租户下已存在同名同版本模板时返回既有模板（created=false），不重复建行。
func (*DeviceTemplate) ImportDeviceTemplate(req model.ImportDeviceTemplateReq, claims *utils.UserClaims) (*model.DeviceTemplate, bool, error) {
	if err := ensureTenantScopedWriteClaims(claims, "import thing model"); err != nil {
		return nil, false, err
	}
	return (*DeviceTemplate)(nil).ImportDeviceTemplateWithTenant(req, claims.TenantID)
}

// ImportDeviceTemplateWithTenant 实体下发核心：把导出载荷创建为指定租户下的新模板
// （无 claims 上下文的中继路径——边端命令通道以配置租户落地）。幂等语义同上。
func (*DeviceTemplate) ImportDeviceTemplateWithTenant(req model.ImportDeviceTemplateReq, tenantID string) (*model.DeviceTemplate, bool, error) {
	if req.Kind != "" && req.Kind != "aetherlink-device-template" {
		return nil, false, errcode.NewWithMessage(errcode.CodeParamError, "unsupported template kind: "+req.Kind)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, false, errcode.WithVars(100005, map[string]interface{}{
			"field": "name",
		})
	}
	version := "1.0.0"
	if req.Version != nil && strings.TrimSpace(*req.Version) != "" {
		version = strings.TrimSpace(*req.Version)
	}
	// 幂等：同租户同名同版本直接复用。
	existing, err := dal.FindDeviceTemplateByNameVersion(tenantID, name, version)
	if err != nil {
		return nil, false, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if existing != nil {
		return existing, false, nil
	}
	t := time.Now().UTC()
	deviceTemplate := model.DeviceTemplate{
		ID:             uuid.New(),
		Name:           name,
		Author:         req.Author,
		Version:        &version,
		Description:    req.Description,
		TenantID:       tenantID,
		CreatedAt:      t,
		UpdatedAt:      t,
		Label:          req.Label,
		WebChartConfig: req.WebChartConfig,
		AppChartConfig: req.AppChartConfig,
		Remark:         req.Remark,
		Path:           req.Path,
		TypeKey:        req.TypeKey,
		Brand:          req.Brand,
		ModelNumber:    req.ModelNumber,
	}
	data, err := dal.CreateDeviceTemplate(&deviceTemplate)
	if err != nil {
		return nil, false, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return data, true, nil
}
