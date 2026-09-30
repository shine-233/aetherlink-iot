// 文件用途：系统用户更新事务（原 sys_user_manage.go 的更新聚合）。
// 核心逻辑：先校验手机号唯一性与新密码强度，加载目标用户并过 ensureUserManagementWriteAccess，
// 再应用密码/邮箱/资料字段变更，最后用户、地址与角色绑定在同一事务落库。
// 关键注意事项：邮箱换绑保持大小写不敏感查重且放行本人原邮箱；地址更新区分「不存在则建、
// 存在则逐字段补丁」两条路径；权限判定不改写错误载荷（required_tenant / operation 等 vars）。
package service

import (
	"errors"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/errcode"

	"gorm.io/gorm"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"
)

// UpdateUser 更新用户资料、地址和角色绑定，并保持租户权限边界。
func (*User) UpdateUser(updateUserReq *model.UpdateUserReq, claims *utils.UserClaims) error {
	if err := prepareUpdateUserRequest(updateUserReq); err != nil {
		return err
	}

	user, err := loadUpdateUser(updateUserReq.ID)
	if err != nil {
		return err
	}

	if err := ensureUserManagementWriteAccess(user, claims, "update_user"); err != nil {
		return err
	}
	if err := ensureAssignableUserRoles(updateUserReq.RoleIDs, user, claims); err != nil {
		return err
	}
	if err := ensureCasbinUserRoleMutationReady(updateUserReq.RoleIDs != nil); err != nil {
		return err
	}

	now := time.Now().UTC()
	if err := applyUpdateUserChanges(user, updateUserReq, now); err != nil {
		return err
	}

	if err := updateUserWithAddressAndRoles(user, updateUserReq, claims); err != nil {
		return err
	}
	if updateUserReq.RoleIDs != nil {
		return reloadCasbinPolicyAfterRoleTransaction()
	}
	return nil
}

func prepareUpdateUserRequest(updateUserReq *model.UpdateUserReq) error {
	if err := ensureUpdateUserPhoneAvailable(updateUserReq); err != nil {
		return err
	}
	return normalizeUpdateUserPassword(updateUserReq)
}

func ensureUpdateUserPhoneAvailable(updateUserReq *model.UpdateUserReq) error {
	if updateUserReq.PhoneNumber == nil || *updateUserReq.PhoneNumber == "" {
		return nil
	}
	if exists, err := dal.CheckPhoneNumberExists(*updateUserReq.PhoneNumber, updateUserReq.ID); err != nil {
		return err
	} else if exists {
		return errcode.New(errcode.CodePhoneDuplicated)
	}
	return nil
}

func normalizeUpdateUserPassword(updateUserReq *model.UpdateUserReq) error {
	if updateUserReq.Password == nil {
		return nil
	}
	if len(*updateUserReq.Password) == 0 {
		updateUserReq.Password = nil
		return nil
	}
	return utils.ValidatePassword(*updateUserReq.Password)
}

func loadUpdateUser(userID string) (*model.User, error) {
	user, err := dal.GetUsersById(userID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": userID,
		})
	}
	return user, nil
}

func applyUpdateUserChanges(user *model.User, updateUserReq *model.UpdateUserReq, now time.Time) error {
	if err := applyUpdateUserPassword(user, updateUserReq.Password, now); err != nil {
		return err
	}

	user.UpdatedAt = &now
	if err := applyUpdateUserEmail(user, updateUserReq.Email); err != nil {
		return err
	}
	applyUpdateUserProfileFields(user, updateUserReq)
	return nil
}

func applyUpdateUserPassword(user *model.User, password *string, now time.Time) error {
	if password == nil {
		return nil
	}
	hashedPassword, hashErr := utils.BcryptHash(*password)
	if hashErr != nil {
		return errcode.WithData(errcode.CodeDecryptError, map[string]interface{}{
			"error": "Failed to hash password",
			"cause": hashErr.Error(),
		})
	}
	user.Password = hashedPassword
	user.PasswordLastUpdated = &now
	return nil
}

