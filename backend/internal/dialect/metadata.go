// 文件用途：元查询（表存在性/列清单）方言映射纯函数（ROADMAP TP-20）——按方言产出可执行的探查 SQL 与绑定参数。
// 核心逻辑：postgres 走标准 information_schema（table_schema=current_schema()）；tdengine 走 3.x 内置
// INFORMATION_SCHEMA 的 ins_tables/ins_columns 视图（db_name=DATABASE() 限定当前库，表名区分大小写不归一）；
// kingbase（Oracle 兼容模式）走 user_tables/user_tab_columns 字典视图，绑定参数按 Oracle 习惯大写归一。
// 关键注意事项：返回 SQL 一律用 ? 占位符（database/sql 系驱动通用），SQL 与参数成对返回由调用方执行；
// 表名先经 ValidateIdentifier 校验（含方言长度上限），防注入；ins_columns/user_tab_columns 的列名
// 依据 TDengine 3.x / Oracle 兼容字典的公开文档映射，接真实驱动后需以服务端实测校准（TP-20 residual）。
// 重构建议：原生驱动落地后，把本文件与 pagination.go 一起作为元查询执行器的底层生成器，
// 并增加"执行+扫描"层；本文件的 SQL/参数生成签名保持稳定。
package dialect

import (
	"fmt"
	"strings"
)

// 表存在性探查 SQL：按方言返回 (sql, args, err)。语义为"当前库下 table 是否存在"，结果取 COUNT(*)。
// args 绑定的表名已按方言做大小写归一（KingBase Oracle 模式大写；TDengine/PG 原样）。
func TableExistsSQL(d Dialect, table string) (string, []any, error) {
	if !d.IsKnown() {
		return "", nil, fmt.Errorf("%w: cannot build table-exists query for %q", ErrUnknownDialect, string(d))
	}
	caps := d.Capabilities()
	if err := ValidateIdentifier(table, caps); err != nil {
		return "", nil, err
	}
	name := normalizeIdentifier(table, caps)
	var sql string
	switch d {
	case DialectTDengine:
		sql = "SELECT COUNT(*) FROM information_schema.ins_tables WHERE db_name = DATABASE() AND table_name = ?"
	case DialectKingbase:
		sql = "SELECT COUNT(*) FROM user_tables WHERE table_name = ?"
	default: // DialectPostgres / DialectPolardb：标准 information_schema
		sql = "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = current_schema() AND table_name = ?"
	}
	return sql, []any{name}, nil
}

// 列清单探查 SQL：按方言返回 (sql, args, err)，列出当前库下 table 的列（列名/类型/可空），按列序排列。
// args 绑定的表名已按方言做大小写归一。
func ListColumnsSQL(d Dialect, table string) (string, []any, error) {
	if !d.IsKnown() {
		return "", nil, fmt.Errorf("%w: cannot build column-list query for %q", ErrUnknownDialect, string(d))
	}
	caps := d.Capabilities()
	if err := ValidateIdentifier(table, caps); err != nil {
		return "", nil, err
	}
	name := normalizeIdentifier(table, caps)
	var sql string
	switch d {
	case DialectTDengine:
		// TDengine 3.x INFORMATION_SCHEMA.INS_COLUMNS：column_name/data_type 为公开字段；
		// 刻意只取最小列集（可空性语义与时序库不同，不与 PG/Oracle 对齐），列序用 ordinal_position。
		sql = "SELECT column_name, data_type FROM information_schema.ins_columns WHERE db_name = DATABASE() AND table_name = ? ORDER BY ordinal_position"
	case DialectKingbase:
		// Oracle 兼容字典 USER_TAB_COLUMNS：column_id 即列序（对应 Oracle 的 COLUMN_ID）。
		sql = "SELECT column_name, data_type, nullable FROM user_tab_columns WHERE table_name = ? ORDER BY column_id"
	default: // DialectPostgres / DialectPolardb：标准 information_schema.columns
		sql = "SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = ? ORDER BY ordinal_position"
	}
	return sql, []any{name}, nil
}

// normalizeIdentifier 按方言能力归一标识符：UpperIdentifier=true 的方言（KingBase Oracle 模式）
// 大写归一——Oracle 系字典视图里未加引号建立的表名默认大写存储，小写入参会查不到行。
func normalizeIdentifier(name string, caps Capabilities) string {
	if caps.UpperIdentifier {
		return strings.ToUpper(name)
	}
	return name
}
