// 文件用途：SecretService 错误/响应契约钉桩（service-kit 迁移前后必须逐字节一致）。
// 核心逻辑：在 sqlite 内存库上逐一断言 Get/Update/Delete/Reveal/Reseal/List 的
//
//	未登录、越权角色、跨租户、不存在、DB 故障（删表）分支的 errcode JSON（含 UseCustomMsg），
//	以及列表 *model.SecretPageResult 的 JSON 形状。
//
// 关键注意事项：期望值一律用 errcode 原始构造器手写，不经 kit，避免钉桩随实现漂移。
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/secrets"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupSecretPinDB(t *testing.T) *gorm.DB {
	t.Helper()
	setupTestMasterKey(t)
	oldDB := global.DB
	name := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:secretpin_%s?mode=memory&cache=shared", name)), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	// 手写 DDL：模型的 timestamptz + DEFAULT 会让 sqlite 走 RETURNING 并扫描失败。
	if err := db.Exec(`CREATE TABLE sys_secrets (id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL DEFAULT '', key TEXT NOT NULL,
		name TEXT NOT NULL, secret_type TEXT NOT NULL, description TEXT, encrypted_value TEXT NOT NULL,
		mask_preview TEXT NOT NULL, key_id TEXT NOT NULL DEFAULT '', created_at DATETIME, updated_at DATETIME)`).Error; err != nil {
		t.Fatal(err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	enc, err := secrets.Seal("s3cr3t-value", "tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := db.Create(&model.SysSecret{ID: "sec-a", TenantID: "tenant-a", Key: "API_KEY", Name: "api", SecretType: model.SecretTypeGeneric,
		EncryptedValue: enc, MaskPreview: "s3cr****", KeyID: "k1", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestSecretContractPins(t *testing.T) {
	ctx := context.Background()
	svc := SecretService{}
	admin := &utils.UserClaims{ID: "u1", TenantID: "tenant-a", Authority: "TENANT_ADMIN"}
	other := &utils.UserClaims{ID: "u2", TenantID: "tenant-b", Authority: "TENANT_ADMIN"}
	user := &utils.UserClaims{ID: "u3", TenantID: "tenant-a", Authority: "TENANT_USER"}

	unauth := errcode.NewWithMessage(errcode.CodeNoPermission, "unauthorized")
	nf := errcode.NewWithMessage(errcode.CodeNotFound, "secret not found")
	noTable := errcode.NewWithMessage(errcode.CodeSystemError, "SQL logic error: no such table: sys_secrets (1)")

	type op struct {
		name string
		call func(id string, c *utils.UserClaims) error
	}
	ops := []op{
		{"get", func(id string, c *utils.UserClaims) error { _, err := svc.GetSecret(ctx, id, c); return err }},
		{"update", func(id string, c *utils.UserClaims) error {
			_, err := svc.UpdateSecret(ctx, id, &model.UpdateSecretReq{Name: "renamed"}, c)
			return err
		}},
		{"reveal", func(id string, c *utils.UserClaims) error { _, err := svc.RevealSecret(ctx, id, c); return err }},
		{"reseal", func(id string, c *utils.UserClaims) error { _, err := svc.ResealSecret(ctx, id, c); return err }},
		{"delete", func(id string, c *utils.UserClaims) error { return svc.DeleteSecret(ctx, id, c) }},
	}
	for _, o := range ops {
		t.Run(o.name, func(t *testing.T) {
			db := setupSecretPinDB(t)
			cases := []struct {
				name string
				id   string
				c    *utils.UserClaims
				want error
			}{
				{"nil claims", "sec-a", nil, unauth},
				{"cross tenant", "sec-a", other, nf},
				{"missing", "nope", admin, nf},
			}
			for _, tc := range cases {
				if g, w := pinWire(o.call(tc.id, tc.c)), pinWire(tc.want); g != w {
					t.Errorf("%s: got %s want %s", tc.name, g, w)
				}
			}
			if err := db.Migrator().DropTable(&model.SysSecret{}); err != nil {
				t.Fatal(err)
			}
			if g, w := pinWire(o.call("sec-a", admin)), pinWire(noTable); g != w {
				t.Errorf("db fault: got %s want %s", g, w)
			}
		})
	}

	t.Run("role gates precede lookup", func(t *testing.T) {
		setupSecretPinDB(t)
		_, err := svc.RevealSecret(ctx, "nope", user)
		if g, w := pinWire(err), pinWire(errcode.NewWithMessage(errcode.CodeNoPermission, "permission denied: cannot reveal secret")); g != w {
			t.Errorf("reveal: got %s want %s", g, w)
		}
		_, err = svc.ResealSecret(ctx, "nope", user)
		if g, w := pinWire(err), pinWire(errcode.NewWithMessage(errcode.CodeNoPermission, "permission denied")); g != w {
			t.Errorf("reseal: got %s want %s", g, w)
		}
	})

	t.Run("happy paths", func(t *testing.T) {
		setupSecretPinDB(t)
		got, err := svc.GetSecret(ctx, "sec-a", admin)
		if err != nil || got.ID != "sec-a" || got.NeedsReseal {
			t.Fatalf("get: %+v %v", got, err)
		}
		rev, err := svc.RevealSecret(ctx, "sec-a", admin)
		if err != nil || rev.Value != "s3cr3t-value" {
			t.Fatalf("reveal: %+v %v", rev, err)
		}
		if err := svc.DeleteSecret(ctx, "sec-a", admin); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := svc.GetSecret(ctx, "sec-a", admin); pinWire(err) != pinWire(nf) {
			t.Fatalf("after delete: %v", err)
		}
	})

	t.Run("list", func(t *testing.T) {
		db := setupSecretPinDB(t)
		if _, err := svc.ListSecrets(ctx, &model.SecretListReq{}, nil); pinWire(err) != pinWire(unauth) {
			t.Errorf("nil claims: %v", err)
		}
		res, err := svc.ListSecrets(ctx, &model.SecretListReq{}, admin)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := json.Marshal(res)
		want := `{"list":[{"id":"sec-a","tenant_id":"tenant-a","key":"API_KEY","name":"api","secret_type":"GENERIC","description":"","mask_preview":"s3cr****","key_id":"k1","needs_reseal":false,"created_at":"2026-01-02T03:04:05Z","updated_at":"2026-01-02T03:04:05Z"}],"total":1}`
		if string(b) != want {
			t.Errorf("list json:\n got %s\nwant %s", b, want)
		}
		empty, err := svc.ListSecrets(ctx, &model.SecretListReq{}, other)
		if err != nil {
			t.Fatal(err)
		}
		if b, _ := json.Marshal(empty); string(b) != `{"list":[],"total":0}` {
			t.Errorf("empty list json: %s", b)
		}
		if err := db.Migrator().DropTable(&model.SysSecret{}); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.ListSecrets(ctx, &model.SecretListReq{}, admin); !strings.Contains(pinWire(err), `"code":`+fmt.Sprint(errcode.CodeSystemError)) {
			t.Errorf("list db fault: %s", pinWire(err))
		}
	})
}
