// 文件用途：维护系统 UI 元素级权限和可见性配置。
// 核心逻辑：处理按钮、页面元素和角色绑定的查询与保存，供前端控制细粒度展示。
// 关键注意事项：UI 权限不能替代后端鉴权，变更时需确保后端接口仍有服务级校验。
// 重构建议：拆分 UI 元素仓储和权限策略映射，补齐角色边界、默认可见性和同步测试。
package service

import (
	"time"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

type UiElements struct{}

func requireSysUIElementsAdmin(claims *utils.UserClaims) error {
	if claims == nil || claims.Authority != constant.SYS_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage ui elements")
	}
	return nil
}

func requireTenantUIElementsViewer(claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query ui elements")
	}
	if claims.Authority != constant.SYS_ADMIN && claims.Authority != constant.TENANT_ADMIN {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query ui elements")
	}
	return nil
}

func (*UiElements) CreateUiElements(CreateUiElementsReq *model.CreateUiElementsReq, claims *utils.UserClaims) error {
	if err := requireSysUIElementsAdmin(claims); err != nil {
		return err
	}

	var UiElements = model.SysUIElement{}

	UiElements.ID = uuid.New()
	UiElements.ParentID = CreateUiElementsReq.ParentID
	UiElements.ElementCode = CreateUiElementsReq.ElementCode
	UiElements.ElementType = int16(CreateUiElementsReq.ElementType)
	aa := int16(CreateUiElementsReq.Orders)
	UiElements.Order_ = &aa
	UiElements.Param1 = CreateUiElementsReq.Param1
	UiElements.Param2 = CreateUiElementsReq.Param2
	UiElements.Param3 = CreateUiElementsReq.Param3
	UiElements.CreatedAt = time.Now().UTC()
	UiElements.Authority = CreateUiElementsReq.Authority
	UiElements.Description = CreateUiElementsReq.Description
	UiElements.Remark = CreateUiElementsReq.Remark
	UiElements.Multilingual = CreateUiElementsReq.Multilingual
	UiElements.RoutePath = CreateUiElementsReq.RoutePath
	err := dal.CreateUiElements(&UiElements)

	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "create_ui_elements",
			"error":     err.Error(),
		})
	}

	return err
}

func (*UiElements) UpdateUiElements(UpdateUiElementsReq *model.UpdateUiElementsReq, claims *utils.UserClaims) error {
	if err := requireSysUIElementsAdmin(claims); err != nil {
		return err
	}
	var UiElements = model.SysUIElement{}
	UiElements.ID = UpdateUiElementsReq.Id
	if UpdateUiElementsReq.ParentID != nil {
		UiElements.ParentID = *UpdateUiElementsReq.ParentID
	}
	if UpdateUiElementsReq.ElementCode != nil {
		UiElements.ElementCode = *UpdateUiElementsReq.ElementCode
	}
	if UpdateUiElementsReq.ElementType != nil {
		UiElements.ElementType = *UpdateUiElementsReq.ElementType
	}
	UiElements.Order_ = UpdateUiElementsReq.Orders
	UiElements.Param1 = UpdateUiElementsReq.Param1
	UiElements.Param2 = UpdateUiElementsReq.Param2
	UiElements.Param3 = UpdateUiElementsReq.Param3
	if UpdateUiElementsReq.Authority != nil {
		UiElements.Authority = *UpdateUiElementsReq.Authority
	}
	UiElements.Description = UpdateUiElementsReq.Description
	UiElements.Multilingual = UpdateUiElementsReq.Multilingual
	UiElements.RoutePath = UpdateUiElementsReq.RoutePath
	UiElements.Remark = UpdateUiElementsReq.Remark

	err := dal.UpdateUiElements(&UiElements)
	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "update_ui_elements",
			"error":     err.Error(),
		})
	}
	return err
}

func (*UiElements) DeleteUiElements(id string, claims *utils.UserClaims) error {
	if err := requireSysUIElementsAdmin(claims); err != nil {
		return err
	}

	err := dal.DeleteUiElements(id)
	if err != nil {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "delete_ui_elements",
			"error":     err.Error(),
		})
	}
	return err
}

func (*UiElements) ServeUiElementsListByPage(Params *model.ServeUiElementsListByPageReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := requireSysUIElementsAdmin(claims); err != nil {
		return nil, err
	}

	total, list, err := dal.ServeUiElementsListByPage(Params)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_ui_elements",
			"error":     err.Error(),
		})
	}
	UiElementsListRsp := make(map[string]interface{})
	UiElementsListRsp["total"] = total
	UiElementsListRsp["list"] = list

	return UiElementsListRsp, err
}

func (*UiElements) ServeUiElementsListByAuthority(u *utils.UserClaims) (map[string]interface{}, error) {
	if u == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query ui elements")
	}

	total, list, err := dal.ServeUiElementsListByAuthority(u)
	if err != nil {
		logrus.Error("[ServeUiElementsListByAuthority] query failed:", err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_ui_elements",
			"user_id":   u.ID,
			"error":     err.Error(),
		})
	}

	return map[string]interface{}{
		"total": total,
		"list":  list,
	}, nil
}

// 获取租户下权限配置表单树
func (*UiElements) GetTenantUiElementsList(claims *utils.UserClaims) (map[string]interface{}, error) {
	if err := requireTenantUIElementsViewer(claims); err != nil {
		return nil, err
	}

	list, err := dal.GetTenantUiElementsList()
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_ui_elements",
			"error":     err.Error(),
		})
	}
	UiElementsListRsp := make(map[string]interface{})
	UiElementsListRsp["list"] = list

	return UiElementsListRsp, err
}
