// 文件用途：边缘下行指令（云 → 边，ROADMAP 边缘计算 MVP 的 RPC 半环）。
// 核心逻辑：边缘节点订阅云 broker 的命令 topic（默认 {prefix}/cmd/{device_id}，
//
//	MQTT 通配符由配置给出），收到消息后解析为本地命令下发请求，经 CommandSink
//	注入本地命令通道——即"云端下发、边缘执行"。
//
// 关键注意事项：
//   - 解析失败/未知设备只记录日志并丢弃（fail-open，绝不影响遥测转发与本地入库）；
//   - 命令落地经 CommandSink 接口注入，生产由 app 层接 service.GroupApp.CommandData，
//     测试用假实现，避免本包反向依赖 service；
//   - 操作人以配置 operator-id 记录，审计可追溯命令来源为边缘中继。
package edgeforward

import (
	"context"
	"encoding/json"
	"sync/atomic"

	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
)

// CommandSink 本地命令落地下游（由 app 层接 service.GroupApp.CommandData）。
type CommandSink interface {
	PutCommand(ctx context.Context, operatorID string, req *model.PutMessageForCommand, operationType string) error
}

// CloudCommand 云端下发的命令信封。
type CloudCommand struct {
	DeviceID  string                 `json:"device_id"`
	Identify  string                 `json:"identify"`
	Params    map[string]interface{} `json:"params"`
	Value     string                 `json:"value"`
	RequestID string                 `json:"request_id"`
}

// EdgeCommand 解析后的下发请求。
type EdgeCommand struct {
	DeviceID  string
	Identify  string
	Value     string
	RequestID string
}

// ParseCommand 解析云端命令信封；返回 nil 表示无效（调用方应丢弃）。
func ParseCommand(payload []byte) *EdgeCommand {
	var cmd CloudCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		return nil
	}
	if cmd.DeviceID == "" || cmd.Identify == "" {
		return nil
	}
	value := cmd.Value
	if value == "" && len(cmd.Params) > 0 {
		if raw, err := json.Marshal(cmd.Params); err == nil {
			value = string(raw)
		}
	}
	if value == "" {
		value = "{}"
	}
	return &EdgeCommand{DeviceID: cmd.DeviceID, Identify: cmd.Identify, Value: value, RequestID: cmd.RequestID}
}

// handleCommand 处理一条云端命令：解析 → 落地；失败仅记录。
func (f *Forwarder) handleCommand(topic string, payload []byte) {
	cmd := ParseCommand(payload)
	if cmd == nil {
		f.log.Warn("edge forward: 云端命令格式无效，已丢弃 topic=", topic)
		return
	}
	if f.sink == nil {
		f.log.Warn("edge forward: 未配置命令落地下游，命令已丢弃 device=", cmd.DeviceID)
		return
	}
	req := &model.PutMessageForCommand{DeviceID: cmd.DeviceID, Identify: cmd.Identify}
	if cmd.Value != "" {
		v := cmd.Value
		req.Value = &v
	}
	if err := f.sink.PutCommand(context.Background(), f.cfg.CommandOperatorID, req, OperationTypeEdgeRelay); err != nil {
		f.log.Warn("edge forward: 云端命令落地失败 device=", cmd.DeviceID, " err=", err)
		return
	}
	atomic.AddUint64(&f.commandsApplied, 1)
	f.log.WithFields(logrus.Fields{
		"device_id":  cmd.DeviceID,
		"identify":   cmd.Identify,
		"request_id": cmd.RequestID,
	}).Info("edge forward: 云端命令已落地")
}

// CommandsApplied 返回已成功落地的云端命令计数（便于观测与测试）。
func (f *Forwarder) CommandsApplied() uint64 { return atomic.LoadUint64(&f.commandsApplied) }
