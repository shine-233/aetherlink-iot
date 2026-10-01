// 文件用途：全新安装的迁移基线（baseline）决策与执行。
// 背景：sql/1..N.sql 已累积 140+ 个增量迁移，全新库要逐个重放。sql/baseline/<B>.sql 是
//
//	由 cmd/migbaseline 从"空库跑完 1..B"导出的等价快照（schema + 种子行 + setval），
//	全新库可一次执行基线后从 B+1 继续走原有增量循环。
//
// 核心规则（任何一条不满足都走原增量路径，行为与引入基线前完全一致）：
//   - sys_version 版本号为 0（从未迁移过）；已有版本的库永远看不到基线；
//   - 开关 AETHERLINK_MIGRATION_BASELINE / viper db.migration.baseline 为 auto（默认 off）；
//   - 存在 sql/baseline/<B>.sql 且 B <= VERSION_NUMBER（取满足条件的最大 B，旧基线仍可用，只是多重放几个增量）；
//   - public 下除 sys_version 外没有任何表（脏库不套基线，避免与既有对象冲突）；
//   - TimescaleDB 的有效决策是"不执行 57.sql"（mode=off，或 auto 且扩展不存在）。
//     基线只在无 TimescaleDB 的环境下生成与验证，hypertable 变体必须走增量路径。
//
// 关键注意事项：基线文件由工具生成，禁止手改；其头部记录 1..B 源文件的 sha256，
//
//	pkg/global 的测试会校验它与当前文件一致（改了旧迁移却没重新生成基线会被拦下）。
package initialize

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// MigrationBaselineDir 基线文件目录（相对 backend/ 工作目录，与 sql/<n>.sql 一致）。
const MigrationBaselineDir = "sql/baseline"

const (
	migrationBaselineOff  = "off"
	migrationBaselineAuto = "auto"
)

// readMigrationBaselineMode 读取基线开关：优先环境变量，其次 viper，空值回落 off；未知取值 fail-fast。
func readMigrationBaselineMode() (string, error) {
	raw := strings.TrimSpace(os.Getenv("AETHERLINK_MIGRATION_BASELINE"))
	if raw == "" {
		raw = strings.TrimSpace(viper.GetString("db.migration.baseline"))
	}
	mode := strings.ToLower(raw)
	switch mode {
	case "":
		return migrationBaselineOff, nil
	case migrationBaselineOff, migrationBaselineAuto:
		return mode, nil
	default:
		return "", fmt.Errorf("非法 AETHERLINK_MIGRATION_BASELINE=%q：仅支持 off|auto", raw)
	}
}

// baselineInputs 是基线决策的全部输入，便于纯函数单测。
type baselineInputs struct {
	DataVersion        int    // sys_version 中的版本号
	Mode               string // off|auto
	TimescaleMode      string // 已归一化的 auto|on|off
	TimescaleInstalled bool   // pg_extension 中是否有 timescaledb
	BaselineNumber     int    // 可用基线编号，0 表示没有
	BusinessTables     int64  // public 下除 sys_version 外的表数
}

// decideBaseline 纯决策：是否用基线替代 1..B 的增量重放。
func decideBaseline(in baselineInputs) bool {
	if in.DataVersion != 0 || in.Mode != migrationBaselineAuto {
		return false
	}
	if in.BaselineNumber <= 0 || in.BusinessTables != 0 {
		return false
	}
	switch in.TimescaleMode {
	case timescaleModeOff:
		return true
	case timescaleModeAuto:
		return !in.TimescaleInstalled
	default: // on：hypertable 变体只走增量路径
		return false
	}
}

// latestBaseline 返回 dir 下编号 <= maxVersion 的最大基线编号与路径；没有则返回 0。
// 目录不存在不是错误（老发行包没有基线）。
func latestBaseline(dir string, maxVersion int) (int, string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, "", nil
		}
		return 0, "", fmt.Errorf("读取基线目录 %s 失败: %w", dir, err)
	}
	best := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".sql") {
			continue
		}
		n, convErr := strconv.Atoi(strings.TrimSuffix(name, ".sql"))
		if convErr != nil || n <= 0 || n > maxVersion {
			continue
		}
		if n > best {
			best = n
		}
	}
	if best == 0 {
		return 0, "", nil
	}
	return best, filepath.Join(dir, fmt.Sprintf("%d.sql", best)), nil
}

// countBusinessTables 统计 public 下除 sys_version 外的表数（含分区/普通表）。
func countBusinessTables(db *gorm.DB) (int64, error) {
	var n int64
	err := db.Raw("SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' AND table_name <> 'sys_version'").Scan(&n).Error
	if err != nil {
		return 0, fmt.Errorf("统计 public 业务表失败: %w", err)
	}
	return n, nil
}

// applyBaseline 在 tx 中按规则执行基线；返回增量循环的起点（已覆盖到的版本号）。
// 不满足条件时返回 dataVersion 本身，调用方照旧从 dataVersion+1 开始。
func applyBaseline(db, tx *gorm.DB, dataVersion, maxVersion int) (int, error) {
	if dataVersion != 0 {
		return dataVersion, nil
	}
	mode, err := readMigrationBaselineMode()
	if err != nil {
		return 0, err
	}
	if mode == migrationBaselineOff {
		return dataVersion, nil
	}
	tsMode, err := normalizeTimescaleMode(readTimescaleMode())
	if err != nil {
		return 0, err
	}
	installed, err := timescaleExtensionInstalled(db)
	if err != nil {
		return 0, err
	}
	number, path, err := latestBaseline(MigrationBaselineDir, maxVersion)
	if err != nil {
		return 0, err
	}
	tables, err := countBusinessTables(db)
	if err != nil {
		return 0, err
	}
	in := baselineInputs{
		DataVersion: dataVersion, Mode: mode, TimescaleMode: tsMode,
		TimescaleInstalled: installed, BaselineNumber: number, BusinessTables: tables,
	}
	if !decideBaseline(in) {
		migrationLogf("不使用迁移基线（mode=%s timescale=%s/%v baseline=%d tables=%d），走增量路径",
			mode, tsMode, installed, number, tables)
		return dataVersion, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("读取基线文件失败 %s: %w", path, err)
	}
	migrationLogf("执行迁移基线：%s（等价于 sql/1..%d.sql）", path, number)
	if err := tx.Exec(string(body)).Error; err != nil {
		return 0, fmt.Errorf("执行基线 %s 失败: %w", path, err)
	}
	return number, nil
}
