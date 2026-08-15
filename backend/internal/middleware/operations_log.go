// 文件用途：提供 HTTP 请求链路中的 operations log 中间件能力。
// 核心逻辑：在 Gin 请求处理前后执行认证、鉴权、跨域、指标、响应包装或操作日志处理，主要围绕 var sensitiveFieldPattern、func OperationLogs、func isModifyMethod、func processRequestBody 等声明展开。
// 关键注意事项：中间件位于安全与兼容边界，修改需保持状态码、上下文键和响应格式稳定。
// 重构建议：后续可将外部依赖抽成接口，便于独立测试和不同部署模式复用。

package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-basic/uuid"
	"github.com/sirupsen/logrus"
)

const (
	maxLoggedBodyBytes          = 4096
	redactedValue               = "[REDACTED]"
	operationLogSafeMetadataKey = "operation_log_safe_metadata"
)

type operationLogCaptureMode int

const (
	operationLogCaptureNone operationLogCaptureMode = iota
	operationLogCaptureMetadataOnly
	operationLogCaptureFull
)

var sensitiveFieldPattern = regexp.MustCompile(`(?i)(("?(password|passwd|pwd|token|secret|api[_-]?key|apikey|authorization|voucher|private[_-]?key)"?\s*[:=]\s*)(bearer\s+)?)(("[^"]*")|([^&,\r\n,}]+))`)

func OperationLogs() gin.HandlerFunc {
	return func(c *gin.Context) {
		captureMode := operationLogCaptureModeFor(c.Request.Method, c.Request.URL.Path)
		if captureMode == operationLogCaptureNone {
			c.Next()
			return
		}

		if captureMode == operationLogCaptureMetadataOnly {
			start := time.Now().UTC()
			c.Next()
			cost := time.Since(start).Milliseconds()
			requestMessage := operationLogMetadataMessage(c)
			responseMessage := fmt.Sprintf(
				"[response body capture skipped by operation log policy; status=%d bytes=%d]",
				c.Writer.Status(),
				c.Writer.Size(),
			)
			logrus.Info("operation completed status=", c.Writer.Status(), " latency_ms=", cost, " response_bytes=", c.Writer.Size())
			saveOperationLog(c, start, cost, requestMessage, responseMessage)
			return
		}

		requestCapture := processRequestBody(c)

		writer := newResponseBodyWriter(c)
		c.Writer = writer

		start := time.Now().UTC()
		c.Next()
		cost := time.Since(start).Milliseconds()
		requestMessage := requestCapture.Message()
		responseMessage := writer.Message()

		logrus.Info("operation completed status=", c.Writer.Status(), " latency_ms=", cost, " response_bytes=", writer.body.Len())
		logrus.Info("operation request body: ", requestMessage)
		logrus.Info("operation response body: ", responseMessage)

		saveOperationLog(c, start, cost, requestMessage, responseMessage)
	}
}

// SetOperationLogSafeMetadata lets a metadata-only handler retain a small
// audit summary without allowing request or response bodies back into logs.
// Callers must never include secrets or opaque user payloads.
func SetOperationLogSafeMetadata(c *gin.Context, metadata map[string]interface{}) {
	if c == nil || len(metadata) == 0 {
		return
	}
	c.Set(operationLogSafeMetadataKey, metadata)
}

func operationLogMetadataMessage(c *gin.Context) string {
	if c == nil {
		return "[request body capture skipped by operation log policy]"
	}
	metadata, exists := c.Get(operationLogSafeMetadataKey)
	if !exists {
		return "[request body capture skipped by operation log policy]"
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return "[safe operation metadata unavailable]"
	}
	return sanitizeLogMessage(string(encoded))
}

func isModifyMethod(method string) bool {
	return method == http.MethodPost ||
		method == http.MethodPut ||
		method == http.MethodDelete
}

func operationLogCaptureModeFor(method, path string) operationLogCaptureMode {
	if !isModifyMethod(method) || isOperationLogExcludedPath(path) {
		return operationLogCaptureNone
	}
	if isOperationLogMetadataOnlyPath(path) {
		return operationLogCaptureMetadataOnly
	}
	return operationLogCaptureFull
}

