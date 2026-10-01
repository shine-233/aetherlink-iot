// 文件用途：启动期安全关键配置校验（fail-fast 防线）。
// 核心逻辑：应用装配完配置后校验 JWT 签名密钥，拒绝空值、已知占位符和弱长度密钥直接启动。
// 关键注意事项：占位符密钥上线等于允许伪造任意用户 token；本文件是有意的"配置失误即拒启"防线，
// 放宽规则必须同步 deploy/doctor 的弱值黑名单与相关契约测试。
// 重构建议：后续把 postgres/redis/mqtt 口令的占位符检查收敛到同一入口统一报告。
package app

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// minJWTKeyLength 与 deploy/doctor.sh 的 JWT 密钥长度阈值保持一致。
const minJWTKeyLength = 32

// validateSecurityCriticalConfig 在 NewApplication 完成选项装配后执行；
// app.Config 为 nil 时跳过（纯单元测试装配路径）。
func validateSecurityCriticalConfig(v *viper.Viper) error {
	return validateJWTSigningKey(v.GetString("jwt.key"))
}

func validateJWTSigningKey(rawKey string) error {
	key := strings.TrimSpace(rawKey)
	if key == "" {
		return fmt.Errorf("refusing to start: jwt.key is empty; %s", insecureJWTKeyFixHint())
	}
	lowered := strings.ToLower(key)
	if strings.Contains(lowered, "change_me") || strings.Contains(lowered, "changeme") {
		return fmt.Errorf("refusing to start: jwt.key is a known placeholder (%s...); %s", key[:min(len(key), 16)], insecureJWTKeyFixHint())
	}
	if len(key) < minJWTKeyLength {
		return fmt.Errorf("refusing to start: jwt.key is shorter than %d characters; %s", minJWTKeyLength, insecureJWTKeyFixHint())
	}
	return nil
}

func insecureJWTKeyFixHint() string {
	return "generate a strong secret (e.g. openssl rand -base64 48) and provide it via GOTP_JWT_KEY or configs jwt.key"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
