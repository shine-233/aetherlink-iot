// 文件用途：CoAP Blockwise（RFC 7959）与 Observe（RFC 7641）的协议组件。
// 核心逻辑：Block1/Block2 选项号常量与 24-bit block 值编解码（num<<4|more<<3|szx，
//   szx=0..6 对应 2^szx×16 字节块大小）；Observe 选项值语义（0 注册/1 注销/2 确认）。
// 关键注意事项：
//   - 本文件只做协议层纯组件（可离线单测）；服务端"按块发送/接收续传"与"长连接观察者表 +
//     变更推送"的集成，由上层基于 Registry 前缀通配与连接地址实现；
//   - 校验：szx 上限 6、block 值 24-bit、块大小 16..1024；非法输入显式报错。
package coap

import "fmt"

// Blockwise/Observe option numbers（RFC 7959 / RFC 7641）。
const (
	OptionBlock2 = 23
	OptionBlock1 = 27
	OptionSize2  = 28
	OptionSize1  = 60
)

// ObserveValue 语义常量。
const (
	ObserveRegister    = 0
	ObserveDeregister  = 1
	ObserveAcknowledge = 2
)

// BlockValue 一个 block 选项的语义值。
type BlockValue struct {
	Num  uint32 // 块编号（0 起）
	More bool   // 是否还有后续块
	SZX  uint8  // 块大小指数：块大小 = 16 << szx (16..1024)
}

// BlockSize 返回按 SZX 推导的字节块大小。
func (b BlockValue) BlockSize() int { return 16 << b.SZX }

// EncodeBlockValue 编码为 RFC 7959 的 3 字节整型选项值（小端序布局：num 20bit,more,szx 3bit）。
func EncodeBlockValue(b BlockValue) ([]byte, error) {
	if b.SZX > 6 {
		return nil, fmt.Errorf("coap: szx %d 超限（0..6）", b.SZX)
	}
	if b.Num > 0x0FFFFF {
		return nil, fmt.Errorf("coap: block num 超 20bit")
	}
	v := (b.Num << 4) | (boolToU32(b.More) << 3) | uint32(b.SZX)
	// 3 字节大端序列化为选项值（RFC 表示顺序为 num…szx 由高到低字节承载同值）
	return []byte{byte(v >> 16), byte(v >> 8), byte(v)}, nil
}

// ParseBlockValue 从选项原始字节解析 BlockValue。
func ParseBlockValue(raw []byte) (BlockValue, error) {
	if len(raw) == 0 || len(raw) > 3 {
		return BlockValue{}, fmt.Errorf("coap: block 值长度非法 %d", len(raw))
	}
	var v uint32
	for _, b := range raw {
		v = v<<8 | uint32(b)
	}
	out := BlockValue{
		Num:  v >> 4,
		More: v&0x08 != 0,
		SZX:  uint8(v & 0x07),
	}
	if out.SZX > 6 {
		return BlockValue{}, fmt.Errorf("coap: szx 超限 %d", out.SZX)
	}
	return out, nil
}

// ParseObserveValue 解析 Observe 选项（0/1/2；值为空视为注册 0 的宽松兼容）。
func ParseObserveValue(raw []byte) (int, error) {
	if len(raw) == 0 {
		return ObserveRegister, nil
	}
	if len(raw) != 1 {
		return -1, fmt.Errorf("coap: observe 值长度非法")
	}
	v := int(raw[0])
	if v < ObserveRegister || v > ObserveAcknowledge {
		return -1, fmt.Errorf("coap: observe 值 %d 非法", v)
	}
	return v, nil
}

func boolToU32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}
