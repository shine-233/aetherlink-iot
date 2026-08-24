// 文件用途：维护系统用户登录认证、密码策略和 token 签发流程。
// 核心逻辑：校验密码复杂度、执行 bcrypt 校验，并串联登录态与用户信息返回。
// 关键注意事项：认证失败、锁定和 token 生成错误必须 fail-closed，日志不得暴露密码或 hash。
// 重构建议：抽出密码策略与 token 存储接口，补齐锁定、验证码、审计和异常 hash 测试。
// sys_user_auth.go owns system user authentication behavior.
//
// It validates credentials, tokens, password state, and user auth boundaries.
// Changes here affect login, API automation accounts, tenant access, and
// security review scope.
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

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/logic"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/constant"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// @description  用户登录
func (u *User) Login(ctx context.Context, loginReq *model.LoginReq) (*model.LoginRsp, error) {
	// 通过邮箱获取用户信息
	user, err := dal.GetUsersByEmail(loginReq.Email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 用户不存在,返回用户模块的业务错误
			return nil, errcode.New(errcode.CodeInvalidAuth)
		}
		// 数据库操作失败,返回系统级数据库错误
		return nil, errcode.New(errcode.CodeDBError)
	}
	// 是否加密配置
	if logic.UserIsEncrypt(ctx) {
		password, err := initialize.DecryptPassword(loginReq.Password)
		if err != nil {
			return nil, errcode.New(errcode.CodeDecryptError)
		}
		passwords := strings.TrimSuffix(string(password), loginReq.Salt)
		loginReq.Password = passwords
	}
	// 对比密码
	if !utils.BcryptCheck(loginReq.Password, user.Password) {
		return nil, errcode.New(errcode.CodeInvalidAuth)
	}

	// 判断用户状态
	if *user.Status != "N" {
		return nil, errcode.New(errcode.CodeUserDisabled)
	}

	logrsp, err := u.UserLoginAfter(user)
	if err != nil {
		return nil, err
	}

	// 更新登录时间
	err = dal.UserQuery{}.UpdateLastVisitTime(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return logrsp, nil
}

// UserLoginAfter
// @description 用户登录后token获取保存
func (*User) UserLoginAfter(user *model.User) (*model.LoginRsp, error) {
	if user == nil {
		return nil, errcode.WithData(errcode.CodeTokenGenerateError, map[string]interface{}{
			"error": "user is nil",
		})
	}
	key := strings.TrimSpace(viper.GetString("jwt.key"))
	if key == "" {
		return nil, errcode.WithData(errcode.CodeTokenGenerateError, map[string]interface{}{
			"error": "jwt.key is empty",
			"email": user.Email,
		})
	}
	// 生成token
	jwt := utils.NewJWT([]byte(key))
	claims, err := buildUserLoginClaims(user, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	token, err := jwt.GenerateToken(claims)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeTokenGenerateError, map[string]interface{}{
			"error": err.Error(),
			"email": user.Email,
		})
	}
	timeout := loginSessionTimeoutMinutes()
	if err := saveUserLoginToken(token, user.Email, timeout); err != nil {
		return nil, err
	}

	return newLoginResponse(token, timeout), nil
}

func buildUserLoginClaims(user *model.User, now time.Time) (utils.UserClaims, error) {
	if user.Authority == nil || user.TenantID == nil {
		return utils.UserClaims{}, errcode.WithData(errcode.CodeTokenGenerateError, map[string]interface{}{
			"error": "user authority or tenant_id is nil",
			"email": user.Email,
		})
	}
	return utils.UserClaims{
		ID:         user.ID,
		Email:      user.Email,
		Authority:  *user.Authority,
		CreateTime: now,
		TenantID:   *user.TenantID,
	}, nil
}

func loginSessionTimeoutMinutes() int {
	timeout := viper.GetInt("session.timeout")
	if viper.GetBool("session.reset_on_request") && timeout == 0 {
		return 60
	}
	return timeout
}

