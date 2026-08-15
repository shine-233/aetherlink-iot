// 文件用途：验证 bitmap 包的基本置位、清位和边界读取行为。
// 核心逻辑：创建最大容量位图，对普通 offset 和边界 offset 执行 Set/Get 断言。
// 关键注意事项：当前测试保留 offset == size 可写的既有行为，不在文档化批次中改变契约。
// 重构建议：后续若调整边界语义，应先补充 size-1、size、size+1 的表驱动用例。
package bitmap

import (
	"testing"
)

func TestBitmap(t *testing.T) {

	size := uint16(MaxSize)
	b := New(size)
	if b.Size() != size {
		t.Fatalf("wrong size %d", size)
	}

	b.Set(1, 1)
	if b.Get(1) != 1 {
		t.Fatalf("wrong value at bit %d", 1)
	}

	b.Set(1, 0)
	if b.Get(100) != 0 {
		t.Fatalf("wrong value at bit %d", 0)
	}

	b.Set(size, 1)
	if b.Get(size) != 1 {
		t.Fatalf("wrong value at bit %d", size)
	}

	b.Set(size, 0)
	if b.Get(size) != 0 {
		t.Fatalf("wrong value at bit %d", size)
	}

	b.Set(MaxSize, 1)
	v := b.Get(MaxSize)
	if v != 1 {
		t.Fatalf("wrong value %d", v)
	}
}
