// 文件用途：维护系统用户创建、更新、删除、初始化和市场注册服务。
// 核心逻辑：校验用户资料、联系方式、角色权限和默认看板，串联用户表、地址、角色与市场接口。
// 关键注意事项：用户管理会改变权限和租户资产，角色绑定失败、市场失败和删除清理必须可回滚或可补偿。
// 重构建议：拆分用户资料事务、角色绑定和市场副作用，补齐权限、事务、补偿和外部失败测试。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
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
		if claims.Authority == "SYS_ADMIN" {
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

// GetUserById 按用户 ID 查询完整用户资料。
func (*User) GetUserById(id string) (*model.User, error) {
	user, err := dal.GetUsersById(id)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// userListScopes 解析用户目录列表读作用域（ROADMAP C2 自上而下）：
// TENANT_USER 保持 self-only（成员目录仅本租户）；TENANT_ADMIN → expandTenantIDScope
// （self∪子孙，可读层级内成员用户）；SYS_ADMIN → nil（平台级管理员目录，无租户过滤）；
// nil/未知声明 → nil，由 DAL 显式拒绝。
func userListScopes(claims *utils.UserClaims) []string {
	if claims == nil {
		return nil
	}
	switch claims.Authority {
	case constant.TENANT_USER:
		if tenantID := strings.TrimSpace(claims.TenantID); tenantID != "" {
			return []string{tenantID}
		}
		return nil
	case constant.TENANT_ADMIN:
		tenantID := strings.TrimSpace(claims.TenantID)
		if tenantID == "" {
			return nil
		}
		return expandTenantIDScope(tenantID)
	default:
		return nil
	}
}

// GetUserListByPage 按当前登录人的权限边界分页查询用户列表。
func (*User) GetUserListByPage(userListReq *model.UserListReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetUserListByPage(userListReq, claims, userListScopes(claims))
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "query_user",
			"error":     err.Error(),
		})
	}
	userListRspMap := make(map[string]interface{})
	userListRspMap["total"] = total
	// 空页显式返回 [] 而非 nil：nil 经 JSON 序列化会变成 null，破坏前端与
	// 自动化断言的分页契约（与 GetDeviceConfigListByPage 的处理保持一致）。
	if list == nil {
		list = make([]map[string]interface{}, 0)
	}
	userListRspMap["list"] = list
	return userListRspMap, nil
}

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

// DeleteUser 删除普通用户并清理其 Casbin 角色绑定。
func (*User) DeleteUser(id string, claims *utils.UserClaims) error {
	// 删除前先加载用户，用于权限判断并避免误删系统管理员。
	user, err := dal.GetUsersById(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":   err.Error(),
			"user_id": id,
		})
	}

	if err := ensureUserManagementWriteAccess(user, claims, "delete_user"); err != nil {
		return err
	}

	if SafeDeref(user.Authority) == constant.SYS_ADMIN {
		return errcode.WithVars(errcode.CodeOpDenied, map[string]interface{}{
			"reason":  "cannot_delete_sys_admin",
			"user_id": id,
		})
	}

	if err := revokeUserRoleBindings(id); err != nil {
		return err
	}

	// 角色绑定已撤销后再删除用户主记录，降低残留授权风险。
	err = dal.DeleteUsersById(id)
	if err != nil {
		return errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":     err.Error(),
			"user_id":   id,
			"operation": "delete_user",
		})
	}

	return nil
}

func (u *User) GetUserEmailByPhoneNumber(phoneNumber string) (string, error) {
	// 手机号登录场景需要先反查邮箱，未命中时返回业务错误码。
	user, err := dal.GetUsersByPhoneNumber(phoneNumber)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errcode.New(200013)
		}
		return "", errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"message": "get_user_by_phone_number",
			"error":   err.Error(),
		})
	}
	return user.Email, nil
}

func (u *User) CheckSysAdminExists() (bool, error) {
	users := query.User
	userList, err := users.Where(users.Authority.Eq("SYS_ADMIN")).Find()
	if err != nil {
		return false, err
	}
	return len(userList) > 0, nil
}

func (u *User) GetTenantSetupState() (*model.TenantSetupStateRsp, error) {
	hasAdmin, err := u.CheckSysAdminExists()
	if err != nil {
		return nil, err
	}
	hasTenantAdmin, hasTenant, err := u.CheckTenantAdminSetupExists()
	if err != nil {
		return nil, err
	}

	baseURL := getMarketBaseURL()
	return buildTenantSetupState(hasAdmin, hasTenantAdmin, hasTenant, baseURL), nil
}

