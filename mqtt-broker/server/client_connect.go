// 文件用途：承载单个 MQTT 客户端的 CONNECT/AUTH 握手流程。
// 核心逻辑：处理连接超时、普通认证、MQTT v5 增强认证多轮 AUTH、连接参数协商和 CONNACK 写出。
// 使用注意：该文件必须保持 CONNECT 阶段的包消费顺序、错误码、CONNACK 属性和 connected channel 关闭时机不变。
// 重构建议：后续若继续拆分，应先补齐 CONNECT v3/v5、增强认证继续/失败、空 ClientID 和 session resume 的协议回归用例。
package server

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/DrmagicE/gmqtt/pkg/codes"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

var (
	ErrConnectTimeOut = errors.New("connect time out")
)

// clientConnectTimeout 是 var 而不是 const，现有测试会临时缩短握手超时时间。
var clientConnectTimeout = 5 * time.Second

func sendErrConnack(cli *client, err error) {
	codeErr := converError(err)
	// Override the error code if it is invalid for V3 client.
	if packets.IsVersion3X(cli.version) && codeErr.Code > codes.V3NotAuthorized {
		codeErr.Code = codes.NotAuthorized
	}
	cli.out <- &packets.Connack{
		Version:    cli.version,
		Code:       codeErr.Code,
		Properties: getErrorProperties(cli, &codeErr.ErrorDetails),
	}
}

type connectAuthState struct {
	conn     *packets.Connect
	authOpts *AuthOptions
	onAuth   OnAuth
}

type connectAuthResult struct {
	code     codes.Code
	authData []byte
}

type connectAuthAction int

const (
	connectAuthActionContinue connectAuthAction = iota
	connectAuthActionComplete
)

type connectAuthDecision struct {
	action   connectAuthAction
	authData []byte
}

func (client *client) connectWithTimeOut() (ok bool) {
	// CONNECT 阶段必须在超时时间内完成普通认证或 MQTT v5 增强认证，否则直接关闭连接。
	var err error
	defer func() {
		if err != nil {
			client.setError(err)
			ok = false
		} else {
			ok = true
		}
		close(client.connected)
	}()
	connectCtx, cancelConnect := context.WithTimeout(context.Background(), clientConnectTimeout)
	defer cancelConnect()
	authState := &connectAuthState{}

	for {
		select {
		case p := <-client.in:
			shouldReturn, nextErr := client.processConnectPacket(connectCtx, p, authState)
			err = nextErr
			if shouldReturn {
				return
			}
		case <-connectCtx.Done():
			err = ErrConnectTimeOut
			return
		}
	}
}

func (client *client) processConnectPacket(ctx context.Context, p packets.Packet, state *connectAuthState) (bool, error) {
	if p == nil {
		return true, nil
	}
	decision, err := client.handleConnectAuthFlow(ctx, p, state)
	if err != nil {
		client.handleConnectFailure(err)
		return true, err
	}
	err = client.applyConnectDecision(decision, state)
	return decision.action == connectAuthActionComplete || err != nil, err
}

func (client *client) handleConnectAuthFlow(ctx context.Context, p packets.Packet, state *connectAuthState) (connectAuthDecision, error) {
	authResult, err := client.handleConnectAuthPacket(ctx, p, state)
	if err != nil {
		return connectAuthDecision{}, err
	}
	return client.connectAuthDecision(authResult), nil
}

func (client *client) connectAuthDecision(result connectAuthResult) connectAuthDecision {
	if result.code == codes.ContinueAuthentication {
		return connectAuthDecision{
			action:   connectAuthActionContinue,
			authData: result.authData,
		}
	}
	return connectAuthDecision{action: connectAuthActionComplete}
}

func (client *client) applyConnectDecision(decision connectAuthDecision, state *connectAuthState) error {
	switch decision.action {
	case connectAuthActionContinue:
		client.sendContinueAuthentication(state.conn, decision.authData)
		return nil
	case connectAuthActionComplete:
		return client.completeAuthenticatedConnect(state.conn, state.authOpts)
	default:
		return nil
	}
}

func (client *client) handleConnectFailure(err error) {
	if errors.Is(err, ErrConnectTimeOut) {
		return
	}
	sendErrConnack(client, err)
}

func (client *client) handleConnectAuthPacket(ctx context.Context, p packets.Packet, state *connectAuthState) (connectAuthResult, error) {
	switch pkt := p.(type) {
	case *packets.Connect:
		return client.handleInitialConnect(ctx, pkt, state)
	case *packets.Auth:
		return client.handleAuthContinue(ctx, pkt, state)
	default:
		return connectAuthResult{}, &codes.Error{
			Code: codes.MalformedPacket,
		}
	}
}

func (client *client) handleInitialConnect(ctx context.Context, conn *packets.Connect, state *connectAuthState) (connectAuthResult, error) {
	if state.conn != nil {
		return connectAuthResult{}, codes.ErrProtocol
	}
	state.conn = conn
	authOpts, resp, err := client.connectHandler(ctx, conn)
	state.authOpts = authOpts
	if err != nil {
		return connectAuthResult{}, connectHookError(ctx, err)
	}
	if resp != nil && resp.Continue {
		// 增强认证进入多轮 AUTH，后续 AUTH 包必须复用同一个 onAuth 回调。
		state.onAuth = resp.OnAuth
		return connectAuthResult{
			code:     codes.ContinueAuthentication,
			authData: resp.AuthData,
		}, nil
	}
	return connectAuthResult{code: codes.Success}, nil
}

