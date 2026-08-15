// 文件用途：维护用户资料、偏好语言和地址更新服务。
// 核心逻辑：读取当前用户资料，更新昵称、联系方式、语言偏好和地址信息。
// 关键注意事项：资料更新需保持用户本人/管理员边界，语言和地址默认值不能破坏前端契约。
// 重构建议：拆分资料仓储、偏好校验和地址事务，补齐权限、部分更新和回滚测试。
package service

import (
	"context"
	"strings"

	"aetherlink-iot/backend/pkg/errcode"

	"aetherlink-iot/backend/initialize"
	dal "aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/logic"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"
)

// 获取用户信息
func (*User) GetUser(id string, claims *utils.UserClaims) (interface{}, error) {
	// 获取用户和地址信息
	userWithAddress, err := dal.GetUserByIdWithAddress(id)
	if err != nil {
		// 数据库错误处理
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": id,
		})
	}

	// 权限检查
	if claims.Authority == "TENANT_ADMIN" || claims.Authority == "TENANT_USER" {
		if tenantID, ok := userWithAddress["tenant_id"]; ok && tenantID != nil {
			if tenantIDStr, ok := tenantID.(*string); ok && tenantIDStr != nil && *tenantIDStr != claims.TenantID {
				return nil, errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
					"required_tenant": *tenantIDStr,
					"current_tenant":  claims.TenantID,
					"user_authority":  claims.Authority,
				})
			}
		}
	}

	return userWithAddress, nil
}

// 获取用户详细信息
func (*User) GetUserDetail(claims *utils.UserClaims) (interface{}, error) {
	userWithAddress, err := dal.GetUserByIdWithAddress(claims.ID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": claims.ID,
		})
	}
	return userWithAddress, nil
}

// @description 修改用户信息（只能修改自己）
func (*User) UpdateUserInfo(ctx context.Context, updateUserReq *model.UpdateUserInfoReq, claims *utils.UserClaims) error {
	user, err := dal.GetUsersById(claims.ID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": claims.ID,
		})
	}

	// 限制用户只能修改自己的信息
	if user.ID != claims.ID {
		return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
			"reason":  "cannot_update_other_user_info",
			"user_id": claims.ID,
		})
	}

	// 是否加密配置
	if logic.UserIsEncrypt(ctx) && updateUserReq.Password != nil {
		password, err := initialize.DecryptPassword(*updateUserReq.Password)
		if err != nil {
			return errcode.WithData(errcode.CodeDecryptError, map[string]interface{}{
				"error": err.Error(),
			})
		}
		passwords := strings.TrimSuffix(string(password), updateUserReq.Salt)
		*updateUserReq.Password = passwords
	}

	// 处理修改密码的情况
	if updateUserReq.Password != nil {
		if len(*updateUserReq.Password) == 0 {
			updateUserReq.Password = nil
		} else if err := utils.ValidatePassword(*updateUserReq.Password); err != nil {
			return err
		}
	}
	if updateUserReq.Password != nil {
		updateUserReq.Password = StringPtr(utils.BcryptHash(*updateUserReq.Password))
	}

	r, err := dal.UpdateUserInfoByIdPersonal(user.ID, updateUserReq)
	if r == 0 {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": claims.ID,
		})
	}
	return err
}

func (*User) UpdatePreferredLanguage(ctx context.Context, req *model.PreferLanguageReq, claims *utils.UserClaims) (map[string]string, error) {
	if claims == nil || strings.TrimSpace(claims.ID) == "" {
		return nil, errcode.New(errcode.CodeNoPermission)
	}
	if req == nil {
		req = &model.PreferLanguageReq{}
	}

	lang, err := normalizePreferredLanguage(req.PreferLang)
	if err != nil && strings.TrimSpace(req.DefaultLanguage) != "" {
		lang, err = normalizePreferredLanguage(req.DefaultLanguage)
	}
	if err != nil {
		return nil, err
	}

	_, err = query.User.WithContext(ctx).
		Where(query.User.ID.Eq(claims.ID)).
		Update(query.User.DefaultLanguage, lang)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "update_preferred_language",
			"user_id":   claims.ID,
			"error":     err.Error(),
		})
	}

	return map[string]string{"prefer_lang": lang, "default_language": lang}, nil
}

func normalizePreferredLanguage(value string) (string, error) {
	lang := strings.TrimSpace(value)
	if lang == "" {
		return "", errcode.NewWithMessage(errcode.CodeParamError, "prefer_lang is required")
	}

	switch strings.ToLower(strings.ReplaceAll(lang, "_", "-")) {
	case "zh", "zh-cn":
		return "zh-CN", nil
	case "en", "en-us":
		return "en-US", nil
	case "fr", "fr-fr":
		return "fr-FR", nil
	case "es", "es-es":
		return "es-ES", nil
	default:
		return "", errcode.NewWithMessage(errcode.CodeParamError, "unsupported prefer_lang")
	}
}

// @description 更新用户地址信息
func (u *User) UpdateUserAddress(userID string, updateAddressReq *model.UpdateUserAddressReq, claims *utils.UserClaims) error {
	// 首先验证用户是否存在
	user, err := dal.GetUsersById(userID)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": userID,
		})
	}

	// 权限检查：租户管理员和租户用户不能修改其他租户的用户地址
	if claims.Authority == "TENANT_ADMIN" || claims.Authority == "TENANT_USER" {
		if *user.TenantID != claims.TenantID {
			return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
				"required_tenant": *user.TenantID,
				"current_tenant":  claims.TenantID,
				"operation":       "update_user_address",
			})
		}
	}

	// 更新地址信息
	err = dal.UpdateUserAddressOnly(userID, updateAddressReq)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":     err.Error(),
			"user_id":   userID,
			"operation": "update_user_address",
		})
	}

	return nil
}
