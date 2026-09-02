// 文件用途：寄存器点位模型与归一化（ROADMAP B1）。
// 核心逻辑：定义四类寄存器的数据类型约束与读写缩放约定。
// 关键注意事项：读值 = raw * Multiplier + Offset；写值做逆变换 (value-Offset)/Multiplier；
//   input/discrete 为只读来源，Writable 仅对 holding/coil 有意义。
package config

import (
	"fmt"
	"strings"
)

// RegisterPoint 单个寄存器点位映射。
type RegisterPoint struct {
	Key        string  `json:"key"`
	Type       string  `json:"type"` // holding | input | coil | discrete
	Address    uint16  `json:"address"`
	DataType   string  `json:"data_type"`
	Multiplier float64 `json:"multiplier"`
	Offset     float64 `json:"offset"`
	Writable   bool    `json:"writable"`
}

// Normalize 校验并填充点位默认值（类型/数据类型/缩放）。
func (r *RegisterPoint) Normalize() error {
	if strings.TrimSpace(r.Key) == "" {
		return fmt.Errorf("key is required")
	}
	r.Type = strings.ToLower(strings.TrimSpace(r.Type))
	switch r.Type {
	case "holding", "input":
		if err := r.validateRegisterDataType(); err != nil {
			return err
		}
	case "coil", "discrete":
		r.DataType = "bool"
	default:
		return fmt.Errorf("unsupported register type %q", r.Type)
	}
	// 缩放默认值必须对所有可归一化的点位生效；
	// 读值 = raw*Multiplier+offset，Multiplier 缺省为 0 会把读值全部清零。
	if r.Multiplier == 0 {
		r.Multiplier = 1
	}
	return nil
}

func (r *RegisterPoint) validateRegisterDataType() error {
	if r.DataType == "" {
		r.DataType = "u16"
	}
	r.DataType = strings.ToLower(strings.TrimSpace(r.DataType))
	switch r.DataType {
	case "u16", "i16", "u32", "i32", "f32":
		return nil
	default:
		return fmt.Errorf("register type %q unsupported data_type %q", r.Type, r.DataType)
	}
}

// RegisterCount 返回该点位需要读取的寄存器数量。
func (r *RegisterPoint) RegisterCount() uint16 {
	switch r.DataType {
	case "u32", "i32", "f32":
		return 2
	default:
		return 1
	}
}
