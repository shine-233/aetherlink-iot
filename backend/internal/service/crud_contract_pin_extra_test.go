// 文件用途：service-kit 迁移评审补钉——crud_contract_pin_test 未覆盖的三条分支。
// 核心逻辑：
//   - 删除阶段 DB 故障：DAL 原始错误原样透传（非 errcode 包装，historical contract）；
//   - widget_bundle 空租户：delete/list 同样被 "claims required" 拒绝；
//   - TestDataConverter 引用他租户 converter_id：掩码为 "data converter not found"。
//
// 关键注意事项：期望值用 errcode 原始构造器手写，不经 kit。
package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/gorm"
)

func TestCRUDContractPins_DeleteDBFaultPassesThrough(t *testing.T) {
	ownA := &utils.UserClaims{ID: "u-a", TenantID: "tenant-a", Authority: "TENANT_ADMIN"}
	injected := errors.New("injected delete failure")
	for _, svc := range crudPinServices() {
		svc := svc
		t.Run(svc.table, func(t *testing.T) {
			db := setupCRUDPinDB(t)
			// 只让 DELETE 失败：存在性预检（SELECT）照常成功，才能走到删除分支。
			if err := db.Callback().Delete().Before("gorm:delete").Register("pin:fail_delete", func(tx *gorm.DB) {
				_ = tx.AddError(injected)
			}); err != nil {
				t.Fatal(err)
			}
			err := svc.del(svc.ownID, ownA)
			if got := pinWire(err); !strings.HasPrefix(got, "raw:") || !errors.Is(err, injected) {
				t.Fatalf("delete db fault: got %s, want raw passthrough of injected error", got)
			}
			// 记录仍在（删除未生效）。
			if err := svc.get(svc.ownID, ownA); err != nil {
				t.Fatalf("record should survive failed delete: %s", pinWire(err))
			}
		})
	}
}

func TestCRUDContractPins_WidgetBundleEmptyTenantDeleteList(t *testing.T) {
	setupCRUDPinDB(t)
	noTenant := &utils.UserClaims{ID: "u-sys", Authority: "SYS_ADMIN"}
	deny := pinWire(errcode.NewWithMessage(errcode.CodeNoPermission, "claims required"))
	wb := &WidgetBundleService{}
	ctx := context.Background()

	if got := pinWire(wb.DeleteWidgetBundle(ctx, "wb-a", noTenant)); got != deny {
		t.Errorf("delete empty tenant: got %s want %s", got, deny)
	}
	if _, err := wb.ListWidgetBundles(ctx, &model.GetWidgetBundleListReq{}, noTenant); pinWire(err) != deny {
		t.Errorf("list empty tenant: got %s want %s", pinWire(err), deny)
	}
	var n int64
	if err := global.DB.Model(&model.WidgetBundle{}).Where("id = ?", "wb-a").Count(&n).Error; err != nil || n != 1 {
		t.Fatalf("denied delete must not remove the row: n=%d err=%v", n, err)
	}
}

func TestCRUDContractPins_TestDataConverterCrossTenantID(t *testing.T) {
	setupCRUDPinDB(t)
	ownB := &utils.UserClaims{ID: "u-b", TenantID: "tenant-b", Authority: "TENANT_ADMIN"}
	notFound := pinWire(errcode.NewWithMessage(errcode.CodeNotFound, "data converter not found"))
	id := "dc-a"
	_, err := (&DataConverterService{}).TestDataConverter(context.Background(), &model.TestDataConverterReq{
		ConverterID: &id,
		Payload:     `{"t":1}`,
	}, ownB)
	if got := pinWire(err); got != notFound {
		t.Fatalf("cross-tenant converter_id: got %s want %s", got, notFound)
	}
	if _, err := (&DataConverterService{}).TestDataConverter(context.Background(), &model.TestDataConverterReq{
		ConverterID: &id, Payload: `{"t":1}`,
	}, nil); pinWire(err) != pinWire(errcode.NewWithMessage(errcode.CodeNoPermission, "claims required")) {
		t.Fatalf("nil claims: got %s", pinWire(err))
	}
}