func saveUserLoginToken(token, email string, timeout int) error {
	if global.REDIS == nil {
		return tokenSaveError(email, errors.New("redis client is not initialized"))
	}
	ctx := context.Background()
	ttl := time.Duration(timeout) * time.Minute
	if logic.UserIsShare(ctx) {
		return setLoginToken(ctx, token, email, ttl)
	}
	return replaceExclusiveLoginToken(ctx, token, email, ttl)
}

func replaceExclusiveLoginToken(ctx context.Context, token, email string, ttl time.Duration) error {
	oldToken, err := loadPreviousLoginToken(ctx, email)
	if err != nil {
		return err
	}
	if oldToken != "" {
		if err := deleteLoginToken(ctx, oldToken, email); err != nil {
			return err
		}
	}
	if err := setLoginToken(ctx, token, email, ttl); err != nil {
		return err
	}
	if err := global.REDIS.Set(ctx, loginEmailTokenKey(email), token, 0).Err(); err != nil {
		_ = global.REDIS.Del(ctx, utils.TokenDigest(token)).Err()
		return tokenSaveError(email, err)
	}
	return nil
}

func loadPreviousLoginToken(ctx context.Context, email string) (string, error) {
	oldToken, err := global.REDIS.Get(ctx, loginEmailTokenKey(email)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return "", tokenSaveError(email, err)
	}
	return oldToken, nil
}

func setLoginToken(ctx context.Context, token, email string, ttl time.Duration) error {
	// P3 修复（2026-08-24，见 VALIDATION.md）：Redis 键统一使用 token 摘要（utils.TokenDigest），
	// 与 middleware/jwt_auth.go、api/telemetry_ws_auth.go 共用同一键空间。
	if err := global.REDIS.Set(ctx, utils.TokenDigest(token), "1", ttl).Err(); err != nil {
		return tokenSaveError(email, err)
	}
	return nil
}

func deleteLoginToken(ctx context.Context, token, email string) error {
	if err := global.REDIS.Del(ctx, utils.TokenDigest(token)).Err(); err != nil && !errors.Is(err, redis.Nil) {
		return tokenSaveError(email, err)
	}
	return nil
}

func loginEmailTokenKey(email string) string {
	return email + "_token"
}

func tokenSaveError(email string, err error) error {
	return errcode.WithData(errcode.CodeTokenSaveError, map[string]interface{}{
		"error": err.Error(),
		"email": email,
	})
}

// @description 退出登录
func (*User) Logout(token string) error {
	if err := global.REDIS.Del(context.Background(), utils.TokenDigest(token)).Err(); err != nil {
		return errcode.New(errcode.CodeTokenDeleteError)
	}
	return nil
}

// @description 刷新token
func (*User) RefreshToken(userClaims *utils.UserClaims) (*model.LoginRsp, error) {
	user, err := loadRefreshTokenUser(userClaims)
	if err != nil {
		return nil, err
	}
	if err := ensureUserCanRefreshToken(user); err != nil {
		return nil, err
	}

	key := strings.TrimSpace(viper.GetString("jwt.key"))
	if key == "" {
		return nil, errcode.WithData(errcode.CodeTokenGenerateError, map[string]interface{}{
			"error": "jwt.key is empty",
			"email": user.Email,
		})
	}

	jwt := utils.NewJWT([]byte(key))
	claims, err := buildUserLoginClaims(user, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	token, err := jwt.GenerateToken(claims)
	if err != nil {
		return nil, errcode.New(errcode.CodeTokenGenerateError)
	}

	timeout := refreshSessionTimeoutMinutes()
	if err := saveRefreshToken(token, user.Email, timeout); err != nil {
		return nil, err
	}

	return newLoginResponse(token, timeout), nil
}

func loadRefreshTokenUser(userClaims *utils.UserClaims) (*model.User, error) {
	if userClaims == nil || strings.TrimSpace(userClaims.Email) == "" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}

	user, err := dal.GetUsersByEmail(userClaims.Email)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_user",
			"email":     userClaims.Email,
			"error":     err.Error(),
		})
	}
	return user, nil
}

