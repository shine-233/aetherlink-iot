// 文件用途：守护「数据库驱动原始错误不得进入响应信封」这条安全契约（wave7-A）。
// 核心逻辑：驱动错误原文带表名/列名/约束名/SQLSTATE，统一出口必须把它收敛成通用系统错误，
//          同时保证原文确实进了服务端日志（按 request-id 可追），且不误伤业务错误与
//          gorm.ErrRecordNotFound。
// 关键注意事项：断言同时检查 code 与响应体文本 —— 只看 code 无法证明没有泄漏。
package response

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"aetherlink-iot/backend/pkg/errcode"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// leakMarkers 是绝对不能出现在响应体里的库内部细节。
var leakMarkers = []string{
	"devices",
	"device_number",
	"23505",
	"SQLSTATE",
	"constraint",
	"relation",
	"duplicate key",
}

func assertNoLeak(t *testing.T, body string) {
	t.Helper()
	for _, marker := range leakMarkers {
		assert.NotContains(t, body, marker, "响应体泄漏了库内部细节：%s", marker)
	}
}

func pgUniqueViolation() *pgconn.PgError {
	return &pgconn.PgError{
		Severity:       "ERROR",
		Code:           "23505",
		Message:        `duplicate key value violates unique constraint "devices_device_number_key"`,
		Detail:         "Key (device_number)=(SN-0001) already exists.",
		SchemaName:     "public",
		TableName:      "devices",
		ColumnName:     "device_number",
		ConstraintName: "devices_device_number_key",
	}
}

func TestDatabaseErrorsAreSanitizedBeforeReachingClient(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"pgx PgError", pgUniqueViolation()},
		{"wrapped pgx PgError", fmt.Errorf("insert device: %w", pgUniqueViolation())},
		{"gorm duplicated key", gorm.ErrDuplicatedKey},
		{"wrapped gorm foreign key", fmt.Errorf("save customer: %w", gorm.ErrForeignKeyViolated)},
		{"raw pq text", errors.New(`pq: relation "devices" does not exist`)},
		{"raw sqlstate text", errors.New("ERROR: SQLSTATE 42P01 undefined_table")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := newTestHandler(t)
			recorder, _ := perform(t, handler, "zh_CN", func(c *gin.Context) {
				c.Error(tc.err)
			})

			require.Equal(t, http.StatusOK, recorder.Code)
			body := recorder.Body.String()
			assert.Contains(t, body, fmt.Sprintf(`"code":%d`, errcode.CodeSystemError))
			assertNoLeak(t, body)
		})
	}
}

func TestDatabaseErrorOriginalTextGoesToServerLog(t *testing.T) {
	var logs bytes.Buffer
	original := logrus.StandardLogger().Out
	logrus.SetOutput(&logs)
	t.Cleanup(func() { logrus.SetOutput(original) })

	handler := newTestHandler(t)
	perform(t, handler, "zh_CN", func(c *gin.Context) {
		// 与 middleware.RequestID() 的写法一致：关联标识经校验后写进 gin 上下文。
		// 这里刻意不去读原始 header —— 未经校验的 header 会带来日志注入风险。
		c.Set("X-Request-ID", "req-sanitize-1")
		c.Error(pgUniqueViolation())
	})

	logged := logs.String()
	assert.Contains(t, logged, "devices_device_number_key", "原文必须留在服务端日志里")
	assert.Contains(t, logged, "req-sanitize-1", "日志必须能按 request-id 追查")
}

func TestRecordNotFoundKeepsItsMessage(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "zh_CN", func(c *gin.Context) {
		c.Error(gorm.ErrRecordNotFound)
	})

	body := recorder.Body.String()
	assert.Contains(t, body, fmt.Sprintf(`"code":%d`, errcode.CodeSystemError))
	assert.Contains(t, body, "record not found", "RecordNotFound 文案固定且不含库细节，应保持原样")
}

func TestNonDatabaseErrorKeepsExistingBehaviour(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "zh_CN", func(c *gin.Context) {
		c.Error(errors.New("business rule violated"))
	})

	assert.Contains(t, recorder.Body.String(), "business rule violated", "非数据库错误维持既有行为")
}

func TestErrcodeErrorIsNotRewritten(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "zh_CN", func(c *gin.Context) {
		c.Error(errcode.NewWithMessage(errcode.CodeParamError, "custom-message"))
	})

	body := recorder.Body.String()
	assert.Contains(t, body, "custom-message")
	assert.NotContains(t, body, fmt.Sprintf(`"code":%d`, errcode.CodeSystemError))
}

func TestPanicWithDatabaseErrorIsSanitized(t *testing.T) {
	var logs bytes.Buffer
	original := logrus.StandardLogger().Out
	logrus.SetOutput(&logs)
	t.Cleanup(func() { logrus.SetOutput(original) })

	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "zh_CN", func(c *gin.Context) {
		panic(pgUniqueViolation())
	})

	body := recorder.Body.String()
	assert.Contains(t, body, fmt.Sprintf(`"code":%d`, errcode.CodeSystemError))
	assertNoLeak(t, body)
	assert.Contains(t, logs.String(), "devices_device_number_key")
}

func TestPanicWithPlainValueKeepsItsText(t *testing.T) {
	handler := newTestHandler(t)
	recorder, _ := perform(t, handler, "zh_CN", func(c *gin.Context) {
		panic("boom")
	})

	assert.True(t, strings.Contains(recorder.Body.String(), "boom"), "非数据库 panic 维持既有文案")
}

func TestIsDatabaseErrorClassification(t *testing.T) {
	assert.True(t, isDatabaseError(pgUniqueViolation()))
	assert.True(t, isDatabaseError(gorm.ErrDuplicatedKey))
	assert.True(t, isDatabaseError(fmt.Errorf("outer: %w", pgUniqueViolation())))
	assert.False(t, isDatabaseError(nil))
	assert.False(t, isDatabaseError(gorm.ErrRecordNotFound))
	assert.False(t, isDatabaseError(errors.New("plain business error")))
}
