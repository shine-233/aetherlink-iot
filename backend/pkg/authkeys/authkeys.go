// 文件用途：认证共享密钥（JWT 签名密钥、插件接入密钥）的唯一读取与规范化入口。
// 核心逻辑：所有签发、校验、启动期检查都经由 Normalize 处理同一配置值，
// 消除"签发侧 TrimSpace、校验侧读原值"导致的密钥不一致（带尾随换行的
// GOTP_JWT_KEY / secret 文件可通过启动检查并签发 token，却使全部 HTTP/WS 校验失败）。
// 关键注意事项：每次调用都重新读取 viper，保证运行期 viper.Set（测试、热更新）立即生效；
// 规范化规则只能在本文件修改，签发/校验/启动检查三处不得各自实现。
package authkeys

import (
	"errors"
	"strings"

	"aetherlink-iot/backend/pkg/utils"

	"github.com/spf13/viper"
)

// 配置键与环境变量（GOTP_ 前缀 + 点号转下划线）一一对应。
const (
	JWTKeyConfigKey           = "jwt.key"
	PluginServiceKeyConfigKey = "plugin.service.key"
)

// ErrJWTKeyEmpty 表示 jwt.key 未配置或规范化后为空。
var ErrJWTKeyEmpty = errors.New("jwt.key is empty")

// Getter 抽象 *viper.Viper 与全局 viper 的读取能力，便于启动检查注入独立实例。
type Getter interface {
	GetString(key string) string
}

type globalViper struct{}

func (globalViper) GetString(key string) string { return viper.GetString(key) }

// Global 返回读取全局 viper 的 Getter。
func Global() Getter { return globalViper{} }

// Normalize 是共享密钥的唯一规范化规则：去除首尾空白（含 secret 文件常见的尾随换行/CRLF）。
func Normalize(raw string) string {
	return strings.TrimSpace(raw)
}

// JWTKeyFrom 从指定配置源读取并规范化 JWT 签名密钥（启动检查使用）。
func JWTKeyFrom(src Getter) string {
	if src == nil {
		return ""
	}
	return Normalize(src.GetString(JWTKeyConfigKey))
}

// PluginServiceKeyFrom 从指定配置源读取并规范化插件接入共享密钥。
func PluginServiceKeyFrom(src Getter) string {
	if src == nil {
		return ""
	}
	return Normalize(src.GetString(PluginServiceKeyConfigKey))
}

// JWTSigningKey 返回运行期签发与校验共用的 JWT 密钥字节；未配置时返回 ErrJWTKeyEmpty。
// 校验侧必须同样处理该错误并拒绝 token：空密钥 HMAC 校验在语义上等同未鉴权。
func JWTSigningKey() ([]byte, error) {
	key := JWTKeyFrom(Global())
	if key == "" {
		return nil, ErrJWTKeyEmpty
	}
	return []byte(key), nil
}

// JWT 返回绑定规范化密钥的 JWT 签发/校验器；签发侧与 HTTP/WS 校验侧统一经此构造。
func JWT() (*utils.JWT, error) {
	key, err := JWTSigningKey()
	if err != nil {
		return nil, err
	}
	return utils.NewJWT(key), nil
}

// PluginServiceKey 返回规范化后的插件接入共享密钥；空串表示未配置（仅放行可信来源）。
func PluginServiceKey() string {
	return PluginServiceKeyFrom(Global())
}
