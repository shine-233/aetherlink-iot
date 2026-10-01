// 文件用途：生成并验证迁移基线 sql/baseline/<N>.sql（全新安装用，等价于 sql/1..N.sql）。
//
// 生成（默认）：在 -dsn-admin 所指集群上新建临时库 A，以 TimescaleDB=off 调用项目自身的
// initialize.CheckVersion 跑完 1..N，pg_dump 全量（schema + 种子行 + setval）并后处理后写入
// sql/baseline/<N>.sql，头部记录生成命令、PG 版本、源区间与 1..N 拼接 sha256。
//
// 验证（-verify）：新建临时库 B，AETHERLINK_MIGRATION_BASELINE=auto 下先断言 PlanBaseline 选中基线，
// 再走同一个 CheckVersion（即生产 Go 路径，tx.Exec 执行基线 + 增量续跑），然后 pg_dump A/B，
// 归一化 varchar IN(...) 的反解析差异后逐行比对 schema 与数据。
//
// -at K（K < VERSION_NUMBER）：升级演练——生成 K 号基线到临时目录，B 走"基线 K + 增量 K+1..N"，
// 与 A（纯增量 1..N）比对，证明基线落后于程序版本时仍正确续跑。
//
// 关键注意事项：工作目录必须是 backend/；只支持无 TimescaleDB 变体（有扩展的安装走增量路径）。
// 退出码：0=一致；1=有差异或迁移失败；2=环境不满足（不是"通过"）。
//
// 用法：
//
//	cd backend
//	go run ./cmd/migbaseline -dsn-admin "host=127.0.0.1 port=55433 user=postgres dbname=postgres sslmode=disable" -verify
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type exitErr struct {
	code int
	msg  string
}

func fail(code int, format string, args ...any) { panic(exitErr{code, fmt.Sprintf(format, args...)}) }

