// 文件用途：OPC UA 接入最小包装（ROADMAP OPC UA 行）——基于 gopcua。
// 核心逻辑：端点配置校验、gopcua 客户端选项构建（SecurityMode/None、匿名或用户名口令）、
//   节点 ID 解析、端点发现（GetEndpoints）与批量读（Read）。
// 关键注意事项：
//   - 本层是「平台侧 OPC UA 网关/设备接入」的接线核心；安全模式当前允许 None（内网）并支持
//     username 认证，证书加密（SignAndEncrypt）预留由 config.SecurityMode 透传；
//   - 测试覆盖本层自有逻辑（配置校验/选项/ID 映射/请求组装），真实服务器读写用 E2E 在接入时验证。
// 重构建议：随 gopcua 上游 API 演进同步；连接池与订阅（MonitoredItem）接入设备影子时扩展。
package opcua

import (
	"context"
	"fmt"
	"strings"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

// Config OPC UA 端点配置。
type Config struct {
	Endpoint      string `json:"endpoint" validate:"required"` // opc.tcp://host:port
	SecurityMode  string `json:"security_mode"`                // None | Sign | SignAndEncrypt（默认 None）
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	ApplicationName string `json:"application_name,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// Normalize 返回默认值填充后的配置副本。
func Normalize(cfg Config) Config {
	if cfg.ApplicationName == "" {
		cfg.ApplicationName = "aetherlink-iot-opcua"
	}
	if cfg.SecurityMode == "" {
		cfg.SecurityMode = "None"
	}
	if cfg.TimeoutSeconds <= 0 {
		cfg.TimeoutSeconds = 10
	}
	return cfg
}

// Validate 校验配置；未知 SecurityMode 直接拒绝。
func Validate(cfg Config) error {
	cfg = Normalize(cfg)
	e := strings.TrimSpace(cfg.Endpoint)
	if e == "" {
		return fmt.Errorf("opcua: endpoint 必填")
	}
	if !strings.HasPrefix(e, "opc.tcp://") {
		return fmt.Errorf("opcua: endpoint 必须以 opc.tcp:// 开头: %q", e)
	}
	switch cfg.SecurityMode {
	case "None", "Sign", "SignAndEncrypt":
	default:
		return fmt.Errorf("opcua: 不支持的 SecurityMode %q（None|Sign|SignAndEncrypt）", cfg.SecurityMode)
	}
	return nil
}

// securityMode 把字符串映射到 gopcua 的 SecurityMode。
func securityMode(s string) ua.MessageSecurityMode {
	switch s {
	case "Sign":
		return ua.MessageSecurityModeSign
	case "SignAndEncrypt":
		return ua.MessageSecurityModeSignAndEncrypt
	default:
		return ua.MessageSecurityModeNone
	}
}

// Client 一个 OPC UA 会话的轻量封装。
type Client struct {
	cfg  Config
	conn *opcua.Client
}

// NewClient 依据配置构造（尚未连接）。
func NewClient(cfg Config) (*Client, error) {
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	cfg = Normalize(cfg)
	return &Client{cfg: cfg}, nil
}

// options 构造 gopcua 客户端选项。
func options(cfg Config) []opcua.Option {
	cfg = Normalize(cfg)
	opts := []opcua.Option{
		opcua.SecurityMode(securityMode(cfg.SecurityMode)),
		opcua.ApplicationName(cfg.ApplicationName),
	}
	if cfg.Username != "" {
		opts = append(opts, opcua.AuthUsername(cfg.Username, cfg.Password))
	} else {
		opts = append(opts, opcua.AuthAnonymous())
	}
	return opts
}

// Discover 执行 GetEndpoints（端点发现，无需连接会话）。
func Discover(ctx context.Context, cfg Config) ([]*ua.EndpointDescription, error) {
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return opcua.GetEndpoints(ctx, cfg.Endpoint, options(cfg)...)
}

// Connect 建立连接并打开会话。
func (c *Client) Connect(ctx context.Context) error {
	conn, err := opcua.NewClient(c.cfg.Endpoint, options(c.cfg)...)
	if err != nil {
		return fmt.Errorf("opcua: 客户端构造失败: %w", err)
	}
	if err := conn.Connect(ctx); err != nil {
		return fmt.Errorf("opcua: 连接失败: %w", err)
	}
	c.conn = conn
	return nil
}

// ReadValue 读取单个节点值；node 形如 "ns=2;s=Device1.Temperature"。
func (c *Client) ReadValue(ctx context.Context, node string) (*ua.DataValue, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("opcua: 未连接")
	}
	id, err := ua.ParseNodeID(node)
	if err != nil {
		return nil, fmt.Errorf("opcua: 非法 NodeID %q: %w", node, err)
	}
	resp, err := c.conn.Read(ctx, &ua.ReadRequest{
		MaxAge:             0,
		TimestampsToReturn: ua.TimestampsToReturnBoth,
		NodesToRead: []*ua.ReadValueID{
			{NodeID: id, AttributeID: ua.AttributeIDValue},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("opcua: 空读取结果")
	}
	return resp.Results[0], nil
}

// Close 关闭连接。
func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	return c.conn.Close(context.Background())
}
