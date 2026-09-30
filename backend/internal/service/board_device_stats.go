// board_device_stats.go 负责看板首页的设备统计聚合：总数/在线/离线概览与趋势。
//
// 9-28 域拆分：自 board.go 按聚合迁出，仅移动代码，函数签名与语义不变。
// GetDeviceTrend 历史上声明在 *Device 接收者上（看板首页趋势端点），随本聚合
// 一起迁入本文件，接收者与签名保持原样。
package service

import (
	"context"

	"aetherlink-iot/backend/internal/authz"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"gorm.io/gen"
)

func (*Board) GetDeviceTotal(ctx context.Context, claims *utils.UserClaims) (int64, error) {
	var (
		total int64
		err   error
		db    = dal.DeviceQuery{}
	)
	if scopeErr := requireSupportedScopeAuthority(claims, "no permission to query device total"); scopeErr != nil {
		return 0, scopeErr
	}
	if authz.IsSysAdmin(claims) {
		total, err = db.Count(ctx)
	} else {
		tenantID, tenantErr := requireDeviceTenantClaims(claims, "no permission to query device total")
		if tenantErr != nil {
			return 0, tenantErr
		}
		device := query.Device
		conditions := []gen.Condition{device.TenantID.Eq(tenantID)}
		if ownerUserID := deviceOwnerUserIDFilterForClaims(claims); ownerUserID != nil {
			conditions = append(conditions, device.OwnerUserID.Eq(*ownerUserID))
		}
		total, err = db.CountByWhere(ctx, conditions...)
	}
	if err != nil {
		return 0, dbError(err)
	}

	return total, err
}

func (*Board) GetDevice(ctx context.Context, U *utils.UserClaims) (data *model.GetBoardDeviceRes, err error) {
	var (
		total, on int64
		device    = query.Device
		db        = dal.DeviceQuery{}
	)
	if scopeErr := requireSupportedScopeAuthority(U, "no permission to query device overview"); scopeErr != nil {
		return nil, scopeErr
	}
	if !authz.IsSysAdmin(U) {
		if _, scopeErr := requireDeviceTenantClaims(U, "no permission to query device overview"); scopeErr != nil {
			return nil, scopeErr
		}
	}
	ownerUserID := deviceOwnerUserIDFilterForClaims(U)

	// 非管理员只能统计本租户设备；管理员需要排除 inactive 设备，保持首页数字语义一致。
	if !authz.IsSysAdmin(U) {
		total, err = countVisibleBoardDevices(ctx, db, U.TenantID, ownerUserID, false)
	} else {
		total, err = db.CountByWhere(ctx, device.ActivateFlag.Neq("inactive"))
	}
	if err != nil {
		logrus.Error(ctx, "[GetDevice]Device count failed:", err)
		err = dbError(err)
		return
	}
	// 在线数只统计 active 且在线的设备，离线数通过 total-on 反推。
	if !authz.IsSysAdmin(U) {
		on, err = countVisibleBoardDevices(ctx, db, U.TenantID, ownerUserID, true)
	} else {
		on, err = db.CountByWhere(ctx, device.ActivateFlag.Eq("active"), device.IsOnline.Eq(1))
	}
	if err != nil {
		logrus.Error(ctx, "[GetDevice]Device count/on failed:", err)
		err = dbError(err)
		return
	}
	data = &model.GetBoardDeviceRes{
		DeviceTotal:   total,
		DeviceOn:      on,
		DeviceOffline: total - on,
	}
	return
}

func countVisibleBoardDevices(ctx context.Context, db dal.DeviceQuery, tenantID string, ownerUserID *string, activeOnly bool) (int64, error) {
	return countBoardDevicesByScope(ctx, db, tenantID, ownerUserID, activeOnly, false)
}

func countBoardDevicesByScope(ctx context.Context, db dal.DeviceQuery, tenantID string, ownerUserID *string, activeOnly bool, allTenants bool) (int64, error) {
	device := query.Device
	conditions := make([]gen.Condition, 0, 3)
	if !allTenants {
		conditions = append(conditions, device.TenantID.Eq(tenantID))
	}
	if activeOnly {
		conditions = append(conditions, device.ActivateFlag.Eq("active"), device.IsOnline.Eq(1))
	} else {
		conditions = append(conditions, device.ActivateFlag.Neq("inactive"))
	}
	if ownerUserID != nil && *ownerUserID != "" {
		conditions = append(conditions, device.OwnerUserID.Eq(*ownerUserID))
	}
	return db.CountByWhere(ctx, conditions...)
}

func (b *Board) GetDeviceByTenantID(ctx context.Context, claims *utils.UserClaims) (data *model.GetBoardDeviceRes, err error) {
	return b.GetDeviceOverview(ctx, &model.GetBoardDeviceReq{}, claims)
}

func (*Board) GetDeviceOverview(ctx context.Context, req *model.GetBoardDeviceReq, claims *utils.UserClaims) (data *model.GetBoardDeviceRes, err error) {
	var (
		total, on int64
		db        = dal.DeviceQuery{}
	)
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "device overview request is required")
	}
	if err := requireSystemAdminAllTenantsScope(
		req.AllTenants,
		claims,
		"all-tenants device overview is only available to system administrators",
	); err != nil {
		return nil, err
	}
	tenantID := ""
	if !req.AllTenants {
		tenantID, err = requireDeviceTenantClaims(claims, "no permission to query tenant device overview")
		if err != nil {
			return nil, err
		}
	}
	ownerUserID := deviceOwnerUserIDFilterForClaims(claims)

	total, err = countBoardDevicesByScope(ctx, db, tenantID, ownerUserID, false, req.AllTenants)
	if err != nil {
		logrus.Error(ctx, "[GetDevice]Device count failed:", err)
		return
	}
	on, err = countBoardDevicesByScope(ctx, db, tenantID, ownerUserID, true, req.AllTenants)
	if err != nil {
		logrus.Error(ctx, "[GetDevice]Device count/on failed:", err)
		return
	}
	data = &model.GetBoardDeviceRes{
		DeviceTotal:   total,
		DeviceOn:      on,
		DeviceOffline: total - on,
	}
	return
}

func (*Device) GetDeviceTrend(ctx context.Context, claims *utils.UserClaims, tenantID string, startTime, endTime *int64) (*model.DeviceTrendRes, error) {
	var points []model.DeviceTrendPoint
	if err := requireSupportedScopeAuthority(claims, "no permission to query device trend"); err != nil {
		return nil, err
	}
	if !authz.IsSysAdmin(claims) {
		claimsTenantID, err := requireDeviceTenantClaims(claims, "no permission to query device trend")
		if err != nil {
			return nil, err
		}
		if tenantID != claimsTenantID {
			return nil, errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query device trend")
		}
	}

	points, err := dal.GetDeviceTrend(tenantID, deviceOwnerUserIDFilterForClaims(claims), startTime, endTime)
	if err != nil {
		return nil, dbError(err)
	}

	return &model.DeviceTrendRes{
		Points: points,
	}, nil
}
