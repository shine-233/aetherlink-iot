// serialize.go is a Go file that contains the code to serialize data to JSON format.

// 文件用途：提供 serialize 相关的后端通用工具能力。
// 核心逻辑：封装可复用的格式处理、校验、加密、脚本或命令构造逻辑，供业务层按需调用，主要围绕 func SerializeData 等声明展开。
// 关键注意事项：工具函数常被多个模块共享，修改需保持入参约束、返回值和错误语义兼容。
// 重构建议：后续可按职责继续拆分工具包，减少无关工具之间的隐式耦合。

package utils

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/sirupsen/logrus"
)

func SerializeData(source, target interface{}) (interface{}, error) {
	jsonData, err := json.Marshal(source)
	if err != nil {
		logrus.Error("JSON serialize failed:", err)
		return nil, err
	}

	targetValue := reflect.ValueOf(target)
	if !targetValue.IsValid() {
		return nil, fmt.Errorf("target cannot be nil")
	}

	if targetValue.Kind() == reflect.Ptr {
		if targetValue.IsNil() {
			return nil, fmt.Errorf("target cannot be nil")
		}
		err = json.Unmarshal(jsonData, target)
		if err != nil {
			logrus.Error("JSON deserialize failed:", err)
			return nil, err
		}
		return target, nil
	}

	typedTarget := reflect.New(targetValue.Type())
	err = json.Unmarshal(jsonData, typedTarget.Interface())
	if err != nil {
		logrus.Error("JSON deserialize failed:", err)
		return nil, err
	}

	return typedTarget.Elem().Interface(), nil
}
