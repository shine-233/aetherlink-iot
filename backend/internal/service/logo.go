// 文件用途：维护租户 Logo、品牌配置和默认展示资源服务。
// 核心逻辑：处理 Logo 配置读取、更新、默认值回退和前端展示所需响应。
// 关键注意事项：品牌资源是租户可见配置，跨租户读写和空路径覆盖都需要防护。
// 重构建议：抽出配置仓储和文件资源校验，补齐权限、默认值迁移、事务和路径异常测试。
package service

import (
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type Logo struct{}

func (*Logo) UpdateLogo(UpdateLogoReq *model.UpdateLogoReq, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to update logo settings")
	}
	// 权限门禁：系统管理员维护全局/系统级品牌（tenant_id=''），租户管理员仅维护本租户品牌。
	switch claims.Authority {
	case constant.SYS_ADMIN:
		// 全局行作用域：tenant_id 为空串，由 DAL 按 (id, tenant_id) 约束兜底。
	case constant.TENANT_ADMIN:
		if claims.TenantID == "" {
			return errcode.NewWithMessage(errcode.CodeNoPermission, "complete tenant initialization before update logo")
		}
	default:
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to update logo settings")
	}

	condsMap, err := StructToMapAndVerifyJson(UpdateLogoReq)
	if err != nil {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{
			"err": err.Error(),
		})
	}

	err = dal.UpdateLogo(claims.TenantID, UpdateLogoReq.Id, condsMap)
	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"err": err.Error(),
		})
	}
	return err
}

func (*Logo) GetLogoList(tenantID string) (map[string]interface{}, error) {

	total, list, err := dal.GetLogoList(tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"err": err.Error(),
		})
	}
	return logoListResponse(total, list), err
}

func logoListResponse(total int64, list interface{}) map[string]interface{} {
	return map[string]interface{}{
		"total": total,
		"list":  list,
	}
}
