// 文件用途：TCP 接入帧解析纯函数（TP-03）——4 字节大端 length-prefix 与换行分隔两种帧格式。
// 核心逻辑：FrameAccumulator 以流式 Feed 消化粘包/半包（一次 Read 可能含多帧或半帧），
//
//	产出完整帧 payload；EncodeFrame 把下行 payload 编码为对应帧格式。全部无 IO，可独立单测。
//
// 关键注意事项：帧违规（超长帧）会使流"中毒"（poisoned）——二进制 length-prefix 流无法重同步，
//
//	中毒后 Feed 恒返回同一错误，调用方必须关闭连接；line 模式空行视为心跳跳过，\r\n 兼容。
//
// 重构建议：新增帧格式（如分隔符可配置）时扩展 FrameMode 与 feed 分支，Feed 的中毒语义保持不变。
package tcp

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// FrameMode 帧格式模式（protocols.tcp.frame-mode）。
type FrameMode string

const (
	// FrameModeLengthPrefix 4 字节大端长度前缀 + payload（默认，二进制安全）。
	FrameModeLengthPrefix FrameMode = "length-prefix"
	// FrameModeLine 换行（'\n'）分隔的文本帧（调试友好，payload 不得含换行）。
	FrameModeLine FrameMode = "line"
)

// DefaultMaxFrameSize 单帧 payload 上限默认值（字节）；防单连接撑爆内存。
const DefaultMaxFrameSize = 4096

// lengthPrefixHeaderSize length-prefix 帧头长度（4 字节大端）。
const lengthPrefixHeaderSize = 4

// 帧层哨兵错误（调用方按 errors.Is 判定；中毒流复用 ErrStreamPoisoned 语义）。
var (
	// ErrBadFrameMode 未知的帧模式（构造期即失败，fail-fast）。
	ErrBadFrameMode = errors.New("tcp: unknown frame mode")
	// ErrFrameTooLarge 帧超过上限——无法在流内恢复，连接必须关闭。
	ErrFrameTooLarge = errors.New("tcp: frame exceeds max size")
	// ErrStreamPoisoned 流已因此前帧错误中毒，后续 Feed 一律拒绝（防半帧静默拼接）。
	ErrStreamPoisoned = errors.New("tcp: stream poisoned by earlier framing error")
	// ErrInvalidPayload 下行 payload 无法按当前帧模式编码（line 模式含换行）。
	ErrInvalidPayload = errors.New("tcp: payload not encodable in this frame mode")
)

// ValidFrameMode 校验帧模式字符串是否受支持。
func ValidFrameMode(m FrameMode) bool {
	return m == FrameModeLengthPrefix || m == FrameModeLine
}

// FrameAccumulator 流式帧累积器：把字节流切成完整帧（纯内存，无 IO）。
// 非并发安全——每个 TCP 连接独占一个实例（单读 goroutine）。
type FrameAccumulator struct {
	mode    FrameMode
	maxSize int
	buf     []byte
	haltErr error // 非 nil 即中毒：后续 Feed 恒返回该错误
	frames  int   // 已完整解析的帧数（诊断面）
}

// NewFrameAccumulator 构造累积器；mode 非法返回 ErrBadFrameMode，
// maxSize<=0 取 DefaultMaxFrameSize。
func NewFrameAccumulator(mode FrameMode, maxSize int) (*FrameAccumulator, error) {
	if !ValidFrameMode(mode) {
		return nil, ErrBadFrameMode
	}
	if maxSize <= 0 {
		maxSize = DefaultMaxFrameSize
	}
	return &FrameAccumulator{mode: mode, maxSize: maxSize}, nil
}

// Feed 喂入一段字节流，返回本次完整解出的帧 payload（每帧独立拷贝，归调用方所有）。
// 粘包（一次多帧）与半包（帧不完整留存待续）都在此消化；帧违规返回致命错误并中毒，
// 调用方收到错误后必须丢弃累积器并关闭连接。中毒后 Feed 恒返回同时匹配
// ErrStreamPoisoned 与原始违规错误的包装错误（errors.Is 双命中）。
func (a *FrameAccumulator) Feed(chunk []byte) ([][]byte, error) {
	if a.haltErr != nil {
		return nil, fmt.Errorf("%w: %w", ErrStreamPoisoned, a.haltErr)
	}
	if len(chunk) > 0 {
		a.buf = append(a.buf, chunk...)
	}
	var out [][]byte
	for {
		frame, err := a.next()
		if err != nil {
			a.haltErr = err
			return out, err
		}
		if frame == nil {
			break // 半包：等待更多字节
		}
		a.frames++
		out = append(out, frame)
	}
	return out, nil
}

