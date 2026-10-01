// 文件用途：全新安装的迁移基线（baseline）决策与执行。
// 背景：sql/1..N.sql 已累积 140+ 个增量迁移，全新库要逐个重放。sql/baseline/<B>.sql 是
//
//	由 cmd/migbaseline 从"空库跑完 1..B"导出的等价快照（schema + 种子行 + setval），
//	全新库可一次执行基线后从 B+1 继续走原有增量循环。
//
// 核心规则（任何一条不满足都走原增量路径，行为与引入基线前完全一致）：
//   - sys_version 版本号为 0（从未迁移过）；已有版本的库永远看不到基线；
//   - 开关 AETHERLINK_MIGRATION_BASELINE / viper db.migration.baseline 为 auto（默认 auto，off 强制增量）；
//   - 基线头部 postgres-major <= 服务器主版本，且 source-sha256 与当前 sql/1..B.sql 一致；
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
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// MigrationBaselineDir 基线文件目录（相对 backend/ 工作目录）。仅 cmd/migbaseline 演练时改指临时目录。
var MigrationBaselineDir = "sql/baseline"

// migrationSQLDir 增量迁移目录（与 applyIncremental 的 sql/<n>.sql 一致）。
const migrationSQLDir = "sql"

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
		// 默认 auto：2026-10-01 经 Go 运行器路径实测基线与 1..143 逐行等价（cmd/migbaseline -verify）。
		return migrationBaselineAuto, nil
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
	SourceMatches      bool   // 基线头部 source-sha256 与当前 sql/1..B.sql 一致
	BaselineMajor      int    // 生成基线的 PostgreSQL 主版本（头部 postgres-major）
	ServerMajor        int    // 当前服务器主版本
}

