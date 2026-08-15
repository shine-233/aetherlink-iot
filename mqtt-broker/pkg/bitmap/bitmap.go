// 文件用途：维护 pkg\bitmap\bitmap.go 所属 broker 包的手写 Go 代码。
// 核心逻辑：承载 MQTT broker 的领域模型、接口定义或测试支撑。
// 关键注意事项：本次仅补文件头不改变运行逻辑，后续修改需按所在包补充验证。
// 重构建议：后续可按职责拆分深模块，并为关键边界补齐契约测试。

// Package bitmap 提供 MQTT Broker 内部使用的轻量位图结构。
package bitmap

// MaxSize 是当前位图支持的最大 bit 位数量。
const MaxSize = uint16(65535)

// Bitmap 使用字节切片存储固定范围内的 bit 状态。
type Bitmap struct {
	vals []byte
	size uint16
}

// New 初始化一个 Bitmap；size 为 0 或超过上限时回退到 MaxSize。
func New(size uint16) *Bitmap {
	if size == 0 || size >= MaxSize {
		size = MaxSize
	} else if remainder := size % 8; remainder != 0 {
		size += 8 - remainder
	}
	return &Bitmap{size: size, vals: make([]byte, size>>3+1)}
}

// Size 返回当前位图声明的 bit 容量。
func (b *Bitmap) Size() uint16 {
	return b.size
}

// Set 将 offset 位置的值设置为 0 或 1，越界时返回 false。
func (b *Bitmap) Set(offset uint16, value uint8) bool {
	if b.size < offset {
		return false
	}

	index, pos := offset>>3, offset&0x07

	if value == 0 {
		b.vals[index] &^= 0x01 << pos
	} else {
		b.vals[index] |= 0x01 << pos
	}

	return true
}

// Get 获取 offset 位置的 bit 值；越界时返回 0。
func (b *Bitmap) Get(offset uint16) uint8 {
	if b.size < offset {
		return 0
	}

	index, pos := offset>>3, offset&0x07

	return (b.vals[index] >> pos) & 0x01
}
