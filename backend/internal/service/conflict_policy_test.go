// 文件用途：实体名冲突策略（TB-15）的契约测试。
//
// 核心逻辑：分三组——策略归一化、更名算法、**长度上限边界**。
//
// 关键注意事项：本文件最重要的一组是「长度上限」。PostgreSQL 的 `varchar(N)` 按
// **字符**计数并在超长时报错，而 HTTP 层允许的名称长度恰好等于列宽
// （`device_configs` 是 `max=99` / `varchar(99)`）。因此
// 「一个合法的 99 字符名称 + RENAME 追加 ` (1)`」会**必然超长**——
// 如果更名算法不带上限，RENAME 会在它唯一该起作用的场景下退化成数据库错误，
// 而这恰恰是最不容易在手工测试里撞到、最容易在生产上炸的组合。
package service

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	model "aetherlink-iot/backend/internal/model"
)

func TestNormalizeConflictPolicy(t *testing.T) {
	cases := []struct {
		input    *string
		expected model.ConflictPolicy
	}{
		{nil, model.ConflictPolicyFail},
		{strPtr(""), model.ConflictPolicyFail},
		{strPtr("   "), model.ConflictPolicyFail},
		{strPtr("allow"), model.ConflictPolicyAllow},
		{strPtr("ALLOW"), model.ConflictPolicyAllow},
		{strPtr("fail"), model.ConflictPolicyFail},
		{strPtr("FAIL"), model.ConflictPolicyFail},
		{strPtr("reject"), model.ConflictPolicyFail},
		{strPtr("error"), model.ConflictPolicyFail},
		{strPtr("rename"), model.ConflictPolicyRename},
		{strPtr("RENAME"), model.ConflictPolicyRename},
		{strPtr("auto_rename"), model.ConflictPolicyRename},
		{strPtr("ignore"), model.ConflictPolicyIgnore},
		{strPtr("IGNORE"), model.ConflictPolicyIgnore},
		{strPtr("skip"), model.ConflictPolicyIgnore},
		{strPtr("update"), model.ConflictPolicyUpdate},
		{strPtr("UPDATE"), model.ConflictPolicyUpdate},
		{strPtr("overwrite"), model.ConflictPolicyUpdate},
		{strPtr("merge"), model.ConflictPolicyUpdate},
		{strPtr("unknown_xyz"), model.ConflictPolicyFail},
	}

	for _, c := range cases {
		val := "<nil>"
		if c.input != nil {
			val = *c.input
		}
		got := model.NormalizeConflictPolicy(c.input)
		if got != c.expected {
			t.Errorf("NormalizeConflictPolicy(%q) = %q, want %q", val, got, c.expected)
		}
	}
}

func TestGenerateRenamedName(t *testing.T) {
	t.Run("BaseNameAvailable", func(t *testing.T) {
		name := model.GenerateRenamedName("PressureSensor", model.NameMaxLengthDefault, func(string) bool {
			return false
		})
		if name != "PressureSensor" {
			t.Errorf("expected 'PressureSensor', got %q", name)
		}
	})

	t.Run("SingleCollision", func(t *testing.T) {
		existing := map[string]bool{"PressureSensor": true}
		name := model.GenerateRenamedName("PressureSensor", model.NameMaxLengthDefault, func(cand string) bool {
			return existing[cand]
		})
		if name != "PressureSensor (1)" {
			t.Errorf("expected 'PressureSensor (1)', got %q", name)
		}
	})

	t.Run("MultipleCollisionsSequential", func(t *testing.T) {
		existing := map[string]bool{
			"PressureSensor":     true,
			"PressureSensor (1)": true,
			"PressureSensor (2)": true,
		}
		name := model.GenerateRenamedName("PressureSensor", model.NameMaxLengthDefault, func(cand string) bool {
			return existing[cand]
		})
		if name != "PressureSensor (3)" {
			t.Errorf("expected 'PressureSensor (3)', got %q", name)
		}
	})

	t.Run("CollisionsWithGap", func(t *testing.T) {
		existing := map[string]bool{
			"PressureSensor":     true,
			"PressureSensor (1)": true,
			"PressureSensor (3)": true,
		}
		name := model.GenerateRenamedName("PressureSensor", model.NameMaxLengthDefault, func(cand string) bool {
			return existing[cand]
		})
		if name != "PressureSensor (2)" {
			t.Errorf("expected 'PressureSensor (2)', got %q", name)
		}
	})
}

