// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"database/sql"

	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"gorm.io/gorm"
)

// StartTransaction 开启一个数据库事务，返回可用于 query 链式调用的 QueryTx
func StartTransaction(opts ...*sql.TxOptions) (*query.QueryTx, error) {
	// 与 application.go 的 gen 装配保持一致：事务侧同样从全新语句会话根出发，
	// 避免 Statement.Model/Dest 跨请求继承导致 gorm 注入陈旧主键条件（P1 修复）。
	tx := query.Use(global.DB.Session(&gorm.Session{NewDB: true})).Begin(opts...)
	if tx.Error != nil {
		return nil, tx.Error
	}
	return tx, nil
}

// Rollback 回滚事务，返回回滚过程中的错误
func Rollback(tx *query.QueryTx) error {
	if err := tx.Rollback(); err != nil {
		return err
	}
	return nil
}

// Commit 提交事务，返回提交过程中的错误
func Commit(tx *query.QueryTx) error {
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}
