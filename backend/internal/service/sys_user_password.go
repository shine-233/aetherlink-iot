// 文件用途：维护用户密码重置、验证码尝试次数和锁定策略。
// 核心逻辑：校验验证码、限制失败次数、更新密码 hash，并清理或锁定验证码状态。
// 关键注意事项：密码重置是账户安全边界，验证码错误、过期和多次尝试必须 fail-closed。
// 重构建议：抽出验证码存储和密码策略接口，补齐并发尝试、锁定释放、审计和事务测试。
package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/errcode"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// 验证码校验最大尝试次数，超过则锁定一段时间
const maxVerifyCodeAttempts = 5
const verifyCodeLockDuration = 10 * time.Minute
const resetPasswordTokenTTL = 15 * time.Minute
const resetPasswordTokenKeyPrefix = "reset_password_token:"

type passwordResetStore interface {
	Set(ctx context.Context, key, value string, expiration time.Duration) error
	Delete(ctx context.Context, keys ...string) error
	Get(ctx context.Context, key string) (string, error)
	GetDel(ctx context.Context, key string) (string, error)
	Increment(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, expiration time.Duration) error
}

type passwordResetEmailSender func(message string, subject string, tenantID string, to ...string) error

type redisPasswordResetStore struct{}

func (redisPasswordResetStore) Set(ctx context.Context, key, value string, expiration time.Duration) error {
	if global.REDIS == nil {
		return errors.New("redis is not initialized")
	}
	return global.REDIS.Set(ctx, key, value, expiration).Err()
}

func (redisPasswordResetStore) Delete(ctx context.Context, keys ...string) error {
	if global.REDIS == nil {
		return errors.New("redis is not initialized")
	}
	return global.REDIS.Del(ctx, keys...).Err()
}

func (redisPasswordResetStore) Get(ctx context.Context, key string) (string, error) {
	if global.REDIS == nil {
		return "", errors.New("redis is not initialized")
	}
	return global.REDIS.Get(ctx, key).Result()
}

func (redisPasswordResetStore) GetDel(ctx context.Context, key string) (string, error) {
	if global.REDIS == nil {
		return "", errors.New("redis is not initialized")
	}
	return global.REDIS.GetDel(ctx, key).Result()
}

func (redisPasswordResetStore) Increment(ctx context.Context, key string) (int64, error) {
	if global.REDIS == nil {
		return 0, errors.New("redis is not initialized")
	}
	return global.REDIS.Incr(ctx, key).Result()
}

func (redisPasswordResetStore) Expire(ctx context.Context, key string, expiration time.Duration) error {
	if global.REDIS == nil {
		return errors.New("redis is not initialized")
	}
	return global.REDIS.Expire(ctx, key, expiration).Err()
}

func (user *User) resetPasswordStore() passwordResetStore {
	if user != nil && user.passwordResetStore != nil {
		return user.passwordResetStore
	}
	return redisPasswordResetStore{}
}

func (user *User) resetPasswordSender() passwordResetEmailSender {
	if user != nil && user.passwordResetEmailSender != nil {
		return user.passwordResetEmailSender
	}
	return sendEmailMessage
}

// RequestPasswordResetLink verifies the email code and sends a one-time reset link.
func (user *User) RequestPasswordResetLink(ctx context.Context, req *model.ResetPasswordLinkReq) (*model.ResetPasswordLinkRsp, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "密码重置链接请求不能为空")
	}
	store := user.resetPasswordStore()
	email := normalizePasswordResetEmail(req.Email)
	if err := verifyPasswordResetCode(ctx, store, email, req.VerifyCode); err != nil {
		return nil, err
	}

	token, err := generateResetPasswordToken()
	if err != nil {
		return nil, errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"operation": "generate_reset_password_token",
			"error":     err.Error(),
		})
	}

	if err := store.Set(ctx, resetPasswordTokenKey(token), email, resetPasswordTokenTTL); err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "save_reset_password_token",
			"email":     email,
			"error":     err.Error(),
		})
	}

	link := buildPasswordResetLink(email, token)
	body := fmt.Sprintf("Use this link to reset your password. The link expires in %d minutes.\n\n%s", int(resetPasswordTokenTTL.Minutes()), link)
	if err := user.resetPasswordSender()(body, "AetherLink password reset", "", email); err != nil {
		_ = store.Delete(ctx, resetPasswordTokenKey(token))
		return nil, errcode.WithData(200010, map[string]interface{}{
			"operation": "send_reset_password_link",
			"email":     email,
			"error":     err.Error(),
		})
	}

	_ = store.Delete(ctx, passwordResetCodeKey(email), passwordResetAttemptKey(email))
	return &model.ResetPasswordLinkRsp{ExpiresIn: int64(resetPasswordTokenTTL.Seconds())}, nil
}

