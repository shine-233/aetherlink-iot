// 文件用途：部件库服务（widget_bundles，ROADMAP TB-04）。
// 核心逻辑：租户级部件库 CRUD、widgets JSON 结构校验（复用 ValidateWidgetDefinition）、
//
//	内置四部件（gauge/chart/valve/twin3d）的导出描述与一键种子落库、资源中心导出/导入。
//
// 关键注意事项：
//  1. widgets 定义校验必须复用 widget_registry.go 的 ValidateWidgetDefinition——
//     另写一份校验会让"画布保存可用的部件"与"部件库能存进去的部件"口径分叉。
//  2. 种子落库 fail-closed：同名 bundle 已存在但内容不同时报错，绝不静默覆盖
//     用户手工管理的同名部件库；内容一致则走幂等路径。
//  3. 导入按 (租户, 名称) 幂等：同版本返回既有记录不重复建，异版本由资源中心
//     的覆盖确认闸门先行拦截。
//
// 重构建议：内置部件升级（version 2 等）后，种子幂等判定按内容比较会自然放行更新。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service/kit"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

// BuiltinWidgetBundleName 内置种子部件库的固定名称（seed 幂等键）。
const BuiltinWidgetBundleName = "内置部件库"

// BuiltinWidgetBundleTypeKey 内置种子部件库的行业分类标记。
const BuiltinWidgetBundleTypeKey = "builtin"

type WidgetBundleService struct{}

// widgetBundleRepo 租户内部件库：claims 与租户均必填，任何加载失败一律视为 not found（防跨租户探测）。
var widgetBundleRepo = kit.TenantRepo[*model.WidgetBundle]{
	Get:      dal.GetWidgetBundleByID,
	Gate:     kit.TenantRequired,
	NotFound: kit.NotFound{Msg: "widget bundle not found"},
}

// validateWidgetsJSON 校验 widgets JSON：必须是对象数组，且每项通过
// ValidateWidgetDefinition（type/version/schema/capabilities 结构约束）。
// 空数组合法（允许先建空库再逐步添加部件）。
func validateWidgetsJSON(raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "widgets must be a JSON array of widget definitions")
	}
	var defs []WidgetDefinition
	if err := json.Unmarshal([]byte(trimmed), &defs); err != nil {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"error": "widgets must be a JSON array of widget definitions: " + err.Error(),
		})
	}
	for i := range defs {
		if err := ValidateWidgetDefinition(&defs[i]); err != nil {
			return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
				"error": fmt.Sprintf("widgets[%d] is invalid: %v", i, err),
			})
		}
	}
	return nil
}

// normalizeWidgetsInput 归一 widgets 入参：空/缺失落为空数组 "[]"。
func normalizeWidgetsInput(raw *string) (string, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return "[]", nil
	}
	out := strings.TrimSpace(*raw)
	if err := validateWidgetsJSON(out); err != nil {
		return "", err
	}
	return out, nil
}

// CreateWidgetBundle 创建部件库
func (*WidgetBundleService) CreateWidgetBundle(ctx context.Context, req *model.CreateWidgetBundleReq, claims *utils.UserClaims) (*model.WidgetBundle, error) {
	if err := kit.TenantRequired.Require(claims); err != nil {
		return nil, err
	}
	widgets, err := normalizeWidgetsInput(req.Widgets)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if _, err := dal.GetWidgetBundleByNameInTenant(ctx, claims.TenantID, name); err == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "widget bundle name already exists in tenant")
	}

	now := time.Now().UTC()
	version := NormalizeDeviceTemplateVersion(req.Version)
	record := &model.WidgetBundle{
		ID:          uuid.New(),
		Name:        name,
		TenantID:    claims.TenantID,
		Widgets:     widgets,
		Description: req.Description,
		Version:     version,
		TypeKey:     req.TypeKey,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
	if err := dal.CreateWidgetBundle(record); err != nil {
		logrus.Errorf("failed to create widget bundle: %v", err)
		return nil, kit.DBErr(kit.KeyError, err)
	}
	return record, nil
}

