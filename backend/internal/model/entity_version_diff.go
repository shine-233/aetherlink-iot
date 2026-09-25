// 文件用途：实体版本差异对比（ROADMAP TB-25）的 HTTP 契约结构。
// 核心逻辑：EntityVersionDiffChange 描述单条路径变更，EntityVersionDiffResult 汇总
// 新增/删除/修改路径列表与逐条变更明细，EntityVersionDiffRsp 附带两侧版本元信息。
// 关键注意事项：路径以点号表达（数组下标作为路径段，如 tags.0），根标量差异记于 "$"；
// 数值统一以 json.Number 字面量参与比较与回显，避免大整数经 float64 丢失精度。
// 重构建议：若后续需要字符级行内 diff，扩展 change 结构而不要改动路径语义。
package model

// EntityVersionDiffChange 单条路径变更；kind ∈ added|removed|modified。
// added 只带 NewValue，removed 只带 OldValue，modified 两者都带（值为 null 时省略键）。
type EntityVersionDiffChange struct {
	Path     string      `json:"path"`
	Kind     string      `json:"kind"`
	OldValue interface{} `json:"old_value,omitempty"`
	NewValue interface{} `json:"new_value,omitempty"`
}

// EntityVersionDiffResult 两份快照的语义差异汇总：
// Added/Removed/Modified 为点号路径列表（按键名与数组下标排序，保证输出确定），
// Changes 为同序的逐条明细，Total 为三类路径总数。
type EntityVersionDiffResult struct {
	Added    []string                  `json:"added"`
	Removed  []string                  `json:"removed"`
	Modified []string                  `json:"modified"`
	Changes  []EntityVersionDiffChange `json:"changes"`
	Total    int                       `json:"total"`
}

// EntityVersionDiffRsp 版本对比响应：两侧版本元信息（含完整快照）+ 语义差异。
// Source 为基准版本（路径参数 id），Target 为对比版本（路径参数 target_id）。
type EntityVersionDiffRsp struct {
	Source *EntityVersion           `json:"source"`
	Target *EntityVersion           `json:"target"`
	Diff   *EntityVersionDiffResult `json:"diff"`
}
