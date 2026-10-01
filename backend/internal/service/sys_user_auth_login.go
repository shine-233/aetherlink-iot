// 文件用途：维护系统用户登录、登出与 token 刷新/存储流程。
// 核心逻辑：校验密码复杂度、执行 bcrypt 校验，签发 JWT 并维护 Redis 会话态（独占/共享会话）。
// 关键注意事项：认证失败、锁定和 token 生成错误必须 fail-closed，日志不得暴露密码或 hash；
// Redis 键统一使用 token 摘要（utils.TokenDigest），与 middleware/jwt_auth.go、
// api/telemetry_ws_auth.go 共用同一键空间。
// 拆分记录：原 sys_user_auth.go（619 行）按关注点拆分为 login / impersonation / register
// 三个文件（2026-10-01），本文件承载登录态生命周期（登录、登出、刷新、token 存储）。
package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/errcode"

	"github.com/redis/go-redis/v9"

	"aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/logic"
	model "aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"gorm.io/gorm"
)

// authRedisOpTimeout 约束认证链路 Redis 会话写删操作时长。
// 这些历史函数未收 ctx，用短超时替代裸 Background，避免无界阻塞登录/登出。
const authRedisOpTimeout = 3 * time.Second

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

	// 2FA（ROADMAP C7）：已启用 TOTP 的用户在密码正确后进入第二因子阶段。
	if totpRow, err := dal.GetUserTOTP(user.ID); err == nil && totpRow.Enabled {
		ticket, terr := GroupApp.UserTotp.IssueChallenge(user.ID)
		if terr != nil {
			return nil, terr
		}
		return nil, errcode.WithData(errcode.CodeTotpRequired, map[string]interface{}{
			"ticket": ticket,
		})
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
	ctx, cancel := context.WithTimeout(context.Background(), authRedisOpTimeout)
	defer cancel()
	ttl := time.Duration(timeout) * time.Minute
	if logic.UserIsShare(ctx) {
		return setLoginToken(ctx, token, email, ttl)
	}
	return replaceExclusiveLoginToken(ctx, token, email, ttl)
}

// replaceExclusiveLoginToken 实现单账号独占会话语义：
// 新 token 白名单写入成功后吊销旧会话，并把 <email>_token 键维护为"当前会话摘要"。
// P3 加固（2026-08-25）：该键不再落地明文 JWT（改存 utils.TokenDigest 摘要），且 TTL 与会话超时对齐；
// 修复了明文令牌长期驻留 Redis（TTL=0）以及旧 access token 在刷新后仍存活至 TTL 的问题（安全审计 F1/F2）。
func replaceExclusiveLoginToken(ctx context.Context, token, email string, ttl time.Duration) error {
	oldDigest, err := loadPreviousLoginTokenDigest(ctx, email)
	if err != nil {
		return err
	}
	if oldDigest != "" {
		// 键值即摘要：直接删除旧摘要键即可吊销上一会话。
		if err := global.REDIS.Del(ctx, oldDigest).Err(); err != nil && !errors.Is(err, redis.Nil) {
			return tokenSaveError(email, err)
		}
	}
	if err := setLoginToken(ctx, token, email, ttl); err != nil {
		return err
	}
	if err := global.REDIS.Set(ctx, loginEmailTokenKey(email), utils.TokenDigest(token), ttl).Err(); err != nil {
		_ = global.REDIS.Del(ctx, utils.TokenDigest(token)).Err()
		return tokenSaveError(email, err)
	}
	return nil
}

// loadPreviousLoginTokenDigest 返回该账号上一会话的 token 摘要（键值即摘要，非明文 JWT）。
func loadPreviousLoginTokenDigest(ctx context.Context, email string) (string, error) {
	oldDigest, err := global.REDIS.Get(ctx, loginEmailTokenKey(email)).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return "", tokenSaveError(email, err)
	}
	return oldDigest, nil
}

func setLoginToken(ctx context.Context, token, email string, ttl time.Duration) error {
	// P3 修复（2026-08-24，见 VALIDATION.md）：Redis 键统一使用 token 摘要（utils.TokenDigest），
	// 与 middleware/jwt_auth.go、api/telemetry_ws_auth.go 共用同一键空间。
	if err := global.REDIS.Set(ctx, utils.TokenDigest(token), "1", ttl).Err(); err != nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), authRedisOpTimeout)
	defer cancel()
	if err := global.REDIS.Del(ctx, utils.TokenDigest(token)).Err(); err != nil {
		return errcode.New(errcode.CodeTokenDeleteError)
	}
	return nil
}

// @description 刷新token
// previousTokenDigest 是本次请求所持旧 token 的摘要（api 层经 middleware.SelectJWTAuthToken 计算）。
// 新 token 写入成功后立即吊销旧摘要，修复刷新后旧 access token 仍可用至 TTL 的问题（安全审计 F1）。
func (*User) RefreshToken(userClaims *utils.UserClaims, previousTokenDigest string) (*model.LoginRsp, error) {
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
	revokePreviousTokenDigest(previousTokenDigest, user.Email)

	return newLoginResponse(token, timeout), nil
}

// revokePreviousTokenDigest 尽力吊销旧会话摘要；失败仅告警不阻断刷新链路——
// 吊销失败时退回旧行为（旧 token 存活至 TTL），避免把用户完全锁死在认证续期上。
func revokePreviousTokenDigest(digest, email string) {
	if digest == "" || global.REDIS == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), authRedisOpTimeout)
	defer cancel()
	if err := global.REDIS.Del(ctx, digest).Err(); err != nil && !errors.Is(err, redis.Nil) {
		logrus.Warnf("revoke previous session digest failed: email=%s err=%v", email, err)
	}
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
	ctx, cancel := context.WithTimeout(context.Background(), authRedisOpTimeout)
	defer cancel()
	if global.REDIS == nil {
		return errcode.WithData(errcode.CodeTokenSaveError, map[string]interface{}{
			"error": "redis client is not initialized",
			"email": email,
		})
	}
	if err := global.REDIS.Set(ctx, utils.TokenDigest(token), "1", time.Duration(timeout)*time.Minute).Err(); err != nil {
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
