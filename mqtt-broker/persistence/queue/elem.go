// 文件用途：维护 persistence\queue\elem.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

package queue

import (
	"bytes"
	"encoding/binary"
	"errors"
	"time"

	"github.com/DrmagicE/gmqtt"
	"github.com/DrmagicE/gmqtt/persistence/encoding"
	"github.com/DrmagicE/gmqtt/pkg/packets"
)

type MessageWithID interface {
	ID() packets.PacketID
	SetID(id packets.PacketID)
}

type Publish struct {
	*gmqtt.Message
}

func (p *Publish) ID() packets.PacketID {
	return p.PacketID
}
func (p *Publish) SetID(id packets.PacketID) {
	p.PacketID = id
}

type Pubrel struct {
	PacketID packets.PacketID
}

func (p *Pubrel) ID() packets.PacketID {
	return p.PacketID
}
func (p *Pubrel) SetID(id packets.PacketID) {
	p.PacketID = id
}

// Elem represents the element store in the queue.
type Elem struct {
	// At represents the entry time.
	At time.Time
	// Expiry represents the expiry time.
	// Empty means never expire.
	Expiry time.Time
	MessageWithID
}

const (
	elemHeaderSize       = 19
	elemTimestampSize    = 8
	elemExpiryOffset     = 9
	elemIdentifierOffset = 18
)

func writeInt64(dst []byte, value int64) {
	buffer := bytes.NewBuffer(dst[:0])
	_ = binary.Write(buffer, binary.BigEndian, value)
}

func readInt64(src []byte) (int64, error) {
	var value int64
	err := binary.Read(bytes.NewReader(src), binary.BigEndian, &value)
	return value, err
}

// Encode encodes the publish structure into bytes and write it to the buffer
func (p *Publish) Encode(b *bytes.Buffer) {
	encoding.EncodeMessage(p.Message, b)
}

func (p *Publish) Decode(b *bytes.Buffer) (err error) {
	msg, err := encoding.DecodeMessage(b)
	if err != nil {
		return err
	}
	p.Message = msg
	return nil
}

// Encode encode the pubrel structure into bytes.
func (p *Pubrel) Encode(b *bytes.Buffer) {
	encoding.WriteUint16(b, p.PacketID)
}

func (p *Pubrel) Decode(b *bytes.Buffer) (err error) {
	p.PacketID, err = encoding.ReadUint16(b)
	return
}

// Encode encode the elem structure into bytes.
// Format: 8 byte timestamp | 1 byte identifier| data
func (e *Elem) Encode() []byte {
	b := bytes.NewBuffer(make([]byte, 0, 100))
	rs := make([]byte, elemHeaderSize)
	writeInt64(rs[:elemTimestampSize], e.At.Unix())
	writeInt64(rs[elemExpiryOffset:elemExpiryOffset+elemTimestampSize], e.Expiry.Unix())
	switch m := e.MessageWithID.(type) {
	case *Publish:
		rs[elemIdentifierOffset] = 0
		b.Write(rs)
		m.Encode(b)
	case *Pubrel:
		rs[elemIdentifierOffset] = 1
		b.Write(rs)
		m.Encode(b)
	}
	return b.Bytes()
}

func (e *Elem) Decode(b []byte) (err error) {
	if len(b) < elemHeaderSize {
		return errors.New("invalid input length")
	}
	at, err := readInt64(b[:elemTimestampSize])
	if err != nil {
		return err
	}
	expiry, err := readInt64(b[elemExpiryOffset : elemExpiryOffset+elemTimestampSize])
	if err != nil {
		return err
	}
	e.At = time.Unix(at, 0)
	e.Expiry = time.Unix(expiry, 0)
	switch b[elemIdentifierOffset] {
	case 0: // publish
		p := &Publish{}
		buf := bytes.NewBuffer(b[19:])
		err = p.Decode(buf)
		e.MessageWithID = p
	case 1: // pubrel
		p := &Pubrel{}
		buf := bytes.NewBuffer(b[19:])
		err = p.Decode(buf)
		e.MessageWithID = p
	default:
		return errors.New("invalid identifier")
	}
	return
}
