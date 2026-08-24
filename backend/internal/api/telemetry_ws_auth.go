package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"aetherlink-iot/backend/internal/middleware"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func validateToken(token string) (*utils.UserClaims, error) {
	ctx := context.Background()
	// P3 修复（2026-08-24，见 VALIDATION.md）：与 HTTP 中间件一致，Redis 键使用 token 摘要。
	tokenKey := utils.TokenDigest(token)
	if global.REDIS.Get(ctx, tokenKey).Val() != "1" {
		return nil, errors.New("token is expired")
	}

	key := viper.GetString("jwt.key")
	j := utils.NewJWT([]byte(key))
	claims, err := j.ParseToken(token)
	if err != nil {
		return nil, errors.New("invalid token")
	}
	if err := checkTelemetryJWTUserStatus(ctx, token, claims); err != nil {
		return nil, err
	}

	timeout := viper.GetInt("session.timeout")
	if timeout == 0 {
		timeout = 60
	}
	global.REDIS.Set(ctx, tokenKey, "1", time.Duration(timeout)*time.Minute)

	return claims, nil
}

// checkTelemetryJWTUserStatus 复用 HTTP 链路的用户状态校验：
// 被禁用或已删除的账号即使 token 未过期，也不允许访问遥测 WebSocket。
func checkTelemetryJWTUserStatus(ctx context.Context, token string, claims *utils.UserClaims) error {
	active, invalidateToken := middleware.ValidateJWTUserStatus(ctx, claims)
	if active {
		return nil
	}
	if invalidateToken {
		middleware.DeleteInvalidJWTToken(ctx, token)
	}
	return errors.New("no permission")
}

func validateAPIKey(apiKey string) (*utils.UserClaims, error) {
	info, err := middleware.NewAPIKeyValidator().ValidateAPIKey(apiKey)
	if err != nil {
		return nil, err
	}

	return telemetryTenantAdminReadClaims(info.TenantID, info.CreatedID), nil
}

var (
	validateTelemetryTokenFn  = validateToken
	validateTelemetryAPIKeyFn = validateAPIKey
)

type telemetryAuthRequest struct {
	values map[string]interface{}
}

func newTelemetryAuthRequest(msgMap map[string]interface{}) telemetryAuthRequest {
	values := make(map[string]interface{}, len(msgMap))
	for key, value := range msgMap {
		values[strings.ToLower(key)] = value
	}
	return telemetryAuthRequest{values: values}
}

func (r telemetryAuthRequest) stringValue(keys ...string) string {
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			if strValue, isStr := value.(string); isStr && strValue != "" {
				return strValue
			}
			return fmt.Sprintf("%v", value)
		}
	}
	return ""
}

func (r telemetryAuthRequest) tokenProvided() bool {
	return r.stringValue("token", "authorization") != ""
}

func (r telemetryAuthRequest) tokenValue() string {
	token := r.stringValue("token")
	if token != "" {
		return token
	}
	return strings.TrimPrefix(r.stringValue("authorization"), "Bearer ")
}

func telemetryAPIKeyCandidateNames() []string {
	return []string{"x-api-key", "x_api_key", "xapikey", "apikey"}
}

func validateTelemetryTokenAuth(req telemetryAuthRequest) (*utils.UserClaims, error, bool) {
	tokenProvided := req.tokenProvided()
	if !tokenProvided {
		return nil, nil, false
	}

	token := req.tokenValue()
	if token == "" {
		return nil, nil, true
	}

	claims, err := validateTelemetryTokenFn(token)
	if err == nil {
		return claims, nil, true
	}

	logrus.Warnf("Token validation failed: %v", err)
	return nil, err, true
}

func validateTelemetryAPIKeyAuth(req telemetryAuthRequest) (*utils.UserClaims, error, bool) {
	var apiKeyErr error
	apiKeyProvided := false

	for _, key := range telemetryAPIKeyCandidateNames() {
		apiKey := req.stringValue(key)
		if apiKey == "" {
			continue
		}

		apiKeyProvided = true
		claims, err := validateTelemetryAPIKeyFn(apiKey)
		if err == nil {
			return claims, nil, true
		}

		apiKeyErr = err
		logrus.Warnf("API Key validation failed for key %s: %v", key, err)
	}

	return nil, apiKeyErr, apiKeyProvided
}

func telemetryTenantAdminReadClaims(tenantID string, userID string) *utils.UserClaims {
	return &utils.UserClaims{
		TenantID:  tenantID,
		Authority: "TENANT_ADMIN",
		ID:        userID,
	}
}

func telemetryAuthFailure(tokenProvided bool, tokenErr error, apiKeyProvided bool, apiKeyErr error) error {
	switch {
	case tokenErr != nil && !apiKeyProvided:
		return tokenErr
	case apiKeyErr != nil && !tokenProvided:
		return apiKeyErr
	case tokenErr != nil && apiKeyErr != nil:
		return fmt.Errorf("token validation failed: %v; api key validation failed: %v", tokenErr, apiKeyErr)
	case !tokenProvided && !apiKeyProvided:
		return errors.New("authentication failed: token or x-api-key is required")
	default:
		return errors.New("authentication failed")
	}
}

func validateAuth(msgMap map[string]interface{}) (*utils.UserClaims, error) {
	req := newTelemetryAuthRequest(msgMap)

	claims, tokenErr, tokenProvided := validateTelemetryTokenAuth(req)
	if claims != nil {
		return claims, nil
	}

	claims, apiKeyErr, apiKeyProvided := validateTelemetryAPIKeyAuth(req)
	if claims != nil {
		return claims, nil
	}

	return nil, telemetryAuthFailure(tokenProvided, tokenErr, apiKeyProvided, apiKeyErr)
}

func telemetryWSStringValue(msgMap map[string]interface{}, keys ...string) string {
	for _, wantKey := range keys {
		for key, value := range msgMap {
			if !strings.EqualFold(key, wantKey) {
				continue
			}
			switch v := value.(type) {
			case string:
				if s := strings.TrimSpace(v); s != "" {
					return s
				}
			default:
				if s := strings.TrimSpace(fmt.Sprintf("%v", v)); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func keysOfMap(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
