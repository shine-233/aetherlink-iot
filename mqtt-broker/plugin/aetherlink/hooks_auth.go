package aetherlink

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/DrmagicE/gmqtt/server"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type mqttVoucherPayload struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

// mqttAuthenticatedClientBindings 以 server.Client 为 key 保存已认证设备绑定。
//
// 生命周期（两阶段）：
//   - OnBasicAuth 成功：写入 pending 绑定（pendingSince=认证时间）；
//   - OnConnected：promoteMQTTAuthenticatedClientBinding 通过 CAS 转为已连接绑定；
//   - OnClosed / 会话吊销 / 路由校验失败：forgetMQTTAuthenticatedClientBinding 删除。
//
// gmqtt 的 client.internalClose 只对到达 Connected 状态的连接触发 OnClosed。
// 认证通过但随后建连失败的连接（外层认证插件拒绝、register 中 sessionStore.Get 出错、
// CONNECT 阶段中断等）永远不会触发 OnClosed，若无兜底，其绑定连同整个 *client
// 会常驻在这张全局表里。这类条目保持 pending 状态，由
// maybeSweepStalePendingMQTTClientBindings 在后续认证时按 TTL 节流回收。
// 已连接条目只由 OnClosed 回收，兜底扫描绝不触碰，避免误删在线会话的 ACL 绑定。
var mqttAuthenticatedClientBindings sync.Map

type mqttAuthenticatedClientBinding struct {
	deviceID           string
	deviceStateVersion time.Time
	// pendingSince 为认证通过但尚未收到 OnConnected 的时间；零值表示已连接
	// （或由旧实例/测试直接写入），兜底扫描只回收非零且超过 TTL 的条目。
	pendingSince time.Time
}

const (
	// mqttPendingClientBindingTTL 远大于 gmqtt CONNECT 超时（5s），
	// 正常建连的 pending 窗口只有毫秒级，不会被误回收。
	mqttPendingClientBindingTTL = 2 * time.Minute
	// mqttPendingClientBindingSweepInterval 限制全表扫描频率，扫描成本只摊到少数认证请求上。
	mqttPendingClientBindingSweepInterval = 30 * time.Second
)

var (
	mqttClientBindingNow             = time.Now
	mqttPendingBindingLastSweepNanos atomic.Int64
)

func (t *AetherLinkPlugin) OnBasicAuthWrapper(pre server.OnBasicAuth) server.OnBasicAuth {
	return func(ctx context.Context, client server.Client, req *server.ConnectRequest) (err error) {
		remoteIP := mqttAuthRemoteIP(client)
		// 入口检查：窗口内失败已达上限的来源 IP 在进入认证链路前直接拒绝，
		// 拒绝本身不再计数，避免持续轰炸把封禁时间无限延长。
		if !mqttAuthRateLimiter.allow(remoteIP) {
			Log.Debug(
				"mqtt auth rate limited",
				zap.String("remote_ip", remoteIP),
				zap.String("client_id", string(req.Connect.ClientID)),
			)
			return errMQTTAuthRateLimited
		}

		err = pre(ctx, client, req)
		if err != nil {
			mqttAuthRateLimiter.record(remoteIP)
			Log.Error(err.Error())
			return err
		}

		username := string(req.Connect.Username)
		password := string(req.Connect.Password)
		clientID := string(req.Connect.ClientID)
		if handled, err := authenticateMQTTSystemUser(username, password); handled {
			if err == nil {
				// 系统账号认证成功也记录 clientID→username 绑定，供 will 钩子区分系统与设备。
				if bindErr := rememberMQTTClientUsername(clientID, username); bindErr != nil {
					Log.Warn("failed to remember mqtt system user binding",
						zap.String("client_id", clientID), zap.Error(bindErr))
				}
			} else {
				// 系统账号（root/plugin）密码错误同样计入来源 IP 失败次数。
				mqttAuthRateLimiter.record(remoteIP)
			}
			return err
		}

		logMQTTAuthStart(username, clientID)

		voucher, err := buildMQTTVoucher(username, password)
		if err != nil {
			mqttAuthRateLimiter.record(remoteIP)
			return err
		}
		device, err := GetDeviceByVoucher(voucher)
		if err != nil {
			mqttAuthRateLimiter.record(remoteIP)
			handleMQTTAuthFailure(username, password, clientID, err)
			return err
		}
		if err := ensureMQTTDeviceActive(device); err != nil {
			forgetMQTTDeviceLookup(voucher, device)
			mqttAuthRateLimiter.record(remoteIP)
			handleMQTTAuthFailure(username, password, clientID, err)
			return err
		}

		// TB-17R：认证通过前做租户传输日配额判定（被拒连接同样计入当日用量，见 transport_quota.go）。
		// 配额拒绝是商业约束而非凭据失败：不计入来源 IP 认证限速，也不走 handleMQTTAuthFailure
		// 的凭证失效清理路径（凭证本身是有效的）。
		if !allowMQTTTransportDailyQuota(device.TenantID) {
			Log.Warn("mqtt transport daily quota exceeded",
				zap.String("tenant_id", device.TenantID),
				zap.String("device_id", device.ID),
				zap.String("client_id", clientID),
			)
			recordMQTTDiagnosticEvent(mqttDiagnosticEvent{
				deviceID:  device.ID,
				clientID:  clientID,
				username:  username,
				action:    "auth",
				direction: "na",
				outcome:   "deny",
				error:     errMQTTTransportQuotaExceeded.Error(),
				code:      "transport_quota_exceeded",
			})
			return errMQTTTransportQuotaExceeded
		}

		handleMQTTAuthSuccess(device, username, clientID)
		if bindErr := rememberMQTTClientUsername(clientID, username); bindErr != nil {
			Log.Warn("failed to remember mqtt device user binding",
				zap.String("client_id", clientID), zap.Error(bindErr))
		}
		err = rememberMQTTAuthenticatedDevice(client, clientID, device)
		if err != nil {
			Log.Error(err.Error())
			return err
		}
		return nil
	}
}

