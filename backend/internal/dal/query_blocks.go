// 文件用途：DAL 列表查询共享构件（*gorm.DB 形态）。
// 核心逻辑：把各列表查询里反复手写的片段收敛为可组合 helper：
//   1. scopeTenantColumn —— 租户作用域谓词（空作用域 fail-closed / 单租户 = / 多租户 IN）；
//   2. requireClaimsTenantID —— claims 租户谓词（空租户 fail-closed，SYS_ADMIN 全局视图放行）；
//   3. whereKeywordContains —— 关键词包含匹配（trim + 通配符转义 + 显式 ESCAPE，多列 OR）；
//   4. normalizePageParams —— page/pageSize 归一（默认值 + 上限）；
//   5. countAndFindPage —— 先 COUNT 再按排序分页取数，二者共用同一过滤条件但互不污染 Statement；
//   6. allowListedOrderFragment —— 裸字符串 ORDER BY 片段形状白名单，countAndFindPage 内置收口。
// 关键注意事项：
//   - whereKeywordContains 使用显式 ESCAPE '\'，PostgreSQL 与 SQLite 行为一致（SQLite 默认无转义符，
//     旧写法在测试库上转义不生效；PG 默认反斜杠，与 EscapeLikePattern 约定一致）。
//   - 列名参数必须来自代码常量，严禁传入用户输入。现有各列表的 ORDER BY 均为代码内固定片段；
//     order 白名单是预防性收口——未来若把用户可控排序接入 countAndFindPage，非白名单形状
//     会 fail-closed 回落为无排序，而不是把片段拼进 SQL。
//   - gen 构建器（query.IXxxDo）路径继续使用 pagination.go 的 applyListPagination 与 ContainsLikePattern
//     （其 Order 走类型安全的列对象，注入面不存在）；gen 版租户谓词参考 device_groups.go 的
//     deviceGroupTenantScope（同语义）。

package dal

import (
	"fmt"
	"regexp"
	"strings"

	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
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

// requireClaimsTenantID 校验 claims 携带的租户作用域并返回 trim 后的租户 ID（2026-09-28 收敛）。
// claims.TenantID 运行期可能因 token 边界条件变为空串，WHERE tenant_id=” 会静默匹配 0 行——
// 表现为"偶发空列表"。tenant 视角显式拒绝（fail-closed）；SYS_ADMIN 全局视图放行并返回空租户，
// 由调用方决定是否叠加租户过滤（空租户 → 不加过滤 = 全局视图）。
func requireClaimsTenantID(claims *utils.UserClaims) (string, error) {
	if claims == nil {
		return "", fmt.Errorf("empty tenant id in claims")
	}
	tenantID := strings.TrimSpace(claims.TenantID)
	if tenantID == "" && claims.Authority != SYS_ADMIN {
		logrus.Warn("dal: tenant-scoped query has empty TenantID in claims; rejecting")
		return "", fmt.Errorf("empty tenant id in claims")
	}
	return tenantID, nil
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

// orderFragmentPattern 是裸字符串 ORDER BY 片段的形状白名单（2026-09-28 新增，防 orderBy 注入）：
// 逗号分隔的「列名（可带 schema/表前缀）+ 可选 ASC/DESC」，仅允许标识符字符与点号。
// 注释、括号、函数调用、引号、分号、子查询等一切其他形状都会被拒绝——调用方传代码常量时
// 恒通过，未来若误把用户输入接入排序参数则 fail-closed 回落为无排序。
var orderFragmentPattern = regexp.MustCompile(
	`(?i)^[a-z_][a-z0-9_$]*(\.[a-z_][a-z0-9_$]*)*(\s+(asc|desc))?(\s*,\s*[a-z_][a-z0-9_$]*(\.[a-z_][a-z0-9_$]*)*(\s+(asc|desc))?)*$`)

// allowListedOrderFragment 校验裸字符串 ORDER BY 片段；不在白名单形状内返回空串（调用方
// 跳过排序）。gen 构建器路径的排序走类型安全列对象，无需本守卫。
func allowListedOrderFragment(order string) string {
	order = strings.TrimSpace(order)
	if order == "" || !orderFragmentPattern.MatchString(order) {
		return ""
	}
	return order
}

// countAndFindPage 在同一过滤条件上执行 COUNT 与分页查询。
// base 不会被修改（两次查询各自使用独立 Session），page/pageSize 需已归一。
// order 片段先过 allowListedOrderFragment 白名单，非白名单形状静默降级为无排序。
func countAndFindPage(base *gorm.DB, order string, page, pageSize int, dest interface{}) (int64, error) {
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return 0, err
	}
	q := base.Session(&gorm.Session{})
	if order = allowListedOrderFragment(order); order != "" {
		q = q.Order(order)
	}
	if err := q.Offset((page - 1) * pageSize).Limit(pageSize).Find(dest).Error; err != nil {
		return 0, err
	}
	return total, nil
}
