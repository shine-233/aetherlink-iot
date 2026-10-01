// 文件用途：读侧（Reader.ReadPacket）热路径的合并分配解码。
// 核心逻辑：
//   - PUBLISH：FixHeader / Publish（v5 再加 Properties）放进同一个结构体一次分配，
//     各指针字段指向该块内部，省去每包 1~2 次堆分配。
//   - PUBACK / PUBREC / PUBREL / PUBCOMP 且 RemainLength==2（v3 与 v5 成功码的常见形态）：
//     直接从 bufio.Reader 读 2 字节报文标识符，不再 make 剩余缓冲区，
//     FixHeader 与报文结构体同块分配。
//
// 关键注意事项：解码语义（错误码、字段取值、Code 默认 Success）与 NewXxxPacket 完全一致；
// 其余包类型及带原因码/属性的 ack 仍走 NewPacket 通用路径。块内指针只延长同一块的生命周期，
// 不引入共享可变状态（每包一块，块之间互不引用）。
package packets

import (
	"bufio"

	"github.com/DrmagicE/gmqtt/pkg/codes"
)

type publishBoxV3 struct {
	pub Publish
	fh  FixHeader
}

type publishBoxV5 struct {
	pub   Publish
	fh    FixHeader
	props Properties
}

type pubackBox struct {
	p  Puback
	fh FixHeader
}

type pubrecBox struct {
	p  Pubrec
	fh FixHeader
}

type pubrelBox struct {
	p  Pubrel
	fh FixHeader
}

type pubcompBox struct {
	p  Pubcomp
	fh FixHeader
}

// readPacketFast 处理热路径包类型；ok=false 表示调用方应走通用 NewPacket 路径。
func readPacketFast(fh FixHeader, version Version, r *bufio.Reader) (pkt Packet, ok bool, err error) {
	switch fh.PacketType {
	case PUBLISH:
		p, err := decodePublishFast(fh, version, r)
		if err != nil {
			return nil, true, err
		}
		return p, true, nil
	case PUBACK, PUBREC, PUBREL, PUBCOMP:
		if fh.RemainLength != 2 {
			return nil, false, nil
		}
		p, err := decodeShortAck(fh, version, r)
		return p, true, err
	}
	return nil, false, nil
}

func decodePublishFast(fh FixHeader, version Version, r *bufio.Reader) (*Publish, error) {
	var p *Publish
	var props *Properties
	if version == Version5 {
		box := &publishBoxV5{fh: fh}
		p, props = &box.pub, &box.props
		p.FixHeader = &box.fh
	} else {
		box := &publishBoxV3{fh: fh}
		p = &box.pub
		p.FixHeader = &box.fh
	}
	p.Version = version
	p.Dup = (1 & (fh.Flags >> 3)) > 0
	p.Qos = (fh.Flags >> 1) & 3
	if p.Qos == 0 && p.Dup { //[MQTT-3.3.1-2]、 [MQTT-4.3.1-1]
		return nil, codes.ErrMalformed
	}
	if p.Qos > Qos2 {
		return nil, codes.ErrMalformed
	}
	if fh.Flags&1 == 1 {
		p.Retain = true
	}
	if err := p.unpack(r, props); err != nil {
		return nil, err
	}
	return p, nil
}

// decodeShortAck 解码 RemainLength==2 的 ack：仅含报文标识符，原因码按协议默认为 Success。
func decodeShortAck(fh FixHeader, version Version, r *bufio.Reader) (Packet, error) {
	hi, err := r.ReadByte()
	if err != nil {
		return nil, codes.ErrMalformed
	}
	lo, err := r.ReadByte()
	if err != nil {
		return nil, codes.ErrMalformed
	}
	pid := PacketID(uint16(hi)<<8 | uint16(lo))
	switch fh.PacketType {
	case PUBACK:
		b := &pubackBox{fh: fh}
		b.p = Puback{Version: version, FixHeader: &b.fh, PacketID: pid, Code: codes.Success}
		return &b.p, nil
	case PUBREC:
		b := &pubrecBox{fh: fh}
		b.p = Pubrec{Version: version, FixHeader: &b.fh, PacketID: pid, Code: codes.Success}
		return &b.p, nil
	case PUBREL:
		b := &pubrelBox{fh: fh}
		b.p = Pubrel{FixHeader: &b.fh, PacketID: pid, Code: codes.Success}
		return &b.p, nil
	default: // PUBCOMP
		b := &pubcompBox{fh: fh}
		b.p = Pubcomp{Version: version, FixHeader: &b.fh, PacketID: pid, Code: codes.Success}
		return &b.p, nil
	}
}
