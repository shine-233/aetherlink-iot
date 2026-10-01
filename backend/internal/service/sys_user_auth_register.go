// 文件用途：维护邮箱自助注册流程（校验、密码策略、建号、默认资源初始化）。
// 核心逻辑：手机号可用性校验、验证码校验（含失败次数限流）、密码确认与复杂度校验、
// 邮箱唯一性校验、密码加密落库，并在事务内创建用户与默认看板。
// 关键注意事项：新建租户 ID 失败、创建用户/默认看板失败均需 fail-closed 并回滚事务；
// 注册成功后复用登录态签发逻辑（UserLoginAfter）保持与 Login 一致的 token 行为。
// 拆分记录：原 sys_user_auth.go（619 行）按关注点拆分（2026-10-01），本文件承载
// EmailRegister 自助注册，与登录态/身份切换解耦。
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

	"aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/logic"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

// EmailRegister 邮箱注册
func (u *User) EmailRegister(ctx context.Context, req *model.EmailRegisterReq) (*model.LoginRsp, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "email register request is required")
	}

	// 手机号是兼容字段；RDI 手册注册流程只要求邮箱、密码和验证码。
	phoneNumber := buildOptionalEmailRegisterPhoneNumber(req.PhonePrefix, req.PhoneNumber)
	if phoneNumber != "" {
		if err := ensureEmailRegisterPhoneAvailable(phoneNumber); err != nil {
			return nil, err
		}
	}

	// 验证码校验
	if err := verifyEmailRegisterCode(req.Email, req.VerifyCode); err != nil {
		return nil, err
	}

	// 密码一致性校验
	if err := validateEmailRegisterPasswordConfirmation(req); err != nil {
		return nil, err
	}

	// 验证邮箱是否已注册
	if err := ensureEmailRegisterEmailAvailable(req.Email); err != nil {
		return nil, err
	}

	// 密码加密处理
	hashedPassword, err := buildEmailRegisterPassword(ctx, req.Password, req.Salt)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	tenantID, err := common.GenerateRandomString(8)
	if err != nil {
		logrus.Error("生成租户ID失败", err)
		return nil, errcode.New(errcode.CodeSystemError)
	}

	// 构建用户信息
	userInfo := newEmailRegisterUser(req.Email, phoneNumber, hashedPassword, tenantID, now)

	// 创建用户
	if err := createEmailRegisterUser(ctx, userInfo, tenantID); err != nil {
		return nil, err
	}

	return u.UserLoginAfter(userInfo)
}

func ensureEmailRegisterPhoneAvailable(phoneNumber string) error {
	exists, err := dal.CheckPhoneNumberExists(phoneNumber)
	if err != nil {
		return err
	}
	if exists {
		return errcode.New(errcode.CodePhoneDuplicated)
	}
	return nil
}

func buildOptionalEmailRegisterPhoneNumber(phonePrefix, phoneNumber string) string {
	phoneNumber = strings.TrimSpace(phoneNumber)
	phonePrefix = strings.TrimSpace(phonePrefix)
	if phoneNumber == "" {
		return ""
	}
	if phonePrefix == "" {
		return phoneNumber
	}
	return fmt.Sprintf("%s %s", phonePrefix, phoneNumber)
}

func verifyEmailRegisterCode(email, verifyCode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), authRedisOpTimeout)
	defer cancel()
	// 失败次数达到上限的验证码立即作废，防止 6 位数字码在有效期内被暴力枚举。
	if err := ensureVerificationCodeAttemptsAllowed(ctx, email); err != nil {
		return err
	}
	verificationCode, err := global.REDIS.Get(ctx, email+"_code").Result()
	if err != nil {
		return errcode.New(200011)
	}
	if subtle.ConstantTimeCompare([]byte(verificationCode), []byte(verifyCode)) != 1 {
		registerVerificationCodeFailure(ctx, email)
		return errcode.New(200012)
	}
	return nil
}

func validateEmailRegisterPasswordConfirmation(req *model.EmailRegisterReq) error {
	if req.ConfirmPassword != nil && *req.ConfirmPassword != req.Password {
		return errcode.New(200041)
	}
	return nil
}

func ensureEmailRegisterEmailAvailable(email string) error {
	user, err := dal.GetUsersByEmail(email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_user",
			"email":     email,
			"error":     err.Error(),
		})
	}
	if user != nil {
		return errcode.New(200008)
	}
	return nil
}

func buildEmailRegisterPassword(ctx context.Context, password string, salt *string) (string, error) {
	if logic.UserIsEncrypt(ctx) {
		if salt == nil {
			return "", errcode.New(200042)
		}
		decryptedPassword, err := initialize.DecryptPassword(password)
		if err != nil {
			return "", errcode.New(200043)
		}
		password = strings.TrimSuffix(string(decryptedPassword), *salt)
	}
	if err := utils.ValidatePassword(password); err != nil {
		return "", err
	}
	hashed, hashErr := utils.BcryptHash(password)
	if hashErr != nil {
		logrus.Error("hash register password failed:", hashErr)
		return "", errcode.NewWithMessage(errcode.CodeDecryptError, "failed to hash password")
	}
	return hashed, nil
}

func newEmailRegisterUser(email, phoneNumber, hashedPassword, tenantID string, now time.Time) *model.User {
	return &model.User{
		ID:                  uuid.New(),
		Name:                &email,
		PhoneNumber:         phoneNumber,
		Email:               email,
		Status:              StringPtr("N"),
		Authority:           StringPtr("TENANT_ADMIN"),
		Password:            hashedPassword,
		TenantID:            StringPtr(tenantID),
		Remark:              StringPtr(now.Add(365 * 24 * time.Hour).String()),
		CreatedAt:           &now,
		UpdatedAt:           &now,
		PasswordLastUpdated: &now,
	}
}

func createEmailRegisterUser(ctx context.Context, userInfo *model.User, tenantID string) error {
	return query.Q.Transaction(func(tx *query.Query) error {
		if err := tx.User.WithContext(ctx).Create(userInfo); err != nil {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"operation": "create_user",
				"email":     userInfo.Email,
				"error":     err.Error(),
			})
		}

		if err := tx.Board.WithContext(ctx).Create(dal.NewDefaultBoard(&tenantID)); err != nil {
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"operation": "create_default_board",
				"tenant_id": tenantID,
				"error":     err.Error(),
			})
		}
		return nil
	})
}