func authenticateMQTTSystemUser(username string, providedPassword string) (bool, error) {
	var expectedPassword string
	switch username {
	case "root":
		expectedPassword = viper.GetString("mqtt.password")
	case "plugin":
		expectedPassword = viper.GetString("mqtt.plugin_password")
	default:
		return false, nil
	}

	if providedPassword == expectedPassword {
		return true, nil
	}

	err := errors.New("password error")
	Log.Warn(err.Error())
	return true, err
}

func logMQTTAuthStart(username string, clientID string) {
	Log.Info(
		"mqtt auth start",
		zap.String("username", username),
		zap.String("client_id", clientID),
	)
}

func buildMQTTVoucher(username string, password string) (string, error) {
	if password != "" {
		v, err := json.Marshal(mqttVoucherPayload{
			Username: username,
			Password: password,
		})
		if err != nil {
			return "", err
		}
		return string(v), nil
	}

	v, err := json.Marshal(mqttVoucherPayload{
		Username: username,
	})
	if err != nil {
		return "", err
	}
	return string(v), nil
}

func handleMQTTAuthFailure(username string, password string, clientID string, authErr error) {
	Log.Warn(
		"mqtt auth failed",
		zap.String("client_id", clientID),
		zap.Error(authErr),
	)

	if isMQTTSystemUser(username) || password == "" {
		return
	}

	fb, fbErr := json.Marshal(mqttVoucherPayload{Username: username})
	if fbErr != nil {
		return
	}
	fallbackVoucher := string(fb)
	if dev, derr := GetDeviceByVoucher(fallbackVoucher); derr == nil && dev != nil {
		recordMQTTDiagnosticEvent(mqttDiagnosticEvent{
			deviceID:  dev.ID,
			clientID:  clientID,
			username:  username,
			action:    "auth",
			direction: "na",
			outcome:   "deny",
			error:     authErr.Error(),
			code:      "auth_denied",
		})
	}
}

func handleMQTTAuthSuccess(device *Device, username string, clientID string) {
	Log.Info(
		"mqtt auth passed",
		zap.String("client_id", clientID),
		zap.String("device_id", device.ID),
	)
	recordMQTTDiagnosticEvent(mqttDiagnosticEvent{
		deviceID:  device.ID,
		clientID:  clientID,
		username:  username,
		action:    "auth",
		direction: "na",
		outcome:   "ok",
		code:      "auth_ok",
	})
}

func rememberMQTTAuthenticatedDevice(client server.Client, clientID string, device *Device) error {
	if device == nil {
		return errors.New("mqtt authenticated device is nil")
	}
	deviceID := strings.TrimSpace(device.ID)
	if err := SetStr("mqtt_client_id_"+clientID, deviceID, 48*time.Hour); err != nil {
		return err
	}
	if client != nil {
		binding := mqttAuthenticatedClientBinding{deviceID: deviceID}
		if device.UpdateAt != nil {
			binding.deviceStateVersion = device.UpdateAt.UTC()
		} else if device.CreatedAt != nil {
			binding.deviceStateVersion = device.CreatedAt.UTC()
		}
		storePendingMQTTAuthenticatedClientBinding(client, binding)
	}
	return nil
}

// storePendingMQTTAuthenticatedClientBinding 写入认证阶段绑定，并顺带触发节流兜底扫描。
func storePendingMQTTAuthenticatedClientBinding(client server.Client, binding mqttAuthenticatedClientBinding) {
	now := mqttClientBindingNow()
	binding.pendingSince = now
	// 同一 client 对象若已处于已连接状态（重复认证），保持已连接语义，
	// 否则兜底扫描可能把在线会话的绑定当作 pending 回收。
	if existing, ok := mqttAuthenticatedClientBindings.Load(client); ok {
		if prev, isBinding := existing.(mqttAuthenticatedClientBinding); isBinding && prev.pendingSince.IsZero() {
			binding.pendingSince = time.Time{}
		}
	}
	mqttAuthenticatedClientBindings.Store(client, binding)
	maybeSweepStalePendingMQTTClientBindings(now)
}

