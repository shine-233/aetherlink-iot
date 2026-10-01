// 文件用途：邮箱验证码的生成、发送限流与校验失败计数（注册、换绑邮箱等流程共用的基础设施）。
// 核心逻辑：按邮箱维度 SetNX 限制发送频率；验证码存于 "<email>_code"；失败计数达到上限即作废验证码。
// 关键注意事项：验证码属于敏感数据，日志只允许输出脱敏结果；Redis key 形状是线上数据契约，不可随意改名。
package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/common"
	"aetherlink-iot/backend/pkg/errcode"

	"gorm.io/gorm"

	dal "aetherlink-iot/backend/internal/dal"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
)

// 验证码防滥用参数：发送频率按邮箱维度限流，校验失败次数有上限以防 6 位数字码被暴力枚举。
const (
	verificationCodeMaxAttempts  = 5
	verificationCodeSendInterval = 60 * time.Second
	verificationCodeValidityTTL  = 5 * time.Minute
	verificationCodeLength       = 6
)

// verificationCodeKey 是验证码本体的 Redis key（"<email>_code"）。
// 发送、换绑校验和失败作废都必须走这一个函数，避免 key 形状分叉导致“作废了别的 key”。
func verificationCodeKey(email string) string {
	return email + "_code"
}

func verificationCodeSendLimitKey(email string) string {
	return "email:" + email + ":code_send_limit"
}

func verificationCodeAttemptsKey(email string) string {
	return "email:" + email + ":code_attempts"
}

func verificationCodeEmailBody(code, language string) string {
	lang := "en-US"
	if normalized, err := normalizePreferredLanguage(language); err == nil {
		lang = normalized
	}

	switch lang {
	case "zh-CN":
		return fmt.Sprintf("您的验证码是 %s", code)
	case "fr-FR":
		return fmt.Sprintf("Votre code de verification est %s", code)
	case "es-ES":
		return fmt.Sprintf("Su codigo de verificacion es %s", code)
	default:
		return fmt.Sprintf("Your verification code is %s", code)
	}
}

// maskVerificationCode 对验证码进行脱敏处理，仅保留前2位和后1位
func maskVerificationCode(code string) string {
	if len(code) <= 3 {
		return strings.Repeat("*", len(code))
	}
	return code[:2] + strings.Repeat("*", len(code)-3) + code[len(code)-1:]
}

// @description 发送验证码
func (userService *User) GetVerificationCode(email, isRegister, language string) error {
	ctx := context.Background()
	if err := acquireVerificationCodeSendSlot(ctx, email); err != nil {
		return err
	}
	if err := ensureVerificationCodeRecipientState(email, isRegister == "1"); err != nil {
		return err
	}

	verificationCode, err := issueVerificationCode(ctx, email)
	if err != nil {
		return err
	}

	if err := userService.deliverVerificationCodeEmail(ctx, email, verificationCode, language); err != nil {
		return errcode.WithData(200010, map[string]interface{}{ // 验证码邮件发送失败
			"email": email,
			"error": err.Error(),
		})
	}

	// 只有 adapter 明确确认投递后才记录“已发送”；生产 SMTP 失败不会降级成本地成功。
	// 日志不携带验证码（即便脱敏也会泄露一半位数，缩小 6 位数字码的枚举空间）。
	logrus.Info("verification email sent")
	return nil
}

// acquireVerificationCodeSendSlot 按邮箱维度限制发送频率：防止公开接口被刷发真实邮件（邮箱轰炸成本转移给部署者）。
func acquireVerificationCodeSendSlot(ctx context.Context, email string) error {
	sent, err := global.REDIS.SetNX(ctx, verificationCodeSendLimitKey(email), 1, verificationCodeSendInterval).Result()
	if err != nil {
		return errcode.WithData(errcode.CodeCacheError, map[string]interface{}{
			"operation": "check_verification_send_limit",
			"email":     email,
			"error":     err.Error(),
		})
	}
	if !sent {
		return errcode.New(errcode.CodeRateLimit)
	}
	return nil
}

// ensureVerificationCodeRecipientState 校验邮箱注册状态与发送场景一致：
// 注册场景要求邮箱未注册，其余场景（找回密码、登录等）要求邮箱已存在。
func ensureVerificationCodeRecipientState(email string, isRegister bool) error {
	user, err := dal.GetUsersByEmail(email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		logrus.Error(err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_user",
			"email":     email,
			"error":     err.Error(),
		})
	}
	return verificationCodeRecipientStateError(user != nil, isRegister)
}

// verificationCodeRecipientStateError 是 ensureVerificationCodeRecipientState 的纯判定部分（便于单测）。
func verificationCodeRecipientStateError(userExists, isRegister bool) error {
	switch {
	case !userExists && !isRegister:
		return errcode.New(errcode.CodeEmailNotFound) // 用户邮箱不存在
	case userExists && isRegister:
		return errcode.New(200008) // 用户邮箱已注册
	}
	return nil
}

// issueVerificationCode 生成并保存新验证码，同时重置该邮箱的校验失败计数，避免旧计数误伤新验证码。
func issueVerificationCode(ctx context.Context, email string) (string, error) {
	verificationCode, err := common.GenerateNumericCode(verificationCodeLength)
	if err != nil {
		return "", errcode.WithData(200009, map[string]interface{}{ // 验证码生成失败
			"email": email,
		})
	}

	if err := global.REDIS.Set(ctx, verificationCodeKey(email), verificationCode, verificationCodeValidityTTL).Err(); err != nil {
		return "", errcode.WithData(errcode.CodeCacheError, map[string]interface{}{
			"operation": "save_verification_code",
			"email":     email,
			"error":     err.Error(),
		})
	}
	global.REDIS.Del(ctx, verificationCodeAttemptsKey(email))
	return verificationCode, nil
}

// verificationCodesEqual 常量时间比较，避免按字节提前返回带来的时序侧信道。
func verificationCodesEqual(stored, provided string) bool {
	return subtle.ConstantTimeCompare([]byte(stored), []byte(provided)) == 1
}

// ensureVerificationCodeAttemptsAllowed 在比对验证码前检查失败次数；超过上限视为验证码已失效。
func ensureVerificationCodeAttemptsAllowed(ctx context.Context, email string) error {
	attempts, err := global.REDIS.Get(ctx, verificationCodeAttemptsKey(email)).Int()
	if err != nil {
		// 计数不存在或不可解析时按"无失败记录"处理，不阻断正常校验。
		return nil
	}
	if attempts >= verificationCodeMaxAttempts {
		return errcode.New(200011)
	}
	return nil
}

// registerVerificationCodeFailure 记录一次验证码校验失败；达到上限时立即作废该验证码。
func registerVerificationCodeFailure(ctx context.Context, email string) {
	attemptsKey := verificationCodeAttemptsKey(email)
	attempts := global.REDIS.Incr(ctx, attemptsKey).Val()
	if attempts == 1 {
		global.REDIS.Expire(ctx, attemptsKey, verificationCodeValidityTTL)
	}
	if attempts >= verificationCodeMaxAttempts {
		global.REDIS.Del(ctx, verificationCodeKey(email))
	}
}
