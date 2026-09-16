// 文件用途：迁移链完整性契约测试（发布门禁硬化，对应路线图 §1.2 的高风险项）。
//
// 核心逻辑：`backend/sql/` 必须构成 **1..VERSION_NUMBER 的无缺口连续链**，
// 且 VERSION_NUMBER 恰好等于最大编号。
//
// 关键注意事项：
//   - `pkg/global.VERSION_NUMBER` 就是 `initialize/pg_init.go` 迁移循环的上界。
//     最大编号 **小于** 它会去执行不存在的文件（断链）；**大于** 它会漏跑最后几版。
//   - 这条缺陷在本地"文件都在磁盘上"时**完全不可见**——只有干净检出才会暴露。
//     实测教训：2026-09-16 曾出现 `VERSION_NUMBER=106` 而 `104/105/106.sql`
//     全部 untracked 的状态，任何全新 clone 启动迁移都会断链。
//   - 缺口同样致命：1..max 中间少一个文件，意味着那些版本的 schema 永远无法到达，
//     但循环上界仍然推进，最终得到"看起来迁移成功、实际缺表"的库。
//   - 校验逻辑抽成纯函数 `validateMigrationChain`，以便用**合成输入**做负向对照——
//     否则"测试永远是绿的"这件事本身就没人能证伪。
//
// 静态审查建议：若将来迁移改为目录化（如 `sql/108_xxx/`）或多文件编号，
// 请同步改写本测试而不是删除它——它是唯一能在 CI 阶段拦住断链的守卫。
package global

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"testing"
)

// migrationFilePattern 迁移文件名必须严格为 `<十进制数字>.sql`。
var migrationFilePattern = regexp.MustCompile(`^([0-9]+)\.sql$`)

// validateMigrationChain 校验编号序列与 VERSION_NUMBER 的关系。
// numbers 必须是已排序、已去重前的原始集合（内部会再排序）。
// 返回 nil 表示链条自洽。
func validateMigrationChain(numbers []int, versionNumber int) error {
	if len(numbers) == 0 {
		return errors.New("backend/sql 下没有任何 <数字>.sql 迁移文件")
	}
	if versionNumber <= 0 {
		return fmt.Errorf("VERSION_NUMBER 必须为正整数，实际 %d", versionNumber)
	}

	sorted := make([]int, len(numbers))
	copy(sorted, numbers)
	sort.Ints(sorted)

	maxNumber := sorted[len(sorted)-1]
	if maxNumber != versionNumber {
		return fmt.Errorf(
			"迁移上界不一致：backend/sql 最大编号 = %d，但 VERSION_NUMBER = %d。"+
				"最大编号小于上界会去执行不存在的迁移文件（全新库启动即断链）；"+
				"大于上界会静默漏跑最后几版 schema",
			maxNumber, versionNumber)
	}

	for index, number := range sorted {
		if want := index + 1; number != want {
			return fmt.Errorf(
				"迁移链有缺口或重复：第 %d 个文件是 %d.sql，按连续编号应为 %d.sql。"+
					"缺口会让那些版本的 schema 永远无法到达，但迁移循环上界照常推进，"+
					"最终得到缺表的库却不报错",
				index+1, number, want)
		}
	}
	return nil
}

func collectMigrationNumbers(t *testing.T) []int {
	t.Helper()

	entries, err := os.ReadDir(filepath.Join("..", "..", "sql"))
	if err != nil {
		t.Fatalf("读取迁移目录 backend/sql 失败：%v", err)
	}

	numbers := make([]int, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// 目录里允许存在 README.md 等非迁移说明文件；只对 .sql 施加命名契约。
		if filepath.Ext(name) != ".sql" {
			continue
		}
		match := migrationFilePattern.FindStringSubmatch(name)
		if match == nil {
			t.Errorf("迁移文件名必须为 <数字>.sql，实际为 %q", name)
			continue
		}
		value, convErr := strconv.Atoi(match[1])
		if convErr != nil {
			t.Errorf("迁移编号无法解析为十进制整数：%q", name)
			continue
		}
		numbers = append(numbers, value)
	}

	if len(numbers) == 0 {
		t.Fatal("backend/sql 下没有任何 <数字>.sql 迁移文件")
	}
	return numbers
}

// TestMigrationChainIsConsistent 用**真实仓库数据**校验迁移链自洽。
func TestMigrationChainIsConsistent(t *testing.T) {
	if err := validateMigrationChain(collectMigrationNumbers(t), VERSION_NUMBER); err != nil {
		t.Fatalf("%v\n修复方式是把迁移文件与 VERSION_NUMBER 一起提交/一起改，不要只改其中一侧。", err)
	}
}

// TestValidateMigrationChainNegativeControls 负向对照：证明上面那条真实校验不是"永远为真"。
// 没有这组用例，"迁移链契约测试通过"这件事本身无法被信任。
func TestValidateMigrationChainNegativeControls(t *testing.T) {
	cases := []struct {
		name          string
		numbers       []int
		versionNumber int
		wantErr       bool
	}{
		{name: "连续链且上界一致", numbers: []int{1, 2, 3}, versionNumber: 3},
		{name: "乱序输入应被容忍（内部排序）", numbers: []int{3, 1, 2}, versionNumber: 3},
		{
			name:          "最大编号小于 VERSION_NUMBER（断链）",
			numbers:       []int{1, 2, 3},
			versionNumber: 5,
			wantErr:       true,
		},
		{
			name:          "最大编号大于 VERSION_NUMBER（漏跑）",
			numbers:       []int{1, 2, 3},
			versionNumber: 2,
			wantErr:       true,
		},
		{
			name:          "中间缺口",
			numbers:       []int{1, 2, 4},
			versionNumber: 4,
			wantErr:       true,
		},
		{
			name:          "重复编号",
			numbers:       []int{1, 2, 2, 3},
			versionNumber: 3,
			wantErr:       true,
		},
		{
			name:          "空集合",
			numbers:       nil,
			versionNumber: 3,
			wantErr:       true,
		},
		{
			name:          "VERSION_NUMBER 非正",
			numbers:       []int{1},
			versionNumber: 0,
			wantErr:       true,
		},
		{name: "单文件链", numbers: []int{1}, versionNumber: 1},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := validateMigrationChain(testCase.numbers, testCase.versionNumber)
			if testCase.wantErr && err == nil {
				t.Fatal("期望校验失败，但返回了 nil —— 该契约存在假绿风险")
			}
			if !testCase.wantErr && err != nil {
				t.Fatalf("期望校验通过，实际报错：%v", err)
			}
		})
	}
}

// TestMigrationFileNamingContract 迁移文件名必须是 `<数字>.sql`。
// 非 .sql 的说明文件（如 README.md）不受约束，但 .sql 文件命名违规必须被拒。
func TestMigrationFileNamingContract(t *testing.T) {
	valid := []string{"1.sql", "107.sql", "0.sql"}
	for _, name := range valid {
		if migrationFilePattern.FindStringSubmatch(name) == nil {
			t.Fatalf("%q 应被接受为合法迁移文件名", name)
		}
	}

	invalid := []string{"108_v2.sql", "v108.sql", "108.SQL", "108.sql.bak", "README.md", "abc.sql"}
	for _, name := range invalid {
		if migrationFilePattern.FindStringSubmatch(name) != nil {
			t.Fatalf("%q 不应被接受为合法迁移文件名", name)
		}
	}
}