func (client *client) handleAuthContinue(ctx context.Context, auth *packets.Auth, state *connectAuthState) (connectAuthResult, error) {
	if state.conn == nil || packets.IsVersion3X(client.version) {
		return connectAuthResult{}, codes.ErrProtocol
	}
	if state.onAuth == nil {
		return connectAuthResult{}, codes.ErrProtocol
	}
	if auth.Code != codes.ContinueAuthentication {
		return connectAuthResult{}, codes.ErrProtocol
	}

	authResp, err := client.authHandler(ctx, auth, state.authOpts, state.onAuth)
	if err != nil {
		return connectAuthResult{}, connectHookError(ctx, err)
	}
	if authResp.Continue {
		return connectAuthResult{
			code:     codes.ContinueAuthentication,
			authData: authResp.AuthData,
		}, nil
	}
	return connectAuthResult{code: codes.Success}, nil
}

func connectHookError(ctx context.Context, err error) error {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return ErrConnectTimeOut
	}
	return err
}

func (client *client) sendContinueAuthentication(conn *packets.Connect, authData []byte) {
	client.out <- &packets.Auth{
		Code: codes.ContinueAuthentication,
		Properties: &packets.Properties{
			AuthMethod: conn.Properties.AuthMethod,
			AuthData:   authData,
		},
	}
}

func (client *client) completeAuthenticatedConnect(conn *packets.Connect, authOpts *AuthOptions) error {
	connackPpt := client.applyConnectOptions(conn, authOpts)
	client.setConnectReadDeadline()
	client.newPacketIDLimiter(client.opts.MaxInflight)
	return client.registerAndSendConnack(conn, connackPpt)
}

// applyConnectOptions 合并认证结果、服务端配置与 CONNECT 属性，生成后续读写循环使用的协商参数。
// 审查建议：该函数是 connect auth flow 的关键边界，后续调整时应配合 CONNACK 属性回归用例一起锁定行为。
func (client *client) applyConnectOptions(conn *packets.Connect, authOpts *AuthOptions) *packets.Properties {
	client.opts.RetainAvailable = authOpts.RetainAvailable
	client.opts.WildcardSubAvailable = authOpts.WildcardSubAvailable
	client.opts.SubIDAvailable = authOpts.SubIDAvailable
	client.opts.SharedSubAvailable = authOpts.SharedSubAvailable
	client.opts.SessionExpiry = authOpts.SessionExpiry
	client.opts.MaxInflight = authOpts.MaxInflight
	client.opts.ReceiveMax = authOpts.ReceiveMax
	client.opts.ClientMaxPacketSize = math.MaxUint32 // unlimited
	client.opts.ServerMaxPacketSize = authOpts.MaxPacketSize
	client.opts.ServerTopicAliasMax = authOpts.TopicAliasMax
	client.opts.Username = string(conn.Username)

	client.applyConnectClientID(conn, authOpts)

	if client.version == packets.Version5 {
		return client.applyConnectV5Options(conn, authOpts)
	}
	client.opts.KeepAlive = conn.KeepAlive
	return nil
}

func (client *client) applyConnectClientID(conn *packets.Connect, authOpts *AuthOptions) {
	if len(conn.ClientID) == 0 {
		if len(authOpts.AssignedClientID) != 0 {
			client.opts.ClientID = string(authOpts.AssignedClientID)
		} else {
			client.opts.ClientID = getRandomUUID()
			authOpts.AssignedClientID = []byte(client.opts.ClientID)
		}
	} else {
		client.opts.ClientID = string(conn.ClientID)
	}
}

func (client *client) applyConnectV5Options(conn *packets.Connect, authOpts *AuthOptions) *packets.Properties {
	client.opts.MaxInflight = convertUint16(conn.Properties.ReceiveMaximum, client.opts.MaxInflight)
	client.opts.ClientMaxPacketSize = convertUint32(conn.Properties.MaximumPacketSize, client.opts.ClientMaxPacketSize)
	client.opts.ClientTopicAliasMax = convertUint16(conn.Properties.TopicAliasMaximum, client.opts.ClientTopicAliasMax)
	client.opts.AuthMethod = conn.Properties.AuthMethod
	client.serverReceiveMaximumQuota = client.opts.ReceiveMax
	client.opts.KeepAlive = authOpts.KeepAlive

	var maxQoS byte
	if authOpts.MaximumQoS >= 2 {
		maxQoS = byte(1)
	} else {
		maxQoS = byte(0)
	}

	return &packets.Properties{
		SessionExpiryInterval: &authOpts.SessionExpiry,
		ReceiveMaximum:        &authOpts.ReceiveMax,
		MaximumQoS:            &maxQoS,
		RetainAvailable:       bool2Byte(authOpts.RetainAvailable),
		TopicAliasMaximum:     &authOpts.TopicAliasMax,
		WildcardSubAvailable:  bool2Byte(authOpts.WildcardSubAvailable),
		SubIDAvailable:        bool2Byte(authOpts.SubIDAvailable),
		SharedSubAvailable:    bool2Byte(authOpts.SharedSubAvailable),
		MaximumPacketSize:     &authOpts.MaxPacketSize,
		ServerKeepAlive:       &authOpts.KeepAlive,
		AssignedClientID:      authOpts.AssignedClientID,
		ResponseInfo:          authOpts.ResponseInfo,
	}
}

