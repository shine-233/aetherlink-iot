// 文件用途：帧解析纯函数单测（TP-03）——length-prefix 与 line 两种模式的粘包/半包/违规流。
// 核心逻辑：Go 表驱动钉死：单帧、粘包（一次多帧）、半包（逐字节喂）、空帧、超限中毒、
// CRLF 兼容、空行心跳跳过；EncodeFrame 往返与 line 模式含换行拒绝。
// 关键注意事项：中毒语义必须验证"后续 Feed 恒错误"（防半帧静默拼接的回归）。
// 重构建议：新增帧模式时在此补对应模式的粘包/半包/违规三组用例，保持契约完整。
package tcp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// lpFrame 按 length-prefix 帧格式编码（测试辅助）。
func lpFrame(payload string) []byte {
	b := make([]byte, lengthPrefixHeaderSize+len(payload))
	binary.BigEndian.PutUint32(b[:lengthPrefixHeaderSize], uint32(len(payload)))
	copy(b[lengthPrefixHeaderSize:], payload)
	return b
}

func mustAcc(t *testing.T, mode FrameMode, max int) *FrameAccumulator {
	t.Helper()
	acc, err := NewFrameAccumulator(mode, max)
	if err != nil {
		t.Fatalf("NewFrameAccumulator(%s): %v", mode, err)
	}
	return acc
}

func joinFrames(frames ...[]byte) []byte {
	var out []byte
	for _, f := range frames {
		out = append(out, f...)
	}
	return out
}

func TestNewFrameAccumulatorBadMode(t *testing.T) {
	if _, err := NewFrameAccumulator(FrameMode("bogus"), 0); !errors.Is(err, ErrBadFrameMode) {
		t.Fatalf("err = %v, want ErrBadFrameMode", err)
	}
}

func TestFeedLengthPrefixSingleFrame(t *testing.T) {
	acc := mustAcc(t, FrameModeLengthPrefix, 64)
	frames, err := acc.Feed(lpFrame(`{"a":1}`))
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(frames) != 1 || string(frames[0]) != `{"a":1}` {
		t.Fatalf("frames = %v, want single {\"a\":1}", frames)
	}
	if acc.Pending() != 0 {
		t.Fatalf("pending = %d, want 0", acc.Pending())
	}
}

func TestFeedLengthPrefixStickyFrames(t *testing.T) {
	acc := mustAcc(t, FrameModeLengthPrefix, 64)
	frames, err := acc.Feed(joinFrames(lpFrame("one"), lpFrame("two"), lpFrame("3")))
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(frames) != 3 || string(frames[0]) != "one" || string(frames[1]) != "two" || string(frames[2]) != "3" {
		t.Fatalf("sticky frames = %v", frames)
	}
}

func TestFeedLengthPrefixHalfPacketByteByByte(t *testing.T) {
	acc := mustAcc(t, FrameModeLengthPrefix, 64)
	stream := joinFrames(lpFrame("hello"), lpFrame("world"))
	var got []string
	for i := 0; i < len(stream); i++ {
		frames, err := acc.Feed(stream[i : i+1])
		if err != nil {
			t.Fatalf("Feed byte %d: %v", i, err)
		}
		for _, f := range frames {
			got = append(got, string(f))
		}
	}
	if len(got) != 2 || got[0] != "hello" || got[1] != "world" {
		t.Fatalf("half-packet frames = %v", got)
	}
}

func TestFeedLengthPrefixEmptyPayloadFrame(t *testing.T) {
	acc := mustAcc(t, FrameModeLengthPrefix, 64)
	frames, err := acc.Feed(lpFrame(""))
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(frames) != 1 || len(frames[0]) != 0 {
		t.Fatalf("frames = %v, want one empty frame", frames)
	}
}

func TestFeedLengthPrefixOversizedPoisons(t *testing.T) {
	acc := mustAcc(t, FrameModeLengthPrefix, 8)
	// 头部声明 100 字节（> 上限 8）：不必等收全即中毒。
	frames, err := acc.Feed([]byte{0, 0, 0, 100})
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("err = %v, want ErrFrameTooLarge", err)
	}
	if frames != nil {
		t.Fatalf("frames = %v, want nil", frames)
	}
	if _, err := acc.Feed([]byte{1}); !errors.Is(err, ErrStreamPoisoned) {
		t.Fatalf("subsequent err = %v, want ErrStreamPoisoned", err)
	}
}