// UpdateWidgetBundle 更新部件库
func (*WidgetBundleService) UpdateWidgetBundle(ctx context.Context, req *model.UpdateWidgetBundleReq, claims *utils.UserClaims) (*model.WidgetBundle, error) {
	record, err := widgetBundleRepo.Load(claims, req.ID)
	if err != nil {
		return nil, err
	}
	if req.Widgets != nil {
		widgets, werr := normalizeWidgetsInput(req.Widgets)
		if werr != nil {
			return nil, werr
		}
		record.Widgets = widgets
	}
	if req.Name != nil && strings.TrimSpace(*req.Name) != "" {
		newName := strings.TrimSpace(*req.Name)
		if newName != record.Name {
			if _, exists := dal.GetWidgetBundleByNameInTenant(ctx, claims.TenantID, newName); exists == nil {
				return nil, errcode.NewWithMessage(errcode.CodeParamError, "widget bundle name already exists in tenant")
			}
		}
		record.Name = newName
	}
	if req.Description != nil {
		record.Description = req.Description
	}
	if req.Version != nil {
		record.Version = NormalizeDeviceTemplateVersion(req.Version)
	}
	if req.TypeKey != nil {
		record.TypeKey = req.TypeKey
	}

	now := time.Now().UTC()
	record.UpdatedAt = &now
	if err := dal.UpdateWidgetBundle(record); err != nil {
		return nil, kit.DBErr(kit.KeyError, err)
	}
	return record, nil
}

// GetWidgetBundleByID 查询单个部件库详情
func (*WidgetBundleService) GetWidgetBundleByID(ctx context.Context, id string, claims *utils.UserClaims) (*model.WidgetBundle, error) {
	return widgetBundleRepo.Load(claims, id)
}

// DeleteWidgetBundle 删除部件库
func (*WidgetBundleService) DeleteWidgetBundle(ctx context.Context, id string, claims *utils.UserClaims) error {
	return widgetBundleRepo.Delete(claims, id, dal.DeleteWidgetBundle, nil)
}

// ListWidgetBundles 分页查询列表
func (*WidgetBundleService) ListWidgetBundles(ctx context.Context, req *model.GetWidgetBundleListReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	return kit.List(widgetBundleRepo, claims, req, dal.ListWidgetBundles, kit.OnDBErr(kit.KeyError))
}

// GetBuiltinWidgetDefinitions 内置四部件定义（gauge/chart/valve/twin3d），
// 直接取自 SCADA 装配的内置注册表常量——两边永远同源，不会漂移。
func (*WidgetBundleService) GetBuiltinWidgetDefinitions() []WidgetDefinition {
	return builtinWidgetDefinitions()
}

