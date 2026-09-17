// 文件用途：全新空库的迁移链全链校验（ROADMAP P0.1 发布门禁「全新库迁移通过」）。
//
// 为什么需要它：P0.1 的证据文档里写明「全新空库验证只到 93.sql，94–99 未做同等全链验证」，
// 而迁移现在已经到 109。也就是说**发布门禁里最基础的那一条，实际上有 16 个迁移
// 从未在干净库上跑过**。这些迁移又恰好全是未提交状态，一旦其中任何一个在空库上失败，
// 用户拿到的是「装不上」而不是「功能少」——这是最贵的失败方式。
//
// 核心逻辑：开一个空库连接，直接调用**项目自身的** `initialize.CheckVersion`
// （与生产启动完全同一条代码路径，不另写一套执行器，避免"校验通过但生产走的是另一条路"），
// 然后核对 `sys_version` 的落点与建表数量。
//
// 关键注意事项：
//   - **必须用空库**。跑在有数据的库上只会验证"增量升级"，验证不了"从零安装"。
//     本工具在开跑前会拒绝任何已有业务表的库（除非显式 -allow-dirty）。
//   - 工作目录必须是 `backend/`：`CheckVersion` 按相对路径 `sql/<n>.sql` 读取迁移文件。
//   - TimescaleDB 默认关闭（`AETHERLINK_TIMESCALE_MODE=off`），与本地/测试栈一致；
//     57.sql 的 hypertable 转换会被跳过但迁移继续。
//   - 退出码：0=全链通过；1=失败；2=环境不满足（不是"通过"）。
//
// 用法：
//
//	cd backend
//	go run ./cmd/migchaincheck -dsn "host=127.0.0.1 port=55433 user=postgres password=x dbname=aetherlink_migrate_fullchain sslmode=disable"
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"aetherlink-iot/backend/initialize"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	dsn := flag.String("dsn", "", "目标空库连接串（必填）")
	allowDirty := flag.Bool("allow-dirty", false, "允许在已有业务表的库上执行（默认拒绝，因为那验证不了从零安装）")
	flag.Parse()

	if *dsn == "" {
		fmt.Fprintln(os.Stderr, "必须提供 -dsn（目标空库连接串）")
		os.Exit(2)
	}

	// 与生产启动同一套 gorm 配置。
	db, err := gorm.Open(postgres.Open(*dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "连接失败（环境不满足，不是校验通过）：%v\n", err)
		os.Exit(2)
	}

	// 空库前置校验：sys_version 之外的表数必须为 0。
	var existingTables int64
	if err := db.Raw(
		"SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE' AND table_name <> 'sys_version'",
	).Scan(&existingTables).Error; err != nil {
		fmt.Fprintf(os.Stderr, "统计现有表失败：%v\n", err)
		os.Exit(2)
	}
	if existingTables > 0 && !*allowDirty {
		fmt.Fprintf(os.Stderr,
			"目标库不是空库（已有 %d 张业务表）。在非空库上跑只能验证增量升级，验证不了从零安装。\n"+
				"请改用新建的空库，或显式加 -allow-dirty。\n", existingTables)
		os.Exit(2)
	}

	fmt.Printf("目标程序版本 VERSION_NUMBER=%d，VERSION=%s\n", global.VERSION_NUMBER, global.VERSION)
	fmt.Printf("空库校验：现有业务表 %d 张\n", existingTables)

	startedAt := time.Now()
	runErr := initialize.CheckVersion(db)
	elapsed := time.Since(startedAt)

	if runErr != nil {
		fmt.Fprintf(os.Stderr, "\n全链迁移失败（耗时 %s）：%v\n", elapsed.Round(time.Millisecond), runErr)
		os.Exit(1)
	}

	// 落点核对：sys_version 必须等于程序版本，否则"跑完了"与"跑对了"是两回事。
	var dataVersion int
	if err := db.Raw("SELECT COALESCE(MAX(version_number), 0) FROM sys_version").Scan(&dataVersion).Error; err != nil {
		fmt.Fprintf(os.Stderr, "读取 sys_version 失败：%v\n", err)
		os.Exit(1)
	}
	var tableCount int64
	if err := db.Raw(
		"SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_type='BASE TABLE'",
	).Scan(&tableCount).Error; err != nil {
		fmt.Fprintf(os.Stderr, "统计建表数失败：%v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n迁移完成：耗时 %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("sys_version      = %d（期望 %d）\n", dataVersion, global.VERSION_NUMBER)
	fmt.Printf("public 表数量     = %d\n", tableCount)

	if dataVersion != global.VERSION_NUMBER {
		fmt.Fprintf(os.Stderr, "\nsys_version=%d 与 VERSION_NUMBER=%d 不一致——迁移链没有跑完\n",
			dataVersion, global.VERSION_NUMBER)
		os.Exit(1)
	}
	if tableCount == 0 {
		fmt.Fprintln(os.Stderr, "\n没有任何表被创建——迁移文件可能被静默跳过")
		os.Exit(1)
	}

	fmt.Println("\nVERDICT=PASS :: 全新空库全链迁移通过")
}