func (u *User) CheckTenantAdminSetupExists() (bool, bool, error) {
	users := query.User
	userList, err := users.Where(users.Authority.Eq(constant.TENANT_ADMIN)).Find()
	if err != nil {
		return false, false, err
	}

	hasTenantAdmin := len(userList) > 0
	hasTenant := false
	for _, user := range userList {
		if strings.TrimSpace(SafeDeref(user.TenantID)) != "" {
			hasTenant = true
			break
		}
	}
	return hasTenantAdmin, hasTenant, nil
}

func buildTenantSetupState(hasAdmin bool, hasTenantAdmin bool, hasTenant bool, baseURL string) *model.TenantSetupStateRsp {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	entry := "login"
	nextStep := "login"
	if !hasAdmin {
		entry = "register"
		nextStep = "create_super_admin"
	} else if !hasTenantAdmin || !hasTenant {
		nextStep = "create_tenant_admin"
	}
	marketRegisterURL := ""
	if baseURL != "" {
		marketRegisterURL = baseURL + "/register"
	}

	return &model.TenantSetupStateRsp{
		HasAdmin:          hasAdmin,
		HasTenantAdmin:    hasTenantAdmin,
		HasTenant:         hasTenant,
		Entry:             entry,
		NextStep:          nextStep,
		MarketBaseURL:     baseURL,
		MarketRegisterURL: marketRegisterURL,
	}
}

func shouldSkipMarketCheck(req *model.SuperAdminInitReq) bool {
	if req == nil || !req.MarketRegistered {
		return false
	}

	requestEmail := strings.TrimSpace(req.Email)
	marketEmail := strings.TrimSpace(req.MarketEmail)
	if requestEmail == "" || marketEmail == "" {
		return false
	}

	return strings.EqualFold(requestEmail, marketEmail)
}

// superAdminInitMu 串行化超管初始化，覆盖单实例部署下的并发初始化竞态。
var superAdminInitMu sync.Mutex

func (u *User) InitSuperAdmin(ctx context.Context, req *model.SuperAdminInitReq) (*model.LoginRsp, error) {
	if req == nil {
		return nil, errcode.NewWithMessage(errcode.CodeParamError, "super admin init request is required")
	}
	// 服务端硬门禁：只要实例上已存在任意 SYS_ADMIN，一律拒绝再次初始化。
	// 该检查只依赖数据库状态，不依赖任何客户端可控字段；市场跳过分支仅影响
	// "谁有资格成为第一个超管"，不能越过本门禁。
	superAdminInitMu.Lock()
	defer superAdminInitMu.Unlock()

	hasAdmin, err := u.CheckSysAdminExists()
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "check_sys_admin_exists",
			"error":     err.Error(),
		})
	}
	if hasAdmin {
		return nil, errcode.New(errcode.CodeSuperAdminExists)
	}

	requestEmail := strings.TrimSpace(req.Email)
	marketEmail := strings.TrimSpace(req.MarketEmail)
	if err := validateSuperAdminMarketEmail(req, requestEmail, marketEmail); err != nil {
		return nil, err
	}

	if err := ensureSuperAdminMarketAccount(ctx, req, requestEmail); err != nil {
		return nil, err
	}
	if err := ensureLocalSuperAdminAbsent(requestEmail); err != nil {
		return nil, err
	}
	if err := utils.ValidatePassword(req.Password); err != nil {
		return nil, err
	}

	userInfo, userInfoErr := newLocalSuperAdminUser(requestEmail, req.Password, time.Now().UTC())
	if userInfoErr != nil {
		return nil, userInfoErr
	}

	bound := false
	if err := query.Q.Transaction(func(tx *query.Query) error {
		if err := tx.User.Create(userInfo); err != nil {
			return err
		}
		if userInfo.Authority != nil && *userInfo.Authority != "" {
			bound = true
			return replaceUserRoleBindingsWithTx(tx, userInfo.ID, []string{*userInfo.Authority})
		}
		return nil
	}); err != nil {
		return nil, errcode.WithData(errcode.CodeLocalInitCreateUserFail, map[string]interface{}{
			"operation": "create_user",
			"email":     requestEmail,
			"error":     err.Error(),
		})
	}
	if bound && global.CasbinEnforcer != nil {
		if global.CasbinEnforcer.GetAdapter() != nil {
			if err := global.CasbinEnforcer.LoadPolicy(); err != nil {
				logrus.Errorf("casbin LoadPolicy after super admin init failed: err=%v", err)
			}
		} else {
			_, _ = GroupApp.Casbin.AddRolesToUserWithError(userInfo.ID, []string{*userInfo.Authority})
		}
	}
	loginRsp, err := u.UserLoginAfter(userInfo)
	if err != nil {
		return nil, buildLocalInitLoginFailure(userInfo, err)
	}

	return loginRsp, nil
}