// decideBaseline 纯决策：是否用基线替代 1..B 的增量重放。
// 高版本 pg_dump 的输出不保证能在低版本服务器上装载，故要求 ServerMajor >= BaselineMajor。
func decideBaseline(in baselineInputs) bool {
	if in.DataVersion != 0 || in.Mode != migrationBaselineAuto {
		return false
	}
	if in.BaselineNumber <= 0 || in.BusinessTables != 0 || !in.SourceMatches {
		return false
	}
	if in.BaselineMajor <= 0 || in.ServerMajor < in.BaselineMajor {
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

// 基线文件头部的机器可读字段（cmd/migbaseline 生成）。
const (
	BaselineHeaderRange = "-- source-range: "
	BaselineHeaderSHA   = "-- source-sha256: "
	BaselineHeaderMajor = "-- postgres-major: "
)

// BaselineSourceSHA256 计算 sqlDir/1..n.sql 依次拼接后的 sha256（先把 CRLF 归一为 LF，
// 避免检出换行差异造成误报）。生成器写入头部，运行时与测试用它判断基线是否过期。
func BaselineSourceSHA256(sqlDir string, n int) (string, error) {
	h := sha256.New()
	for i := 1; i <= n; i++ {
		b, err := os.ReadFile(filepath.Join(sqlDir, fmt.Sprintf("%d.sql", i)))
		if err != nil {
			return "", fmt.Errorf("读取 %d.sql 失败: %w", i, err)
		}
		h.Write(bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// readBaselineHeader 读取基线文件头部（前 50 行内）`-- key: value` 形式的字段。
func readBaselineHeader(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fields := map[string]string{}
	sc := bufio.NewScanner(f)
	for line := 0; line < 50 && sc.Scan(); line++ {
		if rest, ok := strings.CutPrefix(sc.Text(), "-- "); ok {
			if k, v, found := strings.Cut(rest, ": "); found && !strings.Contains(k, " ") {
				fields[k] = strings.TrimSpace(v)
			}
		}
	}
	return fields, sc.Err()
}

// ReadBaselineHeaderSHA 读取基线头部记录的 source-sha256。
func ReadBaselineHeaderSHA(path string) (string, error) {
	fields, err := readBaselineHeader(path)
	if err != nil {
		return "", err
	}
	if v := fields[strings.TrimSuffix(strings.TrimPrefix(BaselineHeaderSHA, "-- "), ": ")]; v != "" {
		return v, nil
	}
	return "", fmt.Errorf("%s 头部缺少 source-sha256", path)
}

// readBaselineMajor 读取基线头部记录的 postgres-major；缺失返回 0（视为不可用）。
func readBaselineMajor(path string) int {
	fields, err := readBaselineHeader(path)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(fields[strings.TrimSuffix(strings.TrimPrefix(BaselineHeaderMajor, "-- "), ": ")])
	return n
}

// serverMajorVersion 查询服务器主版本（server_version_num / 10000）。
func serverMajorVersion(db *gorm.DB) (int, error) {
	var num int
	if err := db.Raw("SELECT current_setting('server_version_num')::int").Scan(&num).Error; err != nil {
		return 0, fmt.Errorf("查询 server_version_num 失败: %w", err)
	}
	return num / 10000, nil
}

// BaselinePlan 是一次基线决策的结果：Use=false 时 Number/Path 仅供日志参考。
type BaselinePlan struct {
	Use    bool
	Number int
	Path   string
	Inputs baselineInputs
}

// PlanBaseline 汇总配置、DB 探测与基线文件，给出是否使用基线（不执行任何写操作）。
// cmd/migbaseline -verify 用它断言"确实走了基线路径"。
func PlanBaseline(db *gorm.DB, dataVersion, maxVersion int) (BaselinePlan, error) {
	plan := BaselinePlan{}
	mode, err := readMigrationBaselineMode()
	if err != nil {
		return plan, err
	}
	in := baselineInputs{DataVersion: dataVersion, Mode: mode}
	if dataVersion != 0 || mode == migrationBaselineOff {
		plan.Inputs = in
		return plan, nil
	}
	if in.TimescaleMode, err = normalizeTimescaleMode(readTimescaleMode()); err != nil {
		return plan, err
	}
	if in.TimescaleInstalled, err = timescaleExtensionInstalled(db); err != nil {
		return plan, err
	}
	if plan.Number, plan.Path, err = latestBaseline(MigrationBaselineDir, maxVersion); err != nil {
		return plan, err
	}
	in.BaselineNumber = plan.Number
	if plan.Number > 0 {
		want, shaErr := BaselineSourceSHA256(migrationSQLDir, plan.Number)
		got, hdrErr := ReadBaselineHeaderSHA(plan.Path)
		in.SourceMatches = shaErr == nil && hdrErr == nil && want == got
		if !in.SourceMatches {
			migrationLogf("警告：基线 %s 与当前 sql/1..%d.sql 不一致（已过期，需重新生成），改走增量路径", plan.Path, plan.Number)
		}
		in.BaselineMajor = readBaselineMajor(plan.Path)
		if in.ServerMajor, err = serverMajorVersion(db); err != nil {
			return plan, err
		}
	}
	if in.BusinessTables, err = countBusinessTables(db); err != nil {
		return plan, err
	}
	plan.Inputs = in
	plan.Use = decideBaseline(in)
	return plan, nil
}

// applyBaseline 在 tx 中按规则执行基线；返回增量循环的起点（已覆盖到的版本号）。
// 不满足条件时返回 dataVersion 本身，调用方照旧从 dataVersion+1 开始。
func applyBaseline(db, tx *gorm.DB, dataVersion, maxVersion int) (int, error) {
	plan, err := PlanBaseline(db, dataVersion, maxVersion)
	if err != nil {
		return 0, err
	}
	if !plan.Use {
		if plan.Inputs.Mode == migrationBaselineAuto && dataVersion == 0 {
			migrationLogf("不使用迁移基线（%+v），走增量路径", plan.Inputs)
		}
		return dataVersion, nil
	}
	body, err := os.ReadFile(plan.Path)
	if err != nil {
		return 0, fmt.Errorf("读取基线文件失败 %s: %w", plan.Path, err)
	}
	migrationLogf("执行迁移基线：%s（等价于 sql/1..%d.sql）", plan.Path, plan.Number)
	if err := tx.Exec(string(body)).Error; err != nil {
		return 0, fmt.Errorf("执行基线 %s 失败: %w", plan.Path, err)
	}
	return plan.Number, nil
}
