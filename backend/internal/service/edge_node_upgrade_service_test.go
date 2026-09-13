// 文件用途：P1.5 节点注册/心跳 与 P1.6 升级/回滚服务流的运行期证据。
// 覆盖：注册→重注册幂等更新→跨租户抢注拒绝→未注册心跳拒绝→心跳触碰；
//
//	导入种子→升级留历史→目标版本必须严格更新→回滚重放旧版本（幂等）。
//
// 说明：需要真实 PostgreSQL（edge_nodes/device_templates/升级历史表）。
// 缺 DSN 或缺表一律 Skip，不得把 Skip 当作通过。
package service

import (
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
	"aetherlink-iot/backend/pkg/utils"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func openNodeUpgradePostgres(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv("AETHERLINK_TEST_PSQL_DSN"))
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; node/upgrade tests require PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	global.DB = db
	var edgeNodes, history bool
	if err := db.Raw("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name='edge_nodes'), EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name='device_template_upgrade_history')").Row().Scan(&edgeNodes, &history); err != nil {
		t.Fatalf("probe tables: %v", err)
	}
	if !edgeNodes {
		t.Skip("edge_nodes missing; apply migration 97 first")
	}
	if !history {
		t.Skip("device_template_upgrade_history missing; apply migration 99 first")
	}
	return db
}

func TestEdgeNodeRegisterHeartbeatChain(t *testing.T) {
	openNodeUpgradePostgres(t)
	svc := &EdgeNodeService{}
	tenantA := "edge-node-test-" + time.Now().Format("150405")
	tenantB := tenantA + "-b"
	claimsA := &utils.UserClaims{TenantID: tenantA, ID: "actor-a", Authority: "TENANT_ADMIN"}
	claimsB := &utils.UserClaims{TenantID: tenantB, ID: "actor-b", Authority: "TENANT_ADMIN"}

	// 1) 首次注册：updated（要写入新节点），健康 online（last_seen=now）。
	rsp, err := svc.RegisterEdgeNode(model.RegisterEdgeNodeReq{NodeID: "node-e2e-1", Version: "1.0.0", Capabilities: []string{"modbus", " modbus ", "opcua"}}, claimsA)
	if err != nil {
		t.Fatalf("first register: %v", err)
	}
	if rsp.Outcome != "updated" || rsp.Health != "online" {
		t.Fatalf("first register = %+v", rsp)
	}
	if got := unmarshalEdgeNodeCapabilities(rsp.Node.Capabilities); len(got) != 2 || got[0] != "modbus" || got[1] != "opcua" {
		t.Fatalf("capabilities not normalized: %q -> %v", rsp.Node.Capabilities, got)
	}

	// 2) 同租户重注册（版本变化）：updated；心跳触碰后 health online。
	if _, err := svc.RegisterEdgeNode(model.RegisterEdgeNodeReq{NodeID: "node-e2e-1", Version: "1.1.0"}, claimsA); err != nil {
		t.Fatalf("re-register: %v", err)
	}
	hb, err := svc.HeartbeatEdgeNode("node-e2e-1", model.EdgeNodeHeartbeatReq{}, claimsA)
	if err != nil || hb.Health != "online" {
		t.Fatalf("heartbeat = %+v err=%v", hb, err)
	}

	// 3) 跨租户抢注拒绝（同 NodeID 换租户是安全事件）。
	if _, err := svc.RegisterEdgeNode(model.RegisterEdgeNodeReq{NodeID: "node-e2e-1", Version: "1.0.0"}, claimsB); err == nil {
		t.Fatal("cross-tenant hijack accepted")
	}

	// 4) 未注册节点心跳拒绝（心跳不能顺带注册）。
	if _, err := svc.HeartbeatEdgeNode("node-not-registered", model.EdgeNodeHeartbeatReq{}, claimsA); err == nil {
		t.Fatal("heartbeat for unregistered node accepted")
	}

	// 5) 列表含健康分类。
	entries, err := svc.ListEdgeNodes(10, claimsA)
	if err != nil || len(entries) == 0 {
		t.Fatalf("list = %d entries err=%v", len(entries), err)
	}
}

func TestTemplateUpgradeRollbackFlow(t *testing.T) {
	openNodeUpgradePostgres(t)
	svc := &DeviceTemplate{}
	claims := &utils.UserClaims{TenantID: "upgrade-flow-" + time.Now().Format("150405"), ID: "actor-1", Authority: "TENANT_ADMIN"}

	// 1) 种子：导入 1.0.0。
	v1 := "1.0.0"
	seed := model.ImportDeviceTemplateReq{Kind: "aetherlink-device-template", Name: "upgrade-flow-tpl", Version: &v1}
	seedTpl, created, err := svc.ImportDeviceTemplate(seed, claims)
	if err != nil || !created {
		t.Fatalf("seed import: created=%v err=%v", created, err)
	}

	// 2) 升级到 2.0.0（载荷名必须一致）：历史落库、新版本行存在。
	v2 := "2.0.0"
	upRsp, err := svc.UpgradeDeviceTemplate(model.UpgradeDeviceTemplateReq{
		Payload: &model.DeviceTemplateExport{Kind: "aetherlink-device-template", Name: "upgrade-flow-tpl", Version: &v2},
	}, claims)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if upRsp.Template.Version == nil || *upRsp.Template.Version != "2.0.0" {
		t.Fatalf("upgraded template version = %v", upRsp.Template.Version)
	}

	// 3) 降级式"升级"拒绝（1.5 < 2.0 必须走回滚通道）。
	v15 := "1.5.0"
	if _, err := svc.UpgradeDeviceTemplate(model.UpgradeDeviceTemplateReq{
		Payload: &model.DeviceTemplateExport{Kind: "aetherlink-device-template", Name: "upgrade-flow-tpl", Version: &v15},
	}, claims); err == nil {
		t.Fatal("downgrade accepted via upgrade")
	}

	// 4) 回滚 = 重放 1.0.0（旧版本行复活/命中，不删 2.0.0 行）。
	rolled, err := svc.RollbackTemplateUpgrade(upRsp.HistoryID, claims)
	if err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if rolled.Version == nil || *rolled.Version != "1.0.0" || rolled.ID != seedTpl.ID {
		t.Fatalf("rollback = %+v, want version 1.0.0 id %s", rolled, seedTpl.ID)
	}
	// 2.0.0 行仍存在（不删行）。
	if _, err := dal.FindDeviceTemplateByNameVersion(claims.TenantID, "upgrade-flow-tpl", "2.0.0"); err != nil {
		t.Fatalf("2.0.0 row missing after rollback: %v", err)
	}
	// 5) 回滚重放幂等：再次回滚同一条历史仍成功。
	if _, err := svc.RollbackTemplateUpgrade(upRsp.HistoryID, claims); err != nil {
		t.Fatalf("rollback replay: %v", err)
	}
}
