// 文件用途：TimescaleDB 显式开关（ROADMAP C1 收尾）。
// 背景：57.sql 目前是"检测扩展自动启用"——只要 pg 装了 timescaledb 就把 telemetry_datas/
//       alarm_info 转 hypertable，无法显式选择普通 PG。本文件把该决策提升为显式配置：
//       AETHERLINK_TIMESCALE_MODE=auto|on|off（默认 auto，保持原行为）。
//   - auto：沿用现状——由 57.sql 自行检测扩展决定是否转换。
//   - off ：显式关闭 TimescaleDB——跳过 57.sql，即使扩展已安装也保持普通 PG。
//   - on  ：显式开启——要求数据库已安装 timescaledb 扩展，否则迁移启动即失败（fail-fast），
//           杜绝"以为开了实际没开"的静默降级。
// 边界：仅控制 57.sql 是否执行；不影响其它 SQL。未知取值视为配置错误并拒绝启动（fail-fast），
//       与 GOTP_JWT_KEY 占位符 fail-fast 的既有策略一致。
// 重构建议：决策函数保持纯逻辑便于单测；DB 探测集中在 timescaleExtensionInstalled。
package initialize

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

// TimescaleSQLFileNumber 迁移文件中 TimescaleDB 转换脚本的版本号（对应 sql/57.sql）。
const TimescaleSQLFileNumber = 57

const (
	timescaleModeAuto = "auto"
	timescaleModeOn   = "on"
	timescaleModeOff  = "off"
)

// readTimescaleMode 读取配置：优先环境变量 AETHERLINK_TIMESCALE_MODE，
// 其次 config 键 storage.timescale_mode，都为空时回落 auto。
func readTimescaleMode() string {
	raw := strings.TrimSpace(os.Getenv("AETHERLINK_TIMESCALE_MODE"))
	if raw == "" {
		raw = strings.TrimSpace(viper.GetString("storage.timescale_mode"))
	}
	return raw
}

// normalizeTimescaleMode 归一化取值；空值回落 auto，未知取值返回错误（fail-fast）。
func normalizeTimescaleMode(raw string) (string, error) {
	mode := strings.ToLower(strings.TrimSpace(raw))
	if mode == "" {
		return timescaleModeAuto, nil
	}
	switch mode {
	case timescaleModeAuto, timescaleModeOn, timescaleModeOff:
		return mode, nil
	default:
		return "", fmt.Errorf("非法 AETHERLINK_TIMESCALE_MODE=%q：仅支持 auto|on|off", raw)
	}
}

// decideTimescaleMigration 纯决策：给定模式与"扩展是否已安装"，返回是否应执行 57.sql。
// run=false 且 failMsg==""：显式关闭，跳过（不报错、不转换）。
// run=false 且 failMsg!=""：显式开启但扩展缺失，迁移应失败并给出可操作指引。
func decideTimescaleMigration(mode string, extInstalled bool) (run bool, failMsg string) {
	switch mode {
	case timescaleModeOff:
		return false, ""
	case timescaleModeOn:
		if !extInstalled {
			return false, "AETHERLINK_TIMESCALE_MODE=on 但数据库未安装 timescaledb 扩展：" +
				"请先安装扩展，或将配置改为 auto（检测到扩展才转换）/ off（保持普通 PG）"
		}
		return true, ""
	default: // auto
		return true, ""
	}
}

// timescaleExtensionInstalled 探测当前 PostgreSQL 是否已安装 timescaledb 扩展。
func timescaleExtensionInstalled(db *gorm.DB) (bool, error) {
	var count int64
	if err := db.Raw("SELECT count(1) FROM pg_extension WHERE extname = 'timescaledb'").Scan(&count).Error; err != nil {
		return false, fmt.Errorf("探测 timescaledb 扩展失败: %w", err)
	}
	return count > 0, nil
}
