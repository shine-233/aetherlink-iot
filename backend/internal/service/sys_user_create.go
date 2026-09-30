// 文件用途：系统用户创建事务（原 sys_user_manage.go 的创建聚合）。
// 核心逻辑：先校验联系方式唯一性，再在业务层生成用户 ID、归属租户与 authority，
// 密码入库前哈希，最后用户、地址、默认看板与角色绑定在同一事务落库并重载 casbin 策略。
// 关键注意事项：TENANT_ADMIN 创建的用户固定为 TENANT_USER 并继承其租户；创建请求未显式
// 给 RoleIDs 时按 users.authority 兜底绑定（与 63.sql 种子口径一致），否则 RBAC 生效后
// 新用户被全量 403；casbin LoadPolicy 仅对带 DB adapter 的 enforcer 执行。
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"

	"aetherlink-iot/backend/internal/authz"
	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

// CreateUser 创建系统用户，并同步初始化其租户、角色、地址和默认看板。
func (u *User) CreateUser(createUserReq *model.CreateUserReq, claims *utils.UserClaims) error {
	if claims == nil {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to create user")
	}

	// 先校验联系方式唯一性，避免事务内创建到一半才发现邮箱或手机号冲突。
	email, err := ensureCreateUserContactAvailable(createUserReq)
	if err != nil {
		return err
	}

	user := model.User{}
	// 用户 ID 在业务层生成，方便后续地址、角色和看板写入复用同一标识。
	user.ID = uuid.New()
	user.Name = createUserReq.Name
	user.PhoneNumber = createUserReq.PhoneNumber
	user.Email = email
	user.Status = StringPtr("N")
	user.Remark = createUserReq.Remark

	// 保留组织、时区和语言偏好，供前端用户资料页和通知模板使用。
	user.Organization = createUserReq.Organization
	user.Timezone = createUserReq.Timezone
	user.DefaultLanguage = createUserReq.DefaultLanguage

	// 额外信息采用 JSON 结构存储，写入前统一规范化，降低空字段和格式漂移。
	if err := setCreateUserAdditionalInfo(&user, createUserReq); err != nil {
		return err
	}
	// 创建人权限决定新用户所在租户和可见范围，必须早于角色绑定计算。
	if err := u.assignCreateUserAuthority(&user, claims); err != nil {
		return err
	}
	t := time.Now().UTC()
	user.CreatedAt = &t
	user.UpdatedAt = &t
	user.PasswordLastUpdated = &t

	// 密码在入库前完成哈希处理，避免明文密码进入模型后被日志或调试输出泄漏。
	if err := setCreateUserPassword(&user, createUserReq.Password); err != nil {
		return err
	}

	if err := ensureAssignableUserRoles(createUserReq.RoleIDs, &user, claims); err != nil {
		return err
	}
	if err := ensureCasbinRoleMutationReady(createUserReq.RoleIDs); err != nil {
		return err
	}

	// 用户、地址、默认看板和角色绑定需要同事务落库，防止出现半初始化账号。
	if err := createUserWithAddressDefaultBoardAndRoles(&user, createUserReq, claims); err != nil {
		return err
	}

	if len(createUserReq.RoleIDs) > 0 {
		return reloadCasbinPolicyAfterRoleTransaction()
	}
	return nil
}

func ensureCreateUserContactAvailable(createUserReq *model.CreateUserReq) (string, error) {
	if exists, err := dal.CheckPhoneNumberExists(createUserReq.PhoneNumber); err != nil {
		return "", err
	} else if exists {
		return "", errcode.New(errcode.CodePhoneDuplicated)
	}

	email := strings.ToLower(strings.TrimSpace(createUserReq.Email))
	if err := ensureNewUserEmailAvailable(email); err != nil {
		return "", err
	}
	return email, nil
}

func ensureNewUserEmailAvailable(email string) error {
	if email == "" {
		return errcode.NewWithMessage(errcode.CodeParamError, "email is required")
	}
	if existing, err := dal.GetUsersByEmail(email); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error": err.Error(),
			"email": email,
		})
	} else if existing != nil {
		return errcode.NewWithMessage(errcode.CodeParamError, "email already exists")
	}
	return nil
}

