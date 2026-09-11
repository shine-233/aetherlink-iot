package initialize

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/global"

	"github.com/stretchr/testify/require"
)

// TestMigrationFilesMatchVersionNumber 守住迁移链的三个不变式。
//
// pg_init.go 的迁移循环是 `for i := dataVersionNumber + 1; i <= global.VERSION_NUMBER; i++`，
// 逐号读取 sql/<i>.sql。因此：
//   - VERSION_NUMBER 小于最大文件号 → 那个迁移**永不执行**，而且不会报任何错，
//     只在某天线上缺列时才暴露，届时已无从追责；
//   - 编号有缺口 → 循环读到不存在的文件直接失败，升级中断；
//   - 编号重复 → 同一个文件被两个号位引用，语义漂移。
//
// 这三类都不是"理论风险"：VERSION_NUMBER 落后于迁移文件已实际发生过一次
// （见 docs/validation 中 P0.1 预检门禁拦截的实例，当时由人工发现）。
// 这里把它变成自动门禁，避免再次靠人眼发现。
func TestMigrationFilesMatchVersionNumber(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "无法定位测试文件路径")
	sqlDir := filepath.Join(filepath.Dir(thisFile), "..", "sql")

	entries, err := os.ReadDir(sqlDir)
	require.NoError(t, err, "读取迁移目录失败")
	require.NotEmpty(t, entries, "迁移目录为空")

	seen := make(map[int]string, len(entries))
	maxNumber := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		numberText := strings.TrimSuffix(entry.Name(), ".sql")
		number, convErr := strconv.Atoi(numberText)
		require.NoError(t, convErr, "迁移文件名必须是纯数字编号，实际为 %q", entry.Name())
		require.NotContains(t, seen, number, "迁移号 %d 重复（%s 与 %s）", number, seen[number], entry.Name())
		seen[number] = entry.Name()
		if number > maxNumber {
			maxNumber = number
		}
	}
	require.NotEmpty(t, seen, "未找到任何 .sql 迁移文件")

	// 编号必须自 1 起连续：缺口会让升级循环读不到文件而中断。
	for i := 1; i <= maxNumber; i++ {
		require.Contains(t, seen, i, "迁移号 %d 缺失：升级循环会尝试读取 sql/%d.sql 并失败", i, i)
	}

	require.Equal(t, maxNumber, global.VERSION_NUMBER,
		"VERSION_NUMBER(%d) 必须等于最大迁移文件号(%d)，否则该迁移永远不会被应用",
		global.VERSION_NUMBER, maxNumber)
}
