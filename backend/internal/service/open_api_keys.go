// 文件用途：维护开放 API key 的创建、查询和吊销服务。
// 核心逻辑：生成 key 标识与密钥材料，记录租户归属、过期时间和状态供开放接口鉴权。
// 关键注意事项：API key 是外部访问凭据，明文、日志、跨租户读取和过期状态必须严格控制。
// 重构建议：抽出密钥生成与哈希存储接口，补齐吊销、轮换、权限和审计日志测试。
// internal/service/open_api_keys.go
package service

import (
	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/internal/authz"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service/kit"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

// openAPIKeyManagerRule 是开放 API key 管理接口的权限规则：只有 SYS_ADMIN 与
// TENANT_ADMIN 可操作，且 TENANT_ADMIN 只能操作本租户的 key。
var openAPIKeyManagerRule = authz.Rule{Roles: authz.ManagerRoles}

// openAPIKeyRoleDenied / openAPIKeyTenantDenied 保留原有错误码与 vars 载荷：
// 角色不符与租户不符是两个不同的报错，前端依赖这两个键做提示。
func openAPIKeyRoleDenied(claims *utils.UserClaims) error {
	return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
		"required_role": "SYS_ADMIN or TENANT_ADMIN",
		"current_role":  userClaimsAuthority(claims),
	})
}

func openAPIKeyTenantDenied(requiredTenant string, claims *utils.UserClaims) error {
	currentTenant := ""
	if claims != nil {
		currentTenant = claims.TenantID
	}
	return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
		"required_tenant": requiredTenant,
		"current_tenant":  currentTenant,
	})
}

type OpenAPIKey struct{}

// openAPIKeyDBErr 是本资源 DB 错误的固定形状 {"error": msg, "id": id}。
func openAPIKeyDBErr(id string, err error) error {
	return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
		kit.KeyError: err.Error(),
		"id":         id,
	})
}

// loadManagedOpenAPIKey 按 ID 加载并校验管理权限（SYS_ADMIN 任意租户，TENANT_ADMIN 仅本租户）。
// 加载失败（含不存在）走 {"error","id"}，权限不符统一回角色错误。
func loadManagedOpenAPIKey(id string, claims *utils.UserClaims) error {
	key, err := dal.GetOpenAPIKeyByID(id)
	if err != nil {
		logrus.Errorf("获取OpenAPI密钥信息失败: %v", err)
		return openAPIKeyDBErr(id, err)
	}
	if err := openAPIKeyManagerRule.Check(claims, authz.OfTenant(key.TenantID)); err != nil {
		return openAPIKeyRoleDenied(claims)
	}
	return nil
}

// CreateOpenAPIKey 创建OpenAPI密钥。
// 返回值是明文 key，仅在本次响应中出现一次；数据库只存 SHA-256 摘要。
func (o *OpenAPIKey) CreateOpenAPIKey(req *model.CreateOpenAPIKeyReq, claims *utils.UserClaims) (string, error) {
	// 校验用户权限：非 SYS_ADMIN/TENANT_ADMIN 一律拒绝（nil claims 同样拒绝）。
	if !authz.HasRole(claims, authz.ManagerRoles...) {
		return "", openAPIKeyRoleDenied(claims)
	}

	// 租户管理员只能创建自己租户的密钥；SYS_ADMIN 不受租户约束。
	if err := authz.TenantRule("").Check(claims, authz.OfTenant(req.TenantID)); err != nil {
		return "", openAPIKeyTenantDenied(req.TenantID, claims)
	}

	// 生成APIKey
	apikey, err := utils.GenerateAPIKey()
	if err != nil {
		logrus.Errorf("生成AppSecret失败: %v", err)
		return "", errcode.New(errcode.CodeSystemError)
	}

	status := int16(1) // 默认启用
	// 创建OpenAPI密钥记录：api_key 列存摘要，key_prefix 供列表辨认。
	key := &model.OpenAPIKey{
		ID:        kit.NewID(),
		TenantID:  req.TenantID,
		APIKey:    utils.HashAPIKey(apikey),
		KeyPrefix: utils.APIKeyDisplayPrefix(apikey),
		Status:    &status,
		Name:      req.Name,
		CreatedID: &claims.ID,
	}

	now := kit.NowUTCPtr()
	key.CreatedAt, key.UpdatedAt = now, now

	if err := dal.CreateOpenAPIKey(key); err != nil {
		logrus.Errorf("创建OpenAPI密钥失败: %v", err)
		return "", kit.DBErr(kit.KeyError, err)
	}

	return apikey, nil
}

// GetOpenAPIKeyList 获取OpenAPI密钥列表
func (o *OpenAPIKey) GetOpenAPIKeyList(req *model.OpenAPIKeyListReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	var tenantID string
	// SYS_ADMIN 看全租户；租户级角色钳到自己的租户。
	if authz.HasRole(claims, authz.TenantAdmin, authz.TenantUser) {
		tenantID = claims.TenantID
	}

	total, list, err := dal.GetOpenAPIKeyListByPage(req, tenantID)
	if err != nil {
		logrus.Errorf("查询OpenAPI密钥列表失败: %v", err)
		return nil, kit.DBErr(kit.KeyError, err)
	}

	// 列表只回显展示前缀；api_key 列存的是摘要，也一律不下发。
	if rows, ok := list.([]model.OpenAPIKeyListRsp); ok {
		for i := range rows {
			rows[i].APIKey = ""
		}
		list = rows
	} else if list != nil {
		// 断言失败意味着 dal 返回类型被改动而脱敏未同步——宁可显式报警也不能无声漏脱敏。
		logrus.Errorf("unexpected OpenAPI key list type %T; masking skipped", list)
	}

	return kit.AnyListMap(total, list), nil
}

// UpdateOpenAPIKey 更新OpenAPI密钥
func (o *OpenAPIKey) UpdateOpenAPIKey(req *model.UpdateOpenAPIKeyReq, claims *utils.UserClaims) error {
	if err := loadManagedOpenAPIKey(req.ID, claims); err != nil {
		return err
	}

	// 构建更新内容
	updates := make(map[string]interface{})
	if req.Status != nil {
		updates["status"] = *req.Status
	}
	if req.Name != nil {
		updates["name"] = *req.Name
	}

	// 执行更新
	if err := dal.UpdateOpenAPIKey(req.ID, updates); err != nil {
		logrus.Errorf("更新OpenAPI密钥失败: %v", err)
		return openAPIKeyDBErr(req.ID, err)
	}

	return nil
}

// DeleteOpenAPIKey 删除OpenAPI密钥
func (o *OpenAPIKey) DeleteOpenAPIKey(id string, claims *utils.UserClaims) error {
	if err := loadManagedOpenAPIKey(id, claims); err != nil {
		return err
	}

	// 执行删除
	if err := dal.DeleteOpenAPIKey(id); err != nil {
		logrus.Errorf("删除OpenAPI密钥失败: %v", err)
		return openAPIKeyDBErr(id, err)
	}

	return nil
}
