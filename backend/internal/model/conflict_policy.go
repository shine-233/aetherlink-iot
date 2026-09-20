// 文件用途：定义实体名称冲突解决策略（对标 ThingsBoard 4.3.0 #14118）常量、归一化与更名算法。
// 核心逻辑：支持 fail（拒绝）、rename（自增更名）、ignore（忽略并返回既有实体）、update（更新现有实体）以及 allow（默认放行）。

package model

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type ConflictPolicy string

const (
	ConflictPolicyAllow  ConflictPolicy = "allow"  // 显式放行同名（**不是默认值**，默认见 NormalizeConflictPolicy）
	ConflictPolicyFail   ConflictPolicy = "fail"   // 拒绝创建并报错 400 (CodeParamError)
	ConflictPolicyRename ConflictPolicy = "rename" // 自动更名，追加 " (1)", " (2)"...
	ConflictPolicyIgnore ConflictPolicy = "ignore" // 跳过创建，返回现有实体
	ConflictPolicyUpdate ConflictPolicy = "update" // 就地更新现有实体并返回
)

// NormalizeConflictPolicy 归一化策略输入，统一转为小写并去空格；若为空或缺省回落为 FAIL（对标 ThingsBoard #14118 默认策略）。
func NormalizeConflictPolicy(policy *string) ConflictPolicy {
	if policy == nil {
		return ConflictPolicyFail
	}
	p := strings.ToLower(strings.TrimSpace(*policy))
	switch p {
	case "fail", "reject", "error", "":
		return ConflictPolicyFail
	case "rename", "auto_rename":
		return ConflictPolicyRename
	case "ignore", "skip":
		return ConflictPolicyIgnore
	case "update", "overwrite", "merge":
		return ConflictPolicyUpdate
	case "allow":
		return ConflictPolicyAllow
	default:
		return ConflictPolicyFail
	}
}

// 各实体名称列的长度上限（**字符数**，不是字节数）。
//
// 必须与两处保持一致，否则 RENAME 会在边界上退化成数据库错误：
//  1. `sql/1.sql` 的列定义（PostgreSQL 的 varchar(N) 按字符计数）；
//  2. HTTP 层的 `validate:"max=N"`。
//
// device_configs 是最紧的一档（99），也是最容易在真实使用中撞上的一档。
const (
	NameMaxLengthDeviceConfig = 99
	NameMaxLengthDefault      = 255
)

// GenerateRenamedName 根据已存在名称的判定函数，生成下一个可用的无冲突名称。
//
// 规则：若 baseName 未被占用且不超长，直接返回；否则自递增尝试
// `baseName (1)`、`baseName (2)`…，**并保证结果不超过 maxLength 个字符**。
//
// 为什么要带长度上限：PostgreSQL 的 `varchar(N)` 按字符计数，超长时直接报错
// （`value too long for type character varying(N)`）。而 HTTP 层允许的名称长度
// 恰好等于列宽（device_configs 是 `max=99` / `varchar(99)`），因此
// 「一个合法的 99 字符名称 + RENAME 追加 ` (1)`」会**必然超长**——
// RENAME 会在它唯一该起作用的场景下退化成一次 500 级数据库错误，
// 与「RENAME 保证租户内唯一并成功创建」的语义直接冲突。
//
// 处理方式是**截断 base 让 `base + 后缀` 恰好放得下**，而不是报错或放任超长。
// 截断按 **rune** 计数：中文名在 `varchar(N)` 里算 N 个字符但占 3N 字节，
// 按字节截断会把一个汉字砍成半个，产出非法 UTF-8。
func GenerateRenamedName(baseName string, maxLength int, nameExists func(candidate string) bool) string {
	baseName = strings.TrimSpace(baseName)
	if maxLength <= 0 {
		maxLength = NameMaxLengthDefault
	}
	if baseName == "" {
		baseName = "unnamed"
	}
	if !nameExists(baseName) && utf8.RuneCountInString(baseName) <= maxLength {
		return baseName
	}

	// 先按上限截断一次，后续每次追加后缀都基于截断后的 base，
	// 保证任何一次候选都不会超过 maxLength。
	trimmedBase := truncateRunes(baseName, maxLength)
	for i := 1; i <= 10000; i++ {
		candidate, ok := composeRenamed(trimmedBase, i, maxLength)
		if !ok {
			// 上限连一个字符都放不下（maxLength 小于后缀本身），只能退回后缀。
			return fmt.Sprintf(" (%d)", i)
		}
		if !nameExists(candidate) {
			return candidate
		}
	}
	// 极端兜底：候选被穷尽时用纳秒时间戳，仍受长度上限约束。
	if candidate, ok := composeRenamed(trimmedBase, int(time.Now().UnixNano()%1e9), maxLength); ok {
		return candidate
	}
	return fmt.Sprintf(" (%d)", time.Now().UnixNano())
}

// composeRenamed 把 base 与 ` (index)` 拼成不超过 maxLength 个字符的名称。
// 第二个返回值表示「上限是否连一个字符都放不下」。
func composeRenamed(base string, index, maxLength int) (string, bool) {
	suffix := fmt.Sprintf(" (%d)", index)
	room := maxLength - utf8.RuneCountInString(suffix)
	if room < 1 {
		return "", false
	}
	return truncateRunes(base, room) + suffix, true
}

// truncateRunes 按**字符**（rune）截断，绝不产生非法 UTF-8。
func truncateRunes(value string, maxLength int) string {
	if maxLength <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= maxLength {
		return value
	}
	return string(runes[:maxLength])
}
