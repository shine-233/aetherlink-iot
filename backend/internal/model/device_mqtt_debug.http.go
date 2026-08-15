// 文件用途：定义设备级 MQTT 调试会话的 HTTP 请求合同。
// 核心逻辑：会话本身固定短时有效，客户端只提交订阅、取消订阅或发布命令以及增量日志游标。
// 关键注意事项：tenant/user/device 作用域由 claims 和设备查询决定，客户端不能自行提交。
package model

type DeviceMQTTDebugCommandReq struct {
	Action  string `json:"action" validate:"required,oneof=subscribe unsubscribe publish"`
	Topic   string `json:"topic" validate:"required,max=512"`
	QoS     byte   `json:"qos" validate:"gte=0,lte=1"`
	Payload string `json:"payload" validate:"omitempty,max=65536"`
}

type DeviceMQTTDebugSnapshotReq struct {
	AfterSequence int64 `json:"after_sequence" form:"after_sequence" validate:"omitempty,gte=0"`
	Limit         int   `json:"limit" form:"limit" validate:"omitempty,gte=1,lte=200"`
}
