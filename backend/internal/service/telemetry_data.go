// 文件用途：为遥测读写相关服务提供共享的设备访问守卫与下行总线装配入口。
// 核心逻辑：在进入具体遥测读写流程前，统一校验设备 ID、用户声明、租户边界和共享读权限，避免下游流程重复拼权限判断。
// 维护注意事项：这里的 helper 会被多个遥测服务复用，修改权限判定或错误路径后应统一回归受影响的遥测查询、写入和共享读取场景。
package service

import (
	"errors"
	"strings"

	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/downlink"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

const (
	telemetryReadPermissionMessage  = "no permission to query device telemetry"
	telemetryWritePermissionMessage = "no permission to modify device telemetry"
)

type TelemetryData struct {
	downlinkBus *downlink.Bus
}

type telemetryAccessContext struct {
	device            *model.Device
	permissionMessage string
}

// SetDownlinkBus 在服务装配阶段注入共享下行总线，供需要下发设备消息的遥测服务复用。
func (t *TelemetryData) SetDownlinkBus(bus *downlink.Bus) {
	t.downlinkBus = bus
}

func requireTelemetryDeviceID(deviceID string) (string, error) {
	trimmed := strings.TrimSpace(deviceID)
	if trimmed == "" {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "device_id is required")
	}
	return trimmed, nil
}

func requireTelemetryClaims(claims *utils.UserClaims, permissionMessage string) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, permissionMessage)
	}
	return nil
}

func loadTelemetryDeviceForAccess(deviceID string, claims *utils.UserClaims, permissionMessage string) (*model.Device, error) {
	normalizedDeviceID, err := requireTelemetryDeviceID(deviceID)
	if err != nil {
		return nil, err
	}
	if err := requireTelemetryClaims(claims, permissionMessage); err != nil {
		return nil, err
	}

	deviceInfo, err := dal.GetDeviceByID(normalizedDeviceID)
	if err != nil {
		// 错误映射加固（2026-08）：ErrRecordNotFound 表示该设备主行在库中根本不存在，
		// 与租户/共享可见性无关，可以安全地映射为明确的"资源不存在"业务码；
		// 裸 error 若继续透传会被响应中间件兜底成 100000 系统内部错误。
		// deviceInfo == nil 的防泄漏分支保持 permission-shaped，语义不变。
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errcode.NewWithMessage(errcode.CodeNotFound, "device not found")
		}
		return nil, err
	}
	if deviceInfo == nil {
		// Keep the not-found path permission-shaped so telemetry reads do not leak whether a device
		// exists outside the caller's tenant/share visibility.
		return nil, errcode.NewWithMessage(errcode.CodeNoPermission, permissionMessage)
	}
	return deviceInfo, nil
}

func hasTelemetryTenantAccess(deviceInfo *model.Device, claims *utils.UserClaims, allowSharedRead bool) bool {
	if deviceInfo == nil || claims == nil {
		return false
	}
	if claims.Authority == constant.SYS_ADMIN {
		return true
	}
	if deviceInfo.TenantID == claims.TenantID {
		if claims.Authority == constant.TENANT_USER {
			if deviceOwnerMatchesClaims(deviceInfo, claims) {
				return true
			}
			if !allowSharedRead {
				return false
			}
			_, ok := rdiShareRecipientForUser(deviceInfo, claims)
			return ok
		}
		return true
	}
	if !allowSharedRead {
		return false
	}
	_, ok := rdiShareRecipientForUser(deviceInfo, claims)
	return ok
}

func ensureTelemetryDeviceAccess(
	deviceID string,
	claims *utils.UserClaims,
	permissionMessage string,
	allowSharedRead bool,
) (telemetryAccessContext, error) {
	deviceInfo, err := loadTelemetryDeviceForAccess(deviceID, claims, permissionMessage)
	if err != nil {
		return telemetryAccessContext{}, err
	}
	if !hasTelemetryTenantAccess(deviceInfo, claims, allowSharedRead) {
		return telemetryAccessContext{}, errcode.NewWithMessage(errcode.CodeNoPermission, permissionMessage)
	}
	return telemetryAccessContext{
		device:            deviceInfo,
		permissionMessage: permissionMessage,
	}, nil
}

func ensureTelemetryDeviceReadAccess(deviceID string, claims *utils.UserClaims) (*model.Device, error) {
	accessContext, err := ensureTelemetryDeviceAccess(deviceID, claims, telemetryReadPermissionMessage, true)
	if err != nil {
		return nil, err
	}
	return accessContext.device, nil
}

func ensureTelemetryDeviceWriteAccess(deviceID string, claims *utils.UserClaims) (*model.Device, error) {
	accessContext, err := ensureTelemetryDeviceAccess(deviceID, claims, telemetryWritePermissionMessage, false)
	if err != nil {
		return nil, err
	}
	return accessContext.device, nil
}
