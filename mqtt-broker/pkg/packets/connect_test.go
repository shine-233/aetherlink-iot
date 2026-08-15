// 文件用途: 覆盖 MQTT CONNECT 控制包解析和编解码样例。
// 核心逻辑: 校验协议版本处理、凭据、遗嘱字段和属性负载。
// 关键注意事项: CONNECT 会进入认证与会话建立流程，属于安全敏感路径。
// 重构建议: 扩展畸形凭据和版本协商的表驱动用例。
package packets

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DrmagicE/gmqtt/pkg/codes"
)

func TestReadConnectPacketErr_V5(t *testing.T) {
	//[MQTT-3.1.2-3],服务端必须验证CONNECT控制报文的保留标志位（第0位）是否为0，如果不为0必须断开客户端连接
	a := assert.New(t)

	b := []byte{16, 12, 0, 4, 'M', 'Q', 'T', 'T', 05, 01, 00, 02, 31, 32}
	buf := bytes.NewBuffer(b)
	r := NewReader(buf)
	r.SetVersion(Version5)
	connectPacket, err := r.ReadPacket()
	a.Nil(connectPacket)
	a.Error(codes.ErrMalformed, err)

}
func TestReadConnectPacketErr_V311(t *testing.T) {
	//[MQTT-3.1.2-3],服务端必须验证CONNECT控制报文的保留标志位（第0位）是否为0，如果不为0必须断开客户端连接
	a := assert.New(t)
	b := []byte{16, 12, 0, 4, 'M', 'Q', 'T', 'T', 04, 01, 00, 02, 31, 32}
	buf := bytes.NewBuffer(b)
	connectPacket, err := NewReader(buf).ReadPacket()
	a.Nil(connectPacket)
	a.Error(codes.ErrMalformed, err)
}

func TestReadConnect_V31(t *testing.T) {
	a := assert.New(t)
	b := []byte{0x10, 0x0f, 0, 0x06, 'M', 'Q', 'I', 's', 'd', 'p', 0x03, 0x02, 0x00, 0x0a, 0x00, 0x01, 0x74}
	buf := bytes.NewBuffer(b)
	connectPacket, err := NewReader(buf).ReadPacket()
	a.NoError(err)
	a.EqualValues(10, connectPacket.(*Connect).KeepAlive)
}
