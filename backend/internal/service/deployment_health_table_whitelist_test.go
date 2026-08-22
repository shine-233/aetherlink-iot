package service

import (
	"testing"

	"aetherlink-iot/backend/pkg/global"

	"github.com/stretchr/testify/require"
)

func TestCountTableRowsRejectsTablesOutsideWhitelist(t *testing.T) {
	oldDB := global.DB
	t.Cleanup(func() { global.DB = oldDB })
	// 白名单校验必须发生在任何 DB 访问之前：DB 为 nil 时也应先拒绝非白名单表名。
	global.DB = nil

	for _, table := range []string{
		"users; DROP TABLE users",
		"users --",
		"pg_catalog.pg_tables",
		"Devices",
	} {
		_, err := countTableRows(table)
		require.Error(t, err, "table %q must be rejected", table)
		require.Contains(t, err.Error(), "白名单")
	}
}

func TestCountTableRowsWhitelistedTablesPassGuardBeforeDBAccess(t *testing.T) {
	oldDB := global.DB
	t.Cleanup(func() { global.DB = oldDB })
	global.DB = nil

	for _, table := range []string{"users", "device_configs", "devices"} {
		_, err := countTableRows(table)
		require.Error(t, err, "whitelisted table %q should pass the guard and reach the DB check", table)
		require.Contains(t, err.Error(), "数据库连接还没有初始化")
	}
}
