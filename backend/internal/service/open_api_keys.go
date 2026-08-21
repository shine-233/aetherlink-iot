// 文件用途：维护开放 API key 的创建、查询和吊销服务。
// 核心逻辑：生成 key 标识与密钥材料，记录租户归属、过期时间和状态供开放接口鉴权。
// 关键注意事项：API key 是外部访问凭据，明文、日志、跨租户读取和过期状态必须严格控制。
// 重构建议：抽出密钥生成与哈希存储接口，补齐吊销、轮换、权限和审计日志测试。
// internal/service/open_api_keys.go
package service

import (
	"time"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

type OpenAPIKey struct{}

// CreateOpenAPIKey 创建OpenAPI密钥
func (o *OpenAPIKey) CreateOpenAPIKey(req *model.CreateOpenAPIKeyReq, claims *utils.UserClaims) error {
	// 校验用户权限
	if claims.Authority != "SYS_ADMIN" && claims.Authority != "TENANT_ADMIN" {
		return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
			"required_role": "SYS_ADMIN or TENANT_ADMIN",
			"current_role":  claims.Authority,
		})
	}

	// 租户管理员只能创建自己租户的密钥
	if claims.Authority != "SYS_ADMIN" && claims.TenantID != req.TenantID {
		return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
			"required_tenant": req.TenantID,
			"current_tenant":  claims.TenantID,
		})
	}

	// 生成APIKey
	apikey, err := utils.GenerateAPIKey()
	if err != nil {
		logrus.Errorf("生成AppSecret失败: %v", err)
		return errcode.New(errcode.CodeSystemError)
	}

	status := int16(1) // 默认启用
	// 创建OpenAPI密钥记录
	key := &model.OpenAPIKey{
		ID:        uuid.New(),
		TenantID:  req.TenantID,
		APIKey:    apikey,
		Status:    &status,
		Name:      req.Name,
		CreatedID: &claims.ID,
	}

	t := time.Now().UTC()
	key.CreatedAt = &t
	key.UpdatedAt = &t

	if err := dal.CreateOpenAPIKey(key); err != nil {
		logrus.Errorf("创建OpenAPI密钥失败: %v", err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	return nil
}

// GetOpenAPIKeyList 获取OpenAPI密钥列表
func (o *OpenAPIKey) GetOpenAPIKeyList(req *model.OpenAPIKeyListReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	var tenantID string
	// 租户管理员只能查看自己租户的密钥
	if claims.Authority == "TENANT_ADMIN" || claims.Authority == "TENANT_USER" {
		tenantID = claims.TenantID
	}

	total, list, err := dal.GetOpenAPIKeyListByPage(req, tenantID)
	if err != nil {
		logrus.Errorf("查询OpenAPI密钥列表失败: %v", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
		})
	}

	result := make(map[string]interface{})
	result["total"] = total
	result["list"] = list
	// 注意：list 内含明文 api_key，禁止把 result 整体写入日志。
	return result, nil
}

// UpdateOpenAPIKey 更新OpenAPI密钥
func (o *OpenAPIKey) UpdateOpenAPIKey(req *model.UpdateOpenAPIKeyReq, claims *utils.UserClaims) error {
	// 获取现有记录
	key, err := dal.GetOpenAPIKeyByID(req.ID)
	if err != nil {
		logrus.Errorf("获取OpenAPI密钥信息失败: %v", err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
			"id":    req.ID,
		})
	}

	// 校验权限
	if claims.Authority != "SYS_ADMIN" {
		if claims.Authority != "TENANT_ADMIN" || key.TenantID != claims.TenantID {
			return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
				"required_role": "SYS_ADMIN or TENANT_ADMIN",
				"current_role":  claims.Authority,
			})
		}
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
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
			"id":    req.ID,
		})
	}

	return nil
}

// DeleteOpenAPIKey 删除OpenAPI密钥
func (o *OpenAPIKey) DeleteOpenAPIKey(id string, claims *utils.UserClaims) error {
	// 获取现有记录
	key, err := dal.GetOpenAPIKeyByID(id)
	if err != nil {
		logrus.Errorf("获取OpenAPI密钥信息失败: %v", err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
			"id":    id,
		})
	}

	// 校验权限
	if claims.Authority != "SYS_ADMIN" {
		if claims.Authority != "TENANT_ADMIN" || key.TenantID != claims.TenantID {
			return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
				"required_role": "SYS_ADMIN or TENANT_ADMIN",
				"current_role":  claims.Authority,
			})
		}
	}

	// 执行删除
	if err := dal.DeleteOpenAPIKey(id); err != nil {
		logrus.Errorf("删除OpenAPI密钥失败: %v", err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
			"id":    id,
		})
	}

	return nil
}
