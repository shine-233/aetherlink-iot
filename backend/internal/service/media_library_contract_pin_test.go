// 文件用途：媒体库服务错误/响应契约钉桩（service-kit 迁移前后必须逐字节一致）。
// 核心逻辑：List/Detail/Delete 的未登录、空租户、跨租户、不存在、DB 故障分支的 errcode JSON
//
//	（含 UseCustomMsg），以及列表响应的类型与 JSON 形状。
//
// 关键注意事项：期望值一律用 errcode 原始构造器手写，不经 kit。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"
)

func TestMediaLibraryContractPins(t *testing.T) {
	setupMediaLibraryTestDB(t)
	ctx := context.Background()
	id := seedMediaFile(t, ctx)
	svc := GroupApp.MediaLibrary
	ownB := &utils.UserClaims{ID: "user-b", TenantID: mediaTestTenantB, Authority: "TENANT_ADMIN"}
	noTenant := &utils.UserClaims{ID: "u-sys", Authority: "SYS_ADMIN"}
	deny := pinWire(errcode.NewWithMessage(errcode.CodeNoPermission, "claims required"))
	notFound := pinWire(errcode.NewWithMessage(errcode.CodeNotFound, "media file not found"))

	ops := map[string]func(string, *utils.UserClaims) error{
		"detail": func(id string, c *utils.UserClaims) error { _, err := svc.GetMediaFileDetail(ctx, id, c); return err },
		"delete": func(id string, c *utils.UserClaims) error { _, err := svc.DeleteMediaFile(ctx, id, c); return err },
	}
	for _, op := range []string{"detail", "delete"} {
		for _, tc := range []struct {
			name, id string
			c        *utils.UserClaims
			want     string
		}{
			{"nil claims", id, nil, deny},
			{"empty tenant", id, noTenant, deny},
			{"cross tenant", id, ownB, notFound},
			{"missing", "nope", mediaTestClaimsA, notFound},
		} {
			if got := pinWire(ops[op](tc.id, tc.c)); got != tc.want {
				t.Errorf("%s/%s: got %s want %s", op, tc.name, got, tc.want)
			}
		}
	}

	// list：类型化响应 + JSON 形状钉死。
	res, err := svc.ListMediaFiles(ctx, &model.GetMediaFileListReq{}, mediaTestClaimsA)
	if err != nil || res == nil || res.Total != 1 || len(res.List) != 1 {
		t.Fatalf("list: %#v %v", res, err)
	}
	b, _ := json.Marshal(res)
	var shape map[string]json.RawMessage
	_ = json.Unmarshal(b, &shape)
	if len(shape) != 2 || shape["total"] == nil || shape["list"] == nil {
		t.Errorf("list json shape: %s", b)
	}
	for _, c := range []*utils.UserClaims{nil, noTenant} {
		if _, err := svc.ListMediaFiles(ctx, &model.GetMediaFileListReq{}, c); pinWire(err) != deny {
			t.Errorf("list denied: %s", pinWire(err))
		}
	}

	// DB 故障：加载失败掩码为 not found；列表错误走 {"error": msg}。
	if err := global.DB.Migrator().DropTable(&model.MediaFile{}); err != nil {
		t.Fatal(err)
	}
	if got := pinWire(ops["detail"](id, mediaTestClaimsA)); got != notFound {
		t.Errorf("detail db fault: %s", got)
	}
	if got := pinWire(ops["delete"](id, mediaTestClaimsA)); got != notFound {
		t.Errorf("delete db fault: %s", got)
	}
	_, lerr := svc.ListMediaFiles(ctx, &model.GetMediaFileListReq{}, mediaTestClaimsA)
	var ec *errcode.Error
	if !errors.As(lerr, &ec) || ec.Code != errcode.CodeDBError || ec.UseCustomMsg {
		t.Fatalf("list db fault: %s", pinWire(lerr))
	}
	data, _ := ec.Data.(map[string]interface{})
	if msg, ok := data["error"].(string); !ok || len(data) != 1 || !strings.Contains(msg, "media_files") {
		t.Errorf("list db fault data: %#v", ec.Data)
	}
}
