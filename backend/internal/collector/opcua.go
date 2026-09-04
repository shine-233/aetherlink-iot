// 文件用途：OPC UA 轮询采集器（ROADMAP OPC UA 行——库级 internal/opcua 的连接器接入）。
// 核心逻辑：解析 protocol_config 点表（endpoint + nodes），按设备维护带 TTL 的连接缓存，
//
//	逐节点 ReadValue 并转为遥测键（数值 → float64，bool/string → 原语义）。
//
// 关键注意事项：
//  - 连接失败/读失败即失效连接条目，下一轮强制重连（懒重连，避免半死连接长期占用）；
//  - 真实 OPC UA 服务器的读写 E2E 属环境绑定（需 opc.tcp 服务沙箱），本层单测覆盖
//    配置校验/发现/失败路径，接入时按 opcua 包既定口径补 E2E；
//  - 节点值转换失败跳过该点，不影响同设备其他点位（部分发布语义与 SNMP 一致）。
package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/opcua"

	"github.com/sirupsen/logrus"
	"github.com/gopcua/opcua/ua"
)

// OpcuaConfig OPC UA protocol_config JSON 结构（连接段复用 opcua.Config 校验）。
type OpcuaConfig struct {
	opcua.Config
	Points []Point `json:"points"`
}

// parseOpcuaConfig 解析并校验点表；连接段经 opcua.Validate（endpoint 前缀/SecurityMode 白名单）。
func parseOpcuaConfig(raw string) (*OpcuaConfig, error) {
	if raw == "" {
		return nil, fmt.Errorf("opcua: protocol_config 为空")
	}
	var cfg OpcuaConfig
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("opcua: protocol_config 非法 JSON: %w", err)
	}
	if len(cfg.Points) == 0 {
		return nil, fmt.Errorf("opcua: points 至少一条")
	}
	for i, p := range cfg.Points {
		if p.Key == "" || p.Node == "" {
			return nil, fmt.Errorf("opcua: points[%d] key/node 必填", i)
		}
	}
	if err := opcua.Validate(cfg.Config); err != nil {
		return nil, err
	}
	cfg.Config = opcua.Normalize(cfg.Config)
	return &cfg, nil
}

// opcuaConnTTL 连接缓存 TTL：TTL 内复用会话；读失败立即失效强制重连。
const opcuaConnTTL = 5 * time.Minute

type opcuaEntry struct {
	client   *opcua.Client
	expireAt time.Time
}

// OpcuaPoller 按设备缓存连接的 OPC UA 采集实现。
type OpcuaPoller struct {
	mu    sync.Mutex
	conns map[string]*opcuaEntry // deviceID → 连接条目
	log   *logrus.Logger
}

// NewOpcuaPoller 构造 OPC UA 采集器。
func NewOpcuaPoller(log *logrus.Logger) *OpcuaPoller {
	if log == nil {
		log = logrus.New()
	}
	return &OpcuaPoller{conns: map[string]*opcuaEntry{}, log: log}
}

// Protocol 返回 source_protocol 遥测标记。
func (p *OpcuaPoller) Protocol() string { return "opcua" }

// ConfigType 返回 device_configs.protocol_type 过滤值。
func (p *OpcuaPoller) ConfigType() string { return "OPCUA" }

// Poll 单目标采集：确保连接 → 逐节点读 → 转换。
func (p *OpcuaPoller) Poll(ctx context.Context, t deviceTarget) (map[string]interface{}, error) {
	cfg, err := parseOpcuaConfig(t.ConfigJSON)
	if err != nil {
		return nil, err
	}
	client, err := p.ensureConn(t.DeviceID, cfg.Config, ctx)
	if err != nil {
		return nil, err
	}

	values := make(map[string]interface{}, len(cfg.Points))
	readFailed := false
	for _, pt := range cfg.Points {
		dv, err := client.ReadValue(ctx, pt.Node)
		if err != nil {
			readFailed = true
			p.log.WithFields(logrus.Fields{
				"device_id": t.DeviceID,
				"node":      pt.Node,
				"error":     err,
			}).Warn("opcua: 节点读取失败")
			continue
		}
		if val, ok := opcuaValue(dv); ok {
			values[pt.Key] = val
		}
	}
	if readFailed {
		// 连接可能半死：失效条目，下一轮重连（本轮已读到的点照常发布）。
		p.invalidate(t.DeviceID)
	}
	return values, nil
}

// ensureConn 取缓存连接或新建（构造/连接失败即返回错误并清理旧条目）。
func (p *OpcuaPoller) ensureConn(deviceID string, cfg opcua.Config, ctx context.Context) (*opcua.Client, error) {
	p.mu.Lock()
	if e, ok := p.conns[deviceID]; ok && time.Now().Before(e.expireAt) {
		p.mu.Unlock()
		return e.client, nil
	}
	p.mu.Unlock()

	client, err := opcua.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(ctx); err != nil {
		p.invalidate(deviceID)
		return nil, fmt.Errorf("opcua: 连接 %s 失败: %w", cfg.Endpoint, err)
	}
	p.mu.Lock()
	p.conns[deviceID] = &opcuaEntry{client: client, expireAt: time.Now().Add(opcuaConnTTL)}
	p.mu.Unlock()
	return client, nil
}

// invalidate 失效设备连接条目（旧连接交给 GC，不主动 Close 阻塞读路径）。
func (p *OpcuaPoller) invalidate(deviceID string) {
	p.mu.Lock()
	delete(p.conns, deviceID)
	p.mu.Unlock()
}

// opcuaValue 把 DataValue 转为遥测值：数值 → float64，bool → "true"/"false"，其余 → 原文；
// nil 值（Bad status 常态）丢弃。独立函数便于单测。
func opcuaValue(dv *ua.DataValue) (interface{}, bool) {
	if dv == nil || dv.Value == nil {
		return nil, false
	}
	v := dv.Value.Value()
	if v == nil {
		return nil, false
	}
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case bool:
		return strconv.FormatBool(x), true
	case string:
		return x, true
	case []byte:
		return string(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	default:
		return fmt.Sprint(x), true
	}
}
