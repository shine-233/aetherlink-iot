// 文件用途：提供 GORM 代码生成入口，负责把数据库表结构转换为后端 model 和 query 代码。
// 核心逻辑：初始化数据库连接后，调用 gorm/gen 按指定表生成类型安全的数据访问代码。
// 静态审查建议：生成目标、输出目录和数据库配置都属于高风险变更点，运行前应再次确认表名单与输出路径。
package main

import (
	initialize "aetherlink-iot/backend/initialize"

	"gorm.io/gen"
)

// main 负责初始化生成器、连接数据库并执行指定表的代码生成。
// 静态审查重点：这里的表名决定了会覆盖哪些生成文件，调整前要确认不会误生成到手写代码目录。
func main() {
	g := gen.NewGenerator(gen.Config{
		OutPath:       "../../internal/query",
		Mode:          gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface, // 生成模式
		FieldNullable: true,
	})

	initialize.ViperInit("../../configs/conf-dev.yml")
	initialize.LogInIt()
	gormdb, err := initialize.PgInit()
	if err != nil {
		panic(err)
	}
	if gormdb == nil {
		panic("gormdb is nil")
	}
	g.UseDB(gormdb) // 复用已有的 GORM 数据库连接

	// 生成指定数据表的 model 与 query 代码
	g.ApplyBasic(
		g.GenerateModel("device_templates"),
	)

	// 执行生成
	g.Execute()
}
