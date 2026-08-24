package middleware

import (
	"context"
	"errors"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/dal"
)

var (
	ErrInvalidAPIKey = errors.New("invalid api key")
)

// ErrCodeAPIKeyRateLimited 标识 API key 认证失败次数超限，与 jwt_auth.go 的 4010x 错误码段保持同段递增。
const ErrCodeAPIKeyRateLimited = 40105

const (
	// openAPIKeyAuthFailLimit 是单 IP 在窗口内允许的 API key 认证失败次数上限。
	openAPIKeyAuthFailLimit = 10
	// openAPIKeyAuthFailWindow 是失败计数的固定时间窗口。
	openAPIKeyAuthFailWindow = time.Minute
)

// P1 修复（2026-08-24，见 VALIDATION.md）：认证失败限流——对验证失败的 key 按客户端 IP
// 做固定窗口计数（默认 10 次/分钟），吸收针对无效/已吊销 key 的高频爆破重放；
// 采用进程内 sync.Map 最小实现，不引入新依赖，多副本部署时各副本独立计数（阈值按副本放宽即可）。

type openAPIKeyAuthFailEntry struct {
	mu        sync.Mutex
	count     int
	windowEnd time.Time
}

// openAPIKeyAuthFailCounts 以客户端 IP 为键记录认证失败计数；
// 键空间受活跃来源 IP 约束，过期条目在下次访问时惰性重置，不做后台清扫。
var openAPIKeyAuthFailCounts sync.Map

// openAPIKeyAuthRateLimited 返回该 IP 当前是否已被限流。
func openAPIKeyAuthRateLimited(clientIP string) bool {
	if clientIP == "" {
		return false
	}
	value, ok := openAPIKeyAuthFailCounts.Load(clientIP)
	if !ok {
		return false
	}
	entry, ok := value.(*openAPIKeyAuthFailEntry)
	if !ok {
		return false
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if time.Now().After(entry.windowEnd) {
		return false
	}
	return entry.count >= openAPIKeyAuthFailLimit
}

// recordOpenAPIKeyAuthFailure 记录一次认证失败；窗口首次创建即开始计时，
// 过期后下次写入自动开新窗口。
func recordOpenAPIKeyAuthFailure(clientIP string) {
	if clientIP == "" {
		return
	}
	now := time.Now()
	actual, _ := openAPIKeyAuthFailCounts.LoadOrStore(clientIP, &openAPIKeyAuthFailEntry{windowEnd: now.Add(openAPIKeyAuthFailWindow)})
	entry, ok := actual.(*openAPIKeyAuthFailEntry)
	if !ok {
		return
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if now.After(entry.windowEnd) {
		entry.count = 0
		entry.windowEnd = now.Add(openAPIKeyAuthFailWindow)
	}
	entry.count++
}

type APIKeyInfo struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	Status    int    `json:"status"`
	Name      string `json:"name"`
	CreatedID string `json:"created_id"`
}

// APIKeyValidator is kept as a compatibility wrapper for historical websocket auth.
// The source of truth is dal.VerifyOpenAPIKey, the same path used by OpenAPIKeyAuth.
type APIKeyValidator struct {
	ctx context.Context
}

func NewAPIKeyValidator() *APIKeyValidator {
	return &APIKeyValidator{ctx: context.Background()}
}

func (v *APIKeyValidator) ValidateAPIKey(apiKey string) (*APIKeyInfo, error) {
	tenantID, createdID, err := dal.VerifyOpenAPIKey(v.ctx, apiKey)
	if err != nil {
		return nil, ErrInvalidAPIKey
	}
	return &APIKeyInfo{
		TenantID:  tenantID,
		Status:    1,
		CreatedID: createdID,
	}, nil
}