// TestGenerateRenamedNameRespectsLengthLimit 是本文件的核心：更名结果必须放得进列宽。
func TestGenerateRenamedNameRespectsLengthLimit(t *testing.T) {
	// device_configs 的真实约束：HTTP max=99 / 列 varchar(99)。
	// 这里用 99 个 ASCII 字符的原名，模拟"合法输入 + RENAME"。
	t.Run("满长 ASCII 名追加后缀不得超限", func(t *testing.T) {
		base := strings.Repeat("a", model.NameMaxLengthDeviceConfig)
		name := model.GenerateRenamedName(base, model.NameMaxLengthDeviceConfig, func(cand string) bool {
			return cand == base // 只有原名被占用
		})
		if runeCount(name) > model.NameMaxLengthDeviceConfig {
			t.Fatalf("更名结果 %d 字符，超出上限 %d：%q", runeCount(name), model.NameMaxLengthDeviceConfig, name)
		}
		if !strings.HasSuffix(name, " (1)") {
			t.Fatalf("应保留自增后缀以维持可读性，实际 %q", name)
		}
	})

	// 中文名是最容易被漏掉的一档：varchar(N) 按字符计数，但按字节截断会砍出半个汉字。
	t.Run("满长中文名不得被截成非法 UTF-8", func(t *testing.T) {
		base := strings.Repeat("温", model.NameMaxLengthDeviceConfig)
		if !utf8.ValidString(base) {
			t.Fatal("构造的基名本身应合法")
		}
		name := model.GenerateRenamedName(base, model.NameMaxLengthDeviceConfig, func(cand string) bool {
			return cand == base
		})
		if runeCount(name) > model.NameMaxLengthDeviceConfig {
			t.Fatalf("更名结果 %d 字符，超出上限 %d", runeCount(name), model.NameMaxLengthDeviceConfig)
		}
		if !utf8.ValidString(name) {
			t.Fatalf("更名结果不是合法 UTF-8（按字节截断把汉字砍成了半个）：%q", name)
		}
		if !strings.HasSuffix(name, " (1)") {
			t.Fatalf("应保留自增后缀，实际 %q", name)
		}
	})

	t.Run("连续冲突时每一个候选都不超限", func(t *testing.T) {
		base := strings.Repeat("x", 99)
		existing := map[string]bool{}
		// 预先把前 50 个候选都占上，强制算法走到高位序号。
		for i := 0; i <= 50; i++ {
			if i == 0 {
				existing[base] = true
				continue
			}
			candidate, ok := composeCandidateForTest(base, i, 99)
			if !ok {
				t.Fatalf("测试构造失败于 i=%d", i)
			}
			existing[candidate] = true
		}
		name := model.GenerateRenamedName(base, 99, func(cand string) bool { return existing[cand] })
		if runeCount(name) > 99 {
			t.Fatalf("更名结果 %d 字符，超出上限 99", runeCount(name))
		}
		if !strings.HasSuffix(name, " (51)") {
			t.Fatalf("期望走到 (51)，实际 %q", name)
		}
	})

	t.Run("原名未占用且刚好满长时原样返回", func(t *testing.T) {
		base := strings.Repeat("y", 99)
		name := model.GenerateRenamedName(base, 99, func(string) bool { return false })
		if name != base {
			t.Fatalf("未冲突且不超长时应原样返回，实际 %q", name)
		}
	})

	// 原名本身就超长（历史数据或绕过 HTTP 校验的写入）时也要收敛到上限内，
	// 否则 RENAME 只会把问题从"冲突"换成"超长"。
	t.Run("原名本身超长时收敛到上限内", func(t *testing.T) {
		base := strings.Repeat("z", 300)
		name := model.GenerateRenamedName(base, 99, func(string) bool { return false })
		if runeCount(name) > 99 {
			t.Fatalf("更名结果 %d 字符，超出上限 99", runeCount(name))
		}
	})

	t.Run("上限小于后缀本身时不产出空名", func(t *testing.T) {
		// 退化输入：上限 3 字符，连 " (1)" 都放不下。
		name := model.GenerateRenamedName("abc", 3, func(string) bool { return true })
		if name == "" {
			t.Fatal("不得返回空名")
		}
		if !strings.HasPrefix(name, " ") {
			t.Fatalf("应退回后缀本身，实际 %q", name)
		}
	})

	t.Run("空名与纯空白名有确定行为", func(t *testing.T) {
		for _, input := range []string{"", "   ", "\t"} {
			name := model.GenerateRenamedName(input, model.NameMaxLengthDefault, func(string) bool { return false })
			if name == "" {
				t.Fatalf("输入 %q 不应产出空名", input)
			}
			if runeCount(name) > model.NameMaxLengthDefault {
				t.Fatalf("输入 %q 的更名结果超限：%q", input, name)
			}
		}
	})

	t.Run("nil 判定函数之外的极端冲突可终止", func(t *testing.T) {
		// 所有候选都被占用时必须终止并返回一个非空结果，而不是死循环。
		name := model.GenerateRenamedName("busy", model.NameMaxLengthDefault, func(string) bool { return true })
		if name == "" {
			t.Fatal("全部候选被占用时也应返回非空结果")
		}
	})
}

func runeCount(value string) int { return utf8.RuneCountInString(value) }

// composeCandidateForTest 复刻生产算法里"截断 base + 后缀"的候选构造，
// 仅用于在测试里预先占位。若它与生产实现漂移，上面的序号断言会立刻失败。
func composeCandidateForTest(base string, index, maxLength int) (string, bool) {
	suffix := fmt.Sprintf(" (%d)", index)
	room := maxLength - len([]rune(suffix))
	if room < 1 {
		return "", false
	}
	runes := []rune(base)
	if len(runes) > room {
		runes = runes[:room]
	}
	return string(runes) + suffix, true
}
