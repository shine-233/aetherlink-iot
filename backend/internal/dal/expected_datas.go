// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"context"
	"errors"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ExpectedDataDal struct {
}

// 创建预期数据
func (ExpectedDataDal) Create(ctx context.Context, data *model.ExpectedData) (err error) {
	err = query.ExpectedData.WithContext(ctx).Create(data)
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

// 删除预期数据
func (ExpectedDataDal) Delete(ctx context.Context, id string) error {
	info, err := query.ExpectedData.WithContext(ctx).Where(query.ExpectedData.ID.Eq(id)).Delete()
	if err != nil {
		logrus.Error(ctx, err)
		return err
	}
	if info.RowsAffected == 0 {
		return errors.New("no data")

	}
	return nil
}

// 详情查询
func (ExpectedDataDal) GetByID(ctx context.Context, id string) (data *model.ExpectedData, err error) {
	data, err = query.ExpectedData.WithContext(ctx).Where(query.ExpectedData.ID.Eq(id)).First()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

// 分页查询
func (ExpectedDataDal) PageList(ctx context.Context, req *model.GetExpectedDataPageReq, tenantID string) (total int64, list []map[string]interface{}, err error) {
	ed := query.ExpectedData
	queryBuilder := ed.WithContext(ctx)
	queryBuilder = queryBuilder.Where(ed.TenantID.Eq(tenantID), ed.DeviceID.Eq(req.DeviceID))

	if req.Label != nil {
		queryBuilder = queryBuilder.Where(ed.Label.Eq(*req.Label))
	}
	if req.SendType != nil {
		queryBuilder = queryBuilder.Where(ed.SendType.Eq(*req.SendType))
	}
	if req.Status != nil {
		queryBuilder = queryBuilder.Where(ed.Status.Eq(*req.Status))
	}

	// 总数
	total, err = queryBuilder.Count()
	if err != nil {
		logrus.Error(ctx, err)
		return
	}

	// 分页
	if req.Page > 0 && req.PageSize > 0 {
		queryBuilder = queryBuilder.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize).Order(ed.CreatedAt.Desc())
	}
	queryBuilder = queryBuilder.Select(ed.ALL)
	err = queryBuilder.Scan(&list)
	if err != nil {
		logrus.Error(ctx, err)
		return
	}
	if len(list) == 0 {
		list = []map[string]interface{}{}
	}
	return
}

func (ExpectedDataDal) ListPendingByDeviceID(ctx context.Context, deviceID, tenantID string) (list []*model.ExpectedData, err error) {
	ed := query.ExpectedData
	list, err = ed.WithContext(ctx).
		Where(
			ed.DeviceID.Eq(deviceID),
			ed.TenantID.Eq(tenantID),
			ed.Status.Eq("pending"),
		).
		Order(ed.CreatedAt.Desc()).
		Find()
	if err != nil {
		logrus.Error(ctx, err)
	}
	return
}

func (ExpectedDataDal) UpsertPendingDesired(ctx context.Context, data *model.ExpectedData) (*model.ExpectedData, error) {
	ed := query.ExpectedData
	existing, err := ed.WithContext(ctx).
		Where(
			ed.DeviceID.Eq(data.DeviceID),
			ed.TenantID.Eq(data.TenantID),
			ed.SendType.Eq(data.SendType),
			ed.Label.Eq(*data.Label),
			ed.Status.Eq("pending"),
		).
		First()
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.Error(ctx, err)
		return nil, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := ed.WithContext(ctx).Create(data); err != nil {
			logrus.Error(ctx, err)
			return nil, err
		}
		return ed.WithContext(ctx).Where(ed.ID.Eq(data.ID)).First()
	}

	updates := model.ExpectedData{
		Payload:    data.Payload,
		CreatedAt:  data.CreatedAt,
		ExpiryTime: data.ExpiryTime,
	}
	if _, err := ed.WithContext(ctx).Where(ed.ID.Eq(existing.ID)).Updates(updates); err != nil {
		logrus.Error(ctx, err)
		return nil, err
	}
	return ed.WithContext(ctx).Where(ed.ID.Eq(existing.ID)).First()
}

// 根据设备ID获取全部未处理的预期数据
func (ExpectedDataDal) GetAllByDeviceID(ctx context.Context, deviceID string) (list []*model.ExpectedData, err error) {
	ed := query.ExpectedData
	queryBuilder := ed.WithContext(ctx)
	queryBuilder = queryBuilder.Where(ed.DeviceID.Eq(deviceID))
	queryBuilder = queryBuilder.Where(ed.Status.Eq("pending"))
	queryBuilder = queryBuilder.Select(ed.ALL)
	list, err = queryBuilder.Find()
	if err != nil {
		logrus.Error(ctx, err)
		return
	}
	if len(list) == 0 {
		list = []*model.ExpectedData{}
	}
	return
}

// 更新状态
func (ExpectedDataDal) UpdateStatus(ctx context.Context, id string, status string, message *string, sendTime *time.Time) error {
	expectedData := model.ExpectedData{Status: status, Message: message, SendTime: sendTime}
	info, err := query.ExpectedData.WithContext(ctx).Where(query.ExpectedData.ID.Eq(id)).Updates(expectedData)
	if err != nil {
		logrus.Error(ctx, err)
		return err
	}
	if info.RowsAffected == 0 {
		return errors.New("no data")
	}
	return nil
}
