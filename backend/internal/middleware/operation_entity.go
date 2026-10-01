// 文件用途：TB-10 实体级审计的纯函数解析器——从 HTTP 方法与请求路径推导审计动作与实体定位。
// 核心逻辑：operationActionForMethod 把 HTTP 方法映射为 create/update/delete/read/other；
// operationEntityForPath 按 /api/v1/<entity>[/<id>] 形态拆出实体类型与实体 ID，不依赖 gin 上下文。
// 关键注意事项：解析输入必须是已脱敏路径（saveOperationLog 先过 safeOperationLogPath），
// 避免 rdi/share-tokens 等路径段泄漏进 entity_id；第二段仅在 UUID 形态时才认作实体 ID。
// 重构建议：若后续路由出现非 UUID 实体主键，应改为路由注册时显式声明实体语义，而非放宽本解析器。
package middleware

import (
	"encoding/json"
	"net/http"
	"strings"
)

const (
	operationLogAPIPrefix = "/api/v1/"
	// 与 127.sql 列宽一致：entity_type VARCHAR(64)、entity_id VARCHAR(36)。
	operationLogEntityTypeMaxLen = 64
	operationLogEntityIDMaxLen   = 36
)

// operationActionForMethod 把 HTTP 方法映射为实体级审计动作（TB-10）。
// 纯函数：大小写不敏感、去空白，未知方法折叠为 "other"，永不返回空串。
func operationActionForMethod(method string) string {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case http.MethodPost:
		return "create"
	case http.MethodPut, http.MethodPatch:
		return "update"
	case http.MethodDelete:
		return "delete"
	case http.MethodGet, http.MethodHead:
		return "read"
	default:
		return "other"
	}
}

// operationEntityForPath 从请求路径解析实体类型与实体 ID（TB-10）。
// 规则（参照既有路由形态 /api/v1/<entity>[/<id>]）：
//   - 非该前缀或首段为空 → 两个空串（落库时存 NULL）；
//   - 实体类型取首段原文并截断到 64 字符；
//   - 实体 ID 取第二段，且仅当其为 UUID 形态（本平台实体主键均为 varchar(36) UUID）
//     时才认作实体 ID——动词形第二段（如 /device/update/voucher 的 "update"）不会误记。
func operationEntityForPath(path string) (entityType, entityID string) {
	if !strings.HasPrefix(path, operationLogAPIPrefix) {
		return "", ""
	}
	segments := strings.Split(strings.TrimPrefix(path, operationLogAPIPrefix), "/")
	if len(segments) == 0 || segments[0] == "" {
		return "", ""
	}
	entityType = truncateOperationLogEntity(segments[0], operationLogEntityTypeMaxLen)
	if len(segments) >= 2 && isUUIDShape(segments[1]) {
		entityID = segments[1]
	}
	return entityType, entityID
}

// truncateOperationLogEntity 按字节截断到上限，避免超长段写库报错。
// 实体类型段由路由前缀构成（ASCII 为主），按字节截断与按字符截断等价。
func truncateOperationLogEntity(segment string, maxLen int) string {
	if len(segment) <= maxLen {
		return segment
	}
	return segment[:maxLen]
}

// isUUIDShape 判断 s 是否为 8-4-4-4-12 十六进制 UUID 形态（含连字符）。
// 纯手写校验而非引入 uuid 库解析：本判定只关心形态，且需对空串快速返回 false。
func isUUIDShape(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, r := range s {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !isHexDigit(r) {
				return false
			}
		}
	}
	return true
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// operationEntityIDFromResponse 从响应 JSON 提取 data.id（仅认 UUID 形态），
// 用于 POST 集合级创建（新实体 ID 在响应体而非路径）的实体定位。
func operationEntityIDFromResponse(respBody string) string {
	var payload struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal([]byte(respBody), &payload); err != nil {
		return ""
	}
	var data struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(payload.Data, &data); err != nil {
		return ""
	}
	if !isUUIDShape(data.ID) {
		return ""
	}
	return data.ID
}

// operationRequestEntityID 提取请求体里的非空 id 字段（POST 带 id 在语义上是更新）。
func operationRequestEntityID(reqBody string) string {
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(reqBody), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.ID)
}

// resolveOperationActionAndEntity 汇总实体级审计的动作与实体定位：
// 1) 动作默认按 HTTP 方法映射；2) 实体默认从路径取（entity_type=第二段、entity_id=UUID 形态第三段）；
// 3) POST 细分：路径无实体 ID 且请求体带 UUID 形态 id → action=update 且实体取请求体；
//    路径无实体 ID 且请求体无 id → action=create，实体 ID 从响应体 data.id 提取。
// 脱敏约定不受影响：requestMsg/responseMsg 已经过 isSensitiveLogKey 脱敏，id 不在敏感键内。
func resolveOperationActionAndEntity(method, path, reqBody, respBody string) (action, entityType, entityID string) {
	action = operationActionForMethod(method)
	entityType, entityID = operationEntityForPath(path)
	if method != "POST" || entityType == "" {
		return action, entityType, entityID
	}
	if entityID != "" {
		return action, entityType, entityID
	}
	if bodyID := operationRequestEntityID(reqBody); bodyID != "" && isUUIDShape(bodyID) {
		return "update", entityType, bodyID
	}
	if respID := operationEntityIDFromResponse(respBody); respID != "" {
		return action, entityType, respID
	}
	return action, entityType, entityID
}
