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

	active, invalidateToken := ValidateJWTUserStatus(ctx, claims)
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

// ValidateJWTUserStatus 校验 token 声明对应用户是否处于正常状态，HTTP 与 WebSocket 认证链路共用。
// 返回 active 表示用户可用；invalidateToken 表示该 token 已失效，应从 Redis 中清除。
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

	tenantID, createdID, err := dal.VerifyOpenAPIKey(context.Background(), appKey)
	if err != nil {
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