func TestFeedLengthPrefixTrailingPartialStaysBuffered(t *testing.T) {
	acc := mustAcc(t, FrameModeLengthPrefix, 64)
	// "ok" 完整帧 + 声明 5 字节但只到 3 字节的半包帧体。
	partial := joinFrames([]byte{0, 0, 0, 5}, []byte("par"))
	frames, err := acc.Feed(joinFrames(lpFrame("ok"), partial))
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	if len(frames) != 1 || string(frames[0]) != "ok" {
		t.Fatalf("frames = %v, want single ok", frames)
	}
	if acc.Pending() != lengthPrefixHeaderSize+3 {
		t.Fatalf("pending = %d, want partial frame buffered", acc.Pending())
	}
}

func TestFeedLineSingleStickyCRLFEmptyLines(t *testing.T) {
	acc := mustAcc(t, FrameModeLine, 64)
	stream := joinFrames(
		[]byte("hello\n"),
		[]byte("\r\n"),   // 空行心跳：跳过
		[]byte("a\r\n"),  // CRLF 兼容
		[]byte("b\nc\n"), // 粘包
	)
	frames, err := acc.Feed(stream)
	if err != nil {
		t.Fatalf("Feed: %v", err)
	}
	want := []string{"hello", "a", "b", "c"}
	if len(frames) != len(want) {
		t.Fatalf("frames = %v, want %v", frames, want)
	}
	for i, w := range want {
		if string(frames[i]) != w {
			t.Fatalf("frames[%d] = %q, want %q", i, frames[i], w)
		}
	}
}

func TestFeedLineHalfPacketByteByByte(t *testing.T) {
	acc := mustAcc(t, FrameModeLine, 64)
	stream := []byte("x\nyz\n")
	var got []string
	for i := 0; i < len(stream); i++ {
		frames, err := acc.Feed(stream[i : i+1])
		if err != nil {
			t.Fatalf("Feed byte %d: %v", i, err)
		}
		for _, f := range frames {
			got = append(got, string(f))
		}
	}
	if len(got) != 2 || got[0] != "x" || got[1] != "yz" {
		t.Fatalf("half-packet frames = %v", got)
	}
}

func TestFeedLineOversizedNoNewlinePoisons(t *testing.T) {
	acc := mustAcc(t, FrameModeLine, 8)
	frames, err := acc.Feed(bytes.Repeat([]byte("a"), 9))
	if !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("err = %v, want ErrFrameTooLarge", err)
	}
	if frames != nil {
		t.Fatalf("frames = %v, want nil", frames)
	}
	if _, err := acc.Feed([]byte("\n")); !errors.Is(err, ErrStreamPoisoned) {
		t.Fatalf("subsequent err = %v, want ErrStreamPoisoned", err)
	}
}

func TestEncodeFrameRoundtrip(t *testing.T) {
	for _, mode := range []FrameMode{FrameModeLengthPrefix, FrameModeLine} {
		acc := mustAcc(t, mode, 64)
		frame, err := EncodeFrame(mode, []byte("payload"), 64)
		if err != nil {
			t.Fatalf("EncodeFrame(%s): %v", mode, err)
		}
		frames, err := acc.Feed(frame)
		if err != nil {
			t.Fatalf("Feed(%s): %v", mode, err)
		}
		if len(frames) != 1 || string(frames[0]) != "payload" {
			t.Fatalf("mode %s roundtrip frames = %v", mode, frames)
		}
	}
}

func TestEncodeFrameLineRejectsEmbeddedNewline(t *testing.T) {
	if _, err := EncodeFrame(FrameModeLine, []byte("a\nb"), 64); !errors.Is(err, ErrInvalidPayload) {
		t.Fatalf("err = %v, want ErrInvalidPayload", err)
	}
}

func TestEncodeFrameOversizedRejected(t *testing.T) {
	if _, err := EncodeFrame(FrameModeLengthPrefix, bytes.Repeat([]byte("x"), 11), 10); !errors.Is(err, ErrFrameTooLarge) {
		t.Fatalf("err = %v, want ErrFrameTooLarge", err)
	}
	if _, err := EncodeFrame(FrameMode("bogus"), []byte("x"), 10); !errors.Is(err, ErrBadFrameMode) {
		t.Fatalf("err = %v, want ErrBadFrameMode", err)
	}
}
