// 文件用途：集中维护设备选择器下拉/分页列表使用的 DAL 查询逻辑。
// 核心逻辑：按租户构建设备基础查询，叠加设备配置存在性、名称搜索和更新时间排序，再返回分页结果。
// 使用注意：这里保持当前分页、搜索和更新时间倒序语义不变，不在 DAL 层修正 Page/PageSize 边界，避免改变前端既有行为。
// 重构建议：后续如继续扩展产品、协议或启用状态筛选，优先新增独立 filter helper，保持主查询链路扁平。
package dal

import (
	"context"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"github.com/sirupsen/logrus"
)

// GetDeviceSelector 返回设备选择器分页结果。
func GetDeviceSelector(req model.DeviceSelectorReq, tenantId string) (*model.DeviceSelectorRes, error) {
	queryBuilder := baseDeviceSelectorQuery(tenantId)
	queryBuilder = applyDeviceSelectorFilters(queryBuilder, req)
	queryBuilder = selectDeviceSelectorFields(queryBuilder)
	queryBuilder = queryBuilder.Order(query.Device.UpdateAt.Desc())

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	queryBuilder = applyDeviceSelectorPagination(queryBuilder, req)

	var list []*model.DeviceSelectorData
	err = queryBuilder.Scan(&list)
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	return &model.DeviceSelectorRes{
		Total: count,
		List:  list,
	}, nil
}

func baseDeviceSelectorQuery(tenantID string) query.IDeviceDo {
	device := query.Device
	deviceConfig := query.DeviceConfig

	return device.
		WithContext(context.Background()).
		LeftJoin(deviceConfig, device.DeviceConfigID.EqCol(deviceConfig.ID)).
		Where(device.TenantID.Eq(tenantID))
}

func applyDeviceSelectorFilters(builder query.IDeviceDo, req model.DeviceSelectorReq) query.IDeviceDo {
	device := query.Device

	if req.HasDeviceConfig != nil {
		if *req.HasDeviceConfig {
			builder = builder.Where(device.DeviceConfigID.IsNotNull())
		} else {
			builder = builder.Where(device.DeviceConfigID.IsNull())
		}
	}
	if req.Search != nil && *req.Search != "" {
		builder = builder.Where(device.Name.Like(ContainsLikePattern(*req.Search)))
	}
	if req.OwnerUserID != nil && *req.OwnerUserID != "" {
		builder = builder.Where(device.OwnerUserID.Eq(*req.OwnerUserID))
	}

	return builder
}

func selectDeviceSelectorFields(builder query.IDeviceDo) query.IDeviceDo {
	device := query.Device
	deviceConfig := query.DeviceConfig

	return builder.Select(device.ID.As("device_id"), device.Name.As("device_name"), deviceConfig.DeviceType.As("device_type"))
}

func applyDeviceSelectorPagination(builder query.IDeviceDo, req model.DeviceSelectorReq) query.IDeviceDo {
	return builder.
		Limit(req.PageSize).
		Offset((req.Page - 1) * req.PageSize)
}
