// 文件用途：为仍依赖 c.Set("data", ...) 的 Gin 路由提供统一响应封装。
// 核心流程：先恢复 panic，再检查是否已写响应，接着处理 Gin 错误，最后把 data 输出为成功包。
// 兼容边界：这是保留中的旧响应链实现，修改响应结构、状态码或本地化消息会影响现有客户端契约。
// 静态审查建议：如果后续要继续收敛响应路径，优先先抽离消息格式化和变量替换 helper，再逐步迁移调用方，
// 这样可以降低双实现并存时的维护成本，也便于把行为差异单独覆盖到测试里。
package response

import (
	"fmt"
	"net/http"
	"strings"

	"aetherlink-iot/backend/pkg/errcode"

	"github.com/gin-gonic/gin"
)

// Response 定义统一的 API 返回结构。
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handler 持有响应链所需的错误码消息管理器。
type Handler struct {
	ErrManager *errcode.ErrorManager
}

// NewHandler 从配置文件加载多语言错误消息并返回处理器。
func NewHandler(configPath string, strConfigPath string) (*Handler, error) {
	errManager := errcode.NewErrorManager(configPath, strConfigPath)
	if err := errManager.LoadMessages(); err != nil {
		return nil, err
	}
	return &Handler{ErrManager: errManager}, nil
}

// Middleware 构造统一响应中间件。
func (h *Handler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. 优先兜住 panic，避免把原始异常直接暴露给客户端。
		defer func() {
			if err := recover(); err != nil {
				sysErr := errcode.NewWithMessage(errcode.CodeSystemError, fmt.Sprint(err))
				h.handleError(c, sysErr)
				c.Abort()
			}
		}()

		c.Next()

		// 2. 如果上游已经写过响应，就保持原样退出。
		if c.Writer.Written() {
			return
		}

		// 3. 优先把 Gin 累积错误转换成统一错误包。
		if len(c.Errors) > 0 {
			h.handleContextError(c, c.Errors.Last().Err)
			return
		}

		// 4. 没有错误时，把 data 字段输出为成功响应。
		if data, exists := c.Get("data"); exists {
			h.responseSuccess(c, data)
		}
	}
}

func (h *Handler) handleContextError(c *gin.Context, err error) {
	switch e := err.(type) {
	case *errcode.Error:
		h.handleError(c, e)
	default:
		sysErr := errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
		h.handleError(c, sysErr)
	}
}

// responseSuccess 使用本地化成功消息写回统一响应。
func (h *Handler) responseSuccess(c *gin.Context, data interface{}) {
	lang := c.GetHeader("Accept-Language")
	c.JSON(http.StatusOK, &Response{
		Code:    errcode.CodeSuccess,
		Message: h.ErrManager.GetMessage(errcode.CodeSuccess, lang),
		Data:    data,
	})
}

// handleError 根据错误码和语言组装响应消息。
func (h *Handler) handleError(c *gin.Context, err *errcode.Error) {
	msg := h.resolveMessage(c, err)

	resp := &Response{
		Code:    err.Code,
		Message: msg,
	}
	if err.Data != nil {
		resp.Data = err.Data
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) resolveMessage(c *gin.Context, err *errcode.Error) string {
	if err.UseCustomMsg {
		return err.CustomMsg
	}

	lang := c.GetHeader("Accept-Language")
	msg := h.ErrManager.GetMessage(err.Code, lang)
	if err.Args != nil {
		msg = fmt.Sprintf(msg, err.Args...)
	}
	if len(err.Variables) > 0 {
		msg = replaceVariables(msg, err.Variables)
	}
	return msg
}

func replaceVariables(msg string, variables map[string]interface{}) string {
	for k, v := range variables {
		msg = strings.ReplaceAll(msg, "${"+k+"}", fmt.Sprint(v))
	}
	return msg
}
