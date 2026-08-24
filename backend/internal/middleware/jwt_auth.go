// 文件用途：提供 HTTP 请求链路中的 jwt auth 中间件能力。
// 核心逻辑：在 Gin 请求处理前后执行认证、鉴权、跨域、指标、响应包装或操作日志处理，主要围绕 type ErrorResponse、func JWTAuth、func isValidJWT、func ValidateJWTUserStatus 等声明展开。
// 关键注意事项：中间件位于安全与兼容边界，修改需保持状态码、上下文键和响应格式稳定；ValidateJWTUserStatus 同时被 WebSocket 链路复用，语义变更需同步两端。
// 重构建议：后续可将外部依赖抽成接口，便于独立测试和不同部署模式复用。

package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/constant"
	"aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

const (
	ErrCodeNoAuth         = 40100
	ErrCodeInvalidToken   = 40101
	ErrCodeTokenExpired   = 40102
	ErrCodeInvalidAPIKey  = 40103
	ErrCodeAPIKeyDisabled = 40104
)

type ErrorResponse struct {
	Code      int    `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// JWTAuth checks JWT tokens first and falls back to OpenAPI key auth.
func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := selectJWTAuthToken(c, c.Request.Header.Get("x-token"))
		if token != "" {
			if isValidJWT(c, token) {
				c.Next()
				return
			}
			return
		}

		if !OpenAPIKeyAuth(c) {
			return
		}

		c.Next()
	}
}

func isValidJWT(c *gin.Context, token string) bool {
	requestID := c.GetString("X-Request-ID")
	ctx := context.Background()

	if global.REDIS == nil || global.REDIS.Get(ctx, token).Val() != "1" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:      ErrCodeTokenExpired,
			Message:   "token has expired",
			RequestID: requestID,
		})
		c.Abort()
		return false
	}

	key := viper.GetString("jwt.key")
	j := utils.NewJWT([]byte(key))
	claims, err := j.ParseToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:      ErrCodeInvalidToken,
			Message:   "invalid token format",
			RequestID: requestID,
		})
		c.Abort()
		return false
	}

	active, invalidateToken := cachedJWTUserStatus(ctx, claims)
	if !active {
		if invalidateToken {
			DeleteInvalidJWTToken(ctx, token)
		}
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:      ErrCodeInvalidToken,
			Message:   "no permission",
			RequestID: requestID,
		})
		c.Abort()
		return false
	}

	timeout := viper.GetInt("session.timeout")
	logrus.Debugf("refresh token expiration: %d minutes", timeout)
	global.REDIS.Set(ctx, token, "1", time.Duration(timeout)*time.Minute)

	c.Set("claims", claims)
	return true
}

// jwtUserStatusCacheTTL 控制用户认证状态的进程内缓存时长。
// 权衡：用户被禁用/删除后，旧 token 最多仍可用约 TTL 时长；该窗口远小于
// token 本身的有效期，且换来认证热路径不再每请求打一次 users 表。
const jwtUserStatusCacheTTL = 30 * time.Second

type jwtUserStatusEntry struct {
	active          bool
	invalidateToken bool
	expiresAt       time.Time
}

// jwtUserStatusCache 以用户 ID 为键缓存最近的确定性状态。
// 键空间受用户总量约束（有限集合、小结构体），无需额外淘汰逻辑。
var jwtUserStatusCache sync.Map

// cachedJWTUserStatus 在 ValidateJWTUserStatus 之上加进程内 TTL 缓存，
// 消除每个认证请求一次的 users 表查询。禁用/删除用户最迟在 TTL 后生效。
// 仅缓存确定性结论：(true,false)=正常用户；(false,true)=不存在或被禁用。
// 数据库瞬时故障返回 (false,false)，不缓存，避免故障期间把全体请求判为未认证。
func cachedJWTUserStatus(ctx context.Context, claims *utils.UserClaims) (bool, bool) {
	if claims != nil && claims.ID != "" {
		if value, ok := jwtUserStatusCache.Load(claims.ID); ok {
			if entry, ok := value.(jwtUserStatusEntry); ok && time.Now().Before(entry.expiresAt) {
				return entry.active, entry.invalidateToken
			}
		}
	}

	active, invalidateToken := ValidateJWTUserStatus(ctx, claims)
	if claims != nil && claims.ID != "" && (active || invalidateToken) {
		jwtUserStatusCache.Store(claims.ID, jwtUserStatusEntry{
			active:          active,
			invalidateToken: invalidateToken,
			expiresAt:       time.Now().Add(jwtUserStatusCacheTTL),
		})
	}
	return active, invalidateToken
}

func ValidateJWTUserStatus(ctx context.Context, claims *utils.UserClaims) (active bool, invalidateToken bool) {
	if claims == nil || claims.ID == "" {
		return false, true
	}
	if global.DB == nil {
		logrus.Warn("jwt user status check skipped: database is not initialized")
		return false, false
	}

	var user model.User
	err := global.DB.WithContext(ctx).
		Select("id", "status").
		Where("id = ?", claims.ID).
		First(&user).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logrus.Warn("jwt user status check failed")
			return false, false
		}
		return false, true
	}
	if user.Status == nil || *user.Status != "N" {
		return false, true
	}
	return true, false
}

// DeleteInvalidJWTToken 删除 Redis 中已失效的 JWT token，供 HTTP 与 WebSocket 认证链路复用。
func DeleteInvalidJWTToken(ctx context.Context, token string) {
	if global.REDIS == nil {
		return
	}
	if err := global.REDIS.Del(ctx, token).Err(); err != nil {
		logrus.Warn("delete invalid jwt token from redis failed")
	}
}

// OpenAPIKeyAuth verifies an OpenAPI key and stores equivalent claims.
func OpenAPIKeyAuth(c *gin.Context) bool {
	requestID := c.GetString("X-Request-ID")

	appKey := c.Request.Header.Get("x-api-key")
	if appKey == "" {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:      ErrCodeNoAuth,
			Message:   "missing authentication (x-token or x-api-key required)",
			RequestID: requestID,
		})
		c.Abort()
		return false
	}

	// P1 修复（2026-08-24，见 VALIDATION.md）：认证失败限流——先查 IP 失败窗口，
	// 超限直接拒绝，不再触发 key 校验链路。
	clientIP := c.ClientIP()
	if openAPIKeyAuthRateLimited(clientIP) {
		c.JSON(http.StatusTooManyRequests, ErrorResponse{
			Code:      ErrCodeAPIKeyRateLimited,
			Message:   "too many failed api key attempts, retry later",
			RequestID: requestID,
		})
		c.Abort()
		return false
	}

	tenantID, createdID, err := dal.VerifyOpenAPIKey(context.Background(), appKey)
	if err != nil {
		recordOpenAPIKeyAuthFailure(clientIP)
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Code:      ErrCodeInvalidAPIKey,
			Message:   "api key verification failed",
			RequestID: requestID,
		})
		c.Abort()
		return false
	}

	c.Set("claims", openAPIKeyClaims(tenantID, createdID))
	return true
}

func openAPIKeyClaims(tenantID string, createdID string) *utils.UserClaims {
	return &utils.UserClaims{
		TenantID:  tenantID,
		Authority: openAPIKeyAuthority(),
		ID:        createdID,
	}
}

// openAPIKeyAuthority 读取 OpenAPI Key 等效 claims 的权限。
// open_api_keys 表没有独立的权限/scope 字段，因此统一取环境变量
// GOTP_OPENAPI_KEY_AUTHORITY（viper 键 openapi.key.authority），
// 未配置时保持默认 TENANT_ADMIN 以兼容存量部署，运维可下调为 TENANT_USER 等。
func openAPIKeyAuthority() string {
	if authority := strings.TrimSpace(viper.GetString("openapi.key.authority")); authority != "" {
		return authority
	}
	return constant.TENANT_ADMIN
}
