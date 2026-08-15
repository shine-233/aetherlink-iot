// 文件用途：提供 jwt 相关的后端通用工具能力。
// 核心逻辑：封装可复用的格式处理、校验、加密、脚本或命令构造逻辑，供业务层按需调用，主要围绕 var ErrInvalidToken、type JWT、type UserClaims、func NewJWT 等声明展开。
// 关键注意事项：签发和验证固定使用 HS256；不得仅凭密钥类型接受其他 JWT 签名算法。
// 重构建议：后续可按职责继续拆分工具包，减少无关工具之间的隐式耦合。

package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/sirupsen/logrus"
)

// ErrInvalidToken 表示 token 校验失败
var ErrInvalidToken = errors.New("invalid token")

// JWT 封装 JWT 的签名与解析逻辑
type JWT struct {
	Key interface{}
}

// UserClaims 自定义 JWT 声明，嵌入标准声明
type UserClaims struct {
	ID         string    `json:"id"`
	Email      string    `json:"email"`
	CreateTime time.Time `json:"create_time"`
	Authority  string    `json:"authority"`
	TenantID   string    `json:"tenant_id"`
	jwt.StandardClaims
}

// NewJWT 使用给定密钥创建 JWT 实例
func NewJWT(key interface{}) *JWT {
	return &JWT{
		Key: key,
	}
}

// GenerateToken 生成 token，有效期 30 天
func (j *JWT) GenerateToken(claims UserClaims) (string, error) {
	claims.ExpiresAt = time.Now().Add(time.Hour * 24 * 30).Unix()
	tokenClaims := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// 生成token
	return tokenClaims.SignedString(j.Key)
}

// ParseToken 解析并校验 token，返回用户声明
func (j *JWT) ParseToken(token string) (*UserClaims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	tokenClaims, err := parser.ParseWithClaims(token, &UserClaims{}, func(parsedToken *jwt.Token) (interface{}, error) {
		if parsedToken.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return j.Key, nil
	})
	if err != nil {
		logrus.Error(err.Error())
		return nil, err
	}
	if claims, ok := tokenClaims.Claims.(*UserClaims); ok && tokenClaims.Valid {
		return claims, nil
	}
	// 走到这里说明 token 无效但解析未返回 error，返回统一的校验失败错误
	logrus.Error("invalid token: claims validation failed")
	return nil, ErrInvalidToken
}
