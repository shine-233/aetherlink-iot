// 文件用途：设备凭证（voucher）展示面脱敏工具。
// 核心逻辑：凭证哈希 Phase 2a（references/backend-hardening-plan.md 车道1）把详情/导出等
// 人读面的明文 voucher 收敛为统一掩码形态：长度 >12 时返回「前 10 字符 + …」；
// ≤12 字符时前缀已接近全文、无法安全截断，整体替换为固定占位 "******"。
// 关键注意事项：
// 1. 掩码值仅用于展示与导出，禁止参与任何认证/匹配逻辑——匹配一律走 VoucherStorageHash
//    双模式（dal 层 hash 优先、明文兜底），掩码串永远不应落库或回写。
// 2. 掩码契约跨端识别：前端以「voucher 值以 … 结尾」或响应字段 voucher_masked=true 判定，
//    进入"凭证已脱敏"降级态；自动化 oracle 从创建响应取完整凭证。
// 3. 一次性回显面不受影响：创建设备响应与更新凭证响应仍按产品语义返回完整凭证。
package utils

// maskedVoucherPlaceholder 短凭证（≤12 字符）的整体占位符。
const maskedVoucherPlaceholder = "******"

// MaskVoucher 返回 voucher 的展示掩码：
//   - 入参字节长度 ≤12（含空串）：返回固定占位 "******"；
//   - 其余：返回前 10 字节 + "…"（voucher 为 ASCII JSON，按字节截断即按字符截断）。
func MaskVoucher(voucher string) string {
	if len(voucher) <= 12 {
		return maskedVoucherPlaceholder
	}
	return voucher[:10] + "…"
}
