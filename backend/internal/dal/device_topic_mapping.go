// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"errors"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func CreateDeviceTopicMapping(mapping *model.DeviceTopicMapping) error {
	return query.DeviceTopicMapping.Create(mapping)
}

func TopicMappingExists(ctx context.Context, deviceConfigID, direction, sourceTopic, targetTopic string) (bool, error) {
	q := query.DeviceTopicMapping
	_, err := q.WithContext(ctx).
		Select(q.ID).
		Where(
			q.DeviceConfigID.Eq(deviceConfigID),
			q.Direction.Eq(direction),
			q.SourceTopic.Eq(sourceTopic),
			q.TargetTopic.Eq(targetTopic),
		).
		First()
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	logrus.Error(err)
	return false, err
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func GetDeviceTopicMappingByID(ctx context.Context, id int64) (*model.DeviceTopicMapping, error) {
	q := query.DeviceTopicMapping
	return q.WithContext(ctx).Where(q.ID.Eq(id)).First()
}

// tenant-scope: caller-enforced?2026-08-26 ?????
func ListDeviceTopicMappings(ctx context.Context, req *model.ListDeviceTopicMappingReq) ([]*model.DeviceTopicMapping, int64, error) {
	q := query.DeviceTopicMapping
	dao := applyDeviceTopicMappingFilters(q.WithContext(ctx), req)
	// order by priority asc, id asc
	dao = dao.Order(q.Priority, q.ID)
	offset := 0
	limit := 20
	if req.Page > 0 && req.PageSize > 0 {
		offset = (req.Page - 1) * req.PageSize
		limit = req.PageSize
	}
	result, err := dao.Offset(offset).Limit(limit).Find()
	if err != nil {
		return nil, 0, err
	}
	if offset == 0 && len(result) < limit {
		return result, int64(len(result)), nil
	}
	daoCount := applyDeviceTopicMappingFilters(q.WithContext(ctx), req)
	total, err := daoCount.Count()
	if err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

func applyDeviceTopicMappingFilters(dao query.IDeviceTopicMappingDo, req *model.ListDeviceTopicMappingReq) query.IDeviceTopicMappingDo {
	q := query.DeviceTopicMapping

	dao = dao.Where(q.DeviceConfigID.Eq(req.DeviceConfigID))
	if req.Direction != nil {
		dao = dao.Where(q.Direction.Eq(*req.Direction))
	}
	if req.SourceTopic != nil && *req.SourceTopic != "" {
		dao = dao.Where(q.SourceTopic.Like("%" + *req.SourceTopic + "%"))
	}
	if req.TargetTopic != nil && *req.TargetTopic != "" {
		dao = dao.Where(q.TargetTopic.Eq(*req.TargetTopic))
	}
	if req.Enabled != nil {
		if *req.Enabled {
			dao = dao.Where(q.Enabled.Is(true))
		} else {
			dao = dao.Where(q.Enabled.Is(false))
		}
	}
	if req.Description != nil && *req.Description != "" {
		dao = dao.Where(q.Description.Like("%" + *req.Description + "%"))
	}
	if req.DataIdentifier != nil && *req.DataIdentifier != "" {
		dao = dao.Where(q.DataIdentifier.Eq(*req.DataIdentifier))
	}

	return dao
}

func UpdateDeviceTopicMappingByID(ctx context.Context, id int64, updateMap map[string]interface{}) error {
	q := query.DeviceTopicMapping
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).UpdateColumns(updateMap)
	return err
}

func DeleteDeviceTopicMappingByID(ctx context.Context, id int64) error {
	q := query.DeviceTopicMapping
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Delete()
	return err
}
