// 文件用途：计算字段服务层，提供租户隔离的 CRUD、参数校验和启用开关。
// 核心逻辑：output_key 必须匹配 ^[a-zA-Z][a-zA-Z0-9_]*$；expression 用 govaluate
// 试解析，失败报 100002 参数错误并附引用变量名提示；模板归属在写入前核验。
// 关键注意事项：所有读写都以 claims.TenantID 作用域；不存在统一映射为 100404，
// message 固定含 "calculated field not found"，是 API 边界契约的一部分。
// 重构建议：若后续支持函数式表达式（如均值窗口），先扩展引擎白名单再放开此处校验。
package service

import (
	"regexp"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/casbin/govaluate"
	"github.com/go-basic/uuid"
	"gorm.io/gorm"
)

// calculatedFieldOutputKeyPattern 派生遥测键名约束：字母开头，仅字母/数字/下划线。
var calculatedFieldOutputKeyPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*$`)

// identifierPattern 从表达式中提取候选变量名，用于解析失败时的提示。
var identifierPattern = regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*`)

// CalculatedFieldService 计算字段业务入口。
type CalculatedFieldService struct{}

// calculatedFieldScope 提取租户作用域；claims 缺失或租户为空一律拒绝。
func calculatedFieldScope(claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage calculated fields")
	}
	tenantID := strings.TrimSpace(claims.TenantID)
	if tenantID == "" {
		return "", errcode.NewWithMessage(errcode.CodeNoPermission, "tenant id is required to manage calculated fields")
	}
	return tenantID, nil
}

// validateCalculatedFieldValue 校验 output_key 与 expression。
// 表达式试解析失败时报参数错误，并附带从表达式中提取到的标识符（即引用的遥测变量名）。
func validateCalculatedFieldValue(outputKey, expression string) error {
	if !calculatedFieldOutputKeyPattern.MatchString(outputKey) {
		return errcode.NewWithMessage(
			errcode.CodeParamError,
			"output_key must match ^[a-zA-Z][a-zA-Z0-9_]*$ (start with a letter, letters/digits/underscores only)",
		)
	}
	if strings.TrimSpace(expression) == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "expression must not be empty")
	}
	if _, err := govaluate.NewEvaluableExpression(expression); err != nil {
		variables := identifierPattern.FindAllString(expression, -1)
		hint := "no variable-like identifiers found; variables must match telemetry keys, e.g. (voltage * current) / 1000"
		if len(variables) > 0 {
			hint = "referenced variables: " + strings.Join(variables, ", ")
		}
		return errcode.NewWithMessage(
			errcode.CodeParamError,
			"invalid expression \""+expression+"\": "+err.Error()+"; "+hint,
		)
	}
	return nil
}

// ensureTemplateInTenant 校验设备模板存在且属于当前租户。
func ensureTemplateInTenant(templateID, tenantID string) error {
	count, err := dal.CountDeviceTemplatesInTenant(templateID, tenantID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": err.Error()})
	}
	if count == 0 {
		return errcode.NewWithMessage(errcode.CodeParamError, "device template not found in current tenant")
	}
	return nil
}

// GetCalculatedFieldList 分页查询作用域内（self∪子孙，ROADMAP C2）的计算字段。
func (*CalculatedFieldService) GetCalculatedFieldList(req *model.CalculatedFieldListReq, claims *utils.UserClaims) (*model.CalculatedFieldListRsp, error) {
	tenantID, err := calculatedFieldScope(claims)
	if err != nil {
		return nil, err
	}
	scopes := expandTenantIDScope(tenantID)
	total, list, dbErr := dal.ListCalculatedFieldsByPage(scopes, req)
	if dbErr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": dbErr.Error()})
	}
	return &model.CalculatedFieldListRsp{Total: total, List: list}, nil
}

// GetCalculatedField 按 id 查询单条计算字段（读走作用域成员判定）。
func (*CalculatedFieldService) GetCalculatedField(id string, claims *utils.UserClaims) (*model.CalculatedField, error) {
	tenantID, err := calculatedFieldScope(claims)
	if err != nil {
		return nil, err
	}
	field, dbErr := dal.GetCalculatedFieldForScopes(id, expandTenantIDScope(tenantID))
	if dbErr != nil {
		if errIsRecordNotFound(dbErr) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "calculated field not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": dbErr.Error()})
	}
	return field, nil
}

