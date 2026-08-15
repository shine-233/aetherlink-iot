// 文件用途：定义统一错误对象和常用构造函数。
// 核心逻辑：用 `Error` 保存错误码、可选数据、自定义消息、格式化参数和模板变量。
// 关键注意事项：序列化字段会影响 API 响应结构，新增字段前需确认响应中间件和前端兼容性。
// 重构建议：后续可把构造函数与响应渲染解耦，并为变量替换规则补充集中测试。
// pkg/errcode/error.go
package errcode

import "fmt"

// Error 统一的错误类型
type Error struct {
	Code         int                    `json:"code"`
	Data         interface{}            `json:"data,omitempty"`
	Variables    map[string]interface{} `json:"-"`                 // 存储错误信息中的变量
	Args         []interface{}          `json:"-"`                 // fmt格式化参数
	CustomMsg    string                 `json:"message,omitempty"` // 用于存储自定义消息
	UseCustomMsg bool                   `json:"-"`                 // 内部标记，是否使用自定义消息
}

func (e *Error) Error() string {
	if e.CustomMsg != "" {
		return fmt.Sprintf("Error Code: %d: %s", e.Code, e.CustomMsg)
	}
	return fmt.Sprintf("Error Code: %d", e.Code)
}

// 创建错误
func New(code int) *Error {
	return &Error{
		Code: code,
	}
}

// NewWithMessage 创建带自定义消息的错误, 用于覆盖默认的错误消息
func NewWithMessage(code int, message string) *Error {
	return &Error{
		Code:         code,
		CustomMsg:    message,
		UseCustomMsg: true,
	}
}

// 携带数据创建错误
func WithData(code int, data interface{}) *Error {
	return &Error{
		Code: code,
		Data: data,
	}
}

// Newf 创建带格式化参数的错误
func Newf(code int, args ...interface{}) *Error {
	return &Error{
		Code: code,
		Args: args,
	}
}

// WithVars 创建带变量的错误
func WithVars(code int, vars map[string]interface{}) *Error {
	return &Error{
		Code:      code,
		Variables: vars,
	}
}
