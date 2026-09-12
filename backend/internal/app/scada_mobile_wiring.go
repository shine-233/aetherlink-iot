// 文件用途：P1.3 / P1.4 的启动装配选项——把 SCADA 控制服务与移动端服务接到
// GroupApp 上，使对应 HTTP 接口从"存在但 fail closed"变成"真的可用"。
// 核心逻辑：读取 scada.control.* 配置，构造 Widget 注册表、二次确认签发器与
// 真实命令下发执行器，然后赋给 service.GroupApp.ScadaControl / .Mobile。
//
// 关键注意事项：
//  1. 本选项必须排在 WithDatabase() 之后——装配本身不查库，但下发链路依赖
//     global.DB 与设备缓存，先于数据库装配会让"接线成功"建立在没就绪的依赖上。
//  2. 二次确认密钥未配置**不阻断启动**，但要打 warn：需要确认的命令会被拒。
//     选择 warn 而不是 fail-fast，是因为未启用 SCADA 的部署不该被一个用不到的
//     密钥挡在门外；代价是必须让运维在日志里看见这句话。
//  3. 装配失败（注册表校验不过）必须阻断启动：那属于代码缺陷，不是配置缺失，
//     带着坏注册表启动会让所有控件都判为"未注册"，症状和"没配 Widget"一样，无从排查。
package app

import (
	"context"
	"fmt"
	"time"

	service "aetherlink-iot/backend/internal/service"

	"github.com/sirupsen/logrus"
)

// SCADA / 移动端装配使用的配置键。
// 环境变量：GOTP_SCADA_CONTROL_CONFIRMATION_SECRET（viper 前缀 GOTP，"."→"_"）。
const (
	scadaControlConfirmationSecretKey     = "scada.control.confirmation_secret"
	scadaControlConfirmationTTLKey        = "scada.control.confirmation_ttl_seconds"
	scadaControlConfirmationSecretEnvHint = "GOTP_SCADA_CONTROL_CONFIRMATION_SECRET"

	// 推送凭据（FCM / APNs）。两者都留空 = 推送投递未接线。
	pushFCMProjectIDKey      = "push.providers.fcm.project_id"
	pushFCMServiceAccountKey = "push.providers.fcm.service_account_json"

	pushAPNsKeyIDKey      = "push.providers.apns.key_id"
	pushAPNsTeamIDKey     = "push.providers.apns.team_id"
	pushAPNsTopicKey      = "push.providers.apns.topic"
	pushAPNsPrivateKeyKey = "push.providers.apns.private_key_p8"
	pushAPNsSandboxKey    = "push.providers.apns.sandbox"
)

// defaultConfirmationTTL 二次确认令牌默认有效期。
const defaultConfirmationTTL = 5 * time.Minute

// WithScadaMobileWiring 装配 SCADA 控制与移动端服务。
func WithScadaMobileWiring() Option {
	return func(app *Application) error {
		secret := ""
		ttl := defaultConfirmationTTL
		if app.Config != nil {
			secret = app.Config.GetString(scadaControlConfirmationSecretKey)
			if seconds := app.Config.GetInt(scadaControlConfirmationTTLKey); seconds > 0 {
				ttl = time.Duration(seconds) * time.Second
			}
		}

		control, issuerConfigured, err := service.AssembleScadaControl(service.ScadaControlWiring{
			ConfirmationSecret: secret,
			ConfirmationTTL:    ttl,
		})
		if err != nil {
			return fmt.Errorf("scada control wiring failed: %w", err)
		}
		service.GroupApp.ScadaControl = control

		if !issuerConfigured {
			logrus.Warnf(
				"SCADA control confirmation secret is not configured (%s / env %s); "+
					"commands that require confirmation will be refused, and no token can be issued",
				scadaControlConfirmationSecretKey, scadaControlConfirmationSecretEnvHint,
			)
		}

		// 推送：只有真的配了 FCM 凭据才注入。没配就保持 nil，
		// 能力矩阵报 Push=false——注入一个空壳 Provider 只会得到"报成功、发不出"。
		pushCfg := service.PushWiringConfig{}
		if app.Config != nil {
			pushCfg.FCM = service.FCMConfig{
				ProjectID:          app.Config.GetString(pushFCMProjectIDKey),
				ServiceAccountJSON: app.Config.GetString(pushFCMServiceAccountKey),
			}
			pushCfg.APNs = service.APNsConfig{
				KeyID:        app.Config.GetString(pushAPNsKeyIDKey),
				TeamID:       app.Config.GetString(pushAPNsTeamIDKey),
				Topic:        app.Config.GetString(pushAPNsTopicKey),
				PrivateKeyP8: app.Config.GetString(pushAPNsPrivateKeyKey),
				Sandbox:      app.Config.GetBool(pushAPNsSandboxKey),
			}
		}
		push, pushConfigured, err := service.AssemblePush(pushCfg)
		if err != nil {
			return fmt.Errorf("push wiring failed: %w", err)
		}
		if !pushConfigured {
			logrus.Infof(
				"push delivery is not wired (no fcm/apns credentials); " +
					"push tokens can still be registered but nothing will be delivered",
			)
		}

		service.GroupApp.Mobile = service.AssembleMobile(push)

		caps := service.GroupApp.Mobile.Capabilities(context.Background())
		logrus.Infof(
			"scada/mobile wired: confirmation_issuer=%t mobile_capabilities(telemetry=%t commands=%t alarms=%t shadow=%t ota=%t dashboards=%t push=%t)",
			issuerConfigured, caps.Telemetry, caps.Commands, caps.Alarms, caps.Shadow, caps.OTA, caps.Dashboards, caps.Push,
		)
		return nil
	}
}
