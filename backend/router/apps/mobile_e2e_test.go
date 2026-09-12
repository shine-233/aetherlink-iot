// 文件用途：移动端接口的**接口级 E2E**（真实 Gin 引擎 + 真实 PostgreSQL）。
// 核心逻辑：注册真实路由、挂上真实统一响应中间件与真实凭证，逐个打移动端端点，
// 断言 HTTP 状态与响应体。
//
// 关键注意事项（先把"这不是什么"说清楚，免得被当成已经做过的门禁）：
//  1. 这是**接口级** E2E：HTTP → 路由 → handler → service → DAL → PostgreSQL 全链路真实，
//     但**不是真机 E2E**——没有 Android/iOS 客户端参与，也没有真实 FCM/APNs 通道。
//     门禁里的"Android/H5 一条完整业务 E2E"仍需真机与真实推送通道。
//  2. 数据库用真实 PostgreSQL 而非 SQLite：本套接口的行为高度依赖 SQL 侧的
//     租户/归属过滤与约束，SQLite 跑绿证明不了真实库上也成立。
//  3. 命令端点只验到"缺幂等键即拒绝"与"服务已接线"这一层，
//     不断言真的下发成功——那需要设备缓存与下行总线，属于另一个环境。
package apps

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/api"
	"aetherlink-iot/backend/internal/dal"
	"aetherlink-iot/backend/internal/middleware/response"
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	service "aetherlink-iot/backend/internal/service"
	"aetherlink-iot/backend/pkg/errcode"
	"aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const mobileE2ETenant = "pg-mobile-e2e"

// mobileE2E 起一个真实的 Gin 引擎 + 真实 PostgreSQL。
func mobileE2E(t *testing.T, authority string) (*gin.Engine, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; mobile API E2E skipped")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	prev := global.DB
	global.DB = db
	// 设备/看板列表读模型用 gen 单例拼条件，未 SetDefault 会 panic。
	query.SetDefault(db.Session(&gorm.Session{NewDB: true}))
	t.Cleanup(func() {
		// 顺序有讲究：OTA 明细外键指向 devices，先删明细，否则设备删不掉会留脏数据，
		// 反噬后面跑的用例（"凭空多出一台设备"这类假象就是这么来的）。
		_ = db.Exec("DELETE FROM " + model.TableNameOtaUpgradeTaskDetail + " WHERE device_id LIKE 'pg-e2e-%'").Error
		_ = db.Exec("DELETE FROM " + model.TableNameOtaUpgradeTask + " WHERE id LIKE 'pg-e2e-ota-task%'").Error
		_ = db.Exec("DELETE FROM " + model.TableNameOtaUpgradePackage + " WHERE id LIKE 'pg-e2e-ota-pkg%'").Error
		_ = db.Exec("DELETE FROM devices WHERE tenant_id = ?", mobileE2ETenant).Error
		_ = db.Exec("DELETE FROM boards WHERE tenant_id = ?", mobileE2ETenant).Error
		_ = db.Exec("DELETE FROM "+model.TableNamePushDeviceRegistration+" WHERE tenant_id = ?", mobileE2ETenant).Error
		global.DB = prev
	})

	// 真实服务装配（与生产同一条路径）。
	control, _, err := service.AssembleScadaControl(service.ScadaControlWiring{ConfirmationSecret: "e2e-secret"})
	if err != nil {
		t.Fatalf("assemble scada control: %v", err)
	}
	prevControl := service.GroupApp.ScadaControl
	prevMobile := service.GroupApp.Mobile
	service.GroupApp.ScadaControl = control
	service.GroupApp.Mobile = service.AssembleMobile(nil)
	t.Cleanup(func() {
		service.GroupApp.ScadaControl = prevControl
		service.GroupApp.Mobile = prevMobile
	})

	handler, err := response.NewHandler("../../configs/messages.yaml", "../../configs/messages_str.yaml")
	if err != nil {
		t.Fatalf("response handler: %v", err)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(func(c *gin.Context) {
		// 真实鉴权中间件不在本用例范围内，这里注入真实结构的凭证。
		c.Set("claims", &utils.UserClaims{
			ID:        "e2e-user",
			TenantID:  mobileE2ETenant,
			Authority: authority,
		})
		c.Next()
	})
	engine.Use(handler.Middleware())
	v1 := engine.Group("/api/v1")
	(&Mobile{}).Init(v1)
	return engine, db
}

func doJSON(t *testing.T, engine *gin.Engine, method, path, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	out := map[string]interface{}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return out
}

// assertRouteReached 断言路由存在且没 panic。
// 注意：本项目的统一响应中间件对业务错误也返回 HTTP 200，错误码在 body 的 code 里，
// 所以 HTTP 状态只能证明"路由命中、没崩"，证明不了业务成功——成败必须再看 code。
func assertRouteReached(t *testing.T, rec *httptest.ResponseRecorder, what string) {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: http = %d, body=%s", what, rec.Code, rec.Body.String())
	}
}

