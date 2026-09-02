// 文件用途：产品选择列表服务层——租户守卫包装 DAL 查询。
package service

import (
	model "aetherlink-iot/backend/internal/model"
	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"
)

func (*Device) GetProductSelectListByPage(req *model.GetProductSelectListReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := ensureTenantScopedWriteClaims(claims, "query product select list"); err != nil {
		return nil, err
	}
	total, list, err := dal.GetProductSelectListByPage(req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return map[string]interface{}{
		"total": total,
		"list":  list,
	}, nil
}
