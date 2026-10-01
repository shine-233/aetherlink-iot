// 文件用途：锁定数据库层热路径优化（139.sql 索引、报表运行列表批量投递查询、审计导出列投影）的行为契约。
package dal

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

// readMigrationBody 读取 backend/sql/<name> 并去掉整行注释，供迁移形态断言使用。
func readMigrationBody(t *testing.T, name string) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve migration test source path")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "..", "..", "sql", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	var body strings.Builder
	for _, line := range strings.Split(string(raw), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			body.WriteString(line)
			body.WriteString("\n")
		}
	}
	return body.String()
}

// assertIdempotentIndexMigration 校验迁移只含 IF NOT EXISTS 索引、不使用 CONCURRENTLY
// （pg_init.go 在单个事务里执行迁移：CONCURRENTLY 会直接报错），并包含期望的索引定义。
func assertIdempotentIndexMigration(t *testing.T, name string, wants []string) {
	t.Helper()
	sql := readMigrationBody(t, name)
	if strings.Contains(strings.ToUpper(sql), "CONCURRENTLY") {
		t.Fatalf("%s must not use CREATE INDEX CONCURRENTLY inside the migration transaction", name)
	}
	creates := regexp.MustCompile(`(?i)CREATE\s+(UNIQUE\s+)?INDEX\s+(\S+\s+\S+\s+\S+\s+)?(\S+)`).FindAllStringSubmatch(sql, -1)
	if len(creates) == 0 {
		t.Fatalf("%s declares no indexes", name)
	}
	if n := len(regexp.MustCompile(`(?i)CREATE\s+INDEX\s+IF\s+NOT\s+EXISTS\s`).FindAllString(sql, -1)); n != len(creates) {
		t.Fatalf("every CREATE INDEX in %s must be IF NOT EXISTS: %d of %d", name, n, len(creates))
	}
	for _, want := range wants {
		if !strings.Contains(sql, want) {
			t.Fatalf("%s missing hot-path index on %s", name, want)
		}
	}
}

func TestMigration139HotPathIndexesAreIdempotentAndTransactionSafe(t *testing.T) {
	if global.VERSION_NUMBER < 139 {
		t.Fatalf("VERSION_NUMBER = %d, want at least 139", global.VERSION_NUMBER)
	}
	assertIdempotentIndexMigration(t, "139.sql", []string{
		"public.operation_logs (tenant_id, created_at DESC)",
		"public.alarm_history (tenant_id, create_at DESC)",
		"public.command_set_logs (device_id, created_at DESC)",
		"public.ota_upgrade_task_details (device_id)",
	})
}

func TestMigration140SceneAutomationIndexesAreIdempotentAndTransactionSafe(t *testing.T) {
	if global.VERSION_NUMBER < 140 {
		t.Fatalf("VERSION_NUMBER = %d, want at least 140", global.VERSION_NUMBER)
	}
	assertIdempotentIndexMigration(t, "140.sql", []string{
		"public.device_trigger_condition (trigger_source, trigger_condition_type)",
		"public.device_trigger_condition (scene_automation_id)",
		"public.action_info (scene_automation_id)",
		"public.action_info (action_target, action_type)",
		"public.scene_action_info (scene_id)",
		"public.r_group_device (device_id)",
	})
}

