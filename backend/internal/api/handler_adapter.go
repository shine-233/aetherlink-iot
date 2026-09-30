// 文件用途：提供 API 层的泛型 Handler 适配器，收敛“绑定 -> 校验 -> 取 claims -> 调 service -> 写响应”的样板流程。
// 核心逻辑：适配器只复用既有 BindAndValidate / c.Error / c.Set("data", ...) 出口，响应信封仍由 middleware/response 统一渲染，
// 因此迁移前后的 JSON 输出逐字节一致（见 handler_adapter_test.go 的新旧对比测试）。
// 行为差异（有意为之）：缺失 claims 时不再因 MustGet panic 被兜底成 CodeSystemError，而是返回标准 CodeUnauthorized。
// 使用约束：流式 / SSE / WebSocket / 文件下载等需要直接写 c.Writer 的 Handler 不适用，保持手写。
package api

import (
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
)

// claimsContextKey 是鉴权中间件写入用户身份的上下文键。
const claimsContextKey = "claims"

// RequireClaims 读取鉴权中间件注入的 claims；缺失或类型不符时写入 CodeUnauthorized 并返回 false。
// 用于替代 c.MustGet("claims").(*utils.UserClaims)，避免路由误配时 panic。
func RequireClaims(c *gin.Context) (*utils.UserClaims, bool) {
	if v, ok := c.Get(claimsContextKey); ok {
		if claims, ok := v.(*utils.UserClaims); ok && claims != nil {
			return claims, true
		}
	}
	c.Error(errcode.New(errcode.CodeUnauthorized))
	return nil, false
}

// bindQueryAndValidate 强制按 query 绑定（不区分 HTTP 方法）并校验，失败时写入参数错误。
func bindQueryAndValidate(c *gin.Context, obj interface{}) bool {
	if err := c.ShouldBindQuery(obj); err != nil {
		reportParamError(c, err)
		return false
	}
	if err := ValidateStructLang(obj, c.GetHeader("Accept-Language")); err != nil {
		reportParamError(c, err)
		return false
	}
	return true
}

// respond 统一出口：err 非空时交给响应中间件渲染错误，否则把 data 写入上下文。
func respond(c *gin.Context, data interface{}, err error) {
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", data)
}

// respondAction 用于无返回数据的写操作：成功时 data 为 nil（响应中省略 data 字段）。
func respondAction(c *gin.Context, err error) {
	if err != nil {
		c.Error(err)
		return
	}
	c.Set("data", nil)
}

// bindBodyLegacyParamError 按 JSON 绑定请求体，失败时返回迁移前手写时代的参数错误包络：
// errcode.WithData(CodeParamError, {"error": err.Error()})——响应带 data 字段、message 取错误码默认文案。
// 注意它与 BindAndValidate -> reportParamError 的 errcode.NewWithMessage(err.Error()) 形态逐字节不同，
// 两者不可混用；仅供为保持迁移前后 JSON 一致而保留旧参数错误出口的入口复用（如 role.go 的 Assign 系列闭包）。
func bindBodyLegacyParamError[Req any](c *gin.Context, req *Req) error {
	if err := c.ShouldBindJSON(req); err != nil {
		return errcode.WithData(errcode.CodeParamError, map[string]interface{}{"error": err.Error()})
	}
	return nil
}

// Handle 按 HTTP 方法绑定请求体（GET 走 query，其余走 JSON）、校验、取 claims 后调用 fn，并写入响应数据。
func Handle[Req, Res any](c *gin.Context, fn func(req *Req, claims *utils.UserClaims) (Res, error)) {
	var req Req
	if !BindAndValidate(c, &req) {
		return
	}
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	res, err := fn(&req, claims)
	respond(c, res, err)
}

// HandleAction 同 Handle，但 fn 只返回 error，成功响应不携带 data。
func HandleAction[Req any](c *gin.Context, fn func(req *Req, claims *utils.UserClaims) error) {
	var req Req
	if !BindAndValidate(c, &req) {
		return
	}
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	respondAction(c, fn(&req, claims))
}

// HandleQuery 强制从 query string 绑定（适用于 DELETE 等带 query 参数的非 GET 接口）。
func HandleQuery[Req, Res any](c *gin.Context, fn func(req *Req, claims *utils.UserClaims) (Res, error)) {
	var req Req
	if !bindQueryAndValidate(c, &req) {
		return
	}
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	res, err := fn(&req, claims)
	respond(c, res, err)
}

