// 文件用途：维护 plugin\auth\hooks.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package auth

import (
	"context"

	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
	"github.com/DrmagicE/gmqtt/server"
)

func (a *Auth) HookWrapper() server.HookWrapper {
	return server.HookWrapper{
		OnBasicAuthWrapper: a.OnBasicAuthWrapper,
	}
}

func (a *Auth) OnBasicAuthWrapper(pre server.OnBasicAuth) server.OnBasicAuth {
	return func(ctx context.Context, client server.Client, req *server.ConnectRequest) (err error) {
		err = pre(ctx, client, req)
		if err != nil {
			return err
		}
		ok, err := a.validate(string(req.Connect.Username), string(req.Connect.Password))
		if err != nil {
			return err
		}
		if !ok {
			log.Debug("authentication failed", zap.String("username", string(req.Connect.Username)))
			v := client.Version()
			if packets.IsVersion3X(v) {
				return &codes.Error{
					Code: codes.V3NotAuthorized,
				}
			}
			if packets.IsVersion5(v) {
				return &codes.Error{
					Code: codes.NotAuthorized,
				}
			}
		}
		return nil
	}
}