func main() {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(exitErr); ok {
				fmt.Fprintln(os.Stderr, e.msg)
				os.Exit(e.code)
			}
			panic(r)
		}
	}()
	adminDSN := flag.String("dsn-admin", "", "可建库的管理连接串（keyword=value 形式，必填）")
	verify := flag.Bool("verify", false, "生成后用 Go 运行器把基线装进新库并与增量库逐行比对")
	at := flag.Int("at", 0, "基线编号（默认 VERSION_NUMBER）；小于 VERSION_NUMBER 时为升级演练，基线写到临时目录")
	keep := flag.Bool("keep", false, "保留临时库与 dump 便于排查")
	workDir := flag.String("work", filepath.Join(os.TempDir(), "migbaseline"), "dump 与演练基线的工作目录")
	flag.Parse()

	if *adminDSN == "" {
		fail(2, "必须提供 -dsn-admin")
	}
	if _, err := os.Stat("sql/1.sql"); err != nil {
		fail(2, "工作目录必须是 backend/（找不到 sql/1.sql）")
	}
	full := global.VERSION_NUMBER
	n := *at
	if n == 0 {
		n = full
	}
	if n < initialize.TimescaleSQLFileNumber || n > full {
		fail(2, "-at 必须在 [%d, %d] 之间", initialize.TimescaleSQLFileNumber, full)
	}
	pgDump := findPGDump()
	if err := os.MkdirAll(*workDir, 0o755); err != nil {
		fail(2, "创建工作目录失败：%v", err)
	}
	admin := open(*adminDSN)
	var pgVersion string
	if err := admin.Raw("SHOW server_version").Scan(&pgVersion).Error; err != nil {
		fail(2, "连接管理库失败（环境不满足）：%v", err)
	}
	// 基线只代表无 TimescaleDB 变体；生成链与比对链都显式 off。
	os.Setenv("AETHERLINK_TIMESCALE_MODE", "off")

	stamp := time.Now().Format("20060102150405")
	dbA, dbB := "migbl_a_"+stamp, "migbl_b_"+stamp
	created := []string{}
	defer func() {
		if *keep {
			fmt.Printf("保留临时库：%s（工作目录 %s）\n", strings.Join(created, ", "), *workDir)
			return
		}
		for _, name := range created {
			admin.Exec(fmt.Sprintf(`DROP DATABASE IF EXISTS %q WITH (FORCE)`, name))
		}
	}()

	// ---- A：纯增量 1..n，导出基线 ----
	createDB(admin, dbA)
	created = append(created, dbA)
	runChain(withDB(*adminDSN, dbA), "off", n)
	baselineDir := initialize.MigrationBaselineDir
	if n < full {
		baselineDir = filepath.Join(*workDir, "baseline_drill")
		os.RemoveAll(baselineDir)
	}
	if err := os.MkdirAll(baselineDir, 0o755); err != nil {
		fail(2, "创建基线目录失败：%v", err)
	}
	outPath := filepath.Join(baselineDir, fmt.Sprintf("%d.sql", n))
	writeBaseline(pgDump, withDB(*adminDSN, dbA), outPath, n, pgVersion)
	fmt.Printf("已写入基线 %s（PG %s，源区间 1..%d）\n", outPath, pgVersion, n)
	if !*verify {
		return
	}
	if n < full {
		// 演练：A 继续增量到 full，作为比对基准。
		runChain(withDB(*adminDSN, dbA), "off", full)
	}

	// ---- B：Go 运行器走基线路径 ----
	createDB(admin, dbB)
	created = append(created, dbB)
	initialize.MigrationBaselineDir = baselineDir
	dbBConn := open(withDB(*adminDSN, dbB))
	os.Setenv("AETHERLINK_MIGRATION_BASELINE", "auto")
	plan, err := initialize.PlanBaseline(dbBConn, 0, full)
	if err != nil || !plan.Use || plan.Number != n {
		fail(1, "PlanBaseline 未选中基线 %d：use=%v number=%d inputs=%+v err=%v", n, plan.Use, plan.Number, plan.Inputs, err)
	}
	closeDB(dbBConn)
	runChain(withDB(*adminDSN, dbB), "auto", full)

	diffs := compareDumps(pgDump, withDB(*adminDSN, dbA), withDB(*adminDSN, dbB), *workDir)
	if diffs > 0 {
		fail(1, "\nVERDICT=FAIL :: 基线路径与增量路径不一致（%d 行差异，dump 见 %s）", diffs, *workDir)
	}
	tail := ""
	if n < full {
		tail = fmt.Sprintf(" + 增量 %d..%d", n+1, full)
	}
	fmt.Printf("\nVERDICT=PASS :: 基线 %d.sql%s 经 Go 运行器与纯增量 1..%d 逐行一致（schema+数据）\n", n, tail, full)
}

// runChain 以指定基线开关跑 CheckVersion 到 target，并核对 sys_version。
func runChain(dsn, baselineMode string, target int) {
	os.Setenv("AETHERLINK_MIGRATION_BASELINE", baselineMode)
	saved := global.VERSION_NUMBER
	global.VERSION_NUMBER = target
	defer func() { global.VERSION_NUMBER = saved }()
	db := open(dsn)
	defer closeDB(db)
	started := time.Now()
	if err := initialize.CheckVersion(db); err != nil {
		fail(1, "迁移失败（baseline=%s，目标 %d）：%v", baselineMode, target, err)
	}
	var v int
	db.Raw("SELECT COALESCE(MAX(version_number),0) FROM sys_version").Scan(&v)
	if v != target {
		fail(1, "sys_version=%d，期望 %d", v, target)
	}
	fmt.Printf("迁移完成 baseline=%s → %d（%s）\n", baselineMode, target, time.Since(started).Round(time.Millisecond))
}

func open(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		fail(2, "连接失败（环境不满足）：%v", err)
	}
	return db
}

func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}

func createDB(admin *gorm.DB, name string) {
	if err := admin.Exec(fmt.Sprintf(`CREATE DATABASE %q TEMPLATE template0 ENCODING 'UTF8'`, name)).Error; err != nil {
		fail(2, "创建临时库 %s 失败：%v", name, err)
	}
}
