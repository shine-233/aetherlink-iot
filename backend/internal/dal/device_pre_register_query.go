package dal

import (
	"context"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"gorm.io/gen/field"

	"github.com/sirupsen/logrus"
)

func GetDevicePreRegisterListByPage(req *model.GetDevicePreRegisterListByPageReq, tenantID string) (int64, []model.GetDevicePreRegisterListByPageRsp, error) {
	var count int64
	deviceList := []model.GetDevicePreRegisterListByPageRsp{}
	queryBuilder := buildDevicePreRegisterListQuery(req, tenantID)

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, deviceList, err
	}

	queryBuilder = applyDevicePreRegisterPagination(queryBuilder, req)

	deviceList, err = scanDevicePreRegisterList(queryBuilder)
	if err != nil {
		logrus.Error(err)
		return count, deviceList, err
	}

	return count, deviceList, err
}

func buildDevicePreRegisterListQuery(req *model.GetDevicePreRegisterListByPageReq, tenantID string) query.IDeviceDo {
	q := query.Device
	queryBuilder := q.WithContext(context.Background()).Where(q.TenantID.Eq(tenantID))

	queryBuilder = applyPreRegisterPointerFilter(queryBuilder, req.ActivateFlag, func(value string) field.Expr {
		return q.ActivateFlag.Eq(value)
	})
	queryBuilder = applyPreRegisterPointerFilter(queryBuilder, req.IsEnabled, func(value string) field.Expr {
		return q.IsEnabled.Eq(value)
	})
	if req.ProductID != "" {
		queryBuilder = queryBuilder.Where(q.ProductID.Eq(req.ProductID))
	}
	queryBuilder = applyPreRegisterPointerFilter(queryBuilder, req.DeviceConfigID, func(value string) field.Expr {
		return q.DeviceConfigID.Eq(value)
	})
	queryBuilder = applyPreRegisterPointerFilter(queryBuilder, req.BatchNumber, func(value string) field.Expr {
		return deviceListLike(q.BatchNumber, value)
	})
	queryBuilder = applyPreRegisterPointerFilter(queryBuilder, req.DeviceNumber, func(value string) field.Expr {
		return deviceListLike(q.DeviceNumber, value)
	})
	queryBuilder = applyPreRegisterPointerFilter(queryBuilder, req.Name, func(value string) field.Expr {
		return deviceListLike(q.Name, value)
	})

	return queryBuilder
}

func applyPreRegisterPointerFilter(builder query.IDeviceDo, value *string, buildCondition func(string) field.Expr) query.IDeviceDo {
	if value == nil || *value == "" {
		return builder
	}
	return builder.Where(buildCondition(*value))
}

func applyDevicePreRegisterPagination(builder query.IDeviceDo, req *model.GetDevicePreRegisterListByPageReq) query.IDeviceDo {
	if req.Page == 0 || req.PageSize == 0 {
		return builder
	}
	return builder.Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize)
}

func scanDevicePreRegisterList(builder query.IDeviceDo) ([]model.GetDevicePreRegisterListByPageRsp, error) {
	q := query.Device
	deviceList := []model.GetDevicePreRegisterListByPageRsp{}
	err := builder.Select(
		q.ID, q.Name, q.DeviceNumber, q.ActivateFlag, q.ActivateAt, q.BatchNumber, q.CurrentVersion).
		Order(q.CreatedAt.Desc()).
		Scan(&deviceList)
	return deviceList, err
}
