// 文件用途：
//
//	提供系统用户、租户初始化与账号偏好相关 HTTP 接口，覆盖登录登出、验证码、密码重置、用户 CRUD、租户初始化及个人资料维护等入口。
//
// 核心链路：
//
//	各 handler 负责完成请求绑定、claims 提取、必要的轻量输入校验，再调用 service.GroupApp.User 或相关 service 完成业务处理，最后通过统一响应中间件输出 data。
//
// 使用注意：
//  1. API 层只保留协议适配、上下文读取和响应写回，不应下沉复杂权限判断或数据拼装逻辑。
//  2. 涉及 token、邮箱、手机号、租户初始化的接口都依赖 service 层兜底权限与状态校验，静态审查时要顺着调用链继续核对。
//
// 静态审查建议：
//  1. 重点检查 claims、x-token、path id 与 query 参数是否在 service 层完成租户隔离、越权访问和敏感信息脱敏。
//  2. 审查登录、注册、初始化、邮箱变更等安全敏感链路的限流、错误码一致性和状态迁移完整性。
package api

import (
	middleware "aetherlink-iot/backend/internal/middleware"
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type UserApi struct{}

// Login 用户登录。
// 核心链路：绑定登录请求后先做账号格式识别，手机号场景会先映射成邮箱，再经过登录锁检查后调用 service 完成认证并返回 token。
// 使用注意：本函数只做轻量校验与节流前置，密码校验、账号状态判断和 token 生成策略都在 service 层。
// 审查重点：关注手机号转邮箱链路、登录失败计数、锁定策略以及错误返回是否泄露账号存在性。
// @Summary      用户登录
// @Description  认证令牌(Token)将在用户成功登录后生成并返回。客户端需要在后续所有需要认证的API请求中，将此令牌添加到HTTP请求头(Header)的'x-token'字段中。服务器将通过验证此令牌来确认用户身份并授权访问受保护资源。
// @Tags         用户认证
// @Accept       json
// @Produce      json
// @Param        request body model.LoginReq true "登录凭证" example({"email":"user@example.com","password":"Aa123456!"})
// @Success      200 {object} model.LoginRsp "成功"
// @Failure      400 {object} errcode.Error "错误响应"
// @Router       /api/v1/login [post]
// @example request - "请求片段" {"email":"user@example.com","password":"Aa123456!"}
func (*UserApi) Login(c *gin.Context) {
	var loginReq model.LoginReq
	if !BindAndValidate(c, &loginReq) {
		return
	}

	result := utils.ValidateInput(loginReq.Email)
	if !result.IsValid {
		c.Error(errcode.WithData(200013, map[string]interface{}{
			"error": result.Message,
		}))
		return
	}

	if result.Type == utils.Phone {
		// 通过手机号换取用户邮箱，复用统一邮箱登录链路。
		email, err := service.GroupApp.User.GetUserEmailByPhoneNumber(loginReq.Email)
		if err != nil {
			c.Error(err)
			return
		}
		loginReq.Email = email
	}

	loginLock := service.NewLoginLock()

	// 检查账号是否因连续失败而被锁定。
	if loginLock.MaxFailedAttempts > 0 {
		if err := loginLock.GetAllowLogin(c, loginReq.Email); err != nil {
			c.Error(err)
			return
		}
	}

	loginRsp, err := service.GroupApp.User.Login(c, &loginReq)
	if err != nil {
		_ = loginLock.LoginFail(c, loginReq.Email)
		c.Error(err)
		return
	}
	_ = loginLock.LoginSuccess(c, loginReq.Email)
	setAuthCookieForLoginResponse(c, loginRsp)
	c.Set("data", loginRsp)
}

// Logout 用户登出。
// 核心链路：从请求头提取 x-token，交给 service 执行会话失效或黑名单处理。
// 审查重点：确认 token 为空、重复登出和多端会话策略的行为是否明确。
func (*UserApi) Logout(c *gin.Context) {
	token := c.GetHeader("x-token")
	err := service.GroupApp.User.Logout(token)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// RefreshToken 刷新当前登录态。
// 双模式：token 来源支持认证 cookie（优先）或 x-token 头（存量客户端兼容），见 middleware.selectJWTAuthToken。
// 审查重点：确认 claims 来源可信，且刷新不会绕过封禁、租户停用或权限变更。
func (*UserApi) RefreshToken(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	loginRsp, err := service.GroupApp.User.RefreshToken(userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	setAuthCookieForLoginResponse(c, loginRsp)
	c.Set("data", loginRsp)
}

// setAuthCookieForLoginResponse 在登录/刷新成功响应上追加 HttpOnly 认证 cookie。
// cookie 开关与 Secure 标志由 GOTP_AUTH_COOKIE_ENABLED / GOTP_AUTH_COOKIE_SECURE 控制，
// 未启用时保持纯 x-token 头 + 响应体 token 的既有行为。
func setAuthCookieForLoginResponse(c *gin.Context, loginRsp *model.LoginRsp) {
	if loginRsp == nil || loginRsp.Token == nil {
		return
	}
	middleware.SetAuthCookie(c, *loginRsp.Token, int(loginRsp.ExpiresIn))
}

// HandleVerificationCode 发送验证码。
// 核心链路：读取邮箱与注册标记参数，由 service 处理验证码生成、发送和发送前校验。
// 审查重点：确认是否有限流、防刷、账号枚举保护和邮件发送失败后的错误码约定。
func (*UserApi) HandleVerificationCode(c *gin.Context) {
	email := c.Query("email")
	isRegister := c.Query("is_register")
	language := c.Query("language")
	err := service.GroupApp.User.GetVerificationCode(email, isRegister, language)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// ResetPassword 重置密码。
// 核心链路：绑定重置参数后调用 service 执行验证码校验、密码更新和后续会话处理。
// 审查重点：确认弱密码限制、验证码失效策略和重置后旧 token 失效是否覆盖。
func (*UserApi) ResetPassword(c *gin.Context) {
	var resetPasswordReq model.ResetPasswordReq
	if !BindAndValidate(c, &resetPasswordReq) {
		return
	}

	err := service.GroupApp.User.ResetPassword(c, &resetPasswordReq)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// RequestPasswordResetLink 校验邮箱验证码并发送一次性密码重置链接。
// 核心链路：邮箱验证码仍作为第一道校验，成功后生成短期 reset_token 邮件链接，兼容用户手册中的链接式重置流程。
// 审查重点：确认 token 一次性消费、过期时间和邮件链接基准地址配置。
func (*UserApi) RequestPasswordResetLink(c *gin.Context) {
	var req model.ResetPasswordLinkReq
	if !BindAndValidate(c, &req) {
		return
	}

	data, err := service.GroupApp.User.RequestPasswordResetLink(c, &req)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// CreateUser 创建用户。
// 核心链路：绑定创建请求并读取当前登录用户 claims，再由 service 执行租户内用户创建。
// 审查重点：确认创建权限、默认角色、初始密码/邀请策略以及跨租户写入保护。
// @Router   /api/v1/user [post]
func (*UserApi) CreateUser(c *gin.Context) {
	var createUserReq model.CreateUserReq

	if !BindAndValidate(c, &createUserReq) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	err := service.GroupApp.User.CreateUser(&createUserReq, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// HandleUserListByPage 分页获取用户列表。
// 审查重点：确认列表查询受租户边界约束，且分页/筛选参数存在上限避免全量扫表。
// @Router   /api/v1/user [get]
func (*UserApi) HandleUserListByPage(c *gin.Context) {
	var userListReq model.UserListReq

	if !BindAndValidate(c, &userListReq) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	userList, err := service.GroupApp.User.GetUserListByPage(&userListReq, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", userList)
}

// UpdateUser 更新指定用户信息。
// 核心链路：绑定更新参数后，以当前 claims 身份调用 service 完成租户内用户资料修改。
// 审查重点：确认角色、状态、组织归属等高权限字段不会被低权限调用者越权修改。
// @Router   /api/v1/user [put]
func (*UserApi) UpdateUser(c *gin.Context) {
	var updateUserReq model.UpdateUserReq

	if !BindAndValidate(c, &updateUserReq) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	err := service.GroupApp.User.UpdateUser(&updateUserReq, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// DeleteUser 删除用户。
// 核心链路：从路径参数读取目标用户 id，再由 service 执行删除或禁用逻辑。
// 审查重点：确认是否防止删除自己、最后一个管理员、已绑定关键资源的用户。
// @Router   /api/v1/user/{id} [delete]
func (*UserApi) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	err := service.GroupApp.User.DeleteUser(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// HandleUser 获取指定用户详情。
// 核心链路：读取目标用户 id，调用 service 获取详情后在 API 层补做密码字段剔除。
// 使用注意：这里的脱敏只删除 map 里的 password，静态审查时要继续确认其他敏感字段是否已在 service 层处理。
// @Router   /api/v1/user/{id} [get]
func (*UserApi) HandleUser(c *gin.Context) {
	id := c.Param("id")

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	user, err := service.GroupApp.User.GetUser(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	// 清除显式暴露的密码字段，避免直接透出敏感信息。
	if userMap, ok := user.(map[string]interface{}); ok {
		delete(userMap, "password")
	}

	c.Set("data", user)
}

// HandleUserDetail 获取当前登录用户详情。
// 审查重点：确认返回字段不会泄露内部密钥、权限缓存或其他仅后台使用的敏感属性。
// @Router   /api/v1/user/detail [get]
func (*UserApi) HandleUserDetail(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	user, err := service.GroupApp.User.GetUserDetail(userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	// 清除显式暴露的密码字段，避免直接透出敏感信息。
	if userMap, ok := user.(map[string]interface{}); ok {
		delete(userMap, "password")
	}

	c.Set("data", user)
}

// UpdateUsers 更新当前用户个人资料。
// 核心链路：绑定个人资料更新请求后，基于当前 claims 调用 service 写回用户自身信息。
// 审查重点：当前实现遇错后未立即 return，需持续关注统一错误处理中间件与 c.Set("data", nil) 的组合是否可能产生歧义响应。
// @Router   /api/v1/user/update [put]
func (*UserApi) UpdateUsers(c *gin.Context) {
	var updateUserInfoReq model.UpdateUserInfoReq

	if !BindAndValidate(c, &updateUserInfoReq) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	err := service.GroupApp.User.UpdateUserInfo(c, &updateUserInfoReq, userClaims)
	if err != nil {
		c.Error(err)
	}

	c.Set("data", nil)
}

// ChangeEmail 修改当前账号邮箱，并保持原租户和设备归属关系。
// 核心链路：绑定换邮请求后交由 service 处理验证码校验、邮箱占用检查、关联数据迁移与结果返回。
// 审查重点：确认换邮后登录凭证、通知邮箱、租户成员关系和第三方绑定状态是否同步更新。
// @Router   /api/v1/user/change-email [post]
func (*UserApi) ChangeEmail(c *gin.Context) {
	var req model.ChangeEmailReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.User.ChangeEmail(c, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// GetWarningEmails 获取当前租户的全局告警接收邮箱。
// 审查重点：确认返回的配置与实际告警消费链路使用的是同一份租户级接收人。
// @Router   /api/v1/user/warning-email [get]
func (*UserApi) GetWarningEmails(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.User.GetWarningEmails(userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// UpdateWarningEmails 更新当前租户的全局告警接收邮箱。
// 审查重点：关注邮箱列表格式校验、去重策略和租户级告警接收人与消费链路的一致性。
// @Router   /api/v1/user/warning-email [put]
func (*UserApi) UpdateWarningEmails(c *gin.Context) {
	var req model.WarningEmailReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.User.UpdateWarningEmails(c, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// UpdatePreferredLanguage 更新当前账号语言偏好，沿用既有 /user/prefer-lang 路径。
// 审查重点：确认语言代码白名单、默认回退逻辑和缓存刷新策略在 service 层一致。
// @Router   /api/v1/user/prefer-lang [post]
func (*UserApi) UpdatePreferredLanguage(c *gin.Context) {
	var req model.PreferLanguageReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	data, err := service.GroupApp.User.UpdatePreferredLanguage(c, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", data)
}

// TransformUser 执行用户身份转换。
// 核心链路：绑定转换请求并读取 claims，再由 service 返回转换后的登录态或目标身份信息。
// 审查重点：确认身份切换不会突破租户边界，也不会保留旧身份的越权缓存。
func (*UserApi) TransformUser(c *gin.Context) {
	var transformUserReq model.TransformUserReq

	if !BindAndValidate(c, &transformUserReq) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	loginRsp, err := service.GroupApp.User.TransformUser(&transformUserReq, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", loginRsp)
}

// EmailRegister 通过邮箱注册租户/用户。
// 审查重点：确认注册开放条件、验证码验证、租户初始化默认值与重复注册行为是否受控。
// @description 租户邮箱注册
func (*UserApi) EmailRegister(c *gin.Context) {
	var req model.EmailRegisterReq
	if !BindAndValidate(c, &req) {
		return
	}
	loginRsp, err := service.GroupApp.EmailRegister(c, &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", loginRsp)
}

// HasAdmin 检查系统是否已存在超级管理员。
// 审查重点：该接口通常用于安装引导，需确认不会暴露多余部署状态细节。
// @description 检查是否存在超级管理员账号
func (*UserApi) HasAdmin(c *gin.Context) {
	exists, err := service.GroupApp.User.CheckSysAdminExists()
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", gin.H{"has_admin": exists})
}

// SetupState 获取首次安装状态，供登录页决定展示登录还是注册。
// 审查重点：确认安装状态的判定来源单一，避免初始化中间态导致前后端分支不一致。
// @description 获取首次安装状态，供登录页决定展示登录还是注册
func (*UserApi) SetupState(c *gin.Context) {
	state, err := service.GroupApp.User.GetTenantSetupState()
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", state)
}

// InitSuperAdmin 首次安装时初始化超级管理员。
// 核心链路：绑定初始化请求后由 service 完成超管创建、租户初始化和可选市场回流参数处理。
// 审查重点：确认该接口只能在首次安装窗口执行，且重复调用、并发调用都有幂等保护。
// @description 首次安装超管初始化（支持市场回流参数）
func (*UserApi) InitSuperAdmin(c *gin.Context) {
	var req model.SuperAdminInitReq
	if !BindAndValidate(c, &req) {
		return
	}

	loginRsp, err := service.GroupApp.User.InitSuperAdmin(c, &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", loginRsp)
}

// MarketRegister 沿用既有路径完成市场联动场景下的超管注册。
// 审查重点：确认与 InitSuperAdmin 复用同一 service 时，路径差异不会引入不同安全假设。
// @description 沿用既有接口路径的超管注册（联动市场）
func (*UserApi) MarketRegister(c *gin.Context) {
	var req model.SuperAdminInitReq
	if !BindAndValidate(c, &req) {
		return
	}

	loginRsp, err := service.GroupApp.User.InitSuperAdmin(c, &req)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", loginRsp)
}

// GetTenantID 获取当前登录用户的租户 ID。
// 审查重点：确认 claims 中 tenant_id 的生成与刷新时机可靠，避免返回过期租户上下文。
// @Router   /api/v1/user/tenant/id [get]
func (*UserApi) GetTenantID(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	tenantID := userClaims.TenantID

	c.Set("data", tenantID)
}

// UpdateUserAddress 更新用户地址信息。
// 核心链路：读取目标用户 id，绑定地址请求后由 service 负责新建或更新地址。
// 审查重点：确认目标 id 的可操作权限，以及地址字段长度、格式和国际化兼容约束。
// @Summary      更新用户地址信息
// @Description  更新指定用户的地址信息，支持创建新地址或更新现有地址
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Param        id path string true "用户ID"
// @Param        request body model.UpdateUserAddressReq true "地址信息"
// @Success      200 {object} interface{} "成功"
// @Failure      400 {object} errcode.Error "错误响应"
// @Router       /api/v1/user/address/{id} [put]
func (*UserApi) UpdateUserAddress(c *gin.Context) {
	id := c.Param("id")
	var updateAddressReq model.UpdateUserAddressReq

	if !BindAndValidate(c, &updateAddressReq) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	err := service.GroupApp.User.UpdateUserAddress(id, &updateAddressReq, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", nil)
}

// GetUserSelector 返回用户选择器数据源。
// 核心链路：绑定名称和分页筛选条件后，由 service 返回管理员与租户用户的候选列表。
// 审查重点：确认模糊匹配不会跨租户泄露用户名单，且 page/page_size 的默认值与上限合理。
// @Summary      用户选择器
// @Description  获取租户管理员和租户用户的选择器列表，支持名称模糊匹配
// @Tags         用户管理
// @Accept       json
// @Produce      json
// @Param        name query string false "用户名称（模糊匹配）"
// @Param        page query int false "页码，默认1"
// @Param        page_size query int false "每页数量，默认10"
// @Success      200 {object} interface{} "成功"
// @Failure      400 {object} errcode.Error "错误响应"
// @Router       /api/v1/user/selector [get]
func (*UserApi) GetUserSelector(c *gin.Context) {
	var req model.UserSelectorReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	result, err := service.GroupApp.User.GetUserSelector(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", result)
}