func ensureUserCanRefreshToken(user *model.User) error {
	if user == nil {
		return errcode.New(errcode.CodeInvalidAuth)
	}
	if user.Status == nil || *user.Status != "N" {
		return errcode.New(errcode.CodeUserDisabled)
	}
	return nil
}

func refreshSessionTimeoutMinutes() int {
	timeout := loginSessionTimeoutMinutes()
	if timeout <= 0 {
		return 24 * 7 * 60
	}
	return timeout
}

func saveRefreshToken(token, email string, timeout int) error {
	if global.REDIS == nil {
		return errcode.WithData(errcode.CodeTokenSaveError, map[string]interface{}{
			"error": "redis client is not initialized",
			"email": email,
		})
	}
	if err := global.REDIS.Set(context.Background(), utils.TokenDigest(token), "1", time.Duration(timeout)*time.Minute).Err(); err != nil {
		return errcode.WithData(errcode.CodeTokenSaveError, map[string]interface{}{
			"error": err.Error(),
			"email": email,
		})
	}
	return nil
}

func newLoginResponse(token string, timeoutMinutes int) *model.LoginRsp {
	return &model.LoginRsp{
		Token:     &token,
		ExpiresIn: int64(timeoutMinutes * 60),
	}
}

// @description SuperAdmin Become Other admin
func (*User) TransformUser(transformUserReq *model.TransformUserReq, claims *utils.UserClaims) (*model.LoginRsp, error) {
	if transformUserReq == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "transform user request is required")
	}

	// 权限检查
	if claims == nil || (claims.Authority != constant.SYS_ADMIN && claims.Authority != constant.TENANT_ADMIN) {
		return nil, errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
			"required_authority": "SYS_ADMIN or TENANT_ADMIN",
			"current_authority":  userClaimsAuthority(claims),
		})
	}

	// 获取目标用户信息
	becomeUser, err := dal.GetUsersById(transformUserReq.BecomeUserID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": transformUserReq.BecomeUserID,
		})
	}

	// 检查用户状态
	if becomeUser.Status == nil || *becomeUser.Status != "N" {
		currentStatus := ""
		if becomeUser.Status != nil {
			currentStatus = *becomeUser.Status
		}
		return nil, errcode.WithVars(errcode.CodeUserDisabled, map[string]interface{}{
			"user_id":         becomeUser.ID,
			"current_status":  currentStatus,
			"required_status": "N",
		})
	}
	if err := ensureUserTransformAccess(becomeUser, claims); err != nil {
		return nil, err
	}

	// 获取JWT密钥
	key := strings.TrimSpace(viper.GetString("jwt.key"))
	if key == "" {
		return nil, errcode.New(errcode.CodeSystemError)
	}

	// 生成用户Claims
	becomeUserClaims, err := buildUserLoginClaims(becomeUser, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	// 生成token
	jwt := utils.NewJWT([]byte(key))
	token, err := jwt.GenerateToken(becomeUserClaims)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeTokenGenerateError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": becomeUser.ID,
		})
	}

	if err := saveTransformUserToken(token, becomeUser.ID, transformUserTokenTTL()); err != nil {
		return nil, err
	}

	return newDurationLoginResponse(token, transformUserTokenTTL()), nil
}

func transformUserTokenTTL() time.Duration {
	return 24 * 7 * time.Hour
}

func saveTransformUserToken(token, userID string, ttl time.Duration) error {
	if global.REDIS == nil {
		return errcode.WithData(errcode.CodeTokenSaveError, map[string]interface{}{
			"error":   "redis client is not initialized",
			"user_id": userID,
		})
	}
	if err := global.REDIS.Set(context.Background(), utils.TokenDigest(token), "1", ttl).Err(); err != nil {
		return errcode.WithData(errcode.CodeTokenSaveError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": userID,
		})
	}
	return nil
}

func newDurationLoginResponse(token string, ttl time.Duration) *model.LoginRsp {
	return &model.LoginRsp{
		Token:     &token,
		ExpiresIn: int64(ttl.Seconds()),
	}
}

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
	ctx := context.Background()
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
	return utils.BcryptHash(password), nil
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