// assertAPIOK 断言业务成功，返回 data。
func assertAPIOK(t *testing.T, rec *httptest.ResponseRecorder, what string) map[string]interface{} {
	t.Helper()
	assertRouteReached(t, rec, what)
	body := decodeBody(t, rec)
	code, _ := body["code"].(float64)
	if int(code) != errcode.CodeSuccess {
		t.Fatalf("%s: code = %v, message = %v, want success", what, body["code"], body["message"])
	}
	data, _ := body["data"].(map[string]interface{})
	return data
}

// assertAPIFail 断言业务失败，并按需检查错误信息里提到某个关键词。
func assertAPIFail(t *testing.T, rec *httptest.ResponseRecorder, what string, wantSubstr string) map[string]interface{} {
	t.Helper()
	assertRouteReached(t, rec, what)
	body := decodeBody(t, rec)
	code, _ := body["code"].(float64)
	if int(code) == errcode.CodeSuccess {
		t.Fatalf("%s must fail, body=%s", what, rec.Body.String())
	}
	if wantSubstr != "" {
		msg, _ := body["message"].(string)
		if !strings.Contains(msg, wantSubstr) {
			t.Fatalf("%s: code = %v, message = %q, want it to mention %q", what, body["code"], msg, wantSubstr)
		}
	}
	return body
}

// TestMobileE2ECapabilitiesAndDevices 能力矩阵 + 设备列表。
func TestMobileE2ECapabilitiesAndDevices(t *testing.T) {
	engine, db := mobileE2E(t, "TENANT_ADMIN")

	name := "E2E 设备"
	device := &model.Device{
		ID:           "pg-e2e-dev-1",
		Name:         &name,
		Voucher:      "pg-e2e-voucher",
		TenantID:     mobileE2ETenant,
		IsEnabled:    "enabled",
		ActivateFlag: "active",
		DeviceNumber: "pg-e2e-dev-1",
		IsOnline:     1,
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("insert device: %v", err)
	}

	rec := doJSON(t, engine, http.MethodGet, "/api/v1/mobile/capabilities", "", nil)
	data := assertAPIOK(t, rec, "capabilities")
	if data == nil {
		t.Fatalf("capabilities response has no data: %s", rec.Body.String())
	}
	// 已接线的能力必须报 true：这是"接口存在"与"能力可用"的分界线。
	for _, key := range []string{"telemetry", "alarms", "shadow", "ota", "dashboards", "commands"} {
		if v, _ := data[key].(bool); !v {
			t.Fatalf("capability %s = %v, want true", key, data[key])
		}
	}
	// 没配推送凭据，必须为 false；离线缓存恒为 false。
	if v, _ := data["push"].(bool); v {
		t.Fatal("push must be false without provider credentials")
	}
	if v, _ := data["offline_cache"].(bool); v {
		t.Fatal("offline_cache must never be claimed by the server")
	}

	rec = doJSON(t, engine, http.MethodGet, "/api/v1/mobile/devices?page=1&page_size=10", "", nil)
	listData := assertAPIOK(t, rec, "devices")
	rows, _ := listData["list"].([]interface{})
	if len(rows) != 1 {
		t.Fatalf("devices list = %v, want 1 row", listData["list"])
	}
	first, _ := rows[0].(map[string]interface{})
	if first["device_id"] != "pg-e2e-dev-1" {
		t.Fatalf("device_id = %v, want pg-e2e-dev-1", first["device_id"])
	}
	// warn_status 是布尔标记而不是条数，接口不该编一个计数出来。
	if _, exists := first["alarm_count"]; exists {
		t.Fatal("device summary must not expose a fabricated alarm_count")
	}
}

