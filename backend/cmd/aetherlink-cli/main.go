// Command aetherlink-cli 提供 AetherLink IoT 平台运维与诊断命令行工具（ROADMAP P3 商业化）。
//
// 使用说明:
//   1. 平台运行状态健康巡检:
//      aetherlink-cli health -target http://127.0.0.1:9999 -pg 127.0.0.1:55433 -mqtt 127.0.0.1:1883
//
//   2. 离线许可证检验:
//      aetherlink-cli license -key-id lk1 -pub <base64_pubkey> -license <base64_doc>
//
//   3. 数据库版本与统计巡检:
//      aetherlink-cli db -dsn "host=127.0.0.1 port=55433 user=postgres password=localdev-only dbname=aetherlink_go99 sslmode=disable"
//
//   4. 租户资产状态巡检:
//      aetherlink-cli tenant -dsn "host=127.0.0.1 port=55433 user=postgres password=localdev-only dbname=aetherlink_go99 sslmode=disable"
//
//   5. 商业化套餐与订阅巡检:
//      aetherlink-cli billing -dsn "host=127.0.0.1 port=55433 user=postgres password=localdev-only dbname=aetherlink_go99 sslmode=disable"
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"aetherlink-iot/backend/pkg/license"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "health":
		err = handleHealth(args)
	case "license":
		err = handleLicense(args)
	case "db":
		err = handleDB(args)
	case "tenant":
		err = handleTenant(args)
	case "billing":
		err = handleBilling(args)
	case "help", "-h", "--help":
		printUsage()
		return
	default:
		fmt.Fprintf(os.Stderr, "未知命令: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "执行失败: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`AetherLink IoT 平台运维与诊断工具 (aetherlink-cli)

用法:
  aetherlink-cli <command> [arguments]

可用命令:
  health   平台各子系统连通性与延迟巡检（HTTP API、PostgreSQL、MQTT Broker）
  license  离线商业许可证验签、有效期与配额参数检查
  db       数据库迁移版本 (sys_version) 与核心业务表行数统计
  tenant   平台租户资产、层级与设备/用户分布巡检
  billing  商业化套餐列表与各租户当前订阅状态巡检

运行 'aetherlink-cli <command> -h' 查看具体命令的参数说明。`)
}

// -------------------------------------------------------------
// 1. health 命令
// -------------------------------------------------------------

func handleHealth(args []string) error {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	targetHTTP := fs.String("target", "http://127.0.0.1:9999", "后端服务 HTTP 地址")
	targetPG := fs.String("pg", "127.0.0.1:55433", "PostgreSQL host:port")
	targetMQTT := fs.String("mqtt", "127.0.0.1:1883", "MQTT Broker host:port")
	timeout := fs.Duration("timeout", 3*time.Second, "探测超时时间")
	_ = fs.Parse(args)

	fmt.Println("=== AetherLink IoT 平台健康巡检 ===")
	fmt.Printf("时间: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	type checkItem struct {
		Name     string
		Target   string
		Status   string
		Latency  time.Duration
		ErrorMsg string
	}

	var results []checkItem

	// 1. HTTP API
	t0 := time.Now()
	client := http.Client{Timeout: *timeout}
	url := strings.TrimRight(*targetHTTP, "/") + "/health"
	resp, err := client.Get(url)
	latHTTP := time.Since(t0)
	if err != nil {
		results = append(results, checkItem{Name: "Backend HTTP", Target: url, Status: "FAIL", Latency: latHTTP, ErrorMsg: err.Error()})
	} else {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 200 {
			results = append(results, checkItem{Name: "Backend HTTP", Target: url, Status: "OK", Latency: latHTTP})
		} else {
			results = append(results, checkItem{Name: "Backend HTTP", Target: url, Status: "WARN", Latency: latHTTP, ErrorMsg: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))})
		}
	}

	// 2. PostgreSQL TCP
	t0 = time.Now()
	connPG, errPG := net.DialTimeout("tcp", *targetPG, *timeout)
	latPG := time.Since(t0)
	if errPG != nil {
		results = append(results, checkItem{Name: "PostgreSQL Port", Target: *targetPG, Status: "FAIL", Latency: latPG, ErrorMsg: errPG.Error()})
	} else {
		connPG.Close()
		results = append(results, checkItem{Name: "PostgreSQL Port", Target: *targetPG, Status: "OK", Latency: latPG})
	}

	// 3. MQTT TCP
	t0 = time.Now()
	connMQTT, errMQTT := net.DialTimeout("tcp", *targetMQTT, *timeout)
	latMQTT := time.Since(t0)
	if errMQTT != nil {
		results = append(results, checkItem{Name: "MQTT Broker", Target: *targetMQTT, Status: "FAIL", Latency: latMQTT, ErrorMsg: errMQTT.Error()})
	} else {
		connMQTT.Close()
		results = append(results, checkItem{Name: "MQTT Broker", Target: *targetMQTT, Status: "OK", Latency: latMQTT})
	}

	// 输出表格
	fmt.Printf("%-18s %-30s %-8s %-12s %s\n", "组件", "目标地址", "状态", "延迟", "说明")
	fmt.Println(strings.Repeat("-", 80))
	for _, r := range results {
		fmt.Printf("%-18s %-30s %-8s %-12s %s\n", r.Name, r.Target, r.Status, r.Latency.Round(time.Millisecond), r.ErrorMsg)
	}

	return nil
}

