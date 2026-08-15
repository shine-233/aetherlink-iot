// 文件用途：定义 message push http 相关 HTTP 入参、出参和列表查询结构，承接 API 层与模型层的数据契约。
// 核心逻辑：使用 json/form/validate 标签描述请求校验、分页筛选和响应字段，保持 handler 与 service 的传参稳定。
// 关键注意事项：这里只维护传输结构和校验标签，不放入权限、事务或数据库访问等业务逻辑。
// 重构建议：接口字段变化时同步 OpenAPI/前端调用和服务层映射，公共分页或筛选结构可继续抽成复用类型。

package model

type CreateMessagePushReq struct {
	PushId     string `json:"pushId" validate:"required"`
	DeviceType string `json:"deviceType" validate:"required"`
}

type MessagePushMangeLogoutReq struct {
	PushId string `json:"pushId" validate:"required"`
}

type MessagePushConfigRes struct {
	Url string `json:"url"`
}

type MessagePushConfigReq struct {
	Url string `json:"url" validate:"required,max=2048"`
}

type MessagePushSend struct {
	PushClientId string                            `json:"push_clientid"`
	Title        string                            `json:"title"`
	Content      string                            `json:"content"`
	AlarmId      *string                           `json:"alarm_id,omitempty"`
	Category     map[string]string                 `json:"category,omitempty"`
	Options      map[string]map[string]interface{} `json:"options,omitempty"`
}
type MessagePushSendPayload struct {
	AlarmConfigId string `json:"alarm_config_id"`
	TenantId      string `json:"tenant_id"`
}

type MessagePushSendRes struct {
	ErrCode interface{} `json:"errCode"`
	ErrMsg  string      `json:"errMsg"`
	Data    interface{} `json:"data,omitempty"`
}