// TestMobileE2EShadowRoundtrip 影子写入 → 读回。
func TestMobileE2EShadowRoundtrip(t *testing.T) {
	engine, db := mobileE2E(t, "TENANT_ADMIN")
	name := "影子设备"
	device := &model.Device{
		ID: "pg-e2e-dev-2", Name: &name, Voucher: "v", TenantID: mobileE2ETenant,
		IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "pg-e2e-dev-2", IsOnline: 0,
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("insert device: %v", err)
	}

	// 离线设备：写入应落到影子队列而不是直接下发。
	rec := doJSON(t, engine, http.MethodPut, "/api/v1/mobile/devices/pg-e2e-dev-2/shadow",
		`{"payload":"{\"method\":\"set_temp\",\"params\":{\"c\":22}}"}`, nil)
	assertAPIOK(t, rec, "shadow put")

	rec = doJSON(t, engine, http.MethodGet, "/api/v1/mobile/devices/pg-e2e-dev-2/shadow", "", nil)
	shadowData := assertAPIOK(t, rec, "shadow get")
	raw, _ := shadowData["shadow"].(string)
	if !strings.Contains(raw, "set_temp") {
		t.Fatalf("shadow = %q, want the queued command payload", raw)
	}

	// 非法 JSON 必须被拒，不能静默存一份设备解析不了的字节。
	rec = doJSON(t, engine, http.MethodPut, "/api/v1/mobile/devices/pg-e2e-dev-2/shadow",
		`{"payload":"not json"}`, nil)
	assertAPIFail(t, rec, "invalid shadow payload", "valid json")
}