func setCreateUserAdditionalInfo(user *model.User, createUserReq *model.CreateUserReq) error {
	if createUserReq.AdditionalInfo == nil {
		user.AdditionalInfo = StringPtr("{}")
		return nil
	}

	var js map[string]interface{}
	if err := json.Unmarshal(*createUserReq.AdditionalInfo, &js); err != nil {
		return errcode.WithData(errcode.CodeSystemError, map[string]interface{}{
			"error": fmt.Sprintf("Failed to unmarshal AdditionalInfo: %v", err),
		})
	}
	user.AdditionalInfo = StringPtr(string(*createUserReq.AdditionalInfo))
	return nil
}

func (u *User) assignCreateUserAuthority(user *model.User, claims *utils.UserClaims) error {
	switch claims.Authority {
	case "SYS_ADMIN":
		user.Authority = StringPtr("TENANT_ADMIN")
		user.TenantID = StringPtr(strings.Split(uuid.New(), "-")[0])
		return nil
	case "TENANT_ADMIN":
		a, err := u.GetUserById(claims.ID)
		if err != nil {
			logrus.Error(err)
			return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
				"error":    err.Error(),
				"admin_id": claims.ID,
			})
		}
		user.TenantID = a.TenantID
		// 租户管理员创建的用户固定为 TENANT_USER；缺失该赋值会让新用户 authority 落空，
		// 导致列表过滤（Authority==TENANT_USER）查不到，且后续按角色鉴权异常。
		user.Authority = StringPtr("TENANT_USER")
		return nil
	default:
		return errcode.WithVars(errcode.CodeNoPermission, map[string]interface{}{
			"required_role": "SYS_ADMIN or TENANT_ADMIN",
			"current_role":  claims.Authority,
		})
	}
}

func setCreateUserPassword(user *model.User, password string) error {
	if err := utils.ValidatePassword(password); err != nil {
		return err
	}

	hashedPassword, hashErr := utils.BcryptHash(password)
	if hashErr != nil {
		return errcode.WithData(errcode.CodeDecryptError, map[string]interface{}{
			"error": "Failed to hash password",
			"cause": hashErr.Error(),
		})
	}
	user.Password = hashedPassword
	return nil
}

func createUserWithAddressDefaultBoardAndRoles(user *model.User, createUserReq *model.CreateUserReq, claims *utils.UserClaims) error {
	// RBAC 激活后 casbin g 表是授权事实源：创建请求未显式给 RoleIDs 时，
	// 按 users.authority 兜底绑定（与 63.sql 对存量用户的种子口径一致），
	// 否则新用户在 RBAC 生效（deny-unregistered / 种子后 Verify 全走 casbin）时被全量 403。
	roleIDs := createUserReq.RoleIDs
	if len(roleIDs) == 0 && user.Authority != nil && *user.Authority != "" {
		roleIDs = []string{*user.Authority}
	}
	bound := false
	if err := query.Q.Transaction(func(tx *query.Query) error {
		if err := tx.User.Create(user); err != nil {
			return err
		}
		if err := createUserAddressWithTx(tx, user.ID, createUserReq.Address); err != nil {
			return err
		}
		if authz.IsSysAdmin(claims) {
			if err := tx.Board.Create(dal.NewDefaultBoard(user.TenantID)); err != nil {
				return err
			}
		}
		if len(roleIDs) == 0 {
			return nil
		}
		bound = true
		return replaceUserRoleBindingsWithTx(tx, user.ID, roleIDs)
	}); err != nil {
		logrus.Error(err)
		if strings.Contains(err.Error(), "users_un") {
			return errcode.New(200008)
		}
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":      err.Error(),
			"user_email": user.Email,
		})
	}
	if bound && global.CasbinEnforcer != nil && global.CasbinEnforcer.GetAdapter() != nil {
		// 仅带 DB adapter 的生产 enforcer 需要重载内存策略（绑定经 tx 直写 DB）；
		// 单测内存模型无 adapter，LoadPolicy 会 panic，其策略本就由内存直管无需重载。
		// 创建频率低，全量重载最简单且与 DB 强一致；失败不回滚用户（重启/下次绑定收敛），仅告警。
		if err := global.CasbinEnforcer.LoadPolicy(); err != nil {
			logrus.Errorf("casbin LoadPolicy after user create failed (memory stale until next reload): err=%v", err)
		}
	}
	return nil
}

func createUserAddressWithTx(tx *query.Query, userID string, addressReq *model.CreateUserAddressReq) error {
	if addressReq == nil {
		return nil
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
