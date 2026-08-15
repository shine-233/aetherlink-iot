// 文件用途: 实现 MQTT PINGRESP 控制包的编码与解码。
// 核心逻辑: 校验仅含固定头的 keepalive 响应包。
// 关键注意事项: PINGRESP 行为参与客户端 keepalive 存活判断。
// 重构建议: 尽量与 PINGREQ 共享空负载控制包处理逻辑。
package packets

import (
	"fmt"
	"io"

	"github.com/DrmagicE/gmqtt/pkg/codes"
)

// Pingresp represents the MQTT Pingresp  packet
type Pingresp struct {
	FixHeader *FixHeader
}

func (p *Pingresp) String() string {
	return fmt.Sprintf("Pingresp")
}

// Pack encodes the packet struct into bytes and writes it into io.Writer.
func (p *Pingresp) Pack(w io.Writer) error {
	p.FixHeader = &FixHeader{PacketType: PINGRESP, Flags: 0, RemainLength: 0}
	return p.FixHeader.Pack(w)
}

// Unpack read the packet bytes from io.Reader and decodes it into the packet struct.
func (p *Pingresp) Unpack(r io.Reader) error {
	if p.FixHeader.RemainLength != 0 {
		return codes.ErrMalformed
	}
	return nil
}

// NewPingrespPacket returns a Pingresp instance by the given FixHeader and io.Reader
func NewPingrespPacket(fh *FixHeader, r io.Reader) (*Pingresp, error) {
	if fh.Flags != FlagReserved {
		return nil, codes.ErrMalformed
	}
	p := &Pingresp{FixHeader: fh}
	err := p.Unpack(r)
	if err != nil {
		return nil, err
	}
	return p, nil
}
