// 文件用途：定义 RDI 设备分享撤销的 HTTP 入参与出参结构。
// 核心逻辑：用 json/validate 标签描述“按 token 撤销”和“按接收人撤销”两种互斥请求形态及其结果计数。
// 关键注意事项：这里只维护传输结构和校验标签，撤销权限和原子性由 service 层负责。
// 重构建议：若后续支持批量撤销，优先扩展本文件的请求结构而不是在 handler 里拼装参数。

package model

// RDIRevokeShareReq 由设备拥有者发起分享撤销，token 与 user_id 必须二选一。
// token 撤销整条分享链接以及通过它接受分享的全部接收人；
// user_id 只撤销该接收人的访问权，分享链接本身仍然有效。
type RDIRevokeShareReq struct {
	Token  string `json:"token" validate:"omitempty,max=64"`
	UserID string `json:"user_id" validate:"omitempty,max=36"`
}

// RDIRevokeShareResponse 汇报本次撤销实际清理掉的 token 与接收人数量。
type RDIRevokeShareResponse struct {
	DeviceID          string `json:"device_id"`
	RevokedTokens     int    `json:"revoked_tokens"`
	RevokedRecipients int    `json:"revoked_recipients"`
	RevokedAt         int64  `json:"revoked_at"`
}