func validateSuperAdminMarketEmail(req *model.SuperAdminInitReq, requestEmail, marketEmail string) error {
	if req.MarketRegistered && marketEmail != "" && !strings.EqualFold(requestEmail, marketEmail) {
		return errcode.WithData(errcode.CodeMarketCheckFailed, map[string]interface{}{
			"error":        "market email does not match request email",
			"email":        requestEmail,
			"market_email": marketEmail,
		})
	}
	return nil
}

func ensureSuperAdminMarketAccount(ctx context.Context, req *model.SuperAdminInitReq, requestEmail string) error {
	if shouldSkipMarketCheck(req) {
		return nil
	}
	if !viper.GetBool("market.enabled") {
		return errcode.WithData(errcode.CodeMarketServiceUnavailable, map[string]interface{}{
			"error":           "market integration is disabled",
			"market_base_url": strings.TrimRight(viper.GetString("market.base_url"), "/"),
		})
	}

	marketClient := NewMarketClient()
	exists, err := marketClient.CheckUserExists(ctx, requestEmail)
	if err != nil {
		code := errcode.CodeMarketCheckFailed
		if errors.Is(err, ErrMarketServiceUnavailable) {
			code = errcode.CodeMarketServiceUnavailable
		}
		return errcode.WithData(code, map[string]interface{}{
			"error":           err.Error(),
			"market_base_url": strings.TrimRight(viper.GetString("market.base_url"), "/"),
			"email":           requestEmail,
		})
	}
	if !exists {
		return errcode.New(200055)
	}
	return nil
}

func ensureLocalSuperAdminAbsent(requestEmail string) error {
	user, err := dal.GetUsersByEmail(requestEmail)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return errcode.WithData(errcode.CodeLocalInitCreateUserFail, map[string]interface{}{
			"operation": "query_user",
			"email":     requestEmail,
			"error":     err.Error(),
		})
	}
	if user != nil {
		return errcode.New(200008)
	}
	return nil
}

func newLocalSuperAdminUser(requestEmail, password string, now time.Time) (*model.User, error) {
	hashedPassword, hashErr := utils.BcryptHash(password)
	if hashErr != nil {
		return nil, errcode.WithData(errcode.CodeDecryptError, map[string]interface{}{
			"error": "Failed to hash password",
			"email": requestEmail,
		})
	}
	return &model.User{
		ID:                  uuid.New(),
		Name:                &requestEmail,
		Email:               requestEmail,
		Status:              StringPtr("N"),
		Authority:           StringPtr("SYS_ADMIN"),
		Password:            hashedPassword,
		TenantID:            StringPtr(""),
		CreatedAt:           &now,
		UpdatedAt:           &now,
		PasswordLastUpdated: &now,
	}, nil
}

func buildLocalInitLoginFailure(userInfo *model.User, cause error) error {
	errorData := map[string]interface{}{
		"email": userInfo.Email,
	}
	if cleanupErr := dal.DeleteUsersById(userInfo.ID); cleanupErr != nil {
		errorData["cleanup_error"] = cleanupErr.Error()
	}
	if global.CasbinEnforcer != nil {
		_, _ = GroupApp.Casbin.RemoveUserAndRoleWithError(userInfo.ID)
	}
	if global.DB != nil {
		_ = global.DB.Where("ptype = ? AND v0 = ?", "g", userInfo.ID).Delete(&model.CasbinRule{}).Error
	}
	if codeErr, ok := cause.(*errcode.Error); ok {
		errorData["cause_code"] = codeErr.Code
		if codeErr.Data != nil {
			errorData["cause_data"] = codeErr.Data
		}
	} else {
		errorData["error"] = cause.Error()
	}
	return errcode.WithData(errcode.CodeLocalInitLoginFail, errorData)
}

// MarketRegister 复用初始化超级管理员流程完成市场注册入口。
func (u *User) MarketRegister(ctx context.Context, req *model.MarketRegisterReq) (*model.LoginRsp, error) {
	return u.InitSuperAdmin(ctx, req)
}

// GetTenantInfo 按租户 ID 读取租户对应的用户资料。
func (u *User) GetTenantInfo(tenantID string) (*model.User, error) {
	tenant, err := dal.GetTenantsById(tenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID,
		})
	}
	return tenant, nil
}

func (*User) GetUserSelector(req *model.UserSelectorReq, claims *utils.UserClaims) (map[string]interface{}, error) {
	total, list, err := dal.GetUserSelector(req, claims.TenantID)
	if err != nil {
		return nil, errcode.WithData(errcode.CodeDBError, map[string]interface{}{
			"operation": "get_user_selector",
			"error":     err.Error(),
		})
	}

	result := make(map[string]interface{})
	result["total"] = total
	result["list"] = list
	return result, nil
}