// HandleNoBody 适用于不绑定请求体、只依赖 claims（以及闭包内自行读取的 path/query）的接口。
func HandleNoBody[Res any](c *gin.Context, fn func(claims *utils.UserClaims) (Res, error)) {
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	res, err := fn(claims)
	respond(c, res, err)
}

// HandleNoBodyAction 同 HandleNoBody，但成功响应不携带 data。
func HandleNoBodyAction(c *gin.Context, fn func(claims *utils.UserClaims) error) {
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	respondAction(c, fn(claims))
}

// HandlePath 读取单个路径参数后调用 fn。
func HandlePath[Res any](c *gin.Context, param string, fn func(value string, claims *utils.UserClaims) (Res, error)) {
	value := c.Param(param)
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	res, err := fn(value, claims)
	respond(c, res, err)
}

// HandlePathAction 同 HandlePath，但成功响应不携带 data。
func HandlePathAction(c *gin.Context, param string, fn func(value string, claims *utils.UserClaims) error) {
	value := c.Param(param)
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	respondAction(c, fn(value, claims))
}

// HandlePathBody 组合路径参数与请求体：常见于 /resource/:id 的 PUT/PATCH 接口。
// 绑定发生在 claims 读取之前，与迁移前“先 BindAndValidate 再取 claims”的顺序一致。
func HandlePathBody[Req, Res any](c *gin.Context, param string, fn func(value string, req *Req, claims *utils.UserClaims) (Res, error)) {
	var req Req
	if !BindAndValidate(c, &req) {
		return
	}
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	res, err := fn(c.Param(param), &req, claims)
	respond(c, res, err)
}

// HandlePathBodyAction 同 HandlePathBody，但成功响应不携带 data。
func HandlePathBodyAction[Req any](c *gin.Context, param string, fn func(value string, req *Req, claims *utils.UserClaims) error) {
	var req Req
	if !BindAndValidate(c, &req) {
		return
	}
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	respondAction(c, fn(c.Param(param), &req, claims))
}

// HandlePathBodyOptional 同 HandlePathBody，但请求体可选：绑定失败（空 body、非 JSON）不拒绝，
// 请求结构体按零值传给 fn，仅 RequireClaims 缺失时才短路。
// 适用于“心跳到达本身即有效”“请求体字段全带缺省值”一类接口（如 edge_node.go 的 Heartbeat/IssueCertificate）。
func HandlePathBodyOptional[Req, Res any](c *gin.Context, param string, fn func(value string, req *Req, claims *utils.UserClaims) (Res, error)) {
	var req Req
	_ = c.ShouldBindJSON(&req)
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	res, err := fn(c.Param(param), &req, claims)
	respond(c, res, err)
}

// HandlePathQuery 组合路径参数与 query string：常见于带 :id 的 GET 明细接口。
func HandlePathQuery[Req, Res any](c *gin.Context, param string, fn func(value string, req *Req, claims *utils.UserClaims) (Res, error)) {
	var req Req
	if !bindQueryAndValidate(c, &req) {
		return
	}
	claims, ok := RequireClaims(c)
	if !ok {
		return
	}
	res, err := fn(c.Param(param), &req, claims)
	respond(c, res, err)
}

// HandlePublicNoBody 同 HandleNoBody，但不要求 claims。
func HandlePublicNoBody[Res any](c *gin.Context, fn func() (Res, error)) {
	res, err := fn()
	respond(c, res, err)
}

// HandlePublicQuery 从 query string 绑定且不要求 claims。
func HandlePublicQuery[Req, Res any](c *gin.Context, fn func(req *Req) (Res, error)) {
	var req Req
	if !bindQueryAndValidate(c, &req) {
		return
	}
	res, err := fn(&req)
	respond(c, res, err)
}

// HandlePublic 同 Handle，但不要求 claims（用于匿名或自带鉴权的接口）。
func HandlePublic[Req, Res any](c *gin.Context, fn func(req *Req) (Res, error)) {
	var req Req
	if !BindAndValidate(c, &req) {
		return
	}
	res, err := fn(&req)
	respond(c, res, err)
}

// HandlePublicAction 同 HandlePublic，但成功响应不携带 data。
func HandlePublicAction[Req any](c *gin.Context, fn func(req *Req) error) {
	var req Req
	if !BindAndValidate(c, &req) {
		return
	}
	respondAction(c, fn(&req))
}
