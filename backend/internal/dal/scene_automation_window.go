// 文件用途: 读取场景自动化的执行窗口（P0.4），把新增列暴露给 service 层。
// 核心逻辑: 用字符串列名做只读查询，因此不依赖 scene_automations.gen.go 是否有对应字段。
// 关键注意事项: 未配置窗口的场景必须返回零值（无界），绝不能因取不到配置而拦截存量场景。

package dal

import (
	"context"
	"time"

	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/pkg/global"
)

type sceneAutomationWindowRow struct {
	ID        string     `gorm:"column:id"`
	StartsAt  *time.Time `gorm:"column:execution_starts_at"`
	ExpiresAt *time.Time `gorm:"column:execution_expires_at"`
	Timezone  *string    `gorm:"column:execution_timezone"`
}

// GetSceneAutomationWindows 批量读取执行窗口，返回按 scene automation id 索引的映射。
// 查不到的 id 不出现在映射中，服务层据此视为无界。
//
// 必须带 tenantID：窗口配置是租户私有数据，少了租户过滤就会把 A 租户的
// 可执行区间套用到 B 租户的场景中（仓库的 TestTenantScopeQueryAudit 会拦下无租户查询）。
func GetSceneAutomationWindows(ctx context.Context, tenantID string, sceneAutomationIDs []string) (map[string]model.SceneAutomationWindow, error) {
	windows := make(map[string]model.SceneAutomationWindow, len(sceneAutomationIDs))
	if len(sceneAutomationIDs) == 0 || global.DB == nil {
		return windows, nil
	}
	var rows []sceneAutomationWindowRow
	err := global.DB.WithContext(ctx).
		Table("scene_automations").
		Select("id, execution_starts_at, execution_expires_at, execution_timezone").
		Where("tenant_id = ?", tenantID).
		Where("id IN ?", sceneAutomationIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		timezone := ""
		if row.Timezone != nil {
			timezone = *row.Timezone
		}
		windows[row.ID] = model.SceneAutomationWindow{
			SceneAutomationID: row.ID,
			StartsAt:          row.StartsAt,
			ExpiresAt:         row.ExpiresAt,
			Timezone:          timezone,
		}
	}
	return windows, nil
}
