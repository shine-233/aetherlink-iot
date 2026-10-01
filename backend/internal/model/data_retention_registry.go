// 文件用途：TB-22 表级保留期注册表 data_retention_registry 的持久化模型。
//
// 核心逻辑：一行 = 一张只增不删的表 + 判定过期的时间列 + 保留天数 + 删除参数，
// 由 CleanSystemDataByCron 遍历后按 batch_size 分批 DELETE。
//
// 关键注意事项：
//   - table_name / time_column 是会被拼进 DELETE 语句的标识符，Go 侧必须经
//     identifierRegexp 白名单 + to_regclass 二次校验（见 dal/data_policy_retention.go），
//     绝不接受来自 API 的自由 SQL 片段；注册表行由迁移种子与运维写入。
//   - customer_data 类默认 enabled='2'（关闭）：清理不可逆，是否丢历史只能由部署方决定。
//     唯一默认开启的是 category='idempotency_receipt' 的幂等回执行。
//   - 本表无租户列：它是平台管理面配置，与 data_policy 同层。
//
// 重构建议：若后续需要"按表暴露管理接口"，复用本模型并在服务层加 SYS_ADMIN 前置校验。
package model

import "time"

const TableNameDataRetentionRegistry = "data_retention_registry"

// 保留期注册表的类别取值（与 141.sql 的 category CHECK 约束一致）。
const (
	RetentionCategoryCustomerData       = "customer_data"
	RetentionCategoryIdempotencyReceipt = "idempotency_receipt"
	RetentionCategoryDeadLetter         = "dead_letter"
	RetentionCategoryAuditLog           = "audit_log"
)

// 时间列类型（与 141.sql 的 time_kind CHECK 约束一致）。
const (
	RetentionTimeKindTimestampTZ = "timestamptz"
	RetentionTimeKindUnixMs      = "unix_ms"
)

// 启停取值，沿用项目既有的 '1' 启用 / '2' 停用口径。
const (
	RetentionEnabled  = "1"
	RetentionDisabled = "2"
)

// 说明：本结构体故意不定义 TableName() 方法——字段 TableName 与方法同名会触发
// "field and method with the same name" 编译错误（gen 生成器遇到同名列时会改写字段名，
// 本模型为手写故直接不提供）。表名一律由 TableNameDataRetentionRegistry 常量显式指定，
// DAL 侧全部使用带 public. 限定的 raw SQL，不依赖 gorm 的默认表名推导。
type DataRetentionRegistry struct {
	ID                  string     `gorm:"column:id;primaryKey" json:"id"`
	TableName           string     `gorm:"column:table_name;not null" json:"table_name"`
	TimeColumn          string     `gorm:"column:time_column;not null" json:"time_column"`
	TimeKind            string     `gorm:"column:time_kind;not null" json:"time_kind"`
	RetentionDays       int32      `gorm:"column:retention_days;not null" json:"retention_days"`
	Category            string     `gorm:"column:category;not null" json:"category"`
	Enabled             string     `gorm:"column:enabled;not null" json:"enabled"`
	BatchSize           int32      `gorm:"column:batch_size;not null" json:"batch_size"`
	ResolvedOnly        bool       `gorm:"column:resolved_only;not null" json:"resolved_only"`
	LastCleanupTime     *time.Time `gorm:"column:last_cleanup_time" json:"last_cleanup_time"`
	LastCleanupDataTime *time.Time `gorm:"column:last_cleanup_data_time" json:"last_cleanup_data_time"`
	Remark              *string    `gorm:"column:remark" json:"remark"`
	CreatedAt           time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at" json:"updated_at"`
}
