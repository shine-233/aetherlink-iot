// 文件用途：P2.1「manifest 注册的 HTTP 运行期路径」运行期证据。
// 核心逻辑：真实 Gin 引擎 + 真实 PostgreSQL，走生产同一条注册路径
// （POST /api/v1/plugins → PluginRegistryService.Create → pluginsdk 校验/验签 → 落库）。
// 关键注意事项：此前该路径"服务层依赖无法离线编译，测试以源码交付"，
// 从无运行期证据；模块缓存恢复后本测试补上这块。签名用 pluginsdk 本尊生成，
// 不在测试里复刻签名算法。
package apps

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/pluginsdk"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const pluginE2ETenant = "pg-plugin-e2e"

// pluginE2E 起真实 Gin + 真实 PostgreSQL（缺 DSN 一律 Skip，不得当通过）。
func pluginE2E(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; plugin registry HTTP E2E skipped")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	prev := global.DB
	global.DB = db
	query.SetDefault(db.Session(&gorm.Session{NewDB: true}))
	// 与 mobile_e2e 同款：cleanup 只还原 global.DB。
	// SetDefault 对同一 gen 单例的二次调用在 nil/重复场景会 panic，不去碰它。
	t.Cleanup(func() {
		global.DB = prev
	})

	handler, err := response.NewHandler("../../configs/messages.yaml", "../../configs/messages_str.yaml")
	if err != nil {
		t.Fatalf("response handler: %v", err)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		c.Set("claims", &utils.UserClaims{
			ID:        "plugin-e2e-user",
			TenantID:  pluginE2ETenant,
			Authority: "SYS_ADMIN",
		})
		c.Next()
	})
	engine.Use(handler.Middleware())
	v1 := engine.Group("/api/v1")
	(&PluginRegistry{}).InitPluginRegistry(v1)
	return engine, db
}

