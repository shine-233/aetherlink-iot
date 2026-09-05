// 文件用途：边缘下行命令解析与落地单元测试（假 sink，无需真实 broker/DB）。
package edgeforward

import (
	"context"
	"testing"

	"aetherlink-iot/backend/internal/model"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSink 记录收到的命令，用于断言解析与落地结果。
type fakeSink struct {
	calls   []*model.PutMessageForCommand
	ops     []string
	opsBy   []string
	failErr error
}

func (s *fakeSink) PutCommand(_ context.Context, operatorID string, req *model.PutMessageForCommand, operationType string) error {
	if s.failErr != nil {
		return s.failErr
	}
	s.calls = append(s.calls, req)
	s.ops = append(s.ops, operationType)
	s.opsBy = append(s.opsBy, operatorID)
	return nil
}

func newCommandForwarder(sink CommandSink) *Forwarder {
	cfg := Config{
		Enabled:           true,
		CommandEnabled:    true,
		CommandTopic:      "aetherlink/edge/cmd/+",
		CommandOperatorID: "edge-relay",
		BufferLimit:       10,
	}
	return New(nil, cfg, logrus.New()).WithCommandSink(sink)
}

func TestEdgeCommandRelayAppliesValidCommand(t *testing.T) {
	sink := &fakeSink{}
	f := newCommandForwarder(sink)
	f.handleCommand("aetherlink/edge/cmd/dev-1",
		[]byte(`{"device_id":"dev-1","identify":"set_switch","params":{"on":true},"request_id":"req-1"}`))

	require.Len(t, sink.calls, 1, "有效命令应落地一次")
	assert.Equal(t, "dev-1", sink.calls[0].DeviceID)
	assert.Equal(t, "set_switch", sink.calls[0].Identify)
	require.NotNil(t, sink.calls[0].Value)
	assert.JSONEq(t, `{"on":true}`, *sink.calls[0].Value, "params 应序列化为 value")
	assert.Equal(t, OperationTypeEdgeRelay, sink.ops[0])
	assert.Equal(t, "edge-relay", sink.opsBy[0], "操作人应为边缘中继标识，便于审计")
	assert.Equal(t, uint64(1), f.CommandsApplied())
}

func TestEdgeCommandRelayUsesExplicitValue(t *testing.T) {
	sink := &fakeSink{}
	f := newCommandForwarder(sink)
	f.handleCommand("t", []byte(`{"device_id":"dev-2","identify":"reboot","value":"reason=manual"}`))

	require.Len(t, sink.calls, 1)
	require.NotNil(t, sink.calls[0].Value)
	assert.Equal(t, "reason=manual", *sink.calls[0].Value, "显式 value 优先于 params")
}

func TestEdgeCommandRelayDropsMalformed(t *testing.T) {
	sink := &fakeSink{}
	f := newCommandForwarder(sink)
	f.handleCommand("t", []byte(`not-json`))                                  // 非法 JSON
	f.handleCommand("t", []byte(`{"device_id":"dev"}`))                       // 缺 identify
	f.handleCommand("t", []byte(`{"identify":"x"}`))                          // 缺 device_id
	assert.Empty(t, sink.calls, "非法/不完整命令必须丢弃（fail-open，不得误下发）")
	assert.Equal(t, uint64(0), f.CommandsApplied())
}

func TestEdgeCommandRelayWithoutSinkIsSafe(t *testing.T) {
	f := New(nil, Config{Enabled: true, CommandEnabled: true}, logrus.New()) // 无 sink
	assert.NotPanics(t, func() {
		f.handleCommand("t", []byte(`{"device_id":"dev","identify":"x"}`))
	}, "未配置落地下游时不得 panic")
	assert.Equal(t, uint64(0), f.CommandsApplied())
}
