// 文件用途：DAL 列表查询共享构件（*gorm.DB 形态）。
// 核心逻辑：把各列表查询里反复手写的五类片段收敛为可组合 helper：
//   1. scopeTenantColumn —— 租户作用域谓词（空作用域 fail-closed / 单租户 = / 多租户 IN）；
//   2. whereKeywordContains —— 关键词包含匹配（trim + 通配符转义 + 显式 ESCAPE，多列 OR）；
//   3. normalizePageParams —— page/pageSize 归一（默认值 + 上限）；
//   4. countAndFindPage —— 先 COUNT 再按排序分页取数，二者共用同一过滤条件但互不污染 Statement。
// 本包所有列表的 ORDER BY 都是代码内固定片段（不存在用户可控排序入参），故未设排序白名单 helper。
// 关键注意事项：
//   - whereKeywordContains 使用显式 ESCAPE '\'，PostgreSQL 与 SQLite 行为一致（SQLite 默认无转义符，
//     旧写法在测试库上转义不生效；PG 默认反斜杠，与 EscapeLikePattern 约定一致）。
//   - 列名参数必须来自代码常量，严禁传入用户输入。
//   - gen 构建器（query.IXxxDo）路径继续使用 pagination.go 的 applyListPagination 与 ContainsLikePattern；
//     若那里也要 fail-closed，参考 device_groups.go 的 deviceGroupTenantScope（gen 版同语义）。

package dal

import (
	"strings"

	"gorm.io/gorm"
)

// likeOperator 选择大小写敏感（LIKE）或不敏感（ILIKE，仅 PG）的匹配运算符。
type likeOperator string

const (
	opLike  likeOperator = "LIKE"
	opILike likeOperator = "ILIKE"
)

// scopeTenantColumn 在 column 上施加租户作用域谓词。
// 返回 empty=true 表示作用域为空，调用方应直接返回空结果（fail-closed，不扫全表）。
func scopeTenantColumn(db *gorm.DB, column string, scopes []string) (scoped *gorm.DB, empty bool) {
	switch len(scopes) {
	case 0:
		return db, true
	case 1:
		return db.Where(column+" = ?", scopes[0]), false
	default:
		return db.Where(column+" IN ?", scopes), false
	}
}

// whereOptionalTenant 仅当 tenantID 非空时追加 column = ? 过滤（用于"空=平台级"语义的旧接口）。
func whereOptionalTenant(db *gorm.DB, column, tenantID string) *gorm.DB {
	if tenantID == "" {
		return db
	}
	return db.Where(column+" = ?", tenantID)
}

// whereKeywordContains 对 keyword（trim 后非空时）在多列上做 OR 包含匹配。
// 用户输入中的 %、_、\ 均按字面量处理。
func whereKeywordContains(db *gorm.DB, op likeOperator, keyword string, columns ...string) *gorm.DB {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" || len(columns) == 0 {
		return db
	}
	pattern := ContainsLikePattern(keyword)
	clauses := make([]string, len(columns))
	args := make([]interface{}, len(columns))
	for i, col := range columns {
		clauses[i] = col + " " + string(op) + ` ? ESCAPE '\'`
		args[i] = pattern
	}
	cond := clauses[0]
	if len(clauses) > 1 {
		cond = "(" + strings.Join(clauses, " OR ") + ")"
	}
	return db.Where(cond, args...)
}

// whereKeywordContainsPtr 是 whereKeywordContains 的可选指针入参版本。
func whereKeywordContainsPtr(db *gorm.DB, op likeOperator, keyword *string, columns ...string) *gorm.DB {
	if keyword == nil {
		return db
	}
	return whereKeywordContains(db, op, *keyword, columns...)
}

// normalizePageParams 归一分页参数：page<1 → 1；pageSize<1 → def；pageSize>max → max。
func normalizePageParams(page, pageSize, def, max int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = def
	}
	if max > 0 && pageSize > max {
		pageSize = max
	}
	return page, pageSize
}

// countAndFindPage 在同一过滤条件上执行 COUNT 与分页查询。
// base 不会被修改（两次查询各自使用独立 Session），page/pageSize 需已归一。
func countAndFindPage(base *gorm.DB, order string, page, pageSize int, dest interface{}) (int64, error) {
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return 0, err
	}
	q := base.Session(&gorm.Session{})
	if order != "" {
		q = q.Order(order)
	}
	if err := q.Offset((page - 1) * pageSize).Limit(pageSize).Find(dest).Error; err != nil {
		return 0, err
	}
	return total, nil
}
