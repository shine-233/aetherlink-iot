// 文件用途：收敛「数据库驱动原始错误」向客户端的泄漏（wave7-A 安全卫生项）。
// 核心逻辑：gorm / pgx 的错误原文带表名、列名、约束名与 SQLSTATE，属于服务端实现细节。
//          统一响应出口识别这一类错误后，响应侧只回通用系统错误码（100000），
//          完整原文只进服务端日志，并带上 request-id / path / method 便于追查。
// 关键注意事项：只在类型或特征串明确命中时才改写。业务自定义错误，以及
//              gorm.ErrRecordNotFound（文案固定为 "record not found"，不含库细节）
//              一律保持原样，避免误伤既有客户端契约。
package response

import (
	"errors"
	"strings"

	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// gormSentinels 是 gorm 中「驱动级失败或必然携带库内部细节」的哨兵错误。
// 注意不含 gorm.ErrRecordNotFound —— 它单独放行，见 isDatabaseError。
var gormSentinels = []error{
	gorm.ErrInvalidData,
	gorm.ErrInvalidDB,
	gorm.ErrInvalidField,
	gorm.ErrInvalidValue,
	gorm.ErrInvalidValueOfLength,
	gorm.ErrInvalidTransaction,
	gorm.ErrMissingWhereClause,
	gorm.ErrUnsupportedDriver,
	gorm.ErrUnsupportedRelation,
	gorm.ErrPrimaryKeyRequired,
	gorm.ErrModelValueRequired,
	gorm.ErrModelAccessibleFieldsRequired,
	gorm.ErrSubQueryRequired,
	gorm.ErrEmptySlice,
	gorm.ErrPreloadNotAllowed,
	gorm.ErrDuplicatedKey,
	gorm.ErrForeignKeyViolated,
	gorm.ErrCheckConstraintViolated,
}

// databaseDetailMarkers 是驱动错误原文的稳定特征串。
// 仅在类型判定失效（错误被 %w/%v 包装、或驱动直接返回裸 error）时作为兜底使用：
// 宁可漏判（保持原样）也不误伤业务自定义错误。
var databaseDetailMarkers = []string{
	"SQLSTATE",
	"ERROR: ",
	"pq: ",
	"pgx: ",
	"gorm: ",
	"UNIQUE constraint",
	"duplicate key value",
	"violates ",
	`constraint "`,
	`relation "`,
	`column "`,
	`table "`,
	"syntax error at or near",
	"invalid input syntax for",
	"too many connections",
}

// isDatabaseError 判断 err 是否属于需要脱敏的数据库驱动错误族。
func isDatabaseError(err error) bool {
	if err == nil {
		return false
	}
	// gorm.ErrRecordNotFound 文案固定且不含库内部细节，放行。
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return true
	}

	for _, sentinel := range gormSentinels {
		if errors.Is(err, sentinel) {
			return true
		}
	}

	return containsDatabaseDetail(err.Error())
}

func containsDatabaseDetail(message string) bool {
	for _, marker := range databaseDetailMarkers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// logDatabaseError 把原始错误原文记到服务端日志（不返回客户端），带请求上下文。
// 所有字段值与错误文本经 SanitizeForLog 净化：URL.Path 可携带外部可控内容，
// 直接落日志可被 CRLF 伪造日志行（CodeQL go/log-injection）。
func logDatabaseError(c *gin.Context, err error) {
	fields := logrus.Fields{"request_id": utils.SanitizeForLog(c.GetString("X-Request-ID"))}
	if c.Request != nil {
		fields["method"] = utils.SanitizeForLog(c.Request.Method)
		fields["path"] = utils.SanitizeForLog(c.Request.URL.Path)
	}
	if route := c.FullPath(); route != "" {
		fields["route"] = utils.SanitizeForLog(route)
	}
	logrus.WithError(errors.New(utils.SanitizeForLog(err.Error()))).WithFields(fields).Error("database error sanitized before reaching the client")
}

// systemErrorFor 把任意错误收敛成可安全返回客户端的 *errcode.Error：
// 数据库驱动错误 → 通用系统错误码 + 原文仅入日志；其余错误维持既有行为（保留原文）。
func systemErrorFor(c *gin.Context, err error) *errcode.Error {
	if isDatabaseError(err) {
		logDatabaseError(c, err)
		return errcode.New(errcode.CodeSystemError)
	}
	return errcode.NewWithMessage(errcode.CodeSystemError, err.Error())
}