// Pending 返回当前缓冲中未成帧的字节数（半包观察面）。
func (a *FrameAccumulator) Pending() int { return len(a.buf) }

// FramesParsed 返回累计完整帧数（诊断面）。
func (a *FrameAccumulator) FramesParsed() int { return a.frames }

// next 解析下一个完整帧：返回 nil 表示半包等待；返回错误表示帧违规（中毒）。
func (a *FrameAccumulator) next() ([]byte, error) {
	switch a.mode {
	case FrameModeLengthPrefix:
		return a.nextLengthPrefixed()
	case FrameModeLine:
		return a.nextLine()
	}
	return nil, ErrBadFrameMode
}

// nextLengthPrefixed length-prefix 分支：先校验头部声明的长度（超限即中毒，不等收全），
// 再判断缓冲是否已含完整帧。
func (a *FrameAccumulator) nextLengthPrefixed() ([]byte, error) {
	if len(a.buf) < lengthPrefixHeaderSize {
		return nil, nil
	}
	n := binary.BigEndian.Uint32(a.buf[:lengthPrefixHeaderSize])
	if uint64(n) > uint64(a.maxSize) {
		return nil, ErrFrameTooLarge
	}
	total := lengthPrefixHeaderSize + int(n)
	if len(a.buf) < total {
		return nil, nil // 半包：帧体未收全
	}
	payload := make([]byte, n)
	copy(payload, a.buf[lengthPrefixHeaderSize:total])
	a.buf = a.buf[total:]
	return payload, nil
}

// nextLine 换行分隔分支：找到 '\n' 即成帧（剥尾部 '\r' 兼容 CRLF）；
// 空行是心跳，跳过不出帧。无换行的残留字节超过上限即中毒（该行不可能再合法）。
func (a *FrameAccumulator) nextLine() ([]byte, error) {
	idx := indexByte(a.buf, '\n')
	if idx < 0 {
		if len(a.buf) > a.maxSize {
			return nil, ErrFrameTooLarge
		}
		return nil, nil
	}
	line := a.buf[:idx]
	a.buf = a.buf[idx+1:]
	trimmed := trimSuffixCR(line)
	if len(trimmed) > a.maxSize {
		return nil, ErrFrameTooLarge
	}
	if len(trimmed) == 0 {
		return a.nextLine() // 空行心跳：继续找下一帧
	}
	payload := make([]byte, len(trimmed))
	copy(payload, trimmed)
	return payload, nil
}

// EncodeFrame 把下行 payload 编码为对应帧格式（下行命令写回设备用）：
// length-prefix 加 4 字节大端长度头；line 模式拒绝含 '\n' 的 payload（fail-closed，
// 绝不静默改写设备侧帧边界语义）。
func EncodeFrame(mode FrameMode, payload []byte, maxSize int) ([]byte, error) {
	if !ValidFrameMode(mode) {
		return nil, ErrBadFrameMode
	}
	if maxSize <= 0 {
		maxSize = DefaultMaxFrameSize
	}
	if len(payload) > maxSize {
		return nil, ErrFrameTooLarge
	}
	switch mode {
	case FrameModeLengthPrefix:
		frame := make([]byte, lengthPrefixHeaderSize+len(payload))
		binary.BigEndian.PutUint32(frame[:lengthPrefixHeaderSize], uint32(len(payload)))
		copy(frame[lengthPrefixHeaderSize:], payload)
		return frame, nil
	case FrameModeLine:
		if indexByte(payload, '\n') >= 0 {
			return nil, ErrInvalidPayload
		}
		frame := make([]byte, 0, len(payload)+1)
		frame = append(frame, payload...)
		return append(frame, '\n'), nil
	}
	return nil, ErrBadFrameMode
}

// indexByte / trimSuffixCR 小工具（避免为两处使用引入 bytes 包别名噪音）。
func indexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

func trimSuffixCR(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\r' {
		return b[:len(b)-1]
	}
	return b
}