// promoteMQTTAuthenticatedClientBinding 在 OnConnected 时把 pending 绑定转为已连接。
// 使用 CAS：若并发的吊销/清理已删除该条目，则不会把它重新写回。
func promoteMQTTAuthenticatedClientBinding(client server.Client) {
	if client == nil {
		return
	}
	for {
		value, ok := mqttAuthenticatedClientBindings.Load(client)
		if !ok {
			return
		}
		binding, isBinding := value.(mqttAuthenticatedClientBinding)
		if !isBinding || binding.pendingSince.IsZero() {
			return
		}
		promoted := binding
		promoted.pendingSince = time.Time{}
		if mqttAuthenticatedClientBindings.CompareAndSwap(client, value, promoted) {
			return
		}
	}
}

// maybeSweepStalePendingMQTTClientBindings 节流回收超过 TTL 仍未建连的 pending 绑定。
// 返回本次回收的条目数；未到扫描间隔或被并发扫描抢先时返回 0。
func maybeSweepStalePendingMQTTClientBindings(now time.Time) int {
	last := mqttPendingBindingLastSweepNanos.Load()
	if last != 0 && now.Sub(time.Unix(0, last)) < mqttPendingClientBindingSweepInterval {
		return 0
	}
	if !mqttPendingBindingLastSweepNanos.CompareAndSwap(last, now.UnixNano()) {
		return 0
	}
	return sweepStalePendingMQTTClientBindings(now)
}

func sweepStalePendingMQTTClientBindings(now time.Time) int {
	removed := 0
	mqttAuthenticatedClientBindings.Range(func(key, value any) bool {
		binding, isBinding := value.(mqttAuthenticatedClientBinding)
		if !isBinding || binding.pendingSince.IsZero() {
			return true
		}
		if now.Sub(binding.pendingSince) < mqttPendingClientBindingTTL {
			return true
		}
		// CompareAndDelete：扫描期间若条目已被 promote 或重新认证覆盖，则保留。
		if mqttAuthenticatedClientBindings.CompareAndDelete(key, value) {
			removed++
		}
		return true
	})
	if removed > 0 && Log != nil {
		Log.Info("swept stale pending mqtt client bindings", zap.Int("removed", removed))
	}
	return removed
}

func mqttAuthenticatedDeviceForClient(client server.Client) (string, bool) {
	binding, ok := mqttAuthenticatedBindingForClient(client)
	return binding.deviceID, ok
}

func mqttAuthenticatedBindingForClient(client server.Client) (mqttAuthenticatedClientBinding, bool) {
	if client == nil {
		return mqttAuthenticatedClientBinding{}, false
	}
	value, ok := mqttAuthenticatedClientBindings.Load(client)
	if !ok {
		return mqttAuthenticatedClientBinding{}, false
	}
	binding, bindingOK := value.(mqttAuthenticatedClientBinding)
	if bindingOK {
		binding.deviceID = strings.TrimSpace(binding.deviceID)
		return binding, binding.deviceID != ""
	}
	// Keep compatibility with bindings created by an older plugin instance during
	// an in-process reload. A zero version is conservatively revocable.
	deviceID, ok := value.(string)
	deviceID = strings.TrimSpace(deviceID)
	return mqttAuthenticatedClientBinding{deviceID: deviceID}, ok && deviceID != ""
}

func forgetMQTTAuthenticatedClientBinding(client server.Client) {
	if client != nil {
		mqttAuthenticatedClientBindings.Delete(client)
	}
}

func forgetMQTTAuthenticatedDevice(clientID string) {
	if strings.TrimSpace(clientID) == "" {
		return
	}
	_ = DelKey("mqtt_client_id_" + clientID)
}

func forgetMQTTDeviceLookup(voucher string, device *Device) {
	if strings.TrimSpace(voucher) != "" {
		_ = DelKey(voucherCacheKey(voucher))
	}
	if device != nil && strings.TrimSpace(device.ID) != "" {
		_ = DelKey(device.ID)
	}
}

func ensureMQTTDeviceActive(device *Device) error {
	if device == nil {
		return errors.New("device not found")
	}
	if !strings.EqualFold(strings.TrimSpace(device.ActivateFlag), "active") ||
		!strings.EqualFold(strings.TrimSpace(device.IsEnabled), "enabled") ||
		strings.TrimSpace(device.TenantID) == "" {
		return errors.New("device is inactive or disabled")
	}
	return nil
}

func loadActiveMQTTDevice(deviceID string) (*Device, error) {
	device, err := GetDeviceById(strings.TrimSpace(deviceID))
	if err != nil {
		return nil, err
	}
	if err := ensureMQTTDeviceActive(device); err != nil {
		return nil, err
	}
	return device, nil
}

func isMQTTSystemUser(username string) bool {
	return username == "root" || username == "plugin"
}
