// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"fmt"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
)

func CreateNotificationGroup(notificationGroup *model.NotificationGroup) error {
	return query.NotificationGroup.Create(notificationGroup)
}

func UpdateNotificationGroup(notificationGroup *model.NotificationGroup) error {
	if _, err := query.NotificationGroup.Where(query.NotificationGroup.ID.Eq(notificationGroup.ID)).Updates(notificationGroup); err != nil {
		return err
	}
	return nil
}

func DeleteNotificationGroup(id string) error {
	res, err := query.NotificationGroup.Where(query.NotificationGroup.ID.Eq(id)).Delete()
	if err != nil {
		logrus.Error(err)
	}
	if res.RowsAffected == 0 {
		logrus.Error("delete notification group failed: not found")
		return fmt.Errorf("delete notification group failed: not found %s", id)
	}
	logrus.Info("delete notification group success")
	return err
}

func GetNotificationGroupList(page, pageSize int) (int64, interface{}, error) {
	var count int64
	queryBuilder := query.NotificationGroup.WithContext(context.Background())
	if page != 0 && pageSize != 0 {
		queryBuilder = queryBuilder.Limit(pageSize)
		queryBuilder = queryBuilder.Offset((page - 1) * pageSize)
	}
	notificationGroupList, err := queryBuilder.Select().Find()
	if err != nil {
		return count, notificationGroupList, err
	}
	count, err = queryBuilder.Count()
	return count, notificationGroupList, err
}

func GetNotificationGroupById(id string) (*model.NotificationGroup, error) {
	p := query.NotificationGroup
	notificationGroup, err := query.NotificationGroup.Where(p.ID.Eq(id)).Select().First()
	if err != nil {
		return nil, err
	}
	return notificationGroup, err
}

func GetNotificationGroupByTenantId(tenantid string) (notificationGroups []*model.NotificationGroup, count int, err error) {
	q := query.NotificationGroup
	notificationGroups, err = q.Where(q.TenantID.Eq(tenantid)).Find()
	if err != nil {
		return nil, 0, err
	}
	count = len(notificationGroups)
	return notificationGroups, count, err
}

func GetNotificationGroupListByPage(notifications *model.GetNotificationGroupListByPageReq, u *utils.UserClaims) (int64, []*model.NotificationGroup, error) {
	q := query.NotificationGroup
	var count int64
	queryBuilder := q.WithContext(context.Background())
	if notifications.Name != nil {
		queryBuilder = queryBuilder.Where(q.Name.Like(fmt.Sprintf("%%%s%%", *notifications.Name)))
	}

	if notifications.NotificationType != nil {
		queryBuilder = queryBuilder.Where(q.NotificationType.Eq(*notifications.NotificationType))
	}

	if notifications.Status != nil {
		queryBuilder = queryBuilder.Where(q.Status.Eq(*notifications.Status))
	}

	queryBuilder = queryBuilder.Where(q.TenantID.Eq(u.TenantID))

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, nil, err
	}

	queryBuilder = queryBuilder.Limit(notifications.PageSize)
	queryBuilder = queryBuilder.Offset((notifications.Page - 1) * notifications.PageSize)

	notificationList, err := queryBuilder.Order(q.CreatedAt.Desc()).Find()
	if err != nil {
		logrus.Error("queryBuilder.Find error: ", err)
	}
	return count, notificationList, err
}
