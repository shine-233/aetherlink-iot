// 文件用途：插件网关 gRPC 契约（PHASE-D-D9）。
// 核心逻辑：线格式结构体 + 服务描述。无 protoc 环境，采用 grpc-go 手写
// ServiceDesc/StreamDesc + JSON 编解码（codec 名 aetherjson）；
// proto 契约基线见本目录 plugin.proto，未来引入代码生成时线字段名保持兼容。
// 关键注意事项：结构体字段即线契约——只增不改删；JSON 编码禁 HTML 转义。
package grpcgateway

// RegisterRequest 插件注册：token 必须与注册表中摘要匹配。
type RegisterRequest struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	Token        string            `json:"token"`
	Description  string            `json:"description,omitempty"`
	Capabilities map[string]string `json:"capabilities,omitempty"`
}

// RegisterResponse 注册结果。
type RegisterResponse struct {
	PluginID string `json:"plugin_id"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}

// HeartbeatRequest 心跳：插件在线续约。
type HeartbeatRequest struct {
	PluginID string `json:"plugin_id"`
	Token    string `json:"token"`
	Version  string `json:"version,omitempty"`
}

// HeartbeatResponse 心跳应答。
type HeartbeatResponse struct {
	Accepted bool   `json:"accepted"`
	Status   string `json:"status,omitempty"`
	Message  string `json:"message,omitempty"`
}

// UplinkFrame 插件上行数据帧：device_number 为设备唯一标识（平台侧映射到设备/租户）。
type UplinkFrame struct {
	PluginID     string            `json:"plugin_id"`
	Token        string            `json:"token"`
	DeviceNumber string            `json:"device_number"`
	DataType     string            `json:"data_type"`
	Payload      map[string]any    `json:"payload"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// UplinkAck 上行应答。
type UplinkAck struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message,omitempty"`
}

// AttachRequest 下行流建立请求。
type AttachRequest struct {
	PluginID string `json:"plugin_id"`
	Token    string `json:"token"`
}

// DownlinkCommand 平台→插件下行命令。
type DownlinkCommand struct {
	CommandID    string         `json:"command_id"`
	DeviceNumber string         `json:"device_number"`
	Identify     string         `json:"identify"`
	Params       map[string]any `json:"params,omitempty"`
	IssuedAt     int64          `json:"issued_at"`
}
