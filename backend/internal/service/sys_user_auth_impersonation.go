// 文件用途：维护超级管理员"切换为其他用户"（impersonation）流程。
// 核心逻辑：权限校验（SYS_ADMIN/TENANT_ADMIN）、目标用户状态与越权校验、签发限时 token。
// 关键注意事项：目标账号必须处于启用状态，权限不足或越权必须 fail-closed；
// 该 token TTL 独立于普通登录会话（固定 7 天），不走 loginSessionTimeoutMinutes。
// 拆分记录：原 sys_user_auth.go（619 行）按关注点拆分（2026-10-01），本文件承载
// TransformUser 身份切换，与登录态/注册流程解耦。
package service

import (
	"context"
	"time"

	"aetherlink-iot/backend/pkg/errcode"

	"aetherlink-iot/backend/internal/authz"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/authkeys"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"
)

// transformUserTokenTTL 切换身份后发放的 token 有效期，固定值，独立于常规登录会话超时。
func transformUserTokenTTL() time.Duration {
	return 24 * 7 * time.Hour
}

// @description SuperAdmin Become Other admin
func (*User) TransformUser(transformUserReq *model.TransformUserReq, claims *utils.UserClaims) (*model.LoginRsp, error) {
	if transformUserReq == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "transform user request is required")
	}

	// 权限检查
	if err := (authz.Rule{Roles: authz.ManagerRoles}).RequireClaims(claims); err != nil {
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

	// 获取JWT签发器（authkeys 统一规范化密钥，与校验侧同源）
	jwt, err := authkeys.JWT()
	if err != nil {
		return nil, errcode.New(errcode.CodeSystemError)
	}

	// 生成用户Claims
	becomeUserClaims, err := buildUserLoginClaims(becomeUser, time.Now().UTC())
	if err != nil {
		return nil, err
	}

	// 生成token
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

func saveTransformUserToken(token, userID string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), authRedisOpTimeout)
	defer cancel()
	if global.REDIS == nil {
		return errcode.WithData(errcode.CodeTokenSaveError, map[string]interface{}{
			"error":   "redis client is not initialized",
			"user_id": userID,
		})
	}
	if err := global.REDIS.Set(ctx, utils.TokenDigest(token), "1", ttl).Err(); err != nil {
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
