package initialize

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

const (
	voucherPurgeEnv         = "AETHERLINK_PURGE_VOUCHER_PLAINTEXT"
	voucherPurgeConfigKey   = "storage.purge_voucher_plaintext"
	voucherPurgeModeOn      = "on"
	voucherPurgeModeOff     = "off"
	voucherPurgeMaxRowsKey  = "storage.purge_voucher_plaintext_max_rows"
	voucherPurgeDefaultRows = 5000
)

// voucherPurgeEnabled reports whether the irreversible plaintext purge should run.
// 默认关闭：清理存量明文是不可逆操作，只有确认双模式明文兜底已不再需要
// （存量行都已回填 voucher_hash、且没有实例仍在依赖明文匹配）之后才应打开。
// 取值非法时**失败关闭**（跳过并告警），而不是报错中断启动——对一个破坏性开关，
// 悄悄不做事远好过按错误的意图删数据。
func voucherPurgeEnabled() (bool, error) {
	raw := strings.TrimSpace(os.Getenv(voucherPurgeEnv))
	if raw == "" {
		raw = strings.TrimSpace(viper.GetString(voucherPurgeConfigKey))
	}
	switch strings.ToLower(raw) {
	case "":
		return false, nil
	case voucherPurgeModeOn:
		return true, nil
	case voucherPurgeModeOff:
		return false, nil
	default:
		return false, fmt.Errorf("非法 %s=%q：仅支持 on|off（默认 off）", voucherPurgeEnv, raw)
	}
}

// voucherPurgeMaxRows 单次启动最多清理多少行，避免首次开启时长事务锁表。
func voucherPurgeMaxRows() int {
	if n := viper.GetInt(voucherPurgeMaxRowsKey); n > 0 {
		return n
	}
	return voucherPurgeDefaultRows
}
