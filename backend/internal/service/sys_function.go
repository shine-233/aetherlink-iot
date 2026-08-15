// 文件用途：维护系统功能点和权限菜单的服务数据。
// 核心逻辑：读取功能定义、菜单绑定和角色授权所需的功能树结构。
// 关键注意事项：功能点变更会影响权限判断，隐藏项、路径和角色绑定需保持一致。
// 重构建议：拆分功能树构建和权限同步，补齐 Casbin 同步、排序、缺失节点和权限边界测试。
package service

import (
	"aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"
)

type SysFunction struct{}

func (*SysFunction) GetSysFuncion(lang string) ([]*model.SysFunction, error) {
	data, err := dal.GetAllSysFunction()
	// 多语言处理
	for _, v := range data {
		description := global.ResponseHandler.ErrManager.GetMessageStr(*v.Description, lang)
		v.Description = &description
	}
	return data, err
}

func (*SysFunction) UpdateSysFuncion(function_id string, claims *utils.UserClaims) error {
	if claims == nil || claims.Authority != constant.SYS_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to update system function")
	}

	old, err := dal.GetSysFunctionById(function_id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if old.ID == "" {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"msg": "id is nil",
		})
	}

	var upTarget string

	if old.EnableFlag == "enable" {
		upTarget = "disable"
	} else {
		upTarget = "enable"
	}

	err = dal.UpdateSysFunction(function_id, upTarget)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return err
}