func TestReportRunListDeliveriesBatchesAndSkipsPayload(t *testing.T) {
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.ReportScheduleDelivery{}); err != nil {
		t.Fatalf("migrate deliveries: %v", err)
	}
	now := time.Now().UTC()
	lastErr := "smtp_timeout"
	rows := []*model.ReportScheduleDelivery{
		{RunID: "run-1", TenantID: "tenant-a", EnvelopeFrom: "a@example.com", EnvelopeRecipients: []string{"b@example.com"},
			MessageID: "m1", Subject: "s", Payload: []byte("large-report-body"), PayloadDigest: "d", Status: model.ReportDeliveryStatusAmbiguous,
			AttemptCount: 2, LastError: &lastErr, CompletedAt: &now, CreatedAt: now, UpdatedAt: now},
		// 其他租户的同 run_id 投递行不得串入。
		{RunID: "run-2", TenantID: "tenant-b", EnvelopeFrom: "a@example.com", EnvelopeRecipients: []string{"b@example.com"},
			MessageID: "m2", Subject: "s", PayloadDigest: "d", Status: "accepted", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(rows).Error; err != nil {
		t.Fatalf("seed deliveries: %v", err)
	}
	runs := []*model.ReportScheduleRun{{ID: "run-1"}, {ID: "run-2"}, {ID: "run-3"}}

	got, err := reportRunListDeliveries(db, "tenant-a", runs)
	if err != nil {
		t.Fatalf("batch deliveries: %v", err)
	}
	if len(got) != 1 || got["run-1"] == nil {
		t.Fatalf("deliveries = %#v, want only run-1 for tenant-a", got)
	}
	d := got["run-1"]
	if d.Status != model.ReportDeliveryStatusAmbiguous || d.AttemptCount != 2 || d.LastError == nil || *d.LastError != lastErr || d.CompletedAt == nil {
		t.Fatalf("delivery projection lost fields used by ToResponse: %#v", d)
	}
	if len(d.Payload) != 0 {
		t.Fatal("list projection must not load the report payload")
	}
	// 响应投影与逐行查询时一致：有投递行取其状态，无投递行为 pending。
	if resp := runs[0].ToResponse(got["run-1"]); !resp.DuplicateDeliveryRisk || resp.DeliveryAttempts != 2 || resp.DeliveryErrorCode != lastErr {
		t.Fatalf("run-1 response = %#v", resp)
	}
	if resp := runs[2].ToResponse(got["run-3"]); resp.DeliveryStatus != model.ReportDeliveryStatusPending {
		t.Fatalf("run without delivery status = %q, want pending", resp.DeliveryStatus)
	}

	empty, err := reportRunListDeliveries(db, "tenant-a", nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty runs = (%#v, %v), want empty map without query error", empty, err)
	}
}

func TestListOperationLogsForExportOmitsPayloadColumns(t *testing.T) {
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.OperationLog{}); err != nil {
		t.Fatalf("migrate operation logs: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	path, name, action, req, rsp := "/api/v1/device", "create", "create", `{"secret":"x"}`, `{"ok":true}`
	latency := int64(12)
	status := int32(200)
	if err := db.Create(&model.OperationLog{
		ID: "log-1", IP: "10.0.0.1", Path: &path, UserID: "u1", Name: &name, CreatedAt: now,
		Latency: &latency, RequestMessage: &req, ResponseMessage: &rsp, TenantID: "tenant-a",
		Action: &action, StatusCode: &status,
	}).Error; err != nil {
		t.Fatalf("seed operation log: %v", err)
	}

	rows, err := ListOperationLogsForExport("tenant-a", now.Add(-time.Minute), now.Add(time.Minute), model.AuditLogExportReq{}, 10)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.RequestMessage != nil || row.ResponseMessage != nil {
		t.Fatal("export must not read request/response payload columns")
	}
	if row.ID != "log-1" || row.IP != "10.0.0.1" || row.UserID != "u1" || row.TenantID != "tenant-a" ||
		row.Path == nil || *row.Path != path || row.Name == nil || row.Action == nil ||
		row.Latency == nil || *row.Latency != latency || row.StatusCode == nil || *row.StatusCode != status {
		t.Fatalf("export row lost CSV columns: %#v", row)
	}
}

func TestCustomerAssignDevicesBatchMovesAndStaysIdempotent(t *testing.T) {
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.Customer{}, &model.CustomerDevice{}, &model.Device{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Now().UTC()
	for _, c := range []model.Customer{{ID: "c1", Name: "c1", TenantID: "t1", AdditionalInfo: "{}", CreatedAt: now, UpdatedAt: now}, {ID: "c2", Name: "c2", TenantID: "t1", AdditionalInfo: "{}", CreatedAt: now, UpdatedAt: now}} {
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("seed customer: %v", err)
		}
	}
	for _, id := range []string{"d1", "d2", "d3"} {
		if err := db.Exec("INSERT INTO devices (id, tenant_id, voucher, is_enabled, activate_flag, device_number, is_online) VALUES (?, ?, ?, ?, ?, ?, 0)", id, "t1", "v-"+id, "enabled", "active", "n-"+id).Error; err != nil {
			t.Fatalf("seed device: %v", err)
		}
	}
	dal := &CustomerDal{}
	ctx := t.Context()
	if err := dal.AssignDevices(ctx, "c1", "t1", []string{"d1", "d2", "d2", ""}); err != nil {
		t.Fatalf("assign c1: %v", err)
	}
	// 再次分配（含已属 c1 的 d2）到 c2：d2 移动、d3 新增、d1 保持在 c1；重复分配不报主键冲突。
	if err := dal.AssignDevices(ctx, "c2", "t1", []string{"d2", "d3"}); err != nil {
		t.Fatalf("assign c2: %v", err)
	}
	if err := dal.AssignDevices(ctx, "c2", "t1", []string{"d2", "d3"}); err != nil {
		t.Fatalf("re-assign c2 must be idempotent: %v", err)
	}
	var links []model.CustomerDevice
	if err := db.Order("device_id").Find(&links).Error; err != nil {
		t.Fatalf("read links: %v", err)
	}
	got := make([]string, 0, len(links))
	for _, l := range links {
		got = append(got, l.DeviceID+"="+l.CustomerID+"/"+l.ID)
	}
	want := []string{"d1=c1/c1_d1", "d2=c2/c2_d2", "d3=c2/c2_d3"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("links = %v, want %v", got, want)
	}
	if err := dal.AssignDevices(ctx, "c1", "t1", []string{"d1", "missing"}); err == nil {
		t.Fatal("unknown device must reject the whole batch")
	}
}
