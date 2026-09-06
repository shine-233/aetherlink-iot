// 文件用途：插件网关核心（PHASE-D-D9）——注册/心跳/上行/下行流。
// 核心逻辑：插件凭 token（sha256 摘要比对）接入；上行经 UplinkSink 缝汇入平台
// uplink 管道（app 装配注入）；下行经 Attach 服务流推送（管理 API 入队）。
// 关键注意事项：凭证仅存摘要；disabled 插件拒绝一切接入；会话退出自动置 offline。
package grpcgateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	model "aetherlink-iot/backend/internal/model"
)

// 契约错误（错误文本即对外语义，保持稳定）。
var (
	ErrPluginUnknown      = errors.New("plugin not registered")
	ErrPluginDisabled     = errors.New("plugin is disabled")
	ErrPluginTokenInvalid = errors.New("plugin token mismatch")
	ErrPluginOffline      = errors.New("plugin session offline")
	ErrSinkNotWired       = errors.New("uplink sink is not wired")
)

// Store 插件注册表读取/状态更新接口（DAL 实现；测试用内存桩）。
type Store interface {
	GetByName(ctx context.Context, name string) (*model.PluginRegistry, error)
	GetByID(ctx context.Context, id string) (*model.PluginRegistry, error)
	UpdateStatusAndHeartbeat(ctx context.Context, id, status string, now time.Time, version *string) error
}

// UplinkSink 插件上行汇入接口（app 装配注入，对接 uplink 管道）。
type UplinkSink interface {
	PublishUplink(frame *UplinkFrame, plugin *model.PluginRegistry) error
}

// pluginSession 一条 Attach 流的下行命令队列。
type pluginSession struct {
	commands chan *DownlinkCommand
}

// Gateway 网关核心。
type Gateway struct {
	store          Store
	sink           UplinkSink
	now            func() time.Time
	downlinkBuffer int

	mu       sync.Mutex
	sessions map[string]*pluginSession
}

// New 创建网关。downlinkBuffer<=0 时取默认 64。
func New(store Store, sink UplinkSink, downlinkBuffer int) *Gateway {
	if downlinkBuffer <= 0 {
		downlinkBuffer = 64
	}
	return &Gateway{
		store:          store,
		sink:           sink,
		now:            time.Now,
		downlinkBuffer: downlinkBuffer,
		sessions:       map[string]*pluginSession{},
	}
}

// SetUplinkSink app 装配注入。
func (g *Gateway) SetUplinkSink(sink UplinkSink) { g.sink = sink }

// HashToken 接入凭证摘要（与注册表存储一致）。
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// lookup 按 id/name 取插件并校验 token（禁用即拒）。
func (g *Gateway) lookup(ctx context.Context, id, name, token string) (*model.PluginRegistry, error) {
	if g.store == nil {
		return nil, ErrPluginUnknown
	}
	var (
		plugin *model.PluginRegistry
		err    error
	)
	if id != "" {
		plugin, err = g.store.GetByID(ctx, id)
	} else {
		plugin, err = g.store.GetByName(ctx, name)
	}
	if err != nil || plugin == nil {
		return nil, ErrPluginUnknown
	}
	if plugin.Status == model.PluginStatusDisabled {
		return nil, ErrPluginDisabled
	}
	if HashToken(token) != plugin.TokenHash {
		return nil, ErrPluginTokenInvalid
	}
	return plugin, nil
}

// Register 注册（name 定位，token 鉴权），成功置 online。
func (g *Gateway) Register(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	plugin, err := g.lookup(ctx, "", req.Name, req.Token)
	if err != nil {
		return &RegisterResponse{Message: err.Error()}, err
	}
	version := req.Version
	if err := g.store.UpdateStatusAndHeartbeat(ctx, plugin.ID, model.PluginStatusOnline, g.now(), &version); err != nil {
		return &RegisterResponse{Message: err.Error()}, err
	}
	return &RegisterResponse{PluginID: plugin.ID, Status: model.PluginStatusOnline}, nil
}

// Heartbeat 心跳续约。
func (g *Gateway) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	plugin, err := g.lookup(ctx, req.PluginID, "", req.Token)
	if err != nil {
		return &HeartbeatResponse{Accepted: false, Message: err.Error()}, err
	}
	version := req.Version
	if err := g.store.UpdateStatusAndHeartbeat(ctx, plugin.ID, model.PluginStatusOnline, g.now(), &version); err != nil {
		return &HeartbeatResponse{Accepted: false, Message: err.Error()}, err
	}
	return &HeartbeatResponse{Accepted: true, Status: model.PluginStatusOnline}, nil
}

// Uplink 上行数据帧汇入。
func (g *Gateway) Uplink(ctx context.Context, req *UplinkFrame) (*UplinkAck, error) {
	plugin, err := g.lookup(ctx, req.PluginID, "", req.Token)
	if err != nil {
		return &UplinkAck{Accepted: false, Message: err.Error()}, err
	}
	if g.sink == nil {
		return &UplinkAck{Accepted: false, Message: ErrSinkNotWired.Error()}, ErrSinkNotWired
	}
	if err := g.sink.PublishUplink(req, plugin); err != nil {
		return &UplinkAck{Accepted: false, Message: err.Error()}, err
	}
	return &UplinkAck{Accepted: true}, nil
}

// Attach 下行命令服务流：会话注册 → 循环推送队列命令 → 退出置 offline。
func (g *Gateway) Attach(ctx context.Context, req *AttachRequest, send func(*DownlinkCommand) error) error {
	plugin, err := g.lookup(ctx, req.PluginID, "", req.Token)
	if err != nil {
		return err
	}
	session := &pluginSession{commands: make(chan *DownlinkCommand, g.downlinkBuffer)}
	g.mu.Lock()
	// 同一插件重复 Attach：顶掉旧会话（旧队列随 GC 释放）。
	g.sessions[plugin.ID] = session
	g.mu.Unlock()
	defer func() {
		g.mu.Lock()
		if g.sessions[plugin.ID] == session {
			delete(g.sessions, plugin.ID)
		}
		g.mu.Unlock()
		_ = g.store.UpdateStatusAndHeartbeat(ctx, plugin.ID, model.PluginStatusOffline, g.now(), nil)
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case cmd := <-session.commands:
			if err := send(cmd); err != nil {
				return fmt.Errorf("send downlink: %w", err)
			}
		}
	}
}

// EnqueueDownlink 向在线插件会话投递下行命令；无会话即离线报错。
func (g *Gateway) EnqueueDownlink(pluginID string, cmd *DownlinkCommand) error {
	g.mu.Lock()
	session, ok := g.sessions[pluginID]
	g.mu.Unlock()
	if !ok {
		return ErrPluginOffline
	}
	select {
	case session.commands <- cmd:
		return nil
	default:
		return fmt.Errorf("downlink queue full for plugin %s", pluginID)
	}
}

// SessionOnline 插件是否在线（管理页展示用）。
func (g *Gateway) SessionOnline(pluginID string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, ok := g.sessions[pluginID]
	return ok
}

// PHASE-D-D9 END
