package service

import (
	"testing"

	"aetherlink-iot/backend/pkg/global"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCheckDBMigrationsRequiresCurrentMigrationVersion(t *testing.T) {
	oldDB := global.DB
	t.Cleanup(func() { global.DB = oldDB })

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(`
		CREATE TABLE sys_version (
			version_number INTEGER NOT NULL PRIMARY KEY,
			version TEXT NOT NULL
		)
	`).Error)

	global.DB = db

	t.Run("rejects a stale migration version", func(t *testing.T) {
		require.NoError(t, db.Exec("DELETE FROM sys_version").Error)
		require.NoError(t, db.Exec("INSERT INTO sys_version (version_number, version) VALUES (?, ?)", global.VERSION_NUMBER-1, "0.0.23").Error)

		got := checkDBMigrations()

		require.False(t, got.OK)
		require.Contains(t, got.Error, "does not match expected version")
		require.Contains(t, got.NextAction, "48")
	})

	t.Run("accepts the current migration version", func(t *testing.T) {
		require.NoError(t, db.Exec("DELETE FROM sys_version").Error)
		require.NoError(t, db.Exec("INSERT INTO sys_version (version_number, version) VALUES (?, ?)", global.VERSION_NUMBER, "0.0.23").Error)

		got := checkDBMigrations()

		require.True(t, got.OK)
		require.Contains(t, got.Detail, "48")
	})
}