// -------------------------------------------------------------
// 2. license 命令
// -------------------------------------------------------------

func handleLicense(args []string) error {
	fs := flag.NewFlagSet("license", flag.ExitOnError)
	keyID := fs.String("key-id", "lk1", "签名密钥标识")
	pubKey := fs.String("pub", "", "Base64 编码的 Ed25519 公钥 (32 bytes)")
	licStr := fs.String("license", "", "Base64 编码的许可证签名载荷")
	_ = fs.Parse(args)

	if *pubKey == "" || *licStr == "" {
		return fmt.Errorf("必须提供 -pub 和 -license 参数")
	}

	verifier, err := license.NewVerifier(map[string]string{*keyID: *pubKey})
	if err != nil {
		return fmt.Errorf("初始化验签器失败: %w", err)
	}

	doc, fingerprint, err := verifier.Parse(*licStr, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("许可证验签失败: %w", err)
	}

	fmt.Println("=== 许可证验签成功 (VALID) ===")
	fmt.Printf("数字指纹: %s\n", fingerprint)
	fmt.Printf("客户名称: %s\n", doc.IssuedTo)
	fmt.Printf("产品版本: %s\n", doc.Edition)
	fmt.Printf("设备上限: %d 台\n", doc.MaxDevices)
	fmt.Printf("租户上限: %d 个\n", doc.MaxTenants)
	if doc.NotBefore > 0 {
		fmt.Printf("生效时间: %s\n", time.UnixMilli(doc.NotBefore).Format(time.RFC3339))
	}
	if doc.NotAfter > 0 {
		fmt.Printf("到期时间: %s\n", time.UnixMilli(doc.NotAfter).Format(time.RFC3339))
	} else {
		fmt.Println("到期时间: 永久有效")
	}
	fmt.Printf("特性列表: %s\n", strings.Join(doc.Features, ", "))

	now := time.Now().UnixMilli()
	if doc.NotBefore > 0 && now < doc.NotBefore {
		fmt.Println("警告: 许可证尚未到达生效时间！")
	} else if doc.NotAfter > 0 && now > doc.NotAfter {
		fmt.Println("警告: 许可证已过期！")
	} else {
		fmt.Println("状态: 当前有效 (ACTIVE)")
	}

	return nil
}

// -------------------------------------------------------------
// 3. db 命令
// -------------------------------------------------------------

