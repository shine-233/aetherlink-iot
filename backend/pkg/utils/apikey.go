// 文件用途：提供 apikey 相关的后端通用工具能力。
// 核心逻辑：封装可复用的格式处理、校验、加密、脚本或命令构造逻辑，供业务层按需调用，主要围绕 func GenerateAPIKey 等声明展开。
// 关键注意事项：工具函数常被多个模块共享，修改需保持入参约束、返回值和错误语义兼容。
// 重构建议：后续可按职责继续拆分工具包，减少无关工具之间的隐式耦合。

package utils

import (
	"crypto/rand"
	"crypto/sha256"
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

// HashAPIKey 返回 API Key 的 SHA-256 十六进制摘要。
// API Key 是 256bit 高熵随机值，不是人类口令，因此用快速哈希即可；
// 不要换成 bcrypt 等慢哈希，那会让每次开放接口鉴权平白增加数百毫秒延迟。
// CodeQL go/weak-sensitive-data-hashing 会按"口令哈希"场景提示 SHA-256 偏弱，
// 这里属于凭据查找指纹而非口令存储，属已知误报（与 Stripe/GitHub 的 key 存储方案一致）。
// 数据库只允许存储该摘要，明文仅在创建响应中返回一次。
func HashAPIKey(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey)) // codeql[go/weak-sensitive-data-hashing]
	return hex.EncodeToString(sum[:])
}

// APIKeyDisplayPrefix 返回用于列表展示的密钥前缀（含 sk_ 头与 8 个十六进制字符）。
// 前缀信息量不足以还原密钥，但足够让用户在列表中辨认条目。
func APIKeyDisplayPrefix(apiKey string) string {
	if len(apiKey) <= 11 {
		return apiKey
	}
	return apiKey[:11]
}
