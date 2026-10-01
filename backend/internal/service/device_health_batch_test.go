// 文件用途：锁定租户批量健康评估的评分身份语义——批量预取已有评分替代逐设备查询后，
// 已有评分的 ID/CreatedAt 必须保持不变，新设备获得新 ID，且不产生重复行。
package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"aetherlink-iot/backend/internal/model"
	global "aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupDeviceHealthBatchTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	oldDB := global.DB
	dbName := "device_health_batch_" + strings.ReplaceAll(t.Name(), "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.Device{}, &model.AlarmHistory{}, &model.DeviceHealthScore{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 与生产表一致：upsert 依赖 (tenant_id, device_id) 唯一约束。
	if err := db.Exec("CREATE UNIQUE INDEX ux_health_tenant_device ON device_health_scores (tenant_id, device_id)").Error; err != nil {
		t.Fatalf("create unique index: %v", err)
	}
	global.DB = db
	t.Cleanup(func() { global.DB = oldDB })
	return db
}

func TestEvaluateTenantDeviceHealthKeepsExistingScoreIdentity(t *testing.T) {
	db := setupDeviceHealthBatchTestDB(t)
	const tenantID = "tenant-health"

	for _, id := range []string{"dev-a", "dev-b"} {
		name := id
		if err := db.Create(&model.Device{ID: id, Name: &name, DeviceNumber: "num-" + id, TenantID: tenantID, Voucher: "{}"}).Error; err != nil {
			t.Fatalf("seed device %s: %v", id, err)
		}
	}
	// 另一租户的同名设备评分不得被复用。
	createdAt := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	seed := []*model.DeviceHealthScore{
		{ID: "score-a", DeviceID: "dev-a", TenantID: tenantID, Score: 10, HealthStatus: model.HealthStatusCritical, Details: "{}", EvaluatedAt: createdAt, CreatedAt: createdAt, UpdatedAt: createdAt},
		{ID: "score-other", DeviceID: "dev-b", TenantID: "tenant-other", Score: 10, HealthStatus: model.HealthStatusCritical, Details: "{}", EvaluatedAt: createdAt, CreatedAt: createdAt, UpdatedAt: createdAt},
	}
	for _, s := range seed {
		if err := db.Create(s).Error; err != nil {
			t.Fatalf("seed score %s: %v", s.ID, err)
		}
	}

	svc := &DeviceHealthService{}
	if _, err := svc.EvaluateTenantDeviceHealth(context.Background(), &utils.UserClaims{TenantID: tenantID}); err != nil {
		t.Fatalf("EvaluateTenantDeviceHealth: %v", err)
	}

	var rows []model.DeviceHealthScore
	if err := db.Where("tenant_id = ?", tenantID).Order("device_id").Find(&rows).Error; err != nil {
		t.Fatalf("load scores: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("score rows = %d, want 2 (one per device)", len(rows))
	}
	if rows[0].DeviceID != "dev-a" || rows[0].ID != "score-a" {
		t.Fatalf("dev-a score identity changed: %+v", rows[0])
	}
	if !rows[0].CreatedAt.Equal(createdAt) {
		t.Fatalf("dev-a CreatedAt = %v, want %v", rows[0].CreatedAt, createdAt)
	}
	if rows[0].Score == 10 {
		t.Fatalf("dev-a score was not re-evaluated")
	}
	if rows[1].DeviceID != "dev-b" || rows[1].ID == "" || rows[1].ID == "score-other" {
		t.Fatalf("dev-b should get a fresh score id, got %+v", rows[1])
	}
}
