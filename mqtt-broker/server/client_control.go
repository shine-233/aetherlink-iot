// 文件用途：承载单客户端的轻量控制包处理，包括 PINGREQ、重认证和主动 DISCONNECT。
// 核心逻辑：保持 keepalive 响应、MQTT v5 re-auth hook 调用、DISCONNECT session expiry 约束和 will message 抑制。
// 使用注意：这些 handler 由 `client_dispatch.go` 调用，返回码和出站包顺序属于 MQTT wire-level 兼容边界。
// 重构建议：后续如继续拆断连/will 生命周期，应与 `server_session_lifecycle.go` 的 will 清理路径一起审查。
package server

import (
	"context"

	"go.uber.org/zap"

	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

func (client *client) pingreqHandler(pingreq *packets.Pingreq) {
	resp := pingreq.NewPingresp()
	client.write(resp)
}

func (client *client) reAuthHandler(auth *packets.Auth) *codes.Error {
	srv := client.server
	// 默认认证成功；插件要求继续认证时才返回 ContinueAuthentication。
	code := codes.Success
	var resp *AuthResponse
	var err error
	if srv.hooks.OnReAuth != nil {
		resp, err = srv.hooks.OnReAuth(context.Background(), client, auth)
		ce := converError(err)
		if ce != nil {
			return ce
		}
	} else {
		return codes.ErrProtocol
	}
	if resp.Continue {
		code = codes.ContinueAuthentication
	}
	client.write(&packets.Auth{
		Code: code,
		Properties: &packets.Properties{
			AuthMethod: client.opts.AuthMethod,
			AuthData:   resp.AuthData,
		},
	})
	return nil
}

// disconnectHandler 处理客户端主动 DISCONNECT，并在正常断连时抑制 will message。
// 使用注意：MQTT v5 不允许原本立即过期的 session 在 DISCONNECT 中被提升为非零过期时间。
func (client *client) disconnectHandler(dis *packets.Disconnect) *codes.Error {
	if client.version == packets.Version5 {
		disExpiry := convertUint32(dis.Properties.SessionExpiryInterval, 0)
		sess, err := client.server.sessionStore.Get(client.opts.ClientID)
		if err != nil {
			return &codes.Error{
				Code: codes.UnspecifiedError,
				ErrorDetails: codes.ErrorDetails{
					ReasonString: []byte(err.Error()),
				},
			}
		}
		if sess.ExpiryInterval == 0 && disExpiry != 0 {
			return &codes.Error{
				Code: codes.ProtocolError,
			}
		}
		if disExpiry != 0 {
			err := client.server.sessionStore.SetSessionExpiry(sess.ClientID, disExpiry)
			if err != nil {
				zaplog.Error("fail to set session expiry",
					zap.String("client_id", client.opts.ClientID),
					zap.Error(err))
			}
		}
	}
	client.disconnect = dis
	// 正常 DISCONNECT 不发送 will message。
	client.cleanWillFlag = true
	return nil
}
