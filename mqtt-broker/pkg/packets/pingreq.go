// 文件用途: 实现 MQTT PINGREQ 控制包的编码与解码。
// 核心逻辑: 校验仅含固定头的 keepalive 请求包。
// 关键注意事项: PINGREQ 应保持低分配，并严格校验剩余长度。
// 重构建议: 尽量与 PINGRESP 共享空负载控制包处理逻辑。
package packets

import (
	"fmt"
	"io"

	"github.com/DrmagicE/gmqtt/pkg/codes"
)

// Pingreq represents the MQTT Pingreq  packet
type Pingreq struct {
	FixHeader *FixHeader
}

func (p *Pingreq) String() string {
	return fmt.Sprintf("Pingreq")
}

// NewPingreqPacket returns a Pingreq instance by the given FixHeader and io.Reader
func NewPingreqPacket(fh *FixHeader, r io.Reader) (*Pingreq, error) {
	if fh.Flags != FlagReserved {
		return nil, codes.ErrMalformed
	}
	p := &Pingreq{FixHeader: fh}
	err := p.Unpack(r)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// NewPingresp returns a Pingresp struct
func (p *Pingreq) NewPingresp() *Pingresp {
	fh := &FixHeader{PacketType: PINGRESP, Flags: 0, RemainLength: 0}
	return &Pingresp{FixHeader: fh}
}

// Pack encodes the packet struct into bytes and writes it into io.Writer.
func (p *Pingreq) Pack(w io.Writer) error {
	p.FixHeader = &FixHeader{PacketType: PINGREQ, Flags: 0, RemainLength: 0}
	return p.FixHeader.Pack(w)
}

// Unpack read the packet bytes from io.Reader and decodes it into the packet struct.
func (p *Pingreq) Unpack(r io.Reader) error {
	if p.FixHeader.RemainLength != 0 {
		return codes.ErrMalformed
	}
	return nil
}