func handleDB(args []string) error {
	fs := flag.NewFlagSet("db", flag.ExitOnError)
	dsn := fs.String("dsn", "host=127.0.0.1 port=55433 user=postgres password=localdev-only dbname=aetherlink_go99 sslmode=disable", "PostgreSQL 连接串")
	_ = fs.Parse(args)

	db, err := gorm.Open(postgres.Open(*dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	// 1. 查询版本号
	var versionNumber int
	var versionName string
	row := db.Raw("SELECT version_number, version FROM sys_version LIMIT 1").Row()
	if err := row.Scan(&versionNumber, &versionName); err != nil {
		return fmt.Errorf("读取 sys_version 失败: %w", err)
	}

	// 2. 统计表总数
	var tableCount int64
	db.Raw("SELECT count(1) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tableCount)

	fmt.Println("=== AetherLink 数据库状态 ===")
	fmt.Printf("当前迁移版本: %d (系统发布版本: %s)\n", versionNumber, versionName)
	fmt.Printf("数据表总数:   %d 张\n\n", tableCount)

	// 3. 核心表行数统计
	coreTables := []string{
		"devices",
		"users",
		"tenants",
		"telemetry_datas",
		"alarm_config",
		"alarm_info",
		"scene_info",
		"rule_chains",
		"subscription_plans",
		"tenant_subscriptions",
	}

	fmt.Printf("%-24s %-12s\n", "核心业务表", "记录行数")
	fmt.Println(strings.Repeat("-", 40))
	for _, tbl := range coreTables {
		var cnt int64
		err := db.Table(tbl).Count(&cnt).Error
		if err != nil {
			fmt.Printf("%-24s %-12s\n", tbl, "ERR")
		} else {
			fmt.Printf("%-24s %-12d\n", tbl, cnt)
		}
	}

	return nil
}

// -------------------------------------------------------------
// 4. tenant 命令
// -------------------------------------------------------------

func handleTenant(args []string) error {
	fs := flag.NewFlagSet("tenant", flag.ExitOnError)
	dsn := fs.String("dsn", "host=127.0.0.1 port=55433 user=postgres password=localdev-only dbname=aetherlink_go99 sslmode=disable", "PostgreSQL 连接串")
	_ = fs.Parse(args)

	db, err := gorm.Open(postgres.Open(*dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	type tenantRow struct {
		ID             string    `gorm:"column:id"`
		Name           string    `gorm:"column:name"`
		ParentTenantID string    `gorm:"column:parent_tenant_id"`
		CreatedAt      time.Time `gorm:"column:created_at"`
	}

	var rows []tenantRow
	err = db.Table("tenants").Order("created_at ASC").Find(&rows).Error
	if err != nil {
		return fmt.Errorf("查询租户列表失败: %w", err)
	}

	fmt.Printf("=== 平台租户资产巡检 (共 %d 个租户) ===\n\n", len(rows))
	fmt.Printf("%-38s %-20s %-10s %-10s %s\n", "租户ID", "租户名称", "设备数", "用户数", "创建时间")
	fmt.Println(strings.Repeat("-", 95))

	for _, t := range rows {
		var devCount int64
		var userCount int64
		db.Table("devices").Where("tenant_id = ?", t.ID).Count(&devCount)
		db.Table("users").Where("tenant_id = ?", t.ID).Count(&userCount)

		fmt.Printf("%-38s %-20s %-10d %-10d %s\n", t.ID, t.Name, devCount, userCount, t.CreatedAt.Format("2006-01-02 15:04:05"))
	}

	return nil
}

// -------------------------------------------------------------
// 5. billing 命令
// -------------------------------------------------------------

func handleBilling(args []string) error {
	fs := flag.NewFlagSet("billing", flag.ExitOnError)
	dsn := fs.String("dsn", "host=127.0.0.1 port=55433 user=postgres password=localdev-only dbname=aetherlink_go99 sslmode=disable", "PostgreSQL 连接串")
	_ = fs.Parse(args)

	db, err := gorm.Open(postgres.Open(*dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return fmt.Errorf("连接数据库失败: %w", err)
	}

	type planRow struct {
		Code               string  `gorm:"column:code"`
		Name               string  `gorm:"column:name"`
		PriceMonthly       float64 `gorm:"column:price_monthly"`
		MaxDevices         int     `gorm:"column:max_devices"`
		MaxTenants         int     `gorm:"column:max_tenants"`
		MaxUsers           int     `gorm:"column:max_users"`
		MaxTelemetryPerDay int     `gorm:"column:max_telemetry_per_day"`
		Features           string  `gorm:"column:features"`
	}

	var plans []planRow
	db.Table("subscription_plans").Where("enabled = 1").Order("price_monthly ASC").Find(&plans)

	fmt.Println("=== 平台商业套餐阶梯 ===")
	fmt.Printf("%-12s %-18s %-10s %-10s %-10s %-10s %s\n", "代码", "套餐名称", "月费(USD)", "最大设备", "最大租户", "最大用户", "功能特性")
	fmt.Println(strings.Repeat("-", 90))
	for _, p := range plans {
		var feats []string
		_ = json.Unmarshal([]byte(p.Features), &feats)
		fmt.Printf("%-12s %-18s %-10.2f %-10d %-10d %-10d %s\n", p.Code, p.Name, p.PriceMonthly, p.MaxDevices, p.MaxTenants, p.MaxUsers, strings.Join(feats, ","))
	}

	type subRow struct {
		TenantID           string    `gorm:"column:tenant_id"`
		PlanCode           string    `gorm:"column:plan_code"`
		Status             string    `gorm:"column:status"`
		CurrentPeriodStart time.Time `gorm:"column:current_period_start"`
		CurrentPeriodEnd   time.Time `gorm:"column:current_period_end"`
	}

	var subs []subRow
	db.Table("tenant_subscriptions").Find(&subs)

	fmt.Printf("\n=== 租户套餐订阅明细 (共 %d 条) ===\n", len(subs))
	fmt.Printf("%-38s %-12s %-10s %-20s %-20s\n", "租户ID", "套餐", "状态", "周期开始", "周期结束")
	fmt.Println(strings.Repeat("-", 105))
	for _, s := range subs {
		fmt.Printf("%-38s %-12s %-10s %-20s %-20s\n", s.TenantID, s.PlanCode, s.Status, s.CurrentPeriodStart.Format("2006-01-02"), s.CurrentPeriodEnd.Format("2006-01-02"))
	}

	return nil
}
