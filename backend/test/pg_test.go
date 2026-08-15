//go:build dangerous_integration
// +build dangerous_integration

// 文件用途：提供会重置数据库 schema 的危险集成测试入口。
// 核心逻辑：按 run_env 加载配置、连接 PostgreSQL、重建 public schema、执行初始 SQL 并插入通知组样例。
// 关键注意事项：该文件受 dangerous_integration build tag 保护，会清空数据库，禁止在默认测试或共享环境中运行。
// 重构建议：建议迁移到隔离容器数据库和独立测试 schema，并把测试数据准备拆成可复用 fixture。

package test

import (
	"os"
	"testing"
	"time"

	"aetherlink-iot/backend/initialize"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var adr = func(s string) *string { return &s }
var config *initialize.DbConfig
var db *gorm.DB

func TestDatebase(t *testing.T) {
	// 要保证测试顺序，下面的函数都不能以Test开头
	testConnect(t)
	testDDLInit(t)
	testNotificationGroup(t)
}

func testConnect(t *testing.T) {
	require := require.New(t)
	if os.Getenv("run_env") == "git-actions" {
		initialize.ViperInit("../configs/conf-push-test.yml")
	} else if os.Getenv("run_env") == "localdev" {
		initialize.ViperInit("../configs/conf-localdev.yml")
	} else {
		t.Skip("run_env must be git-actions or localdev for database integration test")
	}
	var err error
	config, err = initialize.LoadDbConfig()
	require.Nil(err)
	db, err = initialize.PgConnect(config)
	require.Nil(err)
}

func testDDLInit(t *testing.T) {
	require := require.New(t)

	// 清空数据库所有的表
	res := db.Exec("DROP SCHEMA public CASCADE;CREATE SCHEMA public;")
	require.Nil(res.Error)

	// 切换到新创建的数据库
	db, err := initialize.PgConnect(config)
	require.Nil(err)

	// ts := db.Exec("CREATE TABLE sys_version (version_number INT NOT NULL DEFAULT 0, version varchar(255) NOT NULL, PRIMARY KEY (version_number))")
	// err = ts.Error
	// require.Nilf(err,"CREATE TABLE sys_version error %v",err)

	// 执行1.sql文件
	err = initialize.ExecuteSQLFile(db, "../sql/1.sql")
	require.Nilf(err, "执行ddl失败%v", err)

	require.Nilf(err, "ddl提交失败%v", err)
	t.Log("初始化数据库成功")
}

func testNotificationGroup(t *testing.T) {
	require := require.New(t)
	require.NotNil(db, "数据库连接失败")
	query.SetDefault(db)

	// 创建测试数据
	notificationGroup := model.NotificationGroup{
		Name:               "test",
		NotificationType:   "MEMBER",
		Status:             "ON",
		NotificationConfig: adr("{}"),
		Description:        adr("test"),
		TenantID:           "123456",
		Remark:             adr("test"),
		CreatedAt:          time.Now().UTC(),
		UpdatedAt:          time.Now().UTC(),
	}
	err := query.NotificationGroup.Create(&notificationGroup)
	require.Nil(err, "创建数据notificationGroup失败")
	db.Commit()
}
