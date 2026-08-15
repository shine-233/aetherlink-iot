// 文件用途：payload_schema_registry.go 中不依赖 DB 的纯逻辑单元测试。
// 覆盖:字段结构校验(空集/空名/重名)、record->rsp 映射(jsonb fields 回解、时间指针)、
// 唯一约束冲突错误翻译。不测 SaveSchema/ListSchemas 的 DAL 路径——那需真实 PG,属运行时验证。
package service

import (
	"errors"
	"testing"
	"time"

	model "aetherlink-iot/backend/internal/model"
)

func TestValidatePayloadSchemaFields(t *testing.T) {
	cases := []struct {
		name    string
		fields  []model.PayloadSchemaField
		wantErr bool
	}{
		{"empty set rejected", nil, true},
		{"blank name rejected", []model.PayloadSchemaField{{Name: "  ", Type: "number"}}, true},
		{
			"duplicate name rejected",
			[]model.PayloadSchemaField{{Name: "temp", Type: "number"}, {Name: "temp", Type: "string"}},
			true,
		},
		{
			"valid unique fields accepted",
			[]model.PayloadSchemaField{{Name: "temp", Type: "number"}, {Name: "mode", Type: "string"}},
			false,
		},
		{
			"names differing only by surrounding space collide",
			[]model.PayloadSchemaField{{Name: "temp", Type: "number"}, {Name: " temp ", Type: "string"}},
			true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validatePayloadSchemaFields(c.fields)
			if c.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
		})
	}
}

func TestPayloadSchemaRsp_DecodesFieldsAndTimes(t *testing.T) {
	createdBy := "user-1"
	created := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
	updated := time.Date(2026, 7, 28, 11, 0, 0, 0, time.UTC)
	record := &model.PayloadSchemaRecord{
		ID:        "schema-1",
		Name:      "telemetry",
		Strict:    true,
		Fields:    `[{"name":"temp","type":"number","required":true},{"name":"mode","type":"string"}]`,
		CreatedBy: &createdBy,
		CreatedAt: created,
		UpdatedAt: updated,
	}

	rsp := payloadSchemaRsp(record)

	if rsp == nil {
		t.Fatal("rsp is nil")
	}
	if rsp.ID != "schema-1" || rsp.Name != "telemetry" || !rsp.Strict {
		t.Fatalf("scalar fields not mapped: %+v", rsp)
	}
	if len(rsp.Fields) != 2 || rsp.Fields[0].Name != "temp" || rsp.Fields[1].Name != "mode" {
		t.Fatalf("fields not decoded from jsonb: %+v", rsp.Fields)
	}
	if rsp.CreatedAt == nil || !rsp.CreatedAt.Equal(created) || rsp.UpdatedAt == nil || !rsp.UpdatedAt.Equal(updated) {
		t.Fatalf("times not mapped: created=%v updated=%v", rsp.CreatedAt, rsp.UpdatedAt)
	}
}

func TestPayloadSchemaRsp_NilRecord(t *testing.T) {
	if payloadSchemaRsp(nil) != nil {
		t.Fatal("nil record should map to nil rsp")
	}
}

func TestPayloadSchemaRsp_MalformedFieldsDegradesToEmpty(t *testing.T) {
	rsp := payloadSchemaRsp(&model.PayloadSchemaRecord{ID: "x", Fields: "not json"})
	if rsp == nil {
		t.Fatal("rsp is nil")
	}
	if len(rsp.Fields) != 0 {
		t.Fatalf("malformed fields should decode to empty slice, got %+v", rsp.Fields)
	}
}

func TestPayloadSchemaWriteError(t *testing.T) {
	if payloadSchemaWriteError(nil) != nil {
		t.Fatal("nil error should stay nil")
	}

	// 唯一约束冲突翻译为可读参数错误。
	uniqueErr := errors.New(`ERROR: duplicate key value violates unique constraint "payload_schemas_tenant_name_unique" (SQLSTATE 23505)`)
	if got := payloadSchemaWriteError(uniqueErr); got == nil {
		t.Fatal("unique-constraint error should not be nil")
	}

	// 其他 DB 错误按 DB 错误上报(仍非 nil,但不 panic)。
	if got := payloadSchemaWriteError(errors.New("connection refused")); got == nil {
		t.Fatal("generic db error should not be nil")
	}
}
