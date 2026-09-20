// 文件用途：提供设备凭证（voucher）双模式匹配的候选串展开能力，与
// mqtt-broker/plugin/aetherlink/db.go 的 deviceVoucherLookupCandidates 构成跨服务契约。
// 核心逻辑：凭证以 text 列存储/哈希，而同一份凭证存在两种稳定 JSON 编码（结构体序
// username,password 与字典序 password,username），语义相同但字符串不等；读取侧按
// 「原始串 → 结构体序 → 字典序」依次展开候选，逐个匹配。
// 关键注意事项（跨服务契约）：本函数的输出必须逐字节对齐 broker 侧同名逻辑，包括
// deviceVoucherPayload 的字段顺序与 password 的 omitempty 标签。任一侧变化都会让同一份
// 凭证在 MQTT 认证侧命中、在 backend 配置下发/唯一性预检侧落空（或反之）。契约测试见
// voucher_candidates_test.go 与 mqtt-broker/plugin/aetherlink/db_test.go
// （TestDeviceVoucherLookupCandidatesSupportBothJSONKeyOrders）。

package utils

import "encoding/json"

// deviceVoucherPayload 必须与 mqtt-broker/plugin/aetherlink/hooks_auth.go 的
// mqttVoucherPayload 保持一致：同为 Username/Password 两字段、password 带 omitempty。
// 该标签决定无密码凭证的候选串是 {"username":"x"} 而非 {"username":"x","password":""}。
type deviceVoucherPayload struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
}

// DeviceVoucherLookupCandidates 返回按凭证匹配时应尝试的候选串，首个元素恒为原始 voucher。
//
// 展开只发生在 voucher 是合法 JSON 且 username 非空时，且候选之间语义完全等价
// （同一 username、同一 password），因此不会放宽凭证匹配强度：
//   - 无法解析为 JSON、或缺少 username（如 {"default":"..."}、空串）→ 仅返回原始串，
//     避免为任意结构凭空造出匹配项；
//   - password 为空 → 不产出字典序候选（该形态下两种编码本就相同）。
//
// 调用方应先用全部候选走 voucher_hash 索引路径，全部未命中再用全部候选走明文兜底，
// 以保持「hash 优先、明文兜底」的双模式顺序。
func DeviceVoucherLookupCandidates(voucher string) []string {
	candidates := []string{voucher}

	var payload deviceVoucherPayload
	if err := json.Unmarshal([]byte(voucher), &payload); err != nil || payload.Username == "" {
		return candidates
	}

	addCandidate := func(candidate string) {
		for _, existing := range candidates {
			if existing == candidate {
				return
			}
		}
		candidates = append(candidates, candidate)
	}

	// 结构体序：broker 由 mqttVoucherPayload 构造，backend 由手写字面量/结构体构造。
	if canonical, err := json.Marshal(deviceVoucherPayload{
		Username: payload.Username,
		Password: payload.Password,
	}); err == nil {
		addCandidate(string(canonical))
	}
	// 字典序：Go 对 map[string]string 编码时按 key 字典序输出，更新凭证接口把 JSON 主体
	// 绑成 map 后序列化即为此形态。
	if payload.Password != "" {
		if lexical, err := json.Marshal(map[string]string{
			"username": payload.Username,
			"password": payload.Password,
		}); err == nil {
			addCandidate(string(lexical))
		}
	}
	return candidates
}
