package dal

import (
	"context"
	"time"

	global "aetherlink-iot/backend/pkg/global"
)

// listLatestIDsByTypeKey 按租户 + 可选行业类型列出 table 中的记录 ID，
// 同名记录仅保留最新版本（version 字典序最大；版本相同取 created_at 最晚）。
// 供模板市场与资源中心导出共用，保证资源包内不包含重名资源。
// tenant-scope: tenant_id 硬过滤；调用方负责 global.DB 非空检查。
func listLatestIDsByTypeKey(ctx context.Context, table, tenantID, typeKey string) ([]string, error) {
	type item struct {
		ID        string    `gorm:"column:id"`
		Name      string    `gorm:"column:name"`
		Version   *string   `gorm:"column:version"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	q := global.DB.WithContext(ctx).
		Table(table).
		Select("id, name, version, created_at").
		Where("tenant_id = ?", tenantID)
	if typeKey != "" {
		q = q.Where("type_key = ?", typeKey)
	}
	var rows []item
	if err := q.Order("created_at ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	latestByName := make(map[string]item, len(rows))
	for _, r := range rows {
		prev, exists := latestByName[r.Name]
		if !exists || isNewerVersionedRow(r.Version, r.CreatedAt, prev.Version, prev.CreatedAt) {
			latestByName[r.Name] = r
		}
	}
	ids := make([]string, 0, len(latestByName))
	for _, it := range latestByName {
		ids = append(ids, it.ID)
	}
	return ids, nil
}

// isNewerVersionedRow 判断 (ver, at) 是否比 (prevVer, prevAt) 更新；nil 版本视为空串。
func isNewerVersionedRow(ver *string, at time.Time, prevVer *string, prevAt time.Time) bool {
	v, pv := derefString(ver), derefString(prevVer)
	return v > pv || (v == pv && at.After(prevAt))
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
