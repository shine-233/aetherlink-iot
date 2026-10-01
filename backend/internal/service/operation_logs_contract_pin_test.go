// 文件用途：操作日志分页列表的响应与 DB 错误契约钉桩（service-kit 迁移前后必须逐字节一致）。
// 核心逻辑：sqlite 内存库上断言空页 list 为非 nil 的 []model.GetOperationLogListByPageRsp（JSON 为 []）、
//
//	total 为 int64，以及表缺失时走 dbError（{"sql_error": msg}）。
package service

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestOperationLogsListContractPins(t *testing.T) {
	oldDB := global.DB
	db, err := gorm.Open(sqlite.Open("file:oplpin_"+strings.ReplaceAll(t.Name(), "/", "_")+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT, email TEXT)`,
		`CREATE TABLE operation_logs (id TEXT PRIMARY KEY, ip TEXT, path TEXT, user_id TEXT, name TEXT,
			created_at DATETIME, latency INTEGER, request_message TEXT, response_message TEXT, tenant_id TEXT,
			remark TEXT, action TEXT, entity_type TEXT, entity_id TEXT)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })

	svc := &OperationLogs{}
	c := &utils.UserClaims{ID: "u", TenantID: "tenant-a", Authority: "TENANT_ADMIN"}
	req := &model.GetOperationLogListByPageReq{PageReq: model.PageReq{Page: 1, PageSize: 10}}

	res, err := svc.GetListByPage(req, c)
	if err != nil {
		t.Fatal(err)
	}
	if total, ok := res["total"].(int64); !ok || total != 0 || len(res) != 2 {
		t.Errorf("list shape: %#v", res)
	}
	rows, ok := res["list"].([]model.GetOperationLogListByPageRsp)
	if !ok || rows == nil {
		t.Errorf("list type: %T nil=%v", res["list"], rows == nil)
	}
	if b, _ := json.Marshal(res); string(b) != `{"list":[],"total":0}` {
		t.Errorf("empty page JSON: %s", b)
	}

	if err := db.Migrator().DropTable("operation_logs"); err != nil {
		t.Fatal(err)
	}
	_, lerr := svc.GetListByPage(req, c)
	if lw := pinWire(lerr); !strings.HasPrefix(lw, fmt.Sprintf(`{"code":%d,"data":{"sql_error":`, errcode.CodeDBError)) {
		t.Errorf("list db fault: %s", lw)
	}
}
