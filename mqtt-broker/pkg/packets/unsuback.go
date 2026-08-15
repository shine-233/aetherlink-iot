// 文件用途: 实现 MQTT UNSUBACK 控制包的编码与解码。
// 核心逻辑: 处理取消订阅确认原因码列表和属性。
// 关键注意事项: UNSUBACK 是客户端可见的订阅清理确认。
// 重构建议: 与 SUBACK 共享原因码列表 helper。
package packets

import (
	"bytes"
	"fmt"
	"io"

	"github.com/DrmagicE/gmqtt/pkg/codes"
)

// Unsuback represents the MQTT Unsuback  packet.
type Unsuback struct {
	Version    Version
	FixHeader  *FixHeader
	PacketID   PacketID
	Properties *Properties
	Payload    []codes.Code
}

func (p *Unsuback) String() string {
	return fmt.Sprintf("Unsuback, Version: %v, Pid: %v, Payload: %v, Properties: %s", p.Version, p.PacketID, p.Payload, p.Properties)
}

// Pack encodes the packet struct into bytes and writes it into io.Writer.
func (p *Unsuback) Pack(w io.Writer) error {
	p.FixHeader = &FixHeader{PacketType: UNSUBACK, Flags: FlagReserved}
	bufw := &bytes.Buffer{}
	writeUint16(bufw, p.PacketID)
	if p.Version == Version5 {
		p.Properties.Pack(bufw, UNSUBACK)
	}
	bufw.Write(p.Payload)
	p.FixHeader.RemainLength = bufw.Len()
	err := p.FixHeader.Pack(w)
	if err != nil {
		return err
	}
	_, err = bufw.WriteTo(w)
	return err
}

// Unpack read the packet bytes from io.Reader and decodes it into the packet struct.
func (p *Unsuback) Unpack(r io.Reader) error {
	restBuffer := make([]byte, p.FixHeader.RemainLength)
	_, err := io.ReadFull(r, restBuffer)
	if err != nil {
		return codes.ErrMalformed
	}
	bufr := bytes.NewBuffer(restBuffer)
	p.PacketID, err = readUint16(bufr)
	if err != nil {
		return err
	}
	if IsVersion3X(p.Version) {
		return nil
	}

	p.Properties = &Properties{}
	err = p.Properties.Unpack(bufr, UNSUBACK)
	if err != nil {
		return err
	}
	for {
		b, err := bufr.ReadByte()
		if err != nil {
			return codes.ErrMalformed
		}
		if p.Version == Version5 && !ValidateCode(UNSUBACK, b) {
			return codes.ErrProtocol
		}
		p.Payload = append(p.Payload, b)
		if bufr.Len() == 0 {
			return nil
		}
	}
}

// NewUnsubackPacket returns a Unsuback instance by the given FixHeader and io.Reader.
func NewUnsubackPacket(fh *FixHeader, version Version, r io.Reader) (*Unsuback, error) {
	p := &Unsuback{FixHeader: fh, Version: version}
	if fh.Flags != FlagReserved {
		return nil, codes.ErrMalformed
	}
	err := p.Unpack(r)
	if err != nil {
		return nil, err
	}
	return p, err
}