// buildSignedManifest 用 pluginsdk 本尊生成一份带厂商签名的合法 manifest。
func buildSignedManifest(t *testing.T, name, version, keyID string) (pubB64, manifestJSON string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ed25519 key: %v", err)
	}
	manifest := &pluginsdk.Manifest{
		Name:      name,
		Version:   version,
		Title:     "P2.1 runtime e2e plugin",
		Transport: "grpc",
		PointTable: []pluginsdk.PointDefinition{
			{Name: "temperature", Kind: pluginsdk.PointTelemetry},
		},
	}
	if err := pluginsdk.SignManifest(manifest, keyID, priv); err != nil {
		t.Fatalf("sign manifest: %v", err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return base64.StdEncoding.EncodeToString(pub), string(raw)
}

func setTrustedVendorKey(t *testing.T, keyID, pubB64 string) {
	t.Helper()
	prev := viper.Get("plugin.trusted_vendor_keys")
	viper.Set("plugin.trusted_vendor_keys", map[string]string{keyID: pubB64})
	t.Cleanup(func() {
		if prev == nil {
			viper.Set("plugin.trusted_vendor_keys", nil)
		} else {
			viper.Set("plugin.trusted_vendor_keys", prev)
		}
	})
}

func pluginE2EDo(t *testing.T, engine *gin.Engine, body string) map[string]interface{} {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/plugins", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	out := map[string]interface{}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return out
}

func pluginE2ECode(t *testing.T, body map[string]interface{}) float64 {
	t.Helper()
	code, ok := body["code"].(float64)
	if !ok {
		t.Fatalf("response has no numeric code: %v", body)
	}
	return code
}

func TestPluginRegistryManifestHTTPPathOnPostgres(t *testing.T) {
	engine, db := pluginE2E(t)

	// 1) 带合法厂商签名的 manifest → 注册成功，原始 JSON 入库。
	pub, signed := buildSignedManifest(t, "e2e.signed.plugin", "1.0.0", "vendor-a")
	setTrustedVendorKey(t, "vendor-a", pub)
	signedName := fmt.Sprintf("e2e-signed-plugin-%d", timeNowUnix())
	ok := pluginE2EDo(t, engine, fmt.Sprintf(
		`{"name":%s,"version":"1.0.0","manifest":%s}`,
		jsonString(signedName), jsonString(signed)))
	if code := pluginE2ECode(t, ok); code != 200 {
		t.Fatalf("signed manifest registration failed: %v", ok)
	}

	var stored string
	if err := db.Raw("SELECT manifest FROM plugin_registries WHERE name = ?", signedName).Scan(&stored).Error; err != nil || stored == "" {
		t.Fatalf("manifest snapshot must be persisted, got %q err=%v", stored, err)
	}

	// 2) 篡改签名 → 拒绝（坏的签名比没有更危险）。
	tampered := strings.Replace(signed, signed[len(signed)-8:], "AAAAAAAA", 1)
	bad := pluginE2EDo(t, engine, fmt.Sprintf(
		`{"name":"e2e-tampered-%d","version":"1.0.0","manifest":%s}`,
		timeNowUnix(), jsonString(tampered)))
	if code := pluginE2ECode(t, bad); code != 100002 {
		t.Fatalf("tampered signature must be rejected with 100002, got %v", bad)
	}

	// 3) 未受信厂商 → 拒绝。
	otherPub, otherSigned := buildSignedManifest(t, "e2e.rogue.plugin", "1.0.0", "vendor-rogue")
	setTrustedVendorKey(t, "vendor-a", pub)
	_ = otherPub
	rogue := pluginE2EDo(t, engine, fmt.Sprintf(
		`{"name":"e2e-rogue-%d","version":"1.0.0","manifest":%s}`,
		timeNowUnix(), jsonString(otherSigned)))
	if code := pluginE2ECode(t, rogue); code != 100002 {
		t.Fatalf("unknown vendor signature must be rejected with 100002, got %v", rogue)
	}

	// 4) 非法 manifest（transport 白名单外）→ 拒绝。
	invalid := pluginE2EDo(t, engine, fmt.Sprintf(
		`{"name":"e2e-invalid-%d","version":"1.0.0","manifest":%s}`,
		timeNowUnix(), jsonString(`{"name":"e2e.invalid","version":"1.0.0","transport":"http"}`)))
	if code := pluginE2ECode(t, invalid); code != 100002 {
		t.Fatalf("invalid manifest must be rejected with 100002, got %v", invalid)
	}

	// 5) 未签名 manifest → D9 兼容过渡允许注册。
	unsigned := pluginE2EDo(t, engine, fmt.Sprintf(
		`{"name":"e2e-unsigned-%d","version":"1.0.0","manifest":%s}`,
		timeNowUnix(), jsonString(`{"name":"e2e.unsigned","version":"1.0.0","transport":"grpc"}`)))
	if code := pluginE2ECode(t, unsigned); code != 200 {
		t.Fatalf("unsigned manifest registration must stay allowed (D9 transition), got %v", unsigned)
	}

	t.Cleanup(func() {
		db.Exec("DELETE FROM plugin_registries WHERE name LIKE 'e2e-%'")
	})
}

func TestRegisterRealIndustrialAdaptersHTTPPathOnPostgres(t *testing.T) {
	engine, db := pluginE2E(t)

	// 1) 真实工业 CAN 协议适配器 Manifest 签名与 HTTP 注册
	canPub, canPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate can ed25519 key: %v", err)
	}
	canManifest := pluginsdk.CANAdapterManifest()
	if err := pluginsdk.SignManifest(canManifest, "vendor-bosch-industrial", canPriv); err != nil {
		t.Fatalf("sign can manifest: %v", err)
	}
	canRaw, _ := json.Marshal(canManifest)

	// 2) 真实楼宇自控 BACnet 协议适配器 Manifest 签名与 HTTP 注册
	bacnetPub, bacnetPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate bacnet ed25519 key: %v", err)
	}
	bacnetManifest := pluginsdk.BACnetAdapterManifest()
	if err := pluginsdk.SignManifest(bacnetManifest, "vendor-honeywell-hvac", bacnetPriv); err != nil {
		t.Fatalf("sign bacnet manifest: %v", err)
	}
	bacnetRaw, _ := json.Marshal(bacnetManifest)

	// 配置双受信厂商公钥
	prev := viper.Get("plugin.trusted_vendor_keys")
	viper.Set("plugin.trusted_vendor_keys", map[string]string{
		"vendor-bosch-industrial": base64.StdEncoding.EncodeToString(canPub),
		"vendor-honeywell-hvac":   base64.StdEncoding.EncodeToString(bacnetPub),
	})
	t.Cleanup(func() {
		if prev == nil {
			viper.Set("plugin.trusted_vendor_keys", nil)
		} else {
			viper.Set("plugin.trusted_vendor_keys", prev)
		}
	})

	canName := fmt.Sprintf("e2e-can-adapter-%d", timeNowUnix())
	canResp := pluginE2EDo(t, engine, fmt.Sprintf(
		`{"name":%s,"version":"1.0.0","manifest":%s}`,
		jsonString(canName), jsonString(string(canRaw))))
	if code := pluginE2ECode(t, canResp); code != 200 {
		t.Fatalf("can adapter registration failed: %v", canResp)
	}

	var canStored string
	if err := db.Raw("SELECT manifest FROM plugin_registries WHERE name = ?", canName).Scan(&canStored).Error; err != nil || canStored == "" {
		t.Fatalf("can manifest must be persisted in db, got %q err=%v", canStored, err)
	}
	parsedCAN, err := pluginsdk.ParseManifest([]byte(canStored))
	if err != nil {
		t.Fatalf("parse stored can manifest: %v", err)
	}
	if err := pluginsdk.VerifyManifestSignature(parsedCAN, map[string]ed25519.PublicKey{"vendor-bosch-industrial": canPub}); err != nil {
		t.Fatalf("verify stored can manifest signature: %v", err)
	}

	bacnetName := fmt.Sprintf("e2e-bacnet-adapter-%d", timeNowUnix())
	bacnetResp := pluginE2EDo(t, engine, fmt.Sprintf(
		`{"name":%s,"version":"1.0.0","manifest":%s}`,
		jsonString(bacnetName), jsonString(string(bacnetRaw))))
	if code := pluginE2ECode(t, bacnetResp); code != 200 {
		t.Fatalf("bacnet adapter registration failed: %v", bacnetResp)
	}

	var bacnetStored string
	if err := db.Raw("SELECT manifest FROM plugin_registries WHERE name = ?", bacnetName).Scan(&bacnetStored).Error; err != nil || bacnetStored == "" {
		t.Fatalf("bacnet manifest must be persisted in db, got %q err=%v", bacnetStored, err)
	}
	parsedBACnet, err := pluginsdk.ParseManifest([]byte(bacnetStored))
	if err != nil {
		t.Fatalf("parse stored bacnet manifest: %v", err)
	}
	if err := pluginsdk.VerifyManifestSignature(parsedBACnet, map[string]ed25519.PublicKey{"vendor-honeywell-hvac": bacnetPub}); err != nil {
		t.Fatalf("verify stored bacnet manifest signature: %v", err)
	}

	t.Cleanup(func() {
		db.Exec("DELETE FROM plugin_registries WHERE name LIKE 'e2e-%'")
	})
}

// timeNowUnix 让并发/重复执行的名字互不冲突。
func timeNowUnix() int64 {
	return time.Now().UnixNano()
}

// jsonString 把 Go 字符串编码成 JSON 字符串字面量（manifest 内嵌进请求体时必需）。
func jsonString(value string) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}