// @description ResetPassword By VerifyCode and Email
func (user *User) ResetPassword(ctx context.Context, resetPasswordReq *model.ResetPasswordReq) error {
	if resetPasswordReq == nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "密码重置请求不能为空")
	}
	if err := utils.ValidatePassword(resetPasswordReq.Password); err != nil {
		return err
	}
	useResetToken, err := validatePasswordResetMethod(resetPasswordReq)
	if err != nil {
		return err
	}

	store := user.resetPasswordStore()
	email := normalizePasswordResetEmail(resetPasswordReq.Email)
	if useResetToken {
		token := strings.TrimSpace(resetPasswordReq.ResetToken)
		tokenEmail, err := readPasswordResetToken(ctx, store, token)
		if err != nil {
			return err
		}
		if !strings.EqualFold(tokenEmail, email) {
			return errcode.NewWithMessage(errcode.CodeParamError, "重置链接与邮箱不匹配")
		}
		if err := consumePasswordResetToken(ctx, store, token, tokenEmail); err != nil {
			return err
		}
		email = tokenEmail
	} else {
		if err := verifyPasswordResetCode(ctx, store, email, resetPasswordReq.VerifyCode); err != nil {
			return err
		}
	}

	var (
		db        = dal.UserQuery{}
		userQuery = query.User
	)
	info, err := db.First(ctx, userQuery.Email.Eq(email))
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_user",
			"email":     email,
			"error":     err.Error(),
		})
	}
	t := time.Now().UTC()
	info.PasswordLastUpdated = &t
	hashedPassword, hashErr := utils.BcryptHash(resetPasswordReq.Password)
	if hashErr != nil {
		logrus.Error(ctx, "[ResetPasswordByCode]hash password failed:", hashErr)
		return errcode.WithData(errcode.CodeDecryptError, map[string]interface{}{
			"operation": "update_password",
			"error":     "Failed to hash password",
		})
	}
	info.Password = hashedPassword
	if err = db.UpdateByEmail(ctx, info, userQuery.Password, userQuery.PasswordLastUpdated); err != nil {
		logrus.Error(ctx, "[ResetPasswordByCode]Update Users info failed:", err)
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "update_password",
			"email":     email,
			"error":     err.Error(),
		})
	}
	_ = store.Delete(ctx, passwordResetCodeKey(email), passwordResetAttemptKey(email))
	return nil
}

func validatePasswordResetMethod(req *model.ResetPasswordReq) (bool, error) {
	verifyCodeProvided := strings.TrimSpace(req.VerifyCode) != ""
	resetTokenProvided := strings.TrimSpace(req.ResetToken) != ""
	if verifyCodeProvided && resetTokenProvided {
		return false, errcode.NewWithMessage(errcode.CodeParamError, "验证码和重置链接不能同时使用")
	}
	if !verifyCodeProvided && !resetTokenProvided {
		return false, errcode.NewWithMessage(errcode.CodeParamError, "验证码或重置链接不能为空")
	}
	return resetTokenProvided, nil
}

func normalizePasswordResetEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func passwordResetCodeKey(email string) string {
	return normalizePasswordResetEmail(email) + "_code"
}

func passwordResetAttemptKey(email string) string {
	return fmt.Sprintf("%s_code_attempts", normalizePasswordResetEmail(email))
}

func verifyPasswordResetCode(ctx context.Context, store passwordResetStore, email, verifyCode string) error {
	email = normalizePasswordResetEmail(email)
	code := strings.TrimSpace(verifyCode)
	if email == "" || code == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "验证码不能为空")
	}

	attemptKey := passwordResetAttemptKey(email)
	attempts, err := store.Increment(ctx, attemptKey)
	if err != nil {
		logrus.Warnf("记录验证码尝试次数失败: %v", err)
	}
	if attempts == 1 {
		_ = store.Expire(ctx, attemptKey, verifyCodeLockDuration)
	}
	if attempts > maxVerifyCodeAttempts {
		return errcode.NewWithMessage(errcode.CodeParamError, "验证码尝试次数过多，请稍后再试")
	}

	verificationCode, err := store.Get(ctx, passwordResetCodeKey(email))
	if err != nil {
		return errcode.New(200011) // 验证码已过期
	}
	if subtle.ConstantTimeCompare([]byte(verificationCode), []byte(code)) != 1 {
		return errcode.New(200012) // 验证码错误
	}
	return nil
}

func generateResetPasswordToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func resetPasswordTokenKey(token string) string {
	return resetPasswordTokenKeyPrefix + strings.TrimSpace(token)
}

func readPasswordResetToken(ctx context.Context, store passwordResetStore, token string) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "重置链接无效")
	}
	email, err := store.Get(ctx, resetPasswordTokenKey(token))
	if err != nil {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "重置链接已过期或无效")
	}
	return normalizePasswordResetEmail(email), nil
}

func consumePasswordResetToken(ctx context.Context, store passwordResetStore, token, expectedEmail string) error {
	token = strings.TrimSpace(token)
	expectedEmail = normalizePasswordResetEmail(expectedEmail)
	if token == "" || expectedEmail == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "重置链接无效")
	}

	email, err := store.GetDel(ctx, resetPasswordTokenKey(token))
	if err != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "重置链接已过期或无效")
	}
	if !strings.EqualFold(normalizePasswordResetEmail(email), expectedEmail) {
		return errcode.NewWithMessage(errcode.CodeParamError, "重置链接与邮箱不匹配")
	}
	return nil
}

func buildPasswordResetLink(email, token string) string {
	values := url.Values{}
	values.Set("email", normalizePasswordResetEmail(email))
	values.Set("reset_token", token)

	path := "/login/reset-pwd?" + values.Encode()
	baseURL := strings.TrimRight(strings.TrimSpace(viper.GetString("deployment.public_url")), "/")
	if baseURL == "" {
		return path
	}
	return baseURL + path
}