func isOperationLogExcludedPath(path string) bool {
	for _, excludedPath := range operationLogExcludedExactPaths {
		if path == excludedPath {
			return true
		}
	}
	for _, excludedPrefix := range operationLogExcludedPathPrefixes {
		if strings.HasPrefix(path, excludedPrefix) {
			return true
		}
	}
	return false
}

var operationLogExcludedExactPaths = []string{
	"/api/v1/login",
	// Password recovery credentials must never enter application or database logs.
	"/api/v1/reset/password/link",
	"/api/v1/reset/password",
	"/api/v1/tenant/email/register",
	"/api/v1/tenant/super-admin/init",
	"/api/v1/tenant/market-register",
	"/api/v1/device/gateway-register",
	"/api/v1/device/gateway-sub-register",
	"/api/v1/device/auth",
}

var operationLogExcludedPathPrefixes = []string{
	"/api/v1/plugin/",
}

func isOperationLogMetadataOnlyPath(path string) bool {
	if isDeviceMQTTDebugPath(path) {
		return true
	}
	for _, metadataOnlyPath := range operationLogMetadataOnlyExactPaths {
		if path == metadataOnlyPath {
			return true
		}
	}
	for _, metadataOnlyPrefix := range operationLogMetadataOnlyPathPrefixes {
		if strings.HasPrefix(path, metadataOnlyPrefix) {
			return true
		}
	}
	return false
}

func isDeviceMQTTDebugPath(path string) bool {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	return len(segments) >= 6 &&
		segments[0] == "api" &&
		segments[1] == "v1" &&
		segments[2] == "device" &&
		segments[3] != "" &&
		segments[4] == "mqtt-debug" &&
		segments[5] == "session"
}

var operationLogMetadataOnlyExactPaths = []string{
	"/api/v1/board/update/password",
	"/api/v1/user/change-email",
	"/api/v1/device/update/voucher",
	"/api/v1/device/service/access/batch",
	"/api/v1/command/datas/pub",
	"/api/v1/telemetry/datas/pub",
	"/api/v1/attribute/datas/pub",
	"/api/v1/command/datas/jobs/preview",
	"/api/v1/command/datas/jobs/submit",
	"/api/v1/device/template/market/login",
	"/api/v1/notification/services/config/e-mail/test",
}

var operationLogMetadataOnlyPathPrefixes = []string{
	"/api/v1/open/keys",
	"/api/v1/command/datas/jobs/",
	"/api/v1/ota/task",
	"/api/v1/service/access",
	"/api/v1/service/plugin",
	"/api/v1/notification/services/config",
	"/api/v1/data_script",
	"/api/v1/rdi/devices/",
	"/api/v1/rdi/share-tokens/",
}

func processRequestBody(c *gin.Context) *bodyLogCapture {
	if isMultipartRequest(c) {
		return &bodyLogCapture{fixedMessage: fmt.Sprintf("[multipart upload request: %s]", c.Request.URL.Path)}
	}

	capture := &bodyLogCapture{body: c.Request.Body}
	c.Request.Body = capture
	return capture
}

func sanitizeLogMessage(message string) string {
	if message == "" {
		return ""
	}

	if sanitized, ok := sanitizeJSONLogMessage(message); ok {
		return truncateLogMessage(sanitized)
	}

	return truncateLogMessage(sensitiveFieldPattern.ReplaceAllString(message, `${1}`+redactedValue))
}

func sanitizeJSONLogMessage(message string) (string, bool) {
	var payload interface{}
	decoder := json.NewDecoder(strings.NewReader(message))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return "", false
	}

	redactJSONValue(payload)
	sanitized, err := json.Marshal(payload)
	if err != nil {
		return "", false
	}
	return string(sanitized), true
}

func redactJSONValue(value interface{}) {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			if isSensitiveLogKey(key) {
				typed[key] = redactedValue
				continue
			}
			redactJSONValue(child)
		}
	case []interface{}:
		for _, child := range typed {
			redactJSONValue(child)
		}
	}
}

func isSensitiveLogKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
	return strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "passwd") ||
		normalized == "pwd" ||
		strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "authorization") ||
		strings.Contains(normalized, "voucher") ||
		strings.Contains(normalized, "privatekey")
}

