// 文件用途：提供 password utils 相关的后端通用工具能力。
// 核心逻辑：封装可复用的格式处理、校验、加密、脚本或命令构造逻辑，供业务层按需调用，主要围绕 const passwordSpecialChars、func ValidatePassword 等声明展开。
// 关键注意事项：工具函数常被多个模块共享，修改需保持入参约束、返回值和错误语义兼容。
// 重构建议：后续可按职责继续拆分工具包，减少无关工具之间的隐式耦合。

package utils

import (
	"strings"
	"unicode"

	"aetherlink-iot/backend/pkg/errcode"
)

const passwordSpecialChars = "!@#$%^&*()_+-=[]{};\\':\"|,./<>?"

// ValidatePassword validates the RDI account password policy.
// Passwords must be 8-20 ASCII characters and include uppercase,
// lowercase, number, and special character classes.
func ValidatePassword(password string) error {
	if len(password) < 8 || len(password) > 20 {
		return errcode.New(200040)
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)
	invalidChars := make([]rune, 0)

	for _, char := range password {
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsDigit(char):
			hasNumber = true
		case strings.ContainsRune(passwordSpecialChars, char):
			hasSpecial = true
		default:
			invalidChars = append(invalidChars, char)
		}
	}

	if len(invalidChars) > 0 {
		return errcode.WithVars(200053, map[string]interface{}{
			"invalid_chars": string(invalidChars),
		})
	}

	var missingElements []string
	if !hasUpper {
		missingElements = append(missingElements, "大写字母")
	}
	if !hasLower {
		missingElements = append(missingElements, "小写字母")
	}
	if !hasNumber {
		missingElements = append(missingElements, "数字")
	}
	if !hasSpecial {
		missingElements = append(missingElements, "特殊字符")
	}

	if len(missingElements) > 0 {
		return errcode.WithVars(200054, map[string]interface{}{
			"missing_elements": strings.Join(missingElements, "、"),
		})
	}

	return nil
}
