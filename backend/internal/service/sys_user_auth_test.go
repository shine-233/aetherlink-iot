// 文件用途：验证系统用户认证密码策略和 bcrypt helper。
// 核心逻辑：覆盖密码复杂度、长度、hash 校验和安全字符串辅助函数。
// 关键注意事项：认证测试要避免弱密码回归，并确保 hash 错误或格式异常时 fail-closed。
// 重构建议：集中密码策略常量，补齐锁定、验证码和登录审计之间的集成边界。
package service

import (
	"testing"

	"aetherlink-iot/backend/pkg/utils"

	"github.com/stretchr/testify/assert"
)

// --- ValidatePassword (used in sys_user_auth.go EmailRegister) ---

func TestSysUserAuthValidatePassword_ValidPassword(t *testing.T) {
	err := utils.ValidatePassword("Abc123!@")
	assert.NoError(t, err)
}

func TestSysUserAuthValidatePassword_AllCharClasses(t *testing.T) {
	err := utils.ValidatePassword("Aa1!Bb2@")
	assert.NoError(t, err)
}

func TestSysUserAuthValidatePassword_TooShort(t *testing.T) {
	err := utils.ValidatePassword("Aa1!")
	assert.Error(t, err)
}

func TestSysUserAuthValidatePassword_TooLong(t *testing.T) {
	err := utils.ValidatePassword("Aa1!abcdefghijklmnopqrstuvwxyz123456")
	assert.Error(t, err)
}

func TestSysUserAuthValidatePassword_NoUppercase(t *testing.T) {
	err := utils.ValidatePassword("abc123!@")
	assert.Error(t, err)
}

func TestSysUserAuthValidatePassword_NoLowercase(t *testing.T) {
	err := utils.ValidatePassword("ABC123!@")
	assert.Error(t, err)
}

func TestSysUserAuthValidatePassword_NoNumber(t *testing.T) {
	err := utils.ValidatePassword("Abcdef!@")
	assert.Error(t, err)
}

func TestSysUserAuthValidatePassword_NoSpecial(t *testing.T) {
	err := utils.ValidatePassword("Abc12345")
	assert.Error(t, err)
}

func TestSysUserAuthValidatePassword_InvalidChars(t *testing.T) {
	err := utils.ValidatePassword("Abc123中文!@")
	assert.Error(t, err)
}

func TestSysUserAuthValidatePassword_MinLength(t *testing.T) {
	err := utils.ValidatePassword("Aa1!Aa1!")
	assert.NoError(t, err)
}

func TestSysUserAuthValidatePassword_MaxLength(t *testing.T) {
	err := utils.ValidatePassword("Aa1!Aa1!Aa1!Aa1!Aa1!") // 20 chars
	assert.NoError(t, err)
}

// --- BcryptHash & BcryptCheck (used in sys_user_auth.go) ---

func TestSysUserAuthBcryptHashAndCheck(t *testing.T) {
	password := "TestPass123!@"
	hash := utils.BcryptHash(password)
	assert.NotEmpty(t, hash)
	assert.NotEqual(t, password, hash)
	assert.True(t, utils.BcryptCheck(password, hash))
}

func TestSysUserAuthBcryptCheck_WrongPassword(t *testing.T) {
	password := "TestPass123!@"
	hash := utils.BcryptHash(password)
	assert.False(t, utils.BcryptCheck("WrongPass123!@", hash))
}

func TestSysUserAuthBcryptCheck_InvalidHash(t *testing.T) {
	assert.False(t, utils.BcryptCheck("password", "invalid-hash"))
}

func TestBuildOptionalEmailRegisterPhoneNumber(t *testing.T) {
	assert.Equal(t, "", buildOptionalEmailRegisterPhoneNumber("+86", ""))
	assert.Equal(t, "13800138000", buildOptionalEmailRegisterPhoneNumber("", " 13800138000 "))
	assert.Equal(t, "+86 13800138000", buildOptionalEmailRegisterPhoneNumber(" +86 ", " 13800138000 "))
}

// --- SafeDeref & StringPtr (also used in sys_user_auth.go context) ---

func TestSysUserAuthSafeDeref(t *testing.T) {
	assert.Equal(t, "", SafeDeref(nil))
	s := "active"
	assert.Equal(t, "active", SafeDeref(&s))
}

func TestSysUserAuthStringPtr(t *testing.T) {
	p := StringPtr("TENANT_ADMIN")
	assert.NotNil(t, p)
	assert.Equal(t, "TENANT_ADMIN", *p)
}
