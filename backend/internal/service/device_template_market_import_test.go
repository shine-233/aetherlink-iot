// 文件用途：P1.6 打包导入全链的运行期证据（ROADMAP P1.6）。
// 覆盖：出签名密钥后验签拒绝未签名/篡改包、预览区分 create/overwrite、
// 导入逐模板 created、同包重导全幂等命中、阻断项（包内重名）拒绝、覆盖未确认拒绝。
// 说明：需要真实 PostgreSQL（device_templates 表）+ 配置签名密钥。
// 缺 DSN 或缺表一律 Skip，不得把 Skip 当作通过。
package service

import (
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/spf13/viper"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openMarketImportPostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("AETHERLINK_TEST_PSQL_DSN"))
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; market import tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	global.DB = db
	// README 约定：global.DB 之外还要 SetDefault，gen 查询对象否则为 nil。
	// 漏掉这一步时服务层走 gen 查询会拿到 nil 并返回参数错误（100002），
	// 而缺 DSN 时用例被 Skip，问题会被完全掩盖。
	query.SetDefault(db)
	var exists bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema='public' AND table_name='device_templates')").Scan(&exists).Error; err != nil {
		t.Fatalf("probe device_templates: %v", err)
	}
	if !exists {
		t.Skip("device_templates missing; apply migrations first")
	}
	return db
}

func signedTestBundle(t *testing.T, name string, count int) *model.MarketBundle {
	t.Helper()
	// viper 是进程级全局，必须注册清理：不还原会把"已配置签名密钥"泄漏给后续用例，
	// 使 integrity 的"未配置密钥必须拒绝出包"断言失真（此前正是这样失败的）。
	viper.Set(marketActiveSigningKeyKey, "test-key")
	viper.Set(marketBundleSigningKeysKey+".test-key", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWYxMjM0NTY3ODlhYmNkZWYxMjM0NTY3ODlhYmNkZWY=")
	t.Cleanup(func() {
		viper.Set(marketActiveSigningKeyKey, "")
		viper.Set(marketBundleSigningKeysKey+".test-key", "")
	})
	templates := make([]*model.DeviceTemplateExport, 0, count)
	for i := 0; i < count; i++ {
		version := "9.9.9"
		// 模板必须声明与包一致的 type_key：CheckMarketBundleDependencies 把它作为
		// 阻断项（"结果取决于顺序"的问题确认多少遍也改变不了），缺了会在导入/预览
		// 阶段直接以参数错误拒绝。
		typeKey := "test-industry"
		templates = append(templates, &model.DeviceTemplateExport{
			Kind: "aetherlink-device-template", Name: name,
			Version: &version, TypeKey: &typeKey,
		})
	}
	bundle := &model.MarketBundle{
		TypeKey: "test-industry", ExportedAt: time.Now().UnixMilli(),
		Count: len(templates), Templates: templates,
	}
	if err := SignMarketBundle(bundle); err != nil {
		t.Fatalf("sign bundle: %v", err)
	}
	return bundle
}

func TestImportMarketBundleChain(t *testing.T) {
	openMarketImportPostgres(t)
	svc := &DeviceTemplate{}
	claims := &utils.UserClaims{TenantID: "market-import-test-" + time.Now().Format("150405"), ID: "actor-1", Authority: "TENANT_ADMIN"}
	name := "import-chain-tpl"

	// 1) 未签名包拒绝。
	unsigned := &model.MarketBundle{TypeKey: "t", Count: 1, Templates: []*model.DeviceTemplateExport{{Kind: "aetherlink-device-template", Name: name}}}
	if _, err := svc.ImportMarketBundle(model.ImportMarketBundleReq{Bundle: unsigned}, claims); err == nil {
		t.Fatal("unsigned bundle accepted")
	}

	// 2) 篡改包拒绝（签名后改 name）。
	bundle := signedTestBundle(t, name, 1)
	bundle.Templates[0].Name = "tampered"
	if _, err := svc.ImportMarketBundle(model.ImportMarketBundleReq{Bundle: bundle}, claims); err == nil {
		t.Fatal("tampered bundle accepted")
	}

	// 3) 包内重名阻断。
	dup := signedTestBundle(t, name, 2)
	dup.Templates[1].Name = dup.Templates[0].Name
	if _, err := svc.ImportMarketBundle(model.ImportMarketBundleReq{Bundle: dup}, claims); err == nil {
		t.Fatal("duplicate-name bundle accepted")
	}

	// 4) 预览：create 而非落库。
	bundle = signedTestBundle(t, name, 1)
	previewRsp, err := svc.ImportMarketBundle(model.ImportMarketBundleReq{Bundle: bundle, Preview: true}, claims)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if previewRsp.Applied || len(previewRsp.Preview.Create) != 1 {
		t.Fatalf("preview = applied:%v create:%v", previewRsp.Applied, previewRsp.Preview.Create)
	}

	// 5) 正式导入：created；6) 重导：全幂等命中。
	first, err := svc.ImportMarketBundle(model.ImportMarketBundleReq{Bundle: bundle}, claims)
	if err != nil || !first.Applied || len(first.Results) != 1 || first.Results[0].Outcome != "created" {
		t.Fatalf("first import = %+v err=%v", first, err)
	}
	second, err := svc.ImportMarketBundle(model.ImportMarketBundleReq{Bundle: bundle}, claims)
	if err != nil || second.Results[0].Outcome != "idempotent" {
		t.Fatalf("re-import = %+v err=%v", second, err)
	}
}

func TestImportMarketBundleOverwriteConfirmation(t *testing.T) {
	openMarketImportPostgres(t)
	svc := &DeviceTemplate{}
	claims := &utils.UserClaims{TenantID: "market-import-test-" + time.Now().Format("150405"), ID: "actor-1", Authority: "TENANT_ADMIN"}
	name := "overwrite-gate-tpl"

	// 先导一次。
	bundle := signedTestBundle(t, name, 1)
	if _, err := svc.ImportMarketBundle(model.ImportMarketBundleReq{Bundle: bundle}, claims); err != nil {
		t.Fatalf("seed import: %v", err)
	}
	// 同名（不同版本号避免幂等命中）再次导入 → 预览列 overwrite；未确认拒绝；确认后放行。
	v2 := "9.9.10"
	bundle.Templates[0].Version = &v2
	if err := SignMarketBundle(bundle); err != nil {
		t.Fatal(err)
	}
	pre, err := svc.ImportMarketBundle(model.ImportMarketBundleReq{Bundle: bundle, Preview: true}, claims)
	if err != nil || len(pre.Preview.Overwrite) != 1 {
		t.Fatalf("preview overwrite = %+v err=%v", pre, err)
	}
	if _, err := svc.ImportMarketBundle(model.ImportMarketBundleReq{Bundle: bundle}, claims); err == nil {
		t.Fatal("overwrite accepted without confirm_overwrite")
	}
	if _, err := svc.ImportMarketBundle(model.ImportMarketBundleReq{Bundle: bundle, ConfirmOverwrite: true}, claims); err != nil {
		t.Fatalf("overwrite with confirmation rejected: %v", err)
	}
}
