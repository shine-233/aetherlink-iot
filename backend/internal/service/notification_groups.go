// 文件用途：维护通知分组、联系人集合和租户通知目标服务。
// 核心逻辑：处理通知组 CRUD、联系人关系和按告警/消息场景筛选目标。
// 关键注意事项：通知分组包含邮箱或 webhook 等敏感目标，跨租户访问和批量更新需严格校验。
// 重构建议：拆分分组仓储和目标解析器，补齐事务、权限、重复联系人和发送失败边界测试。
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

type NotificationGroup struct{}

func ensureNotificationGroupReadAccess(id string, u *utils.UserClaims) (*model.NotificationGroup, error) {
	notificationGroup, err := dal.GetNotificationGroupById(id)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	if u == nil {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query notification group")
	}
	if u.Authority != constant.SYS_ADMIN && notificationGroup.TenantID != u.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query notification group")
	}
	return notificationGroup, nil
}

func ensureNotificationGroupWriteAccess(id string, u *utils.UserClaims) (*model.NotificationGroup, error) {
	notificationGroup, err := ensureNotificationGroupReadAccess(id, u)
	if err != nil {
		return nil, err
	}
	if u.Authority != constant.SYS_ADMIN && notificationGroup.TenantID != u.TenantID {
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to modify notification group")
	}
	return notificationGroup, nil
}

func (*NotificationGroup) CreateNotificationGroup(createNotificationgroupReq *model.CreateNotificationGroupReq, u *utils.UserClaims) (*model.NotificationGroup, error) {
	var notificationGroup model.NotificationGroup
	notificationGroup.ID = uuid.New()
	notificationGroup.Name = createNotificationgroupReq.Name
	notificationGroup.NotificationConfig = createNotificationgroupReq.NotificationConfig
	notificationGroup.NotificationType = createNotificationgroupReq.NotificationType
	notificationGroup.Status = createNotificationgroupReq.Status
	notificationGroup.Description = createNotificationgroupReq.Description
	notificationGroup.Remark = createNotificationgroupReq.Remark
	notificationGroup.UpdatedAt = time.Now().UTC()
	notificationGroup.CreatedAt = time.Now().UTC()
	notificationGroup.TenantID = u.TenantID
	err := dal.CreateNotificationGroup(&notificationGroup)

	if err != nil {
		logrus.Error(err)
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return &notificationGroup, nil
}

func (*NotificationGroup) GetNotificationGroupById(id string, u *utils.UserClaims) (notificationGroup *model.NotificationGroup, err error) {
	notificationGroup, err = ensureNotificationGroupReadAccess(id, u)
	if err != nil {
		return nil, err
	}
	return
}

func (*NotificationGroup) UpdateNotificationGroup(id string, updateNotificationgroupReq *model.UpdateNotificationGroupReq, u *utils.UserClaims) (*model.NotificationGroup, error) {
	notificationGroup, err := ensureNotificationGroupWriteAccess(id, u)
	if err != nil {
		return nil, err
	}
	utils.SerializeData(updateNotificationgroupReq, notificationGroup)

	notificationGroup.UpdatedAt = time.Now().UTC()
	err = dal.UpdateNotificationGroup(notificationGroup)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return notificationGroup, nil
}

func (*NotificationGroup) DeleteNotificationGroup(id string, u *utils.UserClaims) error {
	if _, err := ensureNotificationGroupWriteAccess(id, u); err != nil {
		return err
	}
	err := dal.DeleteNotificationGroup(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	return nil
}

func (*NotificationGroup) GetNotificationGroupListByPage(pageParam *model.GetNotificationGroupListByPageReq, u *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetNotificationGroupListByPage(pageParam, u)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return notificationGroupListResponse(total, list), err
}

func (*NotificationGroup) GetNotificationGroupListByTenantId(tenantid string) (map[string]interface{}, error) {
	total, list, err := dal.GetNotificationGroupByTenantId(tenantid)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}

	return notificationGroupListResponse(total, list), err
}

func (*NotificationGroup) GetNotificationByTenantId(tenantid string) (map[string]interface{}, error) {
	total, list, err := dal.GetBoardListByTenantId(tenantid)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"sql_error": err.Error(),
		})
	}
	boardListRsp := make(map[string]interface{})
	boardListRsp["total"] = total
	boardListRsp["list"] = list

	return boardListRsp, err
}

func notificationGroupListResponse(total interface{}, list interface{}) map[string]interface{} {
	return map[string]interface{}{
		"total": total,
		"list":  list,
	}
}
