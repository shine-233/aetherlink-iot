// 文件用途：SQL 分页方言适配纯函数（ROADMAP TP-20）——同一份内层查询按方言生成 LIMIT/OFFSET 或 ROWNUM 包裹分页。
// 核心逻辑：Paginate 对 postgres/polardb/tdengine 在内层查询尾部追加 "LIMIT n OFFSET m"（三者语法一致）；
// kingbase（Oracle 兼容模式）用双层 ROWNUM 包裹：外层按 page_rn > offset 过滤，内层 ROWNUM <= offset+limit 收口。
// PageToOffset 把 1 基页码转成 limit/offset，口径与 internal/dal applyTelemetryPagination 的既有约定一致。
// 关键注意事项：limit/offset 校验为非负有界整数后直接内联为 SQL 字面量（无注入面）；内层查询不允许
// 出现语句分隔符分号（尾部分号会被剥除），防多语句；TDengine 要求 OFFSET 必须与 LIMIT 同现，
// 本函数恒同现；ROWNUM 包裹要求内层查询无重名列（.* 展开的 Oracle 系通用限制）。
// 重构建议：接入真实驱动后如需绑定参数形式的分页，可增加 PaginateWithArgs 变体，本函数签名保持不变。
package dialect

import (
	"errors"
	"fmt"
	"math"
	"strings"
)

// 分页参数哨兵错误（调用方可用 errors.Is 判定）。
var (
	// ErrEmptyInnerQuery 内层查询为空。
	ErrEmptyInnerQuery = errors.New("dialect: empty inner query")
	// ErrInnerSemicolon 内层查询含语句分隔符分号（多语句防护）。
	ErrInnerSemicolon = errors.New("dialect: inner query must not contain statement separator ';'")
	// ErrInvalidLimit limit 超界（须在 [1, 2147483647]）。
	ErrInvalidLimit = errors.New("dialect: pagination limit must be in [1, 2147483647]")
	// ErrInvalidOffset offset 超界（须在 [0, 2147483647]）。
	ErrInvalidOffset = errors.New("dialect: pagination offset must be in [0, 2147483647]")
	// ErrInvalidPage 页码非法（须 >=1）。
	ErrInvalidPage = errors.New("dialect: page number must be >= 1")
	// ErrInvalidPageSize 页大小非法（须 >=1）。
	ErrInvalidPageSize = errors.New("dialect: page size must be >= 1")
	// ErrPageRangeOverflow 页码换算结果超出方言可表达范围。
	ErrPageRangeOverflow = errors.New("dialect: page offset exceeds int32 range")
)

// rownumPageTemplate KingBase（Oracle 兼容模式）双层 ROWNUM 分页模板。
// 参数依次为：内层查询、offset+limit（本页最后一行序号）、offset（跨过的行数）。
// 别名用 page_src/page_rn 前缀命名（Oracle 标识符须以字母开头，不用下划线开头）。
const rownumPageTemplate = "SELECT * FROM (SELECT page_src.*, ROWNUM AS page_rn FROM (%s) page_src WHERE ROWNUM <= %d) WHERE page_rn > %d"

// Paginate 按方言给内层查询生成分页 SQL。
//
// 语义：
//   - postgres/polardb/tdengine：返回 "inner LIMIT limit OFFSET offset"；
//   - kingbase：返回 ROWNUM 双层包裹（标准 Oracle 分页写法）；
//   - 未注册方言、空/多语句内层查询、超界参数：返回错误（fail closed）。
//
// limit/offset 内联为字面量是刻意取舍：三者对 LIMIT/ROWNUM 处的绑定参数支持不一致，
// 且此处入参已被严格校验为整数，无注入面。
func Paginate(d Dialect, inner string, limit, offset int) (string, error) {
	if !d.IsKnown() {
		return "", fmt.Errorf("%w: cannot paginate with %q", ErrUnknownDialect, string(d))
	}
	trimmed := strings.TrimSpace(inner)
	trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
	if trimmed == "" {
		// 剥除尾部分号后再判空：";" 这类只剩分隔符的输入同样视为空查询，
		// 否则会拼出 " LIMIT n OFFSET m" 头部悬空的坏 SQL（单测钉死该边界）。
		return "", ErrEmptyInnerQuery
	}
	if strings.Contains(trimmed, ";") {
		return "", ErrInnerSemicolon
	}
	if limit < 1 || limit > math.MaxInt32 {
		return "", fmt.Errorf("%w: got %d", ErrInvalidLimit, limit)
	}
	if offset < 0 || offset > math.MaxInt32 {
		return "", fmt.Errorf("%w: got %d", ErrInvalidOffset, offset)
	}

	if d.Capabilities().PageStyle == PageStyleRownum {
		endRow := int64(offset) + int64(limit)
		return fmt.Sprintf(rownumPageTemplate, trimmed, endRow, offset), nil
	}
	return fmt.Sprintf("%s LIMIT %d OFFSET %d", trimmed, limit, offset), nil
}

// PageToOffset 把 1 基页码换算为 (limit, offset)：limit=pageSize、offset=(page-1)*pageSize。
// 口径与 internal/dal applyTelemetryPagination 的既有约定一致（page 从 1 计）。
// 换算用 int64 防溢出，超出 int32 可表达范围时报错（对齐 Paginate 的参数上界）。
func PageToOffset(page, pageSize int) (limit, offset int, err error) {
	if page < 1 {
		return 0, 0, fmt.Errorf("%w: got %d", ErrInvalidPage, page)
	}
	if pageSize < 1 {
		return 0, 0, fmt.Errorf("%w: got %d", ErrInvalidPageSize, pageSize)
	}
	o := (int64(page) - 1) * int64(pageSize)
	if o > math.MaxInt32 {
		return 0, 0, fmt.Errorf("%w: page %d size %d", ErrPageRangeOverflow, page, pageSize)
	}
	return pageSize, int(o), nil
}
