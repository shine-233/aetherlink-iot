// 文件用途：计算字段服务错误/响应契约钉桩（kit 迁移前后必须逐字节一致）。
// 核心逻辑：在 sqlite 内存库上对 Get/Update/Toggle/Delete/List 断言 nil claims、空白租户、
//
//	跨租户、不存在、DB 故障（删表）五类分支的 errcode JSON（含 UseCustomMsg）与列表响应形状。
//
// 关键注意事项：期望值一律用 errcode 原始构造器手写，不经 kit；复用 crud_contract_pin_test 的 pinWire。
package service

import (
	"encoding/json"
	"errors"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/utils"
)

type calcFieldPinOp struct {
	name string
	call func(c *utils.UserClaims, id string) error
}

func calcFieldPinOps() []calcFieldPinOp {
	svc := &CalculatedFieldService{}
	upd := func() *model.CalculatedFieldUpdateReq {
		return &model.CalculatedFieldUpdateReq{Name: "x", DeviceTemplateID: "tpl-1", OutputKey: "k", Expression: "a + 1"}
	}
	return []calcFieldPinOp{
		{"get", func(c *utils.UserClaims, id string) error { _, err := svc.GetCalculatedField(id, c); return err }},
		{"update", func(c *utils.UserClaims, id string) error { _, err := svc.UpdateCalculatedField(id, upd(), c); return err }},
		{"toggle", func(c *utils.UserClaims, id string) error {
			_, err := svc.ToggleCalculatedField(id, &model.CalculatedFieldToggleReq{}, c)
			return err
		}},
		{"delete", func(c *utils.UserClaims, id string) error { return svc.DeleteCalculatedField(id, c) }},
		{"list", func(c *utils.UserClaims, _ string) error {
			_, err := svc.GetCalculatedFieldList(&model.CalculatedFieldListReq{}, c)
			return err
		}},
	}
}

func TestCalculatedFieldContractPins(t *testing.T) {
	db := setupCalculatedFieldServiceTestDB(t)
	seedTemplateForScope(t, db, "tpl-1", "tenant-a")
	own := &model.CalculatedField{ID: "cf-a", TenantID: "tenant-a", Name: "p", DeviceTemplateID: "tpl-1", OutputKey: "p", Expression: "a"}
	if err := db.Create(own).Error; err != nil {
		t.Fatal(err)
	}

	noClaims := pinWire(errcode.NewWithMessage(errcode.CodeNoPermission, "no permission to manage calculated fields"))
	noTenant := pinWire(errcode.NewWithMessage(errcode.CodeNoPermission, "tenant id is required to manage calculated fields"))
	notFound := pinWire(errcode.NewWithMessage(errcode.CodeNotFound, "calculated field not found"))
	blank := &utils.UserClaims{ID: "u", TenantID: "  "}
	other := &utils.UserClaims{ID: "u2", TenantID: "tenant-b"}

	for _, op := range calcFieldPinOps() {
		if got := pinWire(op.call(nil, "cf-a")); got != noClaims {
			t.Errorf("%s nil claims: %s", op.name, got)
		}
		if got := pinWire(op.call(blank, "cf-a")); got != noTenant {
			t.Errorf("%s blank tenant: %s", op.name, got)
		}
		if op.name == "list" {
			continue
		}
		if got := pinWire(op.call(tenantClaims(), "missing")); got != notFound {
			t.Errorf("%s missing: %s", op.name, got)
		}
		if got := pinWire(op.call(other, "cf-a")); got != notFound {
			t.Errorf("%s cross tenant: %s", op.name, got)
		}
	}

	// 列表形状：typed 响应 {"total":int64,"list":[]*model.CalculatedField}，JSON 键顺序 total→list。
	rsp, err := (&CalculatedFieldService{}).GetCalculatedFieldList(&model.CalculatedFieldListReq{}, tenantClaims())
	if err != nil {
		t.Fatal(err)
	}
	if rsp.Total != 1 || len(rsp.List) != 1 || rsp.List[0].ID != "cf-a" {
		t.Fatalf("list: %+v", rsp)
	}
	b, _ := json.Marshal(&model.CalculatedFieldListRsp{})
	if string(b) != `{"total":0,"list":null}` {
		t.Fatalf("empty list json: %s", b)
	}

	// DB 故障：删表后 get/update/toggle/delete/list 一律 dbError（sql_error 键），不得伪装成 not found。
	if err := db.Migrator().DropTable(&model.CalculatedField{}); err != nil {
		t.Fatal(err)
	}
	for _, op := range calcFieldPinOps() {
		err := op.call(tenantClaims(), "cf-a")
		var e *errcode.Error
		if got := pinWire(err); !errors.As(err, &e) || e.Code != errcode.CodeDBError || e.Data == nil {
			t.Errorf("%s db fault: %s", op.name, got)
		} else if _, ok := e.Data.(map[string]interface{})["sql_error"]; !ok {
			t.Errorf("%s db fault key: %s", op.name, got)
		}
	}
}