func applyUpdateUserEmail(user *model.User, requestedEmail *string) error {
	if requestedEmail == nil {
		return nil
	}

	email := strings.ToLower(strings.TrimSpace(*requestedEmail))
	if email == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "email is required")
	}
	if strings.EqualFold(email, user.Email) {
		return nil
	}
	if existing, err := dal.GetUsersByEmail(email); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
			"email": email,
		})
	} else if existing != nil && existing.ID != user.ID {
		return errcode.NewWithMessage(errcode.CodeParamError, "email already exists")
	}
	user.Email = email
	return nil
}

func applyUpdateUserProfileFields(user *model.User, updateUserReq *model.UpdateUserReq) {
	user.Name = updateUserReq.Name
	if updateUserReq.PhoneNumber != nil {
		user.PhoneNumber = *updateUserReq.PhoneNumber
	}
	user.AdditionalInfo = updateUserReq.AdditionalInfo
	user.Status = updateUserReq.Status
	user.Remark = updateUserReq.Remark
	user.Organization = updateUserReq.Organization
	user.Timezone = updateUserReq.Timezone
	user.DefaultLanguage = updateUserReq.DefaultLanguage
}

func updateUserWithAddressAndRoles(user *model.User, updateUserReq *model.UpdateUserReq, claims *utils.UserClaims) error {
	if err := query.Q.Transaction(func(tx *query.Query) error {
		if _, err := tx.User.Where(tx.User.ID.Eq(user.ID)).Updates(user); err != nil {
			return err
		}
		if err := updateUserAddressWithTx(tx, user.ID, updateUserReq.Address); err != nil {
			return err
		}
		if updateUserReq.RoleIDs != nil {
			return replaceUserRoleBindingsWithTx(tx, updateUserReq.ID, updateUserReq.RoleIDs)
		}
		return nil
	}); err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": claims.ID,
		})
	}
	return nil
}

func updateUserAddressWithTx(tx *query.Query, userID string, addressReq *model.UpdateUserAddressReq) error {
	if addressReq == nil {
		return nil
	}
	existingAddress, err := tx.UserAddress.Where(tx.UserAddress.UserID.Eq(userID)).First()
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return tx.UserAddress.Create(&model.UserAddress{
			UserID:          userID,
			Country:         addressReq.Country,
			Province:        addressReq.Province,
			City:            addressReq.City,
			District:        addressReq.District,
			Street:          addressReq.Street,
			DetailedAddress: addressReq.DetailedAddress,
			PostalCode:      addressReq.PostalCode,
			AddressLabel:    addressReq.AddressLabel,
			Longitude:       addressReq.Longitude,
			Latitude:        addressReq.Latitude,
			AdditionalInfo:  addressReq.AdditionalInfo,
		})
	}

	updates := map[string]interface{}{}
	if addressReq.Country != nil {
		updates["country"] = *addressReq.Country
	}
	if addressReq.Province != nil {
		updates["province"] = *addressReq.Province
	}
	if addressReq.City != nil {
		updates["city"] = *addressReq.City
	}
	if addressReq.District != nil {
		updates["district"] = *addressReq.District
	}
	if addressReq.Street != nil {
		updates["street"] = *addressReq.Street
	}
	if addressReq.DetailedAddress != nil {
		updates["detailed_address"] = *addressReq.DetailedAddress
	}
	if addressReq.PostalCode != nil {
		updates["postal_code"] = *addressReq.PostalCode
	}
	if addressReq.AddressLabel != nil {
		updates["address_label"] = *addressReq.AddressLabel
	}
	if addressReq.Longitude != nil {
		updates["longitude"] = *addressReq.Longitude
	}
	if addressReq.Latitude != nil {
		updates["latitude"] = *addressReq.Latitude
	}
	if addressReq.AdditionalInfo != nil {
		updates["additional_info"] = *addressReq.AdditionalInfo
	}
	if len(updates) == 0 {
		return nil
	}
	_, err = tx.UserAddress.Where(tx.UserAddress.ID.Eq(existingAddress.ID)).Updates(updates)
	return err
}
