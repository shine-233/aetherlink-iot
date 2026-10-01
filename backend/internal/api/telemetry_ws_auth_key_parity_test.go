// 文件用途：遥测 WebSocket 首消息认证（validateToken）与登录签发侧的 JWT 密钥同源回归测试。
// 核心逻辑：jwt.key 带尾随换行时，经 UserLoginAfter 签发的 token 必须能通过 WS 校验；
// 修复前 WS 侧读取未规范化的原值，所有 WS 连接认证失败为 invalid token。
// 关键注意事项：miniredis + 内存 sqlite，全局状态在 Cleanup 中复原。

package api

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/authkeys"
	"aetherlink-iot/backend/pkg/global"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
)

func setViperForWSTest(t *testing.T, key string, value interface{}) {
	t.Helper()
	old := viper.Get(key)
	viper.Set(key, value)
	t.Cleanup(func() { viper.Set(key, old) })
}

func TestTelemetryWSValidateTokenAcceptsTokenSignedWithTrailingNewlineKey(t *testing.T) {
	db := setupTelemetryWSUserStatusDB(t)
	query.SetDefault(db)
	seedTelemetryWSUser(t, db, "ws-parity-user", stringPtrForWS("N"))

	server := miniredis.RunT(t)
	oldRedis := global.REDIS
	global.REDIS = redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() {
		_ = global.REDIS.Close()
		global.REDIS = oldRedis
	})
	setViperForWSTest(t, "session.timeout", 60)
	setViperForWSTest(t, authkeys.JWTKeyConfigKey, "openssl-rand-base64-sample-key-with-48-bytes!!xy\n")

	var user model.User
	if err := db.Where("id = ?", "ws-parity-user").First(&user).Error; err != nil {
		t.Fatalf("load user: %v", err)
	}
	rsp, err := service.GroupApp.User.UserLoginAfter(&user)
	if err != nil || rsp == nil || rsp.Token == nil {
		t.Fatalf("UserLoginAfter = (%+v, %v)", rsp, err)
	}

	claims, err := validateToken(*rsp.Token)
	if err != nil {
		t.Fatalf("WS validateToken rejected a freshly issued token: %v", err)
	}
	if claims.ID != "ws-parity-user" {
		t.Fatalf("claims.ID = %q, want ws-parity-user", claims.ID)
	}

	// 密钥清空后 WS 侧必须 fail-closed。
	viper.Set(authkeys.JWTKeyConfigKey, "\n")
	if _, err := validateToken(*rsp.Token); err == nil {
		t.Fatal("WS validateToken must reject tokens when jwt.key is empty")
	}
}
