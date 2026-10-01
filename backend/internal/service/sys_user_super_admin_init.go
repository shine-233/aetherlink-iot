// 文件用途：超级管理员初始化与市场注册聚合（从 sys_user_manage.go 拆出）。
// 核心逻辑：
//   - 服务端硬门禁：实例上已存在任意 SYS_ADMIN 时一律拒绝再次初始化（不依赖客户端字段）；
//   - 市场校验：按 market.enabled 决定是否校验市场账号，并校验请求邮箱与市场邮箱一致；
//   - 本地建号：事务内创建 SYS_ADMIN 用户并绑定角色，失败按 buildLocalInitLoginFailure 补偿清理。
//
// 关键注意事项：初始化竞态由 superAdminInitMu 串行化；任何"跳过市场校验"的分支都不能
// 越过 CheckSysAdminExists 硬门禁，否则会出现第二个超管。
package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	dal "aetherlink-iot/backend/internal/dal"
	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

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
		if _, err := GroupApp.Casbin.RemoveUserAndRoleWithError(userInfo.ID); err != nil {
			errorData["casbin_cleanup_error"] = err.Error()
		}
	}
	if global.DB != nil {
		if err := global.DB.Where("ptype = ? AND v0 = ?", "g", userInfo.ID).Delete(&model.CasbinRule{}).Error; err != nil {
			errorData["casbin_rule_cleanup_error"] = err.Error()
		}
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
