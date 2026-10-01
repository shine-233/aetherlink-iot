// 文件用途：锁定场景动作批量写入（buildSceneActionRows + 单次多行 INSERT）与创建失败回滚的行为契约。
package dal

import (
	"testing"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"
	utils "aetherlink-iot/backend/pkg/utils"
)

func setupSceneBatchTestDB(t *testing.T) {
	t.Helper()
	db := newDalListLimitTestDB(t)
	if err := db.AutoMigrate(&model.SceneInfo{}, &model.SceneActionInfo{}); err != nil {
		t.Fatalf("migrate scene tables: %v", err)
	}
	oldDB := global.DB
	query.SetDefault(db)
	t.Cleanup(func() {
		if oldDB != nil {
			query.SetDefault(oldDB)
		}
	})
}

func sceneTestActions() []model.SceneActionsReq {
	param, value, remark := "temp", "25", "r"
	return []model.SceneActionsReq{
		{ActionType: "10", ActionTarget: "dev-1", ActionParam: &param, ActionValue: &value, Remark: &remark},
		{ActionType: "11", ActionTarget: "cfg-1"},
		{ActionType: "30", ActionTarget: "alarm-1"},
	}
}

func TestCreateAndUpdateSceneInfoWriteAllActionsInOneBatch(t *testing.T) {
	setupSceneBatchTestDB(t)
	claims := &utils.UserClaims{ID: "user-1", TenantID: "tenant-a"}

	id, err := CreateSceneInfo(model.CreateSceneReq{Name: "s1", Actions: sceneTestActions()}, claims)
	if err != nil {
		t.Fatalf("create scene: %v", err)
	}
	var rows []model.SceneActionInfo
	if err := global.DB.Where("scene_id = ?", id).Order("action_type").Find(&rows).Error; err != nil {
		t.Fatalf("read actions: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("actions = %d, want 3", len(rows))
	}
	seen := map[string]bool{}
	for _, r := range rows {
		if r.TenantID != "tenant-a" || r.SceneID != id || r.UpdatedAt == nil || !r.UpdatedAt.Equal(r.CreatedAt) || seen[r.ID] {
			t.Fatalf("action row fields wrong or duplicate id: %#v", r)
		}
		seen[r.ID] = true
	}
	if rows[0].ActionTarget != "dev-1" || rows[0].ActionParam == nil || *rows[0].ActionParam != "temp" ||
		rows[0].ActionValue == nil || *rows[0].ActionValue != "25" || rows[0].Remark == nil {
		t.Fatalf("first action lost request fields: %#v", rows[0])
	}

	// 编辑：整组替换为 1 条，租户取调用方传入值。
	if _, err := UpdateSceneInfo(model.UpdateSceneReq{ID: id, Name: "s1b", Actions: sceneTestActions()[1:2]}, claims, "tenant-a"); err != nil {
		t.Fatalf("update scene: %v", err)
	}
	rows = nil
	if err := global.DB.Where("scene_id = ?", id).Find(&rows).Error; err != nil {
		t.Fatalf("read actions after update: %v", err)
	}
	if len(rows) != 1 || rows[0].ActionTarget != "cfg-1" || rows[0].TenantID != "tenant-a" {
		t.Fatalf("actions after update = %#v, want only cfg-1", rows)
	}

	// 空动作列表：场景照常写入，不产生动作行也不报错。
	emptyID, err := CreateSceneInfo(model.CreateSceneReq{Name: "empty", Actions: nil}, claims)
	if err != nil {
		t.Fatalf("create scene without actions: %v", err)
	}
	var n int64
	global.DB.Model(&model.SceneActionInfo{}).Where("scene_id = ?", emptyID).Count(&n)
	if n != 0 {
		t.Fatalf("empty scene actions = %d, want 0", n)
	}
}

// 回归：CreateSceneInfo 在 scene_info 插入失败时曾直接 return，事务不回滚，
// 其占用的连接一直处于 in-use，直到连接被回收。
func TestCreateSceneInfoRollsBackWhenSceneInsertFails(t *testing.T) {
	setupSceneBatchTestDB(t)
	if err := global.DB.Migrator().DropTable(&model.SceneInfo{}); err != nil {
		t.Fatalf("drop scene_info: %v", err)
	}
	sqlDB, err := global.DB.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	if _, err := CreateSceneInfo(model.CreateSceneReq{Name: "s", Actions: sceneTestActions()}, &utils.UserClaims{ID: "u", TenantID: "t"}); err == nil {
		t.Fatal("create must fail when scene_info is missing")
	}
	if inUse := sqlDB.Stats().InUse; inUse != 0 {
		t.Fatalf("connections in use after failed create = %d, want 0 (transaction leaked)", inUse)
	}
}
