// 文件用途：blockwise/observe 协议组件单测——block 值编解码往返、边界（szx>6/num 越界/
// 空值/长度>3）、16/64/512/1024 块大小映射、observe 值语义。
package coap

import (
	"testing"
)

func TestBlockValueRoundTrip(t *testing.T) {
	cases := []BlockValue{
		{Num: 0, More: false, SZX: 0},  // 16B 首块
		{Num: 5, More: true, SZX: 2},   // 64B
		{Num: 1048575, More: false, SZX: 6}, // 1024B max num
		{Num: 1, More: true, SZX: 3},   // 128B
	}
	for _, in := range cases {
		raw, err := EncodeBlockValue(in)
		if err != nil {
			t.Fatalf("encode %+v: %v", in, err)
		}
		out, err := ParseBlockValue(raw)
		if err != nil {
			t.Fatalf("parse %+v: %v", in, err)
		}
		if out != in {
			t.Fatalf("往返不一致: in=%+v out=%+v", in, out)
		}
	}
}

func TestBlockValueRejectsBadInput(t *testing.T) {
	if _, err := EncodeBlockValue(BlockValue{SZX: 7}); err == nil {
		t.Fatal("szx=7 应报错")
	}
	if _, err := EncodeBlockValue(BlockValue{Num: 0x100000}); err == nil {
		t.Fatal("num 超 20bit 应报错")
	}
	if _, err := ParseBlockValue(nil); err == nil {
		t.Fatal("空值应报错")
	}
	if _, err := ParseBlockValue([]byte{0, 0, 0, 0}); err == nil {
		t.Fatal("4 字节值应报错")
	}
}

func TestBlockSizeMapping(t *testing.T) {
	if (BlockValue{SZX: 0}).BlockSize() != 16 {
		t.Fatal("szx0=16")
	}
	if (BlockValue{SZX: 6}).BlockSize() != 1024 {
		t.Fatal("szx6=1024")
	}
	if (BlockValue{SZX: 2}).BlockSize() != 64 {
		t.Fatal("szx2=64")
	}
}

func TestParseObserveValueSemantics(t *testing.T) {
	if v, err := ParseObserveValue([]byte{0}); err != nil || v != ObserveRegister {
		t.Fatalf("0=注册: %d %v", v, err)
	}
	if v, _ := ParseObserveValue([]byte{1}); v != ObserveDeregister {
		t.Fatal("1=注销")
	}
	if v, _ := ParseObserveValue(nil); v != ObserveRegister {
		t.Fatal("空值应视为注册")
	}
	if _, err := ParseObserveValue([]byte{9}); err == nil {
		t.Fatal("observe=9 应报错")
	}
	if _, err := ParseObserveValue([]byte{1, 2}); err == nil {
		t.Fatal("observe 多字节应报错")
	}
}