// CreateCalculatedField 创建计算字段；enabled 缺省为 false。
func (*CalculatedFieldService) CreateCalculatedField(req *model.CalculatedFieldCreateReq, claims *utils.UserClaims) (*model.CalculatedField, error) {
	tenantID, err := calculatedFieldScope(claims)
	if err != nil {
		return nil, err
	}
	if validateErr := validateCalculatedFieldValue(req.OutputKey, req.Expression); validateErr != nil {
		return nil, validateErr
	}
	if templateErr := ensureTemplateInTenant(req.DeviceTemplateID, tenantID); templateErr != nil {
		return nil, templateErr
	}

	now := time.Now().UTC()
	field := &model.CalculatedField{
		ID:               uuid.New(),
		TenantID:         tenantID,
		Name:             req.Name,
		DeviceTemplateID: req.DeviceTemplateID,
		OutputKey:        req.OutputKey,
		Expression:       req.Expression,
		Enabled:          req.Enabled != nil && *req.Enabled,
		Remark:           req.Remark,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if createErr := dal.CreateCalculatedField(field); createErr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": createErr.Error()})
	}
	return field, nil
}

// UpdateCalculatedField 更新计算字段基础信息（不含 enabled，启停走 toggle）。
func (*CalculatedFieldService) UpdateCalculatedField(id string, req *model.CalculatedFieldUpdateReq, claims *utils.UserClaims) (*model.CalculatedField, error) {
	tenantID, err := calculatedFieldScope(claims)
	if err != nil {
		return nil, err
	}
	if validateErr := validateCalculatedFieldValue(req.OutputKey, req.Expression); validateErr != nil {
		return nil, validateErr
	}
	if _, dbErr := dal.GetCalculatedFieldForScope(id, tenantID); dbErr != nil {
		if errIsRecordNotFound(dbErr) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "calculated field not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": dbErr.Error()})
	}

	if templateErr := ensureTemplateInTenant(req.DeviceTemplateID, tenantID); templateErr != nil {
		return nil, templateErr
	}

	updates := map[string]interface{}{
		"name":               req.Name,
		"device_template_id": req.DeviceTemplateID,
		"output_key":         req.OutputKey,
		"expression":         req.Expression,
		"remark":             req.Remark,
		"updated_at":         time.Now().UTC(),
	}
	if updateErr := dal.UpdateCalculatedFieldForScope(id, tenantID, updates); updateErr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": updateErr.Error()})
	}
	return dal.GetCalculatedFieldForScope(id, tenantID)
}

// ToggleCalculatedField 启用/停用计算字段；req.Enabled 为空时按当前值取反。
func (*CalculatedFieldService) ToggleCalculatedField(id string, req *model.CalculatedFieldToggleReq, claims *utils.UserClaims) (*model.CalculatedField, error) {
	tenantID, err := calculatedFieldScope(claims)
	if err != nil {
		return nil, err
	}
	field, dbErr := dal.GetCalculatedFieldForScope(id, tenantID)
	if dbErr != nil {
		if errIsRecordNotFound(dbErr) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "calculated field not found")
		}
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": dbErr.Error()})
	}

	nextState := !field.Enabled
	if req != nil && req.Enabled != nil {
		nextState = *req.Enabled
	}
	if toggleErr := dal.UpdateCalculatedFieldForScope(id, tenantID, map[string]interface{}{
		"enabled":    nextState,
		"updated_at": time.Now().UTC(),
	}); toggleErr != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": toggleErr.Error()})
	}
	return dal.GetCalculatedFieldForScope(id, tenantID)
}

// DeleteCalculatedField 删除计算字段；未命中报 100404。
func (*CalculatedFieldService) DeleteCalculatedField(id string, claims *utils.UserClaims) error {
	tenantID, err := calculatedFieldScope(claims)
	if err != nil {
		return err
	}
	dbErr := dal.DeleteCalculatedFieldForScope(id, tenantID)
	if dbErr != nil {
		if errIsRecordNotFound(dbErr) {
			return errcode.NewWithMessage(errcode.CodeNotFound, "calculated field not found")
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{"sql_error": dbErr.Error()})
	}
	return nil
}

// errIsRecordNotFound 统一判断 gorm 未命中错误。
func errIsRecordNotFound(err error) bool {
	return err != nil && err == gorm.ErrRecordNotFound
}
