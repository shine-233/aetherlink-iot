package dal

// 批次二回归测试（2026-08-24，见 references/gen-inheritance-audit.md）：设备列表读模型与
// 设备读路径必须从全新 gorm Statement 出发。若读链仍从包级 query.Device / DeviceConfig /
// TelemetryCurrentData 单例起 Do 链，高并发下毒化的 Statement.Model 会以陈旧主键 WHERE
// 注入列表 SQL（间歇空列表/漏行/读到旧快照）。
//
// 本测试先向三个单例底层语句写入带非零主键的 Model 残留，再走 GetDeviceListByPage 全链路
// （name 过滤 + search 过滤 + 分页，覆盖 count → 分页 id 扫描 → 按id投影扫描 → 遥测回填），
// 断言修复后：
//  1. 捕获的全部 SQL 都不含毒化主键条件；
//  2. name 过滤真实生效：总数为 1 且目标行出现在结果中；
//  3. 嵌套条件构造器产出纯 field 表达式（不再携带 DO/表单例根），BeCond 可直接导出
//     clause.Expression 供 raw 链消费。
//
// 需要 PostgreSQL；默认跳过，设置 AETHERLINK_TEST_PSQL_DSN 后运行。

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	gormlogger "gorm.io/gorm/logger"
)

type deviceListIsolationWriter struct {
	mu    sync.Mutex
	lines []string
}

func (w *deviceListIsolationWriter) Printf(format string, args ...interface{}) {
	w.mu.Lock()
	w.lines = append(w.lines, fmt.Sprintf(format, args...))
	w.mu.Unlock()
}

func (w *deviceListIsolationWriter) captured() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.lines...)
}

func TestDeviceListReadModelChainsStartFromIsolatedStatements(t *testing.T) {
	dsn := os.Getenv("AETHERLINK_TEST_PSQL_DSN")
	if dsn == "" {
		t.Skip("AETHERLINK_TEST_PSQL_DSN not set; device list isolation regression requires PostgreSQL")
	}

	writer := &deviceListIsolationWriter{}
	capture := gormlogger.New(writer, gormlogger.Config{
		LogLevel:                  gormlogger.Info,
		IgnoreRecordNotFoundError: true,
	})

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:                 capture,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	oldDB := global.DB
	global.DB = db
	query.SetDefault(db)
	if oldDB != nil {
		t.Cleanup(func() {
			global.DB = oldDB
			query.SetDefault(oldDB)
		})
	} else {
		t.Cleanup(func() { global.DB = oldDB })
	}

	const (
		poisonDeviceID = "00000000-0000-0000-0000-00000000d021"
		poisonConfigID = "00000000-0000-0000-0000-00000000c022"
	)

	// 直接污染三个共享根的底层语句 Model：模拟高负载下跨请求残留的非零主键。
	// TelemetryCurrentData 根以 model.Device 作为残留体充当金丝雀——若遥测回填链仍走
	// 单例 Do 链，devices.id=poison 条件会出现在捕获 SQL 中。
	for name, poison := range map[string]struct {
		root *gorm.DB
		dest interface{}
	}{
		"query.Device": {
			root: query.Device.UnderlyingDB(),
			dest: &model.Device{ID: poisonDeviceID},
		},
		"query.DeviceConfig": {
			root: query.DeviceConfig.UnderlyingDB(),
			dest: &model.DeviceConfig{ID: poisonConfigID},
		},
		"query.TelemetryCurrentData": {
			root: query.TelemetryCurrentData.UnderlyingDB(),
			dest: &model.Device{ID: poisonDeviceID},
		},
	} {
		if poison.root == nil || poison.root.Statement == nil {
			t.Fatalf("%s underlying statement unavailable", name)
		}
		poison.root.Statement.Model = poison.dest
	}

	const (
		targetID    = "22222222-3333-4444-5555-666666666666"
		decoyID     = "22222222-3333-4444-5555-666666666667"
		markerName  = "p1-isolation-list-target"
		listTenant  = "tenant-p1-list"
		targetNo    = "p1-target-device-number"
		decoyNumber = "p1-decoy-device-number"
	)
	seedRows := []struct{ id, name, number string }{
		{targetID, markerName, targetNo},
		{decoyID, "p1-isolation-decoy", decoyNumber},
	}
	for _, row := range seedRows {
		if err := db.Exec(`INSERT INTO devices (id, tenant_id, name, device_number, activate_flag, created_at)
			VALUES (?, ?, ?, ?, 'active', NOW())
			ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, tenant_id = EXCLUDED.tenant_id,
				device_number = EXCLUDED.device_number, activate_flag = 'active'`,
			row.id, listTenant, row.name, row.number).Error; err != nil {
			t.Fatalf("seed row %s: %v", row.id, err)
		}
		t.Cleanup(func() {
			db.Exec(`DELETE FROM devices WHERE id = ?`, row.id)
		})
	}

	nameFilter := markerName
	searchFilter := markerName
	req := &model.GetDeviceListByPageReq{
		PageReq: model.PageReq{Page: 1, PageSize: 10},
		Name:    &nameFilter,
		Search:  &searchFilter,
	}
	total, rows, err := GetDeviceListByPage(req, listTenant)
	if err != nil {
		t.Fatalf("GetDeviceListByPage failed: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total=1 after name filter, got %d", total)
	}
	if len(rows) != 1 || rows[0].ID != targetID || rows[0].Name != markerName {
		t.Fatalf("target row missing or wrong: total=%d rows=%v", total, rows)
	}

	captured := writer.captured()
	if len(captured) == 0 {
		t.Fatalf("no SQL statements captured")
	}
	for _, line := range captured {
		if strings.Contains(line, poisonDeviceID) || strings.Contains(line, poisonConfigID) {
			t.Fatalf("stale primary key leaked into device list statement: %s", line)
		}
	}

	t.Run("condition builders produce pure field expressions", func(t *testing.T) {
		cases := []struct {
			name string
			cond gen.Condition
		}{
			{"service_identifier_mqtt", deviceListServiceIdentifierCondition("mqtt")},
			{"service_identifier_plain", deviceListServiceIdentifierCondition("coap")},
			{"device_type_gateway", deviceListDeviceTypeCondition("1")},
			{"device_type_sub", deviceListDeviceTypeCondition("2")},
			{"rdi_shared", rdiDeviceSharedStatusCondition(query.Device.AdditionalInfo, true)},
			{"rdi_unshared", rdiDeviceSharedStatusCondition(query.Device.AdditionalInfo, false)},
		}
		for _, tc := range cases {
			// 携带 TableName() 即说明条件仍是绑定表单例的 DO 根而非纯表达式。
			if _, ok := tc.cond.(interface{ TableName() string }); ok {
				t.Fatalf("%s: condition still carries a DO/table root", tc.name)
			}
			expr, ok := tc.cond.BeCond().(clause.Expression)
			if !ok || expr == nil {
				t.Fatalf("%s: BeCond did not yield clause.Expression", tc.name)
			}
			if err := tc.cond.CondError(); err != nil {
				t.Fatalf("%s: CondError: %v", tc.name, err)
			}
		}
	})
}
