// 文件用途：用内存 SQLite(纯 Go 驱动 glebarez/sqlite,无需 cgo)对 payload schema
//
//	持久化 CRUD 编排(SaveSchema/DeleteSchema/ListSchemas)做端到端覆盖。
//
// 关键注意事项：唯一约束(payload_schemas_tenant_name_unique)定义在 41.sql,AutoMigrate 不建,
//
//	故此处不覆盖 23505 冲突路径——该路径由 payload_schema_registry_test.go 直接对
//	payloadSchemaWriteError 单测。这里覆盖创建/更新/删除/列表/租户隔离的真实 DB 往返。
package service

import (
	"fmt"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newPayloadSchemaTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.PayloadSchemaRecord{}); err != nil {
		t.Fatalf("migrate payload_schemas: %v", err)
	}
	global.DB = db
	t.Cleanup(func() {
		global.DB = oldDB
	})
	return db
}

func payloadSchemaClaims(tenantID, userID string) *utils.UserClaims {
	return &utils.UserClaims{TenantID: tenantID, ID: userID}
}

func TestPayloadSchemaSaveCreatesThenLists(t *testing.T) {
	newPayloadSchemaTestDB(t)
	svc := &PayloadSchema{}
	claims := payloadSchemaClaims("tenant-a", "user-1")

	created, err := svc.SaveSchema(&model.SavePayloadSchemaReq{
		Name:   "sensor-uplink",
		Strict: true,
		Fields: []model.PayloadSchemaField{
			{Name: "temp", Type: "number", Required: true, Min: floatPtr(-40), Max: floatPtr(125)},
			{Name: "mode", Type: "string", Enum: []string{"auto", "manual"}},
		},
	}, claims)
	if err != nil {
		t.Fatalf("SaveSchema create: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected a generated id on create")
	}
	if len(created.Fields) != 2 || created.Fields[0].Name != "temp" {
		t.Fatalf("fields not round-tripped: %+v", created.Fields)
	}
	if created.CreatedAt == nil || created.UpdatedAt == nil {
		t.Fatal("expected created_at/updated_at populated")
	}

	list, err := svc.ListSchemas(claims)
	if err != nil {
		t.Fatalf("ListSchemas: %v", err)
	}
	if len(list.List) != 1 || list.List[0].ID != created.ID {
		t.Fatalf("expected 1 schema for tenant, got %+v", list.List)
	}
}

func TestPayloadSchemaUpdatePreservesID(t *testing.T) {
	newPayloadSchemaTestDB(t)
	svc := &PayloadSchema{}
	claims := payloadSchemaClaims("tenant-a", "user-1")

	created, err := svc.SaveSchema(&model.SavePayloadSchemaReq{
		Name:   "sensor-uplink",
		Fields: []model.PayloadSchemaField{{Name: "temp", Type: "number"}},
	}, claims)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := svc.SaveSchema(&model.SavePayloadSchemaReq{
		ID:     created.ID,
		Name:   "sensor-uplink-v2",
		Strict: true,
		Fields: []model.PayloadSchemaField{
			{Name: "temp", Type: "number"},
			{Name: "humidity", Type: "number"},
		},
	}, claims)
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("update must preserve id: created=%s updated=%s", created.ID, updated.ID)
	}
	if updated.Name != "sensor-uplink-v2" || !updated.Strict || len(updated.Fields) != 2 {
		t.Fatalf("update did not persist changes: %+v", updated)
	}

	list, _ := svc.ListSchemas(claims)
	if len(list.List) != 1 {
		t.Fatalf("update must not create a new row, got %d", len(list.List))
	}
}

func TestPayloadSchemaDeleteRemovesRow(t *testing.T) {
	newPayloadSchemaTestDB(t)
	svc := &PayloadSchema{}
	claims := payloadSchemaClaims("tenant-a", "user-1")

	created, err := svc.SaveSchema(&model.SavePayloadSchemaReq{
		Name:   "to-delete",
		Fields: []model.PayloadSchemaField{{Name: "x", Type: "number"}},
	}, claims)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := svc.DeleteSchema(created.ID, claims); err != nil {
		t.Fatalf("delete: %v", err)
	}

	list, _ := svc.ListSchemas(claims)
	if len(list.List) != 0 {
		t.Fatalf("expected empty after delete, got %d", len(list.List))
	}
}

func TestPayloadSchemaListIsTenantIsolated(t *testing.T) {
	newPayloadSchemaTestDB(t)
	svc := &PayloadSchema{}
	tenantA := payloadSchemaClaims("tenant-a", "user-1")
	tenantB := payloadSchemaClaims("tenant-b", "user-2")

	if _, err := svc.SaveSchema(&model.SavePayloadSchemaReq{
		Name:   "a-schema",
		Fields: []model.PayloadSchemaField{{Name: "x", Type: "number"}},
	}, tenantA); err != nil {
		t.Fatalf("create for tenant-a: %v", err)
	}
	if _, err := svc.SaveSchema(&model.SavePayloadSchemaReq{
		Name:   "b-schema",
		Fields: []model.PayloadSchemaField{{Name: "y", Type: "string"}},
	}, tenantB); err != nil {
		t.Fatalf("create for tenant-b: %v", err)
	}

	listA, _ := svc.ListSchemas(tenantA)
	if len(listA.List) != 1 || listA.List[0].Name != "a-schema" {
		t.Fatalf("tenant-a should only see its own schema, got %+v", listA.List)
	}

	// tenant-b must not be able to delete tenant-a's schema.
	listAIDs := listA.List[0].ID
	if err := svc.DeleteSchema(listAIDs, tenantB); err != nil {
		t.Fatalf("cross-tenant delete should be a no-op, not an error: %v", err)
	}
	listAAfter, _ := svc.ListSchemas(tenantA)
	if len(listAAfter.List) != 1 {
		t.Fatal("cross-tenant delete must NOT remove another tenant's row")
	}
}

func TestPayloadSchemaSaveRejectsInvalidFields(t *testing.T) {
	newPayloadSchemaTestDB(t)
	svc := &PayloadSchema{}
	claims := payloadSchemaClaims("tenant-a", "user-1")

	if _, err := svc.SaveSchema(&model.SavePayloadSchemaReq{
		Name:   "no-fields",
		Fields: nil,
	}, claims); err == nil {
		t.Fatal("expected error for empty field set")
	}

	if _, err := svc.SaveSchema(&model.SavePayloadSchemaReq{
		Name:   "  ",
		Fields: []model.PayloadSchemaField{{Name: "x", Type: "number"}},
	}, claims); err == nil {
		t.Fatal("expected error for blank name")
	}
}

func TestPayloadSchemaCRUDRequiresClaims(t *testing.T) {
	newPayloadSchemaTestDB(t)
	svc := &PayloadSchema{}

	if _, err := svc.SaveSchema(&model.SavePayloadSchemaReq{Name: "x"}, nil); err == nil {
		t.Fatal("SaveSchema must reject nil claims")
	}
	if err := svc.DeleteSchema("id", nil); err == nil {
		t.Fatal("DeleteSchema must reject nil claims")
	}
	if _, err := svc.ListSchemas(nil); err == nil {
		t.Fatal("ListSchemas must reject nil claims")
	}
}