func (client *client) setConnectReadDeadline() {
	if keepAlive := client.opts.KeepAlive; keepAlive != 0 {
		_ = client.rwc.SetReadDeadline(time.Now().Add(time.Duration(keepAlive/2+keepAlive) * time.Second))
	}
}

func (client *client) registerAndSendConnack(conn *packets.Connect, connackPpt *packets.Properties) error {
	sessionResume, err := client.register(conn, client)
	if err != nil {
		sendErrConnack(client, err)
		return err
	}
	connack := conn.NewConnackPacket(codes.Success, sessionResume)
	if conn.Version == packets.Version5 {
		connack.Properties = connackPpt
	}
	client.write(connack)
	return nil
}

func (client *client) basicAuth(ctx context.Context, conn *packets.Connect, authOpts *AuthOptions) (err error) {
	srv := client.server
	if srv.hooks.OnBasicAuth == nil {
		if client.config.MQTT.AllowAnonymous {
			return nil
		}
		return &codes.Error{
			Code: codes.NotAuthorized,
		}
	}
	return srv.hooks.OnBasicAuth(ctx, client, &ConnectRequest{
		Connect: conn,
		Options: authOpts,
	})
}

func (client *client) enhancedAuth(ctx context.Context, conn *packets.Connect, authOpts *AuthOptions) (resp *EnhancedAuthResponse, err error) {
	srv := client.server
	if srv.hooks.OnEnhancedAuth == nil {
		return nil, errors.New("OnEnhancedAuth hook is nil")
	}

	resp, err = srv.hooks.OnEnhancedAuth(ctx, client, &ConnectRequest{
		Connect: conn,
		Options: authOpts,
	})
	if err == nil && resp == nil {
		err = errors.New("return nil response from OnEnhancedAuth hook")
	}
	return resp, err
}

func (client *client) connectHandler(ctx context.Context, conn *packets.Connect) (authOpts *AuthOptions, enhancedResp *EnhancedAuthResponse, err error) {
	if !client.config.MQTT.AllowZeroLenClientID && len(conn.ClientID) == 0 {
		err = &codes.Error{
			Code: codes.ClientIdentifierNotValid,
		}
		return
	}
	client.version = conn.Version
	// 先落服务端默认值，再允许 basic/enhanced auth hook 收窄能力或补充分配 ClientID。
	authOpts = client.defaultAuthOptions(conn)

	if packets.IsVersion3X(client.version) || (packets.IsVersion5(client.version) && conn.Properties.AuthMethod == nil) {
		err = client.basicAuth(ctx, conn, authOpts)
	}
	if client.version == packets.Version5 && conn.Properties.AuthMethod != nil {
		enhancedResp, err = client.enhancedAuth(ctx, conn, authOpts)
	}

	return
}

func (client *client) authHandler(ctx context.Context, auth *packets.Auth, authOpts *AuthOptions, onAuth OnAuth) (resp *AuthResponse, err error) {
	authResp, err := onAuth(ctx, client, &AuthRequest{
		Auth:    auth,
		Options: authOpts,
	})
	if err == nil && authResp == nil {
		return nil, errors.New("return nil response from OnAuth hook")
	}
	return authResp, err
}

func (client *client) defaultAuthOptions(connect *packets.Connect) *AuthOptions {
	opts := &AuthOptions{
		SessionExpiry:        uint32(client.config.MQTT.SessionExpiry.Seconds()),
		ReceiveMax:           client.config.MQTT.ReceiveMax,
		MaximumQoS:           client.config.MQTT.MaximumQoS,
		MaxPacketSize:        client.config.MQTT.MaxPacketSize,
		TopicAliasMax:        client.config.MQTT.TopicAliasMax,
		RetainAvailable:      client.config.MQTT.RetainAvailable,
		WildcardSubAvailable: client.config.MQTT.WildcardAvailable,
		SubIDAvailable:       client.config.MQTT.SubscriptionIDAvailable,
		SharedSubAvailable:   client.config.MQTT.SharedSubAvailable,
		KeepAlive:            client.config.MQTT.MaxKeepAlive,
		MaxInflight:          client.config.MQTT.MaxInflight,
	}
	if connect.KeepAlive < opts.KeepAlive {
		opts.KeepAlive = connect.KeepAlive
	}
	if client.version == packets.Version5 {
		if i := connect.Properties.SessionExpiryInterval; i == nil {
			opts.SessionExpiry = 0
		} else if *i < opts.SessionExpiry {
			opts.SessionExpiry = *i

		}
	}
	return opts
}
