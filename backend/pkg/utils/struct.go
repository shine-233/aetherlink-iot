// 文件用途：提供 struct 相关的后端通用工具能力。
// 核心逻辑：封装可复用的格式处理、校验、加密、脚本或命令构造逻辑，供业务层按需调用，主要围绕 func StructToMap 等声明展开。
// 关键注意事项：工具函数常被多个模块共享，修改需保持入参约束、返回值和错误语义兼容。
// 重构建议：后续可按职责继续拆分工具包，减少无关工具之间的隐式耦合。

package utils

import (
	"fmt"
	"reflect"
)

func StructToMap(obj interface{}) (map[string]interface{}, error) {
	// 确保输入是一个指针，并且不是nil
	if reflect.ValueOf(obj).Kind() != reflect.Ptr || reflect.ValueOf(obj).IsNil() {
		return nil, fmt.Errorf("input must be a non-nil pointer")
	}

	// 获取指针指向的实际结构体
	val := reflect.ValueOf(obj).Elem()

	// 创建映射
	output := make(map[string]interface{})

	// 遍历结构体的所有字段
	for i := 0; i < val.NumField(); i++ {
		// 获取字段的值
		valueField := val.Field(i)

		// 获取字段的类型
		typeField := val.Type().Field(i)

		// 获取字段名
		fieldName := typeField.Name

		// 将字段名和对应的值添加到映射中
		output[fieldName] = valueField.Interface()
	}

	return output, nil
}
