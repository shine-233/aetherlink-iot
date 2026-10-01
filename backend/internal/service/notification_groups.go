// 文件用途：维护通知分组、联系人集合和租户通知目标服务。
// 核心逻辑：处理通知组 CRUD、联系人关系和按告警/消息场景筛选目标。
// 关键注意事项：通知分组包含邮箱或 webhook 等敏感目标，跨租户访问和批量更新需严格校验。
// 重构建议：拆分分组仓储和目标解析器，补齐事务、权限、重复联系人和发送失败边界测试。
package service

import (
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/service/kit"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

type NotificationGroup struct{}

const directAppNotificationTypeMessage = "direct APP notification type is not supported; use MEMBER notification config with notificationType APP"

func validateNotificationGroupType(rawTypes string) error {
	for _, notifyType := range strings.Split(rawTypes, ",") {
		if strings.EqualFold(strings.TrimSpace(notifyType), model.NoticeType_APP) {
			return errcode.NewWithMessage(errcode.CodeParamError, directAppNotificationTypeMessage)
		}
	}
	return nil
}

func (*NotificationGroup) CreateNotificationGroup(createNotificationgroupReq *model.CreateNotificationGroupReq, u *utils.UserClaims) (*model.NotificationGroup, error) {
	if err := validateNotificationGroupType(createNotificationgroupReq.NotificationType); err != nil {
		return nil, err
	}

	now := kit.NowUTC()
	notificationGroup := &model.NotificationGroup{
		ID:                 kit.NewID(),
		Name:               createNotificationgroupReq.Name,
		NotificationConfig: createNotificationgroupReq.NotificationConfig,
		NotificationType:   createNotificationgroupReq.NotificationType,
		Status:             createNotificationgroupReq.Status,
		Description:        createNotificationgroupReq.Description,
		Remark:             createNotificationgroupReq.Remark,
		CreatedAt:          now,
		UpdatedAt:          now,
		TenantID:           u.TenantID,
	}
	if err := dal.CreateNotificationGroup(notificationGroup); err != nil {
		logrus.Error(err)
		return nil, dbError(err)
	}
	return notificationGroup, nil
}

func (*NotificationGroup) GetNotificationGroupById(id string, u *utils.UserClaims) (*model.NotificationGroup, error) {
	return ensureNotificationGroupReadAccess(id, u)
}

func (*NotificationGroup) UpdateNotificationGroup(id string, updateNotificationgroupReq *model.UpdateNotificationGroupReq, u *utils.UserClaims) (*model.NotificationGroup, error) {
	notificationGroup, err := ensureNotificationGroupWriteAccess(id, u)
	if err != nil {
		return nil, err
	}
	if updateNotificationgroupReq.NotificationType != nil {
		notificationGroup.NotificationType = *updateNotificationgroupReq.NotificationType
	}
	if err := validateNotificationGroupType(notificationGroup.NotificationType); err != nil {
		return nil, err
	}
	utils.SerializeData(updateNotificationgroupReq, notificationGroup)

	notificationGroup.UpdatedAt = kit.NowUTC()
	if err := dal.UpdateNotificationGroup(notificationGroup); err != nil {
		return nil, dbError(err)
	}
	return notificationGroup, nil
}

func (*NotificationGroup) DeleteNotificationGroup(id string, u *utils.UserClaims) error {
	if _, err := ensureNotificationGroupWriteAccess(id, u); err != nil {
		return err
	}
	if err := dal.DeleteNotificationGroup(id); err != nil {
		return dbError(err)
	}
	return nil
}

func (*NotificationGroup) GetNotificationGroupListByPage(pageParam *model.GetNotificationGroupListByPageReq, u *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetNotificationGroupListByPage(pageParam, u)
	if err != nil {
		return nil, dbError(err)
	}
	return kit.ListMap(total, list), nil
}