func truncateLogMessage(message string) string {
	if len(message) <= maxLoggedBodyBytes {
		return message
	}
	return message[:maxLoggedBodyBytes] + "...[truncated]"
}

func isMultipartRequest(c *gin.Context) bool {
	contentType := c.Request.Header.Get("Content-Type")
	return strings.HasPrefix(contentType, "multipart/form-data")
}

func saveOperationLog(c *gin.Context, start time.Time, cost int64, requestMsg, responseMsg string) {
	claims, exists := c.Get("claims")
	if !exists {
		logrus.Info("operation log skipped: missing user claims")
		return
	}

	userClaims, ok := claims.(*utils.UserClaims)
	if !ok {
		logrus.Info("operation log skipped: invalid user claims type")
		return
	}

	if userClaims.TenantID == "" {
		logrus.Info("operation log skipped: empty tenant id")
		return
	}

	path := safeOperationLogPath(c.Request.URL.Path)

	log := &model.OperationLog{
		ID:              uuid.New(),
		IP:              c.ClientIP(),
		Path:            &path,
		UserID:          userClaims.ID,
		Name:            &c.Request.Method,
		CreatedAt:       start,
		Latency:         &cost,
		RequestMessage:  &requestMsg,
		ResponseMessage: &responseMsg,
		TenantID:        userClaims.TenantID,
	}

	if err := query.OperationLog.Create(log); err != nil {
		logrus.Warnf("save operation log failed: %v", err)
	}
}

func safeOperationLogPath(path string) string {
	if strings.HasPrefix(path, "/api/v1/rdi/share-tokens/") {
		parts := strings.Split(path, "/")
		if len(parts) >= 6 {
			parts[5] = redactedValue
			return strings.Join(parts, "/")
		}
	}
	return path
}

type responseBodyWriter struct {
	gin.ResponseWriter
	body      *bytes.Buffer
	truncated bool
}

func newResponseBodyWriter(c *gin.Context) *responseBodyWriter {
	return &responseBodyWriter{
		ResponseWriter: c.Writer,
		body:           &bytes.Buffer{},
	}
}

func (r *responseBodyWriter) Write(b []byte) (int, error) {
	r.capture(b)
	return r.ResponseWriter.Write(b)
}

func (r *responseBodyWriter) Message() string {
	message := sanitizeLogMessage(r.body.String())
	if r.truncated && !strings.HasSuffix(message, "...[truncated]") {
		return message + "...[truncated]"
	}
	return message
}

func (r *responseBodyWriter) capture(b []byte) {
	remaining := maxLoggedBodyBytes - r.body.Len()
	if remaining <= 0 {
		r.truncated = true
		return
	}
	if len(b) > remaining {
		r.body.Write(b[:remaining])
		r.truncated = true
		return
	}
	r.body.Write(b)
}

type bodyLogCapture struct {
	body         io.ReadCloser
	buffer       bytes.Buffer
	truncated    bool
	readErr      error
	fixedMessage string
}

func (b *bodyLogCapture) Read(p []byte) (int, error) {
	if b.body == nil {
		return 0, io.EOF
	}
	n, err := b.body.Read(p)
	if n > 0 {
		b.capture(p[:n])
	}
	if err != nil && err != io.EOF {
		b.readErr = err
	}
	return n, err
}

func (b *bodyLogCapture) Close() error {
	if b.body == nil {
		return nil
	}
	return b.body.Close()
}

func (b *bodyLogCapture) Message() string {
	if b.fixedMessage != "" {
		return b.fixedMessage
	}
	if b.readErr != nil {
		return "[request body read error]"
	}
	message := sanitizeLogMessage(b.buffer.String())
	if b.truncated && !strings.HasSuffix(message, "...[truncated]") {
		return message + "...[truncated]"
	}
	return message
}

func (b *bodyLogCapture) capture(chunk []byte) {
	remaining := maxLoggedBodyBytes - b.buffer.Len()
	if remaining <= 0 {
		b.truncated = true
		return
	}
	if len(chunk) > remaining {
		b.buffer.Write(chunk[:remaining])
		b.truncated = true
		return
	}
	b.buffer.Write(chunk)
}
