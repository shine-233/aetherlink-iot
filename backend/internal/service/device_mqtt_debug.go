// 文件用途：把设备权限与独立 MQTT 调试 Runtime 连接起来。
// 核心逻辑：每次操作都重新验证设备写权限，再以 claims/device 生成不可伪造的 session scope。
// 关键注意事项：调试 Runtime 不复用生产 MQTT adapter，也不向响应暴露 broker 凭据。
package service

import (
	"context"
	"errors"
	"strings"
	"sync"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/mqttdebug"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

var deviceMQTTDebugRuntime struct {
	sync.RWMutex
	runtime mqttdebug.Runtime
}

func SetDeviceMQTTDebugRuntime(runtime mqttdebug.Runtime) {
	deviceMQTTDebugRuntime.Lock()
	deviceMQTTDebugRuntime.runtime = runtime
	deviceMQTTDebugRuntime.Unlock()
}

func getDeviceMQTTDebugRuntime() mqttdebug.Runtime {
	deviceMQTTDebugRuntime.RLock()
	defer deviceMQTTDebugRuntime.RUnlock()
	return deviceMQTTDebugRuntime.runtime
}

func (*DeviceDebug) OpenMQTTDebugSession(ctx context.Context, deviceID string, claims *utils.UserClaims) (mqttdebug.Snapshot, error) {
	runtime, scope, platformDeviceOnline, err := prepareDeviceMQTTDebugRuntime(deviceID, claims)
	if err != nil {
		return mqttdebug.Snapshot{}, err
	}
	snapshot, err := runtime.Open(ctx, scope)
	return decorateDeviceMQTTDebugSnapshot(snapshot, platformDeviceOnline), mapDeviceMQTTDebugError(err)
}

func (*DeviceDebug) ApplyMQTTDebugCommand(ctx context.Context, deviceID, sessionID string, req *model.DeviceMQTTDebugCommandReq, claims *utils.UserClaims) (mqttdebug.Snapshot, error) {
	if req == nil {
		return mqttdebug.Snapshot{}, errcode.NewWithMessage(errcode.CodeParamError, "mqtt debug command is required")
	}
	runtime, scope, platformDeviceOnline, err := prepareDeviceMQTTDebugRuntime(deviceID, claims)
	if err != nil {
		return mqttdebug.Snapshot{}, err
	}
	snapshot, err := runtime.Apply(ctx, scope, strings.TrimSpace(sessionID), mqttdebug.Command{
		Action:  req.Action,
		Topic:   req.Topic,
		QoS:     req.QoS,
		Payload: req.Payload,
	})
	return decorateDeviceMQTTDebugSnapshot(snapshot, platformDeviceOnline), mapDeviceMQTTDebugError(err)
}

func (*DeviceDebug) GetMQTTDebugSession(ctx context.Context, deviceID, sessionID string, req *model.DeviceMQTTDebugSnapshotReq, claims *utils.UserClaims) (mqttdebug.Snapshot, error) {
	runtime, scope, platformDeviceOnline, err := prepareDeviceMQTTDebugRuntime(deviceID, claims)
	if err != nil {
		return mqttdebug.Snapshot{}, err
	}
	afterSequence := int64(0)
	limit := 100
	if req != nil {
		afterSequence = req.AfterSequence
		if req.Limit > 0 {
			limit = req.Limit
		}
	}
	snapshot, err := runtime.Snapshot(ctx, scope, strings.TrimSpace(sessionID), afterSequence, limit)
	return decorateDeviceMQTTDebugSnapshot(snapshot, platformDeviceOnline), mapDeviceMQTTDebugError(err)
}

func (*DeviceDebug) CloseMQTTDebugSession(ctx context.Context, deviceID, sessionID string, claims *utils.UserClaims) error {
	runtime, scope, _, err := prepareDeviceMQTTDebugRuntime(deviceID, claims)
	if err != nil {
		return err
	}
	return mapDeviceMQTTDebugError(runtime.Close(ctx, scope, strings.TrimSpace(sessionID)))
}

func prepareDeviceMQTTDebugRuntime(deviceID string, claims *utils.UserClaims) (mqttdebug.Runtime, mqttdebug.Scope, bool, error) {
	if !canUseDeviceMQTTDebug(claims) {
		return nil, mqttdebug.Scope{}, false, errcode.NewWithMessage(errcode.CodeNoPermission, "authenticated platform role is required for mqtt debugging")
	}
	device, err := ensureTelemetryDeviceWriteAccess(strings.TrimSpace(deviceID), claims)
	if err != nil {
		return nil, mqttdebug.Scope{}, false, err
	}
	runtime := getDeviceMQTTDebugRuntime()
	if runtime == nil {
		return nil, mqttdebug.Scope{}, false, errcode.NewWithMessage(errcode.CodeSystemError, "mqtt debug runtime is unavailable")
	}
	return runtime, mqttdebug.Scope{
		TenantID:     device.TenantID,
		UserID:       claims.ID,
		DeviceID:     device.ID,
		DeviceNumber: device.DeviceNumber,
	}, device.IsOnline == 1, nil
}

func decorateDeviceMQTTDebugSnapshot(snapshot mqttdebug.Snapshot, platformDeviceOnline bool) mqttdebug.Snapshot {
	snapshot.PlatformDeviceOnline = platformDeviceOnline
	return snapshot
}

func canUseDeviceMQTTDebug(claims *utils.UserClaims) bool {
	if claims == nil || strings.TrimSpace(claims.ID) == "" {
		return false
	}
	switch claims.Authority {
	case constant.SYS_ADMIN, constant.TENANT_ADMIN, constant.TENANT_USER:
		return true
	default:
		return false
	}
}

func mapDeviceMQTTDebugError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, mqttdebug.ErrSessionScope), errors.Is(err, mqttdebug.ErrTopicDenied):
		return errcode.NewWithMessage(errcode.CodeNoPermission, err.Error())
	case errors.Is(err, mqttdebug.ErrSessionNotFound):
		return errcode.NewWithMessage(errcode.CodeNotFound, err.Error())
	case errors.Is(err, mqttdebug.ErrRateLimited), errors.Is(err, mqttdebug.ErrSessionCapacity):
		return errcode.NewWithMessage(errcode.CodeRateLimit, err.Error())
	case errors.Is(err, mqttdebug.ErrInvalidCommand), errors.Is(err, mqttdebug.ErrInvalidTopic):
		return errcode.NewWithMessage(errcode.CodeParamError, err.Error())
	default:
		return errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
	}
}
