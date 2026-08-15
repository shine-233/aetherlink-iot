// 文件用途：承载看板域的 HTTP Handler，包括看板 CRUD、首页看板、设备统计、
// 租户概览、租户维度明细、个人信息维护以及设备趋势查询等接口。
// 调用链约定：路由层将 /api/v1/board* 请求分发到本文件；本层负责 BindAndValidate、
// 读取 gin.Context 中的 claims 与路径参数，再调用 service.GroupApp.Board、
// service.GroupApp.Device、service.UsersService 等服务完成业务处理，最后统一通过
// c.Set("data", ...) 交给响应中间件输出。
// 权限边界：本层默认信任认证中间件已写入 claims，但仍需在入口明确“是否只允许当前租户、
// 是否只允许系统管理员、是否允许跨租户查询”这三类边界；凡是依赖 service 再次兜底鉴权的
// 接口，都应在注释中写清楚，避免未来维护时误以为 API 已经完成全部校验。
package api

import (
	model "aetherlink-iot/backend/internal/model"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/errcode"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

type BoardApi struct{}

// CreateBoard 创建看板。
// 调用链：POST /api/v1/board -> BindAndValidate(CreateBoardReq) -> 从 claims 读取当前用户与租户
// -> service.GroupApp.Board.CreateBoard -> service 内部校验配置 JSON、按 claims.TenantID 落租户、
// 必要时重置该租户现有首页看板标记 -> 返回新建看板。
// 权限边界：接口本身不接受外部 tenantID，租户归属完全以 claims.TenantID 为准；只要能进入该路由，
// 就会尝试创建当前租户下的看板，细粒度写权限依赖 service 侧规则。
// @Router   /api/v1/board [post]
func (*BoardApi) CreateBoard(c *gin.Context) {
	var req model.CreateBoardReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	boardInfo, err := service.GroupApp.Board.CreateBoard(c, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", boardInfo)
}

// UpdateBoard 更新看板。
// 调用链：PUT /api/v1/board -> BindAndValidate(UpdateBoardReq) -> 读取 claims
// -> service.GroupApp.Board.UpdateBoard -> service 内部完成配置校验、目标看板写权限校验、
// 首页看板切换以及持久化。
// 权限边界：当前层不直接判断看板是否属于当前租户，而是把 claims 交给 service 做写权限兜底；
// 另外该 service 在 Id 为空时会退化为创建流程，调用方需要清楚这是“更新接口带 upsert 语义”。
// @Router   /api/v1/board [put]
func (*BoardApi) UpdateBoard(c *gin.Context) {
	var req model.UpdateBoardReq
	if !BindAndValidate(c, &req) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)
	d, err := service.GroupApp.Board.UpdateBoard(c, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", d)
}

// DeleteBoard 删除看板。
// 调用链：DELETE /api/v1/board/{id} -> 读取路径 id 与 claims
// -> service.GroupApp.Board.DeleteBoard -> service 先校验目标看板读写归属，再执行删除。
// 权限边界：API 层不接受 tenantID，删除范围由 id + claims 共同决定；跨租户删除能力不在此层开放，
// 是否允许删除指定看板完全以 service.ensureBoardWriteAccess 的判定为准。
// @Router   /api/v1/board/{id} [delete]
func (*BoardApi) DeleteBoard(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	err := service.GroupApp.Board.DeleteBoard(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// PublishBoard publishes a native board and returns its public share token.
func (*BoardApi) PublishBoard(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	board, err := service.GroupApp.Board.PublishBoard(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", board)
}

// GetPublishedBoardByShareToken is intentionally registered before JWT
// middleware. It exposes only the native board payload selected by a valid
// published share token.
func (*BoardApi) GetPublishedBoardByShareToken(c *gin.Context) {
	token := c.Param("token")
	board, err := service.GroupApp.Board.GetPublishedBoardByShareToken(token)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", board)
}

// HandleBoardListByPage 分页查询当前租户下的看板列表。
// 调用链：GET /api/v1/board -> BindAndValidate(GetBoardListByPageReq) -> 读取 claims
// -> service.GroupApp.Board.GetBoardListByPage -> DAL 按 claims.TenantID 查询 total/list。
// 权限边界：接口不允许通过查询参数切换 tenantID，分页结果天然限定在当前 claims 所属租户；
// 只要鉴权通过，当前租户内的看板列表就可被读取。
// @Router   /api/v1/board [get]
func (*BoardApi) HandleBoardListByPage(c *gin.Context) {
	var req model.GetBoardListByPageReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	boardList, err := service.GroupApp.Board.GetBoardListByPage(&req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", boardList)
}

// HandleBoard 查询单个看板详情。
// 调用链：GET /api/v1/board/{id} -> 读取路径 id 与 claims
// -> service.GroupApp.Board.GetBoard -> service 内部通过 ensureBoardReadAccess 校验读权限后返回实体。
// 权限边界：是否可读取目标看板不在 API 层硬编码，而在 service 侧按看板归属与 claims 判定；
// 本层只负责把当前登录态与目标 id 传递下去，不提供跨租户绕过入口。
// @Router   /api/v1/board/{id} [get]
func (*BoardApi) HandleBoard(c *gin.Context) {
	id := c.Param("id")
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	board, err := service.GroupApp.Board.GetBoard(id, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", board)
}

// HandleBoardListByTenantId 查询当前租户首页可见的看板集合。
// 调用链：GET /api/v1/board/home -> 读取 claims.TenantID
// -> service.GroupApp.Board.GetBoardListByTenantId -> DAL 按 tenantID 返回首页看板列表。
// 权限边界：接口不接受外部 tenantID，首页看板始终绑定当前 claims.TenantID；当前层没有额外角色限制，
// 因而同一租户内能访问该路由的用户都可看到本租户首页看板结果。
// @Router   /api/v1/board/home [get]
func (*BoardApi) HandleBoardListByTenantId(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	boardList, err := service.GroupApp.Board.GetBoardHomeForClaims(c.Query("tenant_id"), userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", boardList)
}

// HandleDeviceTotal 获取设备总数统计。
// 调用链：GET /api/v1/board/device/total -> 读取 claims.Authority 与 claims.TenantID
// -> service.GroupApp.Board.GetDeviceTotal -> service 依据 authority 选择“全局统计”或“租户统计”。
// 权限边界：系统管理员可看全局设备总量，非管理员只看本租户设备总量；该边界由 service 内
// common.CheckUserIsAdmin 判定，本层只转发 authority 与 tenantID。
// @Router   /api/v1/board/device/total [get]
func (*BoardApi) HandleDeviceTotal(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	board := service.GroupApp.Board
	// The service derives tenant and owner scope from the full identity; request
	// parameters cannot widen an ordinary user's device total.
	total, err := board.GetDeviceTotal(c, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", total)
}

// HandleDevice 获取设备总量、在线量、离线量概览。
// 调用链：GET /api/v1/board/device -> 读取 claims -> service.GroupApp.Board.GetDevice
// -> service 根据 claims.Authority 判断统计全局还是当前租户，并汇总 total/on/offline。
// 权限边界：系统管理员可读取全局设备概览，非管理员只读取当前租户概览；API 层不提供 tenantID
// 输入参数，避免普通用户主动指定其他租户。
// @Router   /api/v1/board/device [get]
func (*BoardApi) HandleDevice(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	board := service.GroupApp.Board
	data, err := board.GetDevice(c, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleTenant 获取租户总览统计。
// 调用链：GET /api/v1/board/tenant -> 读取 claims -> 先在 API 层判断是否为 SYS_ADMIN
// -> service.UsersService.GetTenant -> 汇总租户管理员数量、昨日新增、本月新增与按月趋势。
// 权限边界：该接口是本文件里少数在 API 入口就做强角色约束的接口，只有系统管理员可访问；
// 普通租户用户即使已登录，也会在进入 service 前直接返回无权限错误。
// @Router   /api/v1/board/tenant [get]
func (*BoardApi) HandleTenant(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	if userClaims.Authority != constant.SYS_ADMIN {
		c.Error(errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to query tenant overview"))
		return
	}

	users := service.UsersService{}
	data, err := users.GetTenant(c)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleTenantUserInfo 获取当前租户关联的用户统计信息。
// 调用链：GET /api/v1/board/tenant/user/info -> 读取 claims.TenantID
// -> service.GroupApp.User.GetTenantInfo(tenantID) 取出租户主信息 -> 以租户邮箱调用
// service.UsersService.GetTenantUserInfo -> 返回该租户维度的用户统计。
// 权限边界：接口不接受外部 tenantID，查询范围固定为 claims.TenantID；当前层未额外要求
// SYS_ADMIN 或 TENANT_ADMIN，能访问该路由的当前租户用户都将共享这一租户级统计视图。
// @Router   /api/v1/board/tenant/user/info [get]
func (*BoardApi) HandleTenantUserInfo(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)
	tenantID := userClaims.TenantID
	// 根据租户ID查询租户信息
	tenantInfo, err := service.GroupApp.User.GetTenantInfo(tenantID)
	if err != nil {
		c.Error(err)
		return
	}
	users := service.UsersService{}
	data, err := users.GetTenantUserInfo(c, tenantInfo.Email)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// HandleTenantDeviceInfo 获取当前租户下的设备统计信息。
// 默认查询范围固定为当前 claims 所属租户；all_tenants=true 仅允许 SYS_ADMIN 显式启用。
// @Param all_tenants query bool false "仅 SYS_ADMIN 可显式汇总全部租户设备"
// @Router   /api/v1/board/tenant/device/info [get]
func (*BoardApi) HandleTenantDeviceInfo(c *gin.Context) {
	var req model.GetBoardDeviceReq
	if !BindAndValidate(c, &req) {
		return
	}
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	board := service.GroupApp.Board
	total, err := board.GetDeviceOverview(c, &req, userClaims)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", total)
}

// HandleUserInfo 查询当前登录用户的个人信息。
// 调用链：GET /api/v1/board/user/info -> 读取 claims.Email
// -> service.UsersService.GetTenantInfo(email) 查询当前用户记录 -> API 层删除 password 字段
// -> 返回脱敏后的个人信息。
// 权限边界：接口只允许查询当前 claims.Email 对应的用户，不接受外部 userID/email 参数；
// 敏感字段脱敏发生在 API 层，因此若 service 返回结构变化，这里的脱敏逻辑也需要同步维护。
// @Router   /api/v1/board/user/info [get]
func (*BoardApi) HandleUserInfo(c *gin.Context) {
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	// 根据租户ID查询租户信息
	users := service.UsersService{}
	data, err := users.GetTenantInfo(c, userClaims.Email)
	if err != nil {
		c.Error(err)
		return
	}
	// 清除敏感信息
	if dataMap, ok := data.(map[string]interface{}); ok {
		delete(dataMap, "password")
	}
	c.Set("data", data)
}

// UpdateUserInfo 更新当前登录用户的个人信息。
// 调用链：POST /api/v1/board/user/update -> BindAndValidate(UsersUpdateReq) -> 读取 claims
// -> service.UsersService.UpdateTenantInfo -> 按当前登录用户身份更新资料。
// 权限边界：更新目标不由请求体自由指定，而由 claims 锚定到当前用户；是否允许修改特定字段、
// 是否涉及跨租户字段写入，依赖 service 层进一步校验。
// @Router   /api/v1/board/user/update [post]
func (*BoardApi) UpdateUserInfo(c *gin.Context) {
	var param model.UsersUpdateReq
	if !BindAndValidate(c, &param) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	users := service.UsersService{}
	err := users.UpdateTenantInfo(c, userClaims, &param)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// UpdateUserInfoPassword 更新当前登录用户密码。
// 调用链：POST /api/v1/board/user/update/password -> BindAndValidate(UsersUpdatePasswordReq)
// -> 读取 claims -> service.UsersService.UpdateTenantInfoPassword -> 完成密码校验与更新。
// 权限边界：只处理当前登录用户自己的密码变更，不提供替他人改密入口；旧密码校验、密码强度
// 与持久化安全性都由 service 层负责。
// @Router   /api/v1/board/user/update/password [post]
func (*BoardApi) UpdateUserInfoPassword(c *gin.Context) {
	var param model.UsersUpdatePasswordReq
	if !BindAndValidate(c, &param) {
		return
	}

	userClaims := c.MustGet("claims").(*utils.UserClaims)

	users := service.UsersService{}
	err := users.UpdateTenantInfoPassword(c, userClaims, &param)
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// GetDeviceTrend 获取设备在线趋势。
// 调用链：GET /api/v1/board/trend -> BindAndValidate(DeviceTrendReq) -> 读取 claims
// -> 若请求未带 tenantID 则回填 claims.TenantID -> 校验跨租户访问权限与时间范围
// -> service.GroupApp.Device.GetDeviceTrend 返回趋势数据。
// 权限边界：普通用户只能查询自己的 claims.TenantID；只有系统管理员才允许查询其他租户趋势。
// 同时接口在 API 层限制 start_time <= end_time 且跨度不超过 30 天，避免无界查询。
// @Router   /api/v1/board/trend [get]
func (*BoardApi) GetDeviceTrend(c *gin.Context) {
	var deviceTrendReq model.DeviceTrendReq
	if !BindAndValidate(c, &deviceTrendReq) {
		return
	}

	// 获取用户claims
	userClaims := c.MustGet("claims").(*utils.UserClaims)

	// 如果请求中没有指定tenantID,则使用当前用户的tenantID
	if deviceTrendReq.TenantID == nil || *deviceTrendReq.TenantID == "" {
		deviceTrendReq.TenantID = &userClaims.TenantID
	}

	// 权限检查 - 只有系统管理员可以查看其他租户的数据
	if *deviceTrendReq.TenantID != userClaims.TenantID && userClaims.Authority != "SYS_ADMIN" {
		c.Error(errcode.New(errcode.CodeNoPermission))
		return
	}

	// 校验时间范围
	if deviceTrendReq.StartTime != nil && deviceTrendReq.EndTime != nil {
		if *deviceTrendReq.StartTime > *deviceTrendReq.EndTime {
			c.Error(errcode.WithVars(errcode.CodeParamError, map[string]interface{}{
				"message": "start_time must be less than or equal to end_time",
			}))
			return
		}
		const maxRangeSeconds = int64(30 * 24 * 3600) // 30天
		if *deviceTrendReq.EndTime-*deviceTrendReq.StartTime > maxRangeSeconds {
			c.Error(errcode.WithVars(errcode.CodeParamError, map[string]interface{}{
				"message": "time range must not exceed 30 days",
			}))
			return
		}
	}

	// 调用service层获取趋势数据
	trend, err := service.GroupApp.Device.GetDeviceTrend(c, userClaims, *deviceTrendReq.TenantID, deviceTrendReq.StartTime, deviceTrendReq.EndTime)
	if err != nil {
		c.Error(err)
		return
	}

	c.Set("data", trend)
}