// TestMobileE2EOTAAndDashboards OTA 与看板端点。
func TestMobileE2EOTAAndDashboards(t *testing.T) {
	engine, db := mobileE2E(t, "TENANT_ADMIN")
	name := "OTA 设备"
	device := &model.Device{
		ID: "pg-e2e-dev-3", Name: &name, Voucher: "v", TenantID: mobileE2ETenant,
		IsEnabled: "enabled", ActivateFlag: "active", DeviceNumber: "pg-e2e-dev-3", IsOnline: 1,
	}
	if err := db.Create(device).Error; err != nil {
		t.Fatalf("insert device: %v", err)
	}

	// 没有升级记录时是 none，不是错误。
	rec := doJSON(t, engine, http.MethodGet, "/api/v1/mobile/devices/pg-e2e-dev-3/ota", "", nil)
	otaData := assertAPIOK(t, rec, "ota with no upgrade record")
	if otaData["status"] != service.OTAStatusNone {
		t.Fatalf("ota status = %v, want %q", otaData["status"], service.OTAStatusNone)
	}

	now := time.Now().UTC()
	tenant := mobileE2ETenant
	pkg := &model.OtaUpgradePackage{ID: "pg-e2e-ota-pkg", Name: "p", TenantID: &tenant, CreatedAt: now}
	if err := db.Create(pkg).Error; err != nil {
		t.Fatalf("insert package: %v", err)
	}
	task := &model.OtaUpgradeTask{
		ID: "pg-e2e-ota-task", Name: "t", OtaUpgradePackageID: pkg.ID, CreatedAt: now,
		Status: "running", TargetMode: "explicit", TimeoutSeconds: 3600, RolloutRatePerMinute: 60,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("insert task: %v", err)
	}
	detail := &model.OtaUpgradeTaskDetail{
		ID: "pg-e2e-ota-detail", OtaUpgradeTaskID: task.ID, DeviceID: device.ID, Status: 3, UpdatedAt: &now,
	}
	if err := db.Create(detail).Error; err != nil {
		t.Fatalf("insert detail: %v", err)
	}

	rec = doJSON(t, engine, http.MethodGet, "/api/v1/mobile/devices/pg-e2e-dev-3/ota", "", nil)
	otaData = assertAPIOK(t, rec, "ota while upgrading")
	if otaData["status"] != "upgrading" {
		t.Fatalf("ota status = %v, want upgrading", otaData["status"])
	}

	// 看板：普通用户被既有规则拒绝，管理员可见。
	board := &model.Board{ID: "pg-e2e-board", Name: "看板", TenantID: mobileE2ETenant, CreatedAt: now, UpdatedAt: now, HomeFlag: "N"}
	if err := db.Create(board).Error; err != nil {
		t.Fatalf("insert board: %v", err)
	}
	rec = doJSON(t, engine, http.MethodGet, "/api/v1/mobile/dashboards", "", nil)
	dashData := assertAPIOK(t, rec, "dashboards as TENANT_ADMIN")
	dashRows, _ := dashData["list"].([]interface{})
	if len(dashRows) != 1 {
		t.Fatalf("dashboards = %v, want 1 board", dashData["list"])
	}
}

// TestMobileE2EDashboardsRejectsTenantUser 看板对普通用户应被既有规则拒绝。
// 与上一条形成对照：同一个端点，不同角色不同结果，说明角色裁决真的生效了。
func TestMobileE2EDashboardsRejectsTenantUser(t *testing.T) {
	engine, _ := mobileE2E(t, "TENANT_USER")
	rec := doJSON(t, engine, http.MethodGet, "/api/v1/mobile/dashboards", "", nil)
	body := assertAPIFail(t, rec, "dashboards as TENANT_USER", "")
	// 必须是权限裁决失败（既有看板规则），而不是"参数不对"之类的旁支原因。
	if code, _ := body["code"].(float64); int(code) != errcode.CodeNoPermission {
		t.Fatalf("dashboards as TENANT_USER: code = %v, want %d (no permission)", body["code"], errcode.CodeNoPermission)
	}
}

// TestMobileE2ECommandRequiresIdempotencyKey 命令缺幂等键必须被拒。
// 弱网下没有幂等键的重试必然重复下发，这是门禁里明确要求的一条。
func TestMobileE2ECommandRequiresIdempotencyKey(t *testing.T) {
	engine, _ := mobileE2E(t, "TENANT_ADMIN")
	rec := doJSON(t, engine, http.MethodPost, "/api/v1/mobile/commands",
		`{"device_id":"pg-e2e-dev-1","command":"open_valve"}`, nil)
	assertAPIFail(t, rec, "command without idempotency key", "Idempotency-Key")
}