// ExportBuiltinWidgetBundle 把内置四部件组装为部件库导出描述符（种子 bundle 导出能力）。
func (*WidgetBundleService) ExportBuiltinWidgetBundle() (*model.WidgetBundleExport, error) {
	defs := builtinWidgetDefinitions()
	raw, err := json.Marshal(defs)
	if err != nil {
		return nil, errcode.NewWithMessage(errcode.CodeSystemError, "builtin widget definitions are not serializable")
	}
	description := "SCADA 内置四部件（gauge/chart/valve/twin3d）种子部件库"
	return &model.WidgetBundleExport{
		Kind:        "aetherlink-widget-bundle",
		Name:        BuiltinWidgetBundleName,
		Version:     widgetStrPtr("1.0.0"),
		Description: &description,
		TypeKey:     widgetStrPtr(BuiltinWidgetBundleTypeKey),
		Widgets:     string(raw),
		ExportedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// SeedBuiltinWidgetBundle 一键把内置四部件落为租户可管理的种子 bundle。
// 幂等口径（fail-closed，见文件头注意事项 2）：
//   - 不存在           → 创建，Idempotent=false；
//   - 存在且内容一致   → 返回既有记录，Idempotent=true；
//   - 存在但内容不同   → 报错，要求改名或手工处理，不静默覆盖。
func (*WidgetBundleService) SeedBuiltinWidgetBundle(ctx context.Context, claims *utils.UserClaims) (*model.WidgetBundleSeedRsp, error) {
	if err := kit.TenantRequired.Require(claims); err != nil {
		return nil, err
	}
	exported, err := (*WidgetBundleService)(nil).ExportBuiltinWidgetBundle()
	if err != nil {
		return nil, err
	}
	existing, err := dal.GetWidgetBundleByNameInTenant(ctx, claims.TenantID, BuiltinWidgetBundleName)
	if err == nil && existing != nil {
		if widgetBundleContentEqual(existing.Widgets, exported.Widgets) {
			return &model.WidgetBundleSeedRsp{Bundle: existing, Idempotent: true}, nil
		}
		return nil, errcode.NewWithMessage(errcode.CodeParamError,
			"widget bundle named "+BuiltinWidgetBundleName+" already exists with different content; rename or delete it first")
	}

	widgets := exported.Widgets
	req := &model.CreateWidgetBundleReq{
		Name:        BuiltinWidgetBundleName,
		Widgets:     &widgets,
		Description: exported.Description,
		Version:     exported.Version,
		TypeKey:     exported.TypeKey,
	}
	created, err := (*WidgetBundleService)(nil).CreateWidgetBundle(ctx, req, claims)
	if err != nil {
		return nil, err
	}
	return &model.WidgetBundleSeedRsp{Bundle: created, Idempotent: false}, nil
}

// ExportWidgetBundle 导出单个部件库为资源中心描述符（只读，不脱敏需求）。
func (*WidgetBundleService) ExportWidgetBundle(ctx context.Context, id string, claims *utils.UserClaims) (*model.WidgetBundleExport, error) {
	record, err := (*WidgetBundleService)(nil).GetWidgetBundleByID(ctx, id, claims)
	if err != nil {
		return nil, err
	}
	return widgetBundleToExport(record), nil
}

// widgetBundleToExport 实体 → 导出描述符。
func widgetBundleToExport(record *model.WidgetBundle) *model.WidgetBundleExport {
	version := record.Version
	if strings.TrimSpace(version) == "" {
		version = "1.0.0"
	}
	return &model.WidgetBundleExport{
		Kind:        "aetherlink-widget-bundle",
		Name:        record.Name,
		Author:      nil,
		Version:     &version,
		Description: record.Description,
		TypeKey:     record.TypeKey,
		Widgets:     record.Widgets,
		ExportedAt:  time.Now().UTC().Format(time.RFC3339),
	}
}

// ImportWidgetBundleWithTenant 租户幂等导入（资源中心逐项回放用）：
//   - 租户内同名同版本 → 幂等返回既有记录（created=false）；
//   - 同名异版本 → 更新为导入内容（created=false，覆盖语义由资源中心确认闸门保证）；
//   - 不存在 → 创建（created=true）。
//
// widgets 内容校验失败整条拒绝（rejected），绝不落半截 bundle。
func (*WidgetBundleService) ImportWidgetBundleWithTenant(exported model.ImportWidgetBundleReq, tenantID string) (*model.WidgetBundle, bool, error) {
	if strings.TrimSpace(tenantID) == "" {
		return nil, false, errcode.NewWithMessage(errcode.CodeNoPermission, "empty tenant scope")
	}
	if strings.TrimSpace(exported.Name) == "" {
		return nil, false, errcode.NewWithMessage(errcode.CodeParamError, "widget bundle name is required")
	}
	widgets, err := normalizeWidgetsInput(&exported.Widgets)
	if err != nil {
		return nil, false, err
	}

	ctx := context.Background()
	existing, err := dal.GetWidgetBundleByNameInTenant(ctx, tenantID, strings.TrimSpace(exported.Name))
	if err == nil && existing != nil {
		incomingVersion := NormalizeDeviceTemplateVersion(exported.Version)
		if existing.Version == incomingVersion && strings.TrimSpace(existing.Widgets) == widgets {
			return existing, false, nil
		}
		existing.Widgets = widgets
		existing.Version = incomingVersion
		existing.Description = exported.Description
		existing.TypeKey = exported.TypeKey
		now := time.Now().UTC()
		existing.UpdatedAt = &now
		if err := dal.UpdateWidgetBundle(existing); err != nil {
			return nil, false, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"error": err.Error(),
			})
		}
		return existing, false, nil
	}

	now := time.Now().UTC()
	record := &model.WidgetBundle{
		ID:          uuid.New(),
		Name:        strings.TrimSpace(exported.Name),
		TenantID:    tenantID,
		Widgets:     widgets,
		Description: exported.Description,
		Version:     NormalizeDeviceTemplateVersion(exported.Version),
		TypeKey:     exported.TypeKey,
		CreatedAt:   &now,
		UpdatedAt:   &now,
	}
	if err := dal.CreateWidgetBundle(record); err != nil {
		logrus.Errorf("failed to import widget bundle: %v", err)
		return nil, false, kit.DBErr(kit.KeyError, err)
	}
	return record, true, nil
}

// widgetStrPtr 轻量字符串指针 helper（避免引入额外的工具依赖；
// 命名避开测试文件里已有的 strPtr，防止同包重声明）。
func widgetStrPtr(s string) *string {
	out := s
	return &out
}

// widgetBundleContentEqual 对 widgets JSON 做语义等价比较：
// CreateWidgetBundle 落库时会重排/规范化 JSON（键序与空白可能与导出串不同），
// 幂等判定必须按反序列化后的结构比较，而不是文本比较，否则二次 seed 永远误报内容不一致。
func widgetBundleContentEqual(stored, exported string) bool {
	if strings.TrimSpace(stored) == strings.TrimSpace(exported) {
		return true
	}
	var left, right interface{}
	if err := json.Unmarshal([]byte(stored), &left); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(exported), &right); err != nil {
		return false
	}
	return reflect.DeepEqual(left, right)
}
