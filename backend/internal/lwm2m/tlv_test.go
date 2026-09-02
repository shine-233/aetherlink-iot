// 文件用途：LwM2M TLV 上报编码单测——单资源/多资源/对象实例逐层往返、整数宽度编码、
// 长度/截断边界、16bit id 与 4 字节长度路径。
package lwm2m

import (
	"bytes"
	"testing"
)

func TestResourceWithValueRoundTrip(t *testing.T) {
	raw := EncodeResourceWithValue(5700, ValueInt(2350))
	entries, err := DecodeTLV(raw)
	if err != nil || len(entries) != 1 {
		t.Fatalf("解析失败: %v %d", err, len(entries))
	}
	e := entries[0]
	if e.Type != tlvResourceWithValue || e.ID != 5700 {
		t.Fatalf("type/id 不符: %02x/%d", e.Type, e.ID)
	}
	if !bytes.Equal(e.Value, ValueInt(2350)) {
		t.Fatalf("值不符: %x", e.Value)
	}
}

func TestNestedObjectInstance(t *testing.T) {
	// /19/0/0 温度模拟：ObjectInstance(0) 内包 ResourceWithValue(0)
	child := EncodeResourceWithValue(0, []byte("payload"))
	inst := EncodeObjectInstance(0, child)
	entries, err := DecodeTLV(inst)
	if err != nil || len(entries) != 1 {
		t.Fatalf("解析: %v", err)
	}
	if entries[0].Type != tlvObjectInstance || entries[0].ID != 0 {
		t.Fatalf("对象实例不符: %02x/%d", entries[0].Type, entries[0].ID)
	}
	// 展开内层
	inner, err := DecodeTLV(entries[0].Value)
	if err != nil || len(inner) != 1 || inner[0].ID != 0 {
		t.Fatalf("内层不符: %v %d", err, len(inner))
	}
}

func TestMultipleResourceComposition(t *testing.T) {
	// 多资源：两个资源实例（01）组成 MultipleResource（10）
	ri1 := EncodeResourceInstance(1, ValueInt(10))
	ri2 := EncodeResourceInstance(2, []byte("b"))
	mr := EncodeMultipleResource(6000, append(ri1, ri2...))
	entries, err := DecodeTLV(mr)
	if err != nil || len(entries) != 1 || entries[0].Type != tlvMultipleResource {
		t.Fatalf("解析: %v", err)
	}
	inner, err := DecodeTLV(entries[0].Value)
	if err != nil || len(inner) != 2 {
		t.Fatalf("多资源子项应 2: %v %d", err, len(inner))
	}
}

func TestValueIntWidths(t *testing.T) {
	cases := map[int64]int{0: 1, 127: 1, 128: 2, 30000: 2, 40000: 4, -1: 1, 1 << 31: 8}
	for v, want := range cases {
		if got := len(ValueInt(v)); got != want {
			t.Errorf("ValueInt(%d) 长度=%d want %d", v, got, want)
		}
	}
}

func TestDecodeRejectsTruncated(t *testing.T) {
	raw := EncodeResourceWithValue(0, []byte("hello world"))
	for cut := 1; cut < len(raw); cut++ {
		if _, err := DecodeTLV(raw[:cut]); err == nil {
			t.Fatalf("截断到 %d 应报错", cut)
		}
	}
	// 16bit id 与 2 字节长度
	big := EncodeResourceWithValue(0x1234, bytes.Repeat([]byte{1}, 300))
	entries, err := DecodeTLV(big)
	if err != nil || entries[0].ID != 0x1234 || len(entries[0].Value) != 300 {
		t.Fatalf("16bit id/2B 长度路径失败: %v", err)
	}
}
