// 文件用途：LwM2M 1.0 上报编码核心——TLV（OMA-TS-LightweightM2M-V1_0，§6.4.4）。
// 核心逻辑：资源/实例的 TLV 字节编码（type|idLen|lenType 头，id/长度小端），
//   提供 ResourceWithValue / MultipleResource / ObjectInstance 逐层组装，
//   供设备上报通知（notify/报告）与读响应使用。
// 关键注意事项：
//   - 类型位：11=Resource with Value、10=Multiple Resource、01=Resource Instance、00=Object Instance；
//   - id/长度域均小端；lenType: 0→0B,1→1B,2→2B,3→4B；
//   - 长度字段按需最小化（0/1/2/4）；本实现限制单层值≤2^16。
package lwm2m

import (
	"fmt"
)

// TLV type bits。
const (
	tlvObjectInstance     = 0x00
	tlvResourceInstance   = 0x40 // 01 前缀
	tlvMultipleResource   = 0x80 // 10 前缀
	tlvResourceWithValue  = 0xC0 // 11 前缀
)

func tlvHeader(typ byte, id uint16, length int) []byte {
	var idLen byte
	var idBytes []byte
	switch {
	case id <= 0xFF:
		idLen, idBytes = 1, []byte{byte(id)}
	case id <= 0xFFFF:
		idLen, idBytes = 2, []byte{byte(id), byte(id >> 8)}
	}
	var lenType byte
	var lenBytes []byte
	switch {
	case length == 0:
		lenType, lenBytes = 0, nil
	case length <= 0xFF:
		lenType, lenBytes = 1, []byte{byte(length)}
	case length <= 0xFFFF:
		lenType, lenBytes = 2, []byte{byte(length), byte(length >> 8)}
	default:
		lenType, lenBytes = 3, []byte{byte(length), byte(length >> 8), byte(length >> 16), byte(length >> 24)}
	}
	out := []byte{typ | idLen<<4 | lenType<<2}
	return append(out, append(idBytes, lenBytes...)...)
}

// EncodeResourceWithValue 编码 11 类型资源（带值）。
func EncodeResourceWithValue(id uint16, value []byte) []byte {
	return append(tlvHeader(tlvResourceWithValue, id, len(value)), value...)
}

// EncodeMultipleResource 编码 10 类型多实例资源（children 为资源实例 TLV 串）。
func EncodeMultipleResource(id uint16, children []byte) []byte {
	return append(tlvHeader(tlvMultipleResource, id, len(children)), children...)
}

// EncodeResourceInstance 编码 01 类型资源实例。
func EncodeResourceInstance(id uint16, value []byte) []byte {
	return append(tlvHeader(tlvResourceInstance, id, len(value)), value...)
}

// EncodeObjectInstance 编码 00 类型对象实例（children 为资源/多资源 TLV 串）。
func EncodeObjectInstance(id uint16, children []byte) []byte {
	return append(tlvHeader(tlvObjectInstance, id, len(children)), children...)
}

// ValueInt 把 int64 编码为 LwM2M 整数资源值字节。
func ValueInt(v int64) []byte {
	switch {
	case v >= -128 && v <= 127:
		return []byte{byte(v)}
	case v >= -32768 && v <= 32767:
		return []byte{byte(v), byte(v >> 8)}
	case v >= -2147483648 && v <= 2147483647:
		return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
	default:
		return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24), byte(v >> 32), byte(v >> 40), byte(v >> 48), byte(v >> 56)}
	}
}

// tlvHdrLen 返回头长度：1 + idLen + lenTypeLen(lenType=0→0,1→1,2→2,3→4)。
func tlvHdrLen(b byte) (byte, int) {
	idLen := (b >> 4) & 0x03
	lenType := (b >> 2) & 0x03
	lenBytes := 0
	switch lenType {
	case 1:
		lenBytes = 1
	case 2:
		lenBytes = 2
	case 3:
		lenBytes = 4
	}
	return idLen, 1 + int(idLen) + lenBytes
}

// DecodeTLV 解析单层 TLV（返回条目 list：type/id/value），不递归展开值。
func DecodeTLV(raw []byte) ([]TLVEntry, error) {
	var out []TLVEntry
	pos := 0
	for pos < len(raw) {
		if pos+1 > len(raw) {
			return nil, fmt.Errorf("tlv: 头截断")
		}
		b := raw[pos]
		typ := b & 0xC0
		idLen, hdrLen := tlvHdrLen(b)
		if pos+hdrLen > len(raw) {
			return nil, fmt.Errorf("tlv: 头越界")
		}
		var id uint16
		for i := 0; i < int(idLen); i++ {
			id |= uint16(raw[pos+1+i]) << (8 * i)
		}
		lenType := (b >> 2) & 0x03
		var length int
		switch lenType {
		case 0:
		case 1:
			length = int(raw[pos+1+int(idLen)])
		case 2:
			length = int(raw[pos+1+int(idLen)]) | int(raw[pos+2+int(idLen)])<<8
		case 3:
			length = int(raw[pos+1+int(idLen)]) | int(raw[pos+2+int(idLen)])<<8 |
				int(raw[pos+3+int(idLen)])<<16 | int(raw[pos+4+int(idLen)])<<24
		}
		valueStart := pos + hdrLen
		if valueStart+length > len(raw) {
			return nil, fmt.Errorf("tlv: 值越界")
		}
		out = append(out, TLVEntry{Type: typ, ID: id, Value: raw[valueStart : valueStart+length]})
		pos = valueStart + length
	}
	return out, nil
}

// TLVEntry 解析出的一条 TLV。
type TLVEntry struct {
	Type  byte
	ID    uint16
	Value []byte
}
