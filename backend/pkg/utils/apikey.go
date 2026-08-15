// 文件用途：提供 apikey 相关的后端通用工具能力。
// 核心逻辑：封装可复用的格式处理、校验、加密、脚本或命令构造逻辑，供业务层按需调用，主要围绕 func GenerateAPIKey 等声明展开。
// 关键注意事项：工具函数常被多个模块共享，修改需保持入参约束、返回值和错误语义兼容。
// 重构建议：后续可按职责继续拆分工具包，减少无关工具之间的隐式耦合。

package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateAPIKey 生成一个 API Key
func GenerateAPIKey() (string, error) {
	// 生成32字节的随机数
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("生成APIKey失败: %v", err)
	}

	// 添加sk_前缀并转为hex格式
	return fmt.Sprintf("sk_%s", hex.EncodeToString(bytes)), nil
}
