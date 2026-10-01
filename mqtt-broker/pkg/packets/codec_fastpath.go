// 文件用途：控制包编解码热路径的零分配辅助函数。
// 核心逻辑：
//   - appendRemainLength：把变长整数（Remaining Length / Subscription Identifier）追加进
//     调用方提供的切片，替代每次 make 新切片的 DecodeRemainLength。
//   - readVarInt：具体类型 *bytes.Buffer 版的变长整数读取；EncodeRemainLength 接收
//     io.ByteReader 接口会让传入的 bytes.Buffer 逃逸到堆上。
//   - topicMatchFast：逐层比较主题与过滤器，不再切分出两个 [][]byte。
//
// 关键注意事项：对外函数（DecodeRemainLength/EncodeRemainLength/TopicMatch）签名与语义不变，
// 仅内部热路径改用本文件实现；超限/畸形输入的错误码与旧实现一致。
package packets

import (
	"bytes"
	"io"

	"github.com/DrmagicE/gmqtt/pkg/codes"
)

// maxRemainLength 是 MQTT 变长整数可表示的最大值（4 字节）。
const maxRemainLength = 268435455

// appendRemainLength 将 length 以 MQTT 变长整数编码追加到 dst；length 超出范围时返回 ErrMalformed。
func appendRemainLength(dst []byte, length int) ([]byte, error) {
	if length < 0 || length > maxRemainLength {
		return dst, codes.ErrMalformed
	}
	for {
		encodedByte := byte(length % 128)
		length /= 128
		if length > 0 {
			encodedByte |= 128
		}
		dst = append(dst, encodedByte)
		if length == 0 {
			return dst, nil
		}
	}
}

// writeRemainLength 把变长整数直接写入 bytes.Buffer，使用栈上暂存数组，不产生堆分配。
func writeRemainLength(w *bytes.Buffer, length int) {
	var tmp [4]byte
	b, _ := appendRemainLength(tmp[:0], length)
	w.Write(b)
}

// readVarInt 是 EncodeRemainLength 的 *bytes.Buffer 特化版本，语义完全一致
// （读到 EOF 时按 0 字节继续，与旧实现保持兼容）。
func readVarInt(r *bytes.Buffer) (int, error) {
	var vbi uint32
	var multiplier uint32
	for {
		digit, err := r.ReadByte()
		if err != nil && err != io.EOF {
			return 0, err
		}
		vbi |= uint32(digit&127) << multiplier
		if vbi > maxRemainLength {
			return 0, codes.ErrMalformed
		}
		if (digit & 128) == 0 {
			break
		}
		multiplier += 7
	}
	return int(vbi), nil
}

// packFixHeader 编码固定报头。目标实现 io.ByteWriter（bufio.Writer / bytes.Buffer）时
// 逐字节写入以避免堆分配；否则退回一次性 Write。
func packFixHeader(w io.Writer, first byte, remainLength int) error {
	var tmp [5]byte
	b := append(tmp[:0], first)
	b, err := appendRemainLength(b, remainLength)
	if err != nil {
		return err
	}
	if bw, ok := w.(io.ByteWriter); ok {
		for _, c := range b {
			if err := bw.WriteByte(c); err != nil {
				return err
			}
		}
		return nil
	}
	// 非 ByteWriter：拷贝到堆切片再写，避免把栈数组暴露给未知 Writer 实现。
	_, err = w.Write(append([]byte(nil), b...))
	return err
}

// topicMatchFast 逐层匹配 topic 与 topicFilter，语义与旧的 splitTopicLevels 实现一致。
func topicMatchFast(topic []byte, filter []byte) bool {
	for {
		var tLevel, fLevel []byte
		tEnd, fEnd := false, false
		if pos := bytes.IndexByte(filter, '/'); pos >= 0 {
			fLevel, filter = filter[:pos], filter[pos+1:]
		} else {
			fLevel, fEnd = filter, true
		}
		// 多层通配符只能是过滤器最后一层，匹配剩余所有层级（含零层）。
		if fEnd && len(fLevel) == 1 && fLevel[0] == '#' {
			return true
		}
		if topic == nil {
			// 主题层级已耗尽而过滤器还有层级。
			return false
		}
		if pos := bytes.IndexByte(topic, '/'); pos >= 0 {
			tLevel, topic = topic[:pos], topic[pos+1:]
		} else {
			tLevel, tEnd = topic, true
		}
		if !(len(fLevel) == 1 && fLevel[0] == '+') && !bytes.Equal(tLevel, fLevel) {
			return false
		}
		if fEnd {
			return tEnd
		}
		if tEnd {
			// 用 nil 标记主题已无层级（区别于空层级 []byte{}）。
			topic = nil
		}
	}
}

// isASCIITopicSafe 判断字节是否为可直接通过 UTF-8/控制字符校验的 ASCII（0x20..0x7e）。
func isASCIITopicSafe(c byte) bool {
	return c >= 0x20 && c < 0x7f
}
