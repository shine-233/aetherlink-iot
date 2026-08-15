// 文件用途：验证 MQTT reason code 错误包装和共享错误实例的稳定性。
// 核心逻辑：检查 Error.Code、ReasonString 输出和共享错误实例是否对应 MQTT v5 code。
// 关键注意事项：这些测试保护协议常量，不应因格式化或重命名改动而放松。
// 重构建议：新增 code 时补充表驱动用例，覆盖 code 数值和字符串输出。
package codes

import "testing"

func TestMQTTReasonCodeErrorsExposeStableCodeAndReasonString(t *testing.T) {
	err := NewError(NotAuthorized)
	err.ReasonString = []byte("acl denied")

	if err.Code != NotAuthorized {
		t.Fatalf("code = %x, want NotAuthorized", err.Code)
	}
	if got := err.Error(); got == "" || got == "operation error: Code = 0, reasonString: " {
		t.Fatalf("unexpected error string %q", got)
	}
}

func TestSharedMalformedAndProtocolErrorsUseMQTTV5Codes(t *testing.T) {
	if ErrMalformed.Code != MalformedPacket {
		t.Fatalf("ErrMalformed code = %x, want %x", ErrMalformed.Code, MalformedPacket)
	}
	if ErrProtocol.Code != ProtocolError {
		t.Fatalf("ErrProtocol code = %x, want %x", ErrProtocol.Code, ProtocolError)
	}
	if Success != NormalDisconnection || GrantedQoS0 != Success {
		t.Fatal("success, normal disconnect, and QoS0 should share MQTT code 0")
	}
}