// TestMobileE2EPushTokenLifecycle 令牌登记 → 撤销。
// 推送投递本身没接线，但登记/撤销走 DAL，是真实可用的。
func TestMobileE2EPushTokenLifecycle(t *testing.T) {
	engine, _ := mobileE2E(t, "TENANT_ADMIN")

	rec := doJSON(t, engine, http.MethodPost, "/api/v1/mobile/push/subscribe",
		`{"platform":"android","token":"e2e-token-1","provider":"fcm"}`, nil)
	subData := assertAPIOK(t, rec, "push subscribe")
	regID, _ := subData["id"].(string)
	if regID == "" {
		t.Fatalf("subscribe returned no id: %s", rec.Body.String())
	}

	// 重复登记同一令牌不应产生第二行（数据库 upsert 保证）。
	rec = doJSON(t, engine, http.MethodPost, "/api/v1/mobile/push/subscribe",
		`{"platform":"android","token":"e2e-token-1","provider":"fcm"}`, nil)
	assertAPIOK(t, rec, "push re-subscribe")
	regs, err := dal.ListPushRegistrationsByUser(mobileE2ETenant, "e2e-user", true)
	if err != nil {
		t.Fatalf("list registrations: %v", err)
	}
	if len(regs) != 1 {
		t.Fatalf("registrations = %d, want 1 (upsert must not duplicate)", len(regs))
	}

	rec = doJSON(t, engine, http.MethodDelete, "/api/v1/mobile/push/"+regID, "", nil)
	assertAPIOK(t, rec, "push unsubscribe")
	regs, err = dal.ListPushRegistrationsByUser(mobileE2ETenant, "e2e-user", true)
	if err != nil {
		t.Fatalf("list registrations after unsubscribe: %v", err)
	}
	if len(regs) != 0 {
		t.Fatalf("registrations = %d, want 0 after unsubscribe", len(regs))
	}

	// 撤销一个不存在的登记必须报 404，而不是"删了 0 行也算成功"。
	rec = doJSON(t, engine, http.MethodDelete, "/api/v1/mobile/push/no-such-registration", "", nil)
	body := assertAPIFail(t, rec, "unsubscribe unknown registration", "")
	if code, _ := body["code"].(float64); int(code) != errcode.CodeNotFound {
		t.Fatalf("unsubscribe unknown: code = %v, want %d", body["code"], errcode.CodeNotFound)
	}
	// id 列是 uuid：非 uuid 的输入若直接进 SQL，PG 的 22P02 报错会原样回到客户端。
	// 这里同时卡住"报错文本外泄"，而不只看它失败。
	if strings.Contains(rec.Body.String(), "SQLSTATE") || strings.Contains(rec.Body.String(), "sql_error") {
		t.Fatalf("unsubscribe unknown must not leak driver errors, body=%s", rec.Body.String())
	}
}

// TestMobileE2EAlarmsEndpointIsReachable 告警列表端点可达且返回分页结构。
func TestMobileE2EAlarmsEndpointIsReachable(t *testing.T) {
	engine, _ := mobileE2E(t, "TENANT_ADMIN")
	rec := doJSON(t, engine, http.MethodGet, "/api/v1/mobile/alarms?page=1&page_size=5", "", nil)
	alarmData := assertAPIOK(t, rec, "alarms")
	if _, exists := alarmData["total"]; !exists {
		t.Fatalf("alarms response must carry total: %s", rec.Body.String())
	}
	if _, exists := alarmData["list"]; !exists {
		t.Fatalf("alarms response must carry list: %s", rec.Body.String())
	}
}

// TestMobileE2EAckMalformedIDDoesNotLeakSQL 探测告警确认端点：
// 非 uuid 的 id 不应把 PG 的 22P02 报错原文带回客户端（与撤销登记那条同源的风险）。
func TestMobileE2EAckMalformedIDDoesNotLeakSQL(t *testing.T) {
	engine, _ := mobileE2E(t, "TENANT_ADMIN")
	rec := doJSON(t, engine, http.MethodPost, "/api/v1/mobile/alarms/no-such-alarm/ack", "", nil)
	assertRouteReached(t, rec, "ack malformed alarm id")
	if strings.Contains(rec.Body.String(), "SQLSTATE") || strings.Contains(rec.Body.String(), "sql_error") {
		t.Fatalf("alarm ack must not leak driver errors, body=%s", rec.Body.String())
	}
}

// 保证 api 包被引用（控制器由 routes 使用，此处显式锚定避免误删）。
var _ = api.MobileApi{}
