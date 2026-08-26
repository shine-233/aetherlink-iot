// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"errors"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func normalizeOwnerUserID(ownerUserID *string) string {
	if ownerUserID == nil {
		return ""
	}
	return strings.TrimSpace(*ownerUserID)
}

func GetVisibleGroupIDsForOwner(tenantId string, ownerUserID string) ([]string, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	if tenantId == "" || ownerUserID == "" {
		return []string{}, nil
	}

	var groupIDs []string
	sql := `
	WITH RECURSIVE directly_visible_groups AS (
		SELECT DISTINCT rgd.group_id AS group_id
		FROM r_group_devices rgd
		INNER JOIN devices d ON d.id = rgd.device_id
		WHERE rgd.tenant_id = ?
		  AND d.tenant_id = ?
		  AND d.owner_user_id = ?
		UNION
		SELECT g.id AS group_id
		FROM groups g
		WHERE g.tenant_id = ?
		  AND g.owner_user_id = ?
	),
	group_tree AS (
		SELECT g.id, g.parent_id
		FROM groups g
		INNER JOIN directly_visible_groups dvg ON dvg.group_id = g.id
		UNION
		SELECT parent.id, parent.parent_id
		FROM groups parent
		INNER JOIN group_tree gt ON gt.parent_id = parent.id
	)
	SELECT DISTINCT id
	FROM group_tree;
	`
	err := global.DB.Raw(sql, tenantId, tenantId, ownerUserID, tenantId, ownerUserID).Scan(&groupIDs).Error
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return groupIDs, nil
}

func IsGroupVisibleToOwner(groupID string, tenantId string, ownerUserID string) (bool, error) {
	groupIDs, err := GetVisibleGroupIDsForOwner(tenantId, ownerUserID)
	if err != nil {
		return false, err
	}
	for _, id := range groupIDs {
		if id == groupID {
			return true, nil
		}
	}
	return false, nil
}

func CreateDeviceGroup(r *model.Group) error {
	return query.Group.Create(r)
}

// DeleteDeviceGroupForTenant 按 id+tenant 双条件删除分组，DAL 层强制租户隔离（安全审计 F4）。
func DeleteDeviceGroupForTenant(id, tenantID string) error {
	_, err := query.Group.Where(query.Group.ID.Eq(id), query.Group.TenantID.Eq(tenantID)).Delete()
	return err
}

func UpdateDeviceGroup(r *model.Group) error {
	_, err := query.Group.Where(query.Group.ID.Eq(r.ID)).Updates(r)
	return err
}

func GetDeviceGroupListByPage(req model.GetDeviceGroupsListByPageReq, tenantId string, ownerUserID *string) (int64, interface{}, error) {
	q := query.Group
	var count int64
	var groupList interface{}
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantId))
	if normalizedOwnerUserID := normalizeOwnerUserID(ownerUserID); normalizedOwnerUserID != "" {
		visibleGroupIDs, err := GetVisibleGroupIDsForOwner(tenantId, normalizedOwnerUserID)
		if err != nil {
			return 0, groupList, err
		}
		if len(visibleGroupIDs) == 0 {
			return 0, []*model.Group{}, nil
		}
		queryBuilder = queryBuilder.Where(q.ID.In(visibleGroupIDs...))
	}
	if req.Name != nil && *req.Name != "" {
		// 转义 LIKE 通配符，防止用户输入的 % 和 _ 被当作通配符
		queryBuilder = queryBuilder.Where(q.Name.Like(ContainsLikePattern(*req.Name)))
	}

	if req.ParentId != nil && *req.ParentId != "" {
		queryBuilder = queryBuilder.Where(q.ParentID.Eq(*req.ParentId))
	}

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, groupList, err
	}

	queryBuilder = applyListPagination(queryBuilder, req.Page, req.PageSize)
	queryBuilder = queryBuilder.Order(q.CreatedAt.Desc())
	groupList, err = queryBuilder.Select().Find()
	if err != nil {
		logrus.Error(err)
		return count, groupList, err
	}

	return count, groupList, err
}

func GetDeviceGroupAll(tenantId string, ownerUserID *string) ([]*model.Group, error) {
	queryBuilder := query.Group.Where(query.Group.TenantID.Eq(tenantId))
	if normalizedOwnerUserID := normalizeOwnerUserID(ownerUserID); normalizedOwnerUserID != "" {
		visibleGroupIDs, err := GetVisibleGroupIDsForOwner(tenantId, normalizedOwnerUserID)
		if err != nil {
			return nil, err
		}
		if len(visibleGroupIDs) == 0 {
			return []*model.Group{}, nil
		}
		queryBuilder = queryBuilder.Where(query.Group.ID.In(visibleGroupIDs...))
	}
	g, err := queryBuilder.Order(query.Group.CreatedAt.Desc()).Find()
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return g, nil
}

func GetAutoBindRootDeviceGroupID(tx *query.Query, tenantId string) (string, error) {
	rootGroups, err := tx.Group.
		Where(tx.Group.TenantID.Eq(tenantId)).
		Where(tx.Group.ParentID.Eq("0")).
		Order(tx.Group.CreatedAt.Asc()).
		Find()
	if err != nil {
		logrus.Error(err)
		return "", err
	}

	if len(rootGroups) != 1 {
		return "", nil
	}

	return rootGroups[0].ID, nil
}

func GetDeviceGroupDetail(id string) (*model.Group, error) {
	d, err := query.Group.Where(query.Group.ID.Eq(id)).First()
	if err != nil {
		logrus.Error(err)
	}
	return d, err
}

type deviceGroupStatisticsRow struct {
	DeviceTotal  int64 `json:"device_total"`
	OnlineTotal  int64 `json:"online_total"`
	OfflineTotal int64 `json:"offline_total"`
	AlarmTotal   int64 `json:"alarm_total"`
}

func GetDeviceGroupStatistics(groupID string, tenantID string, ownerUserID *string) (*model.DeviceGroupStatistics, error) {
	groupIDs, err := GetGroupChildrenIds(groupID)
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		return &model.DeviceGroupStatistics{}, nil
	}

	deviceIDs, err := GetDeviceIdsByGroupIds(groupIDs)
	if err != nil {
		return nil, err
	}
	if len(deviceIDs) == 0 {
		return &model.DeviceGroupStatistics{}, nil
	}

	var row deviceGroupStatisticsRow
	placeholders := strings.TrimRight(strings.Repeat("?,", len(deviceIDs)), ",")
	sql := `
		SELECT
			COUNT(DISTINCT d.id) AS device_total,
			COALESCE(SUM(CASE WHEN d.is_online = 1 THEN 1 ELSE 0 END), 0) AS online_total,
			COALESCE(SUM(CASE WHEN d.is_online = 1 THEN 0 ELSE 1 END), 0) AS offline_total,
			COALESCE(SUM(CASE WHEN lda.alarm_status IN ('H', 'M', 'L') THEN 1 ELSE 0 END), 0) AS alarm_total
		FROM devices d
		LEFT JOIN latest_device_alarms lda ON lda.device_id = d.id AND lda.tenant_id = d.tenant_id
		WHERE d.tenant_id = ?
		  AND (? = '' OR d.owner_user_id = ?)
		  AND d.activate_flag = 'active'
		  AND d.id IN (` + placeholders + `)
	`
	normalizedOwnerUserID := normalizeOwnerUserID(ownerUserID)
	args := make([]interface{}, 0, len(deviceIDs)+3)
	args = append(args, tenantID)
	args = append(args, normalizedOwnerUserID, normalizedOwnerUserID)
	for _, deviceID := range deviceIDs {
		args = append(args, deviceID)
	}

	err = global.DB.Raw(sql, args...).Scan(&row).Error
	if err != nil {
		logrus.Error(err)
		return nil, err
	}

	return &model.DeviceGroupStatistics{
		DeviceTotal:  row.DeviceTotal,
		OnlineTotal:  row.OnlineTotal,
		OfflineTotal: row.OfflineTotal,
		AlarmTotal:   row.AlarmTotal,
	}, nil
}

func GetDeviceGroupTierById(id string) (map[string]interface{}, error) {
	r := make(map[string]interface{})
	sql := `
	WITH RECURSIVE group_chain AS (
		SELECT id, parent_id, name, 1 as level
		FROM groups
		WHERE id = ?
		UNION ALL
		SELECT g.id, g.parent_id, g.name, gc.level + 1
		FROM groups g
		INNER JOIN group_chain gc ON gc.parent_id = g.id
	  )
	  SELECT string_agg(name, '/' ORDER BY level DESC) AS group_path
	  FROM group_chain;
	`
	err := global.DB.Raw(sql, id).Scan(&r)
	if err.Error != nil {
		return nil, err.Error
	}
	return r, nil
}

// GetDeviceGroupTierByIds 批量解析分组层级路径，返回 groupID -> group_path。
// 用单条递归 CTE（带 root_id 分组）替代逐分组查询，消除列表构建时的 N+1。
// 查不到的分组不出现在结果中，与单条版返回空 map 的语义一致。
func GetDeviceGroupTierByIds(ids []string) (map[string]interface{}, error) {
	result := make(map[string]interface{}, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		RootID    string `gorm:"column:root_id"`
		GroupPath string `gorm:"column:group_path"`
	}
	sql := `
	WITH RECURSIVE group_chain AS (
		SELECT id, parent_id, name, 1 as level, id as root_id
		FROM groups
		WHERE id IN (?)
		UNION ALL
		SELECT g.id, g.parent_id, g.name, gc.level + 1, gc.root_id
		FROM groups g
		INNER JOIN group_chain gc ON gc.parent_id = g.id
	  )
	  SELECT root_id, string_agg(name, '/' ORDER BY level DESC) AS group_path
	  FROM group_chain
	  GROUP BY root_id;
	`
	if err := global.DB.Raw(sql, ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.RootID] = row.GroupPath
	}
	return result, nil
}

// 获取目标分组的所有子分组id
func GetGroupChildrenIds(id string) ([]string, error) {
	var ids []string
	sql := `
	WITH RECURSIVE group_chain AS (
		SELECT id, parent_id
		FROM groups
		WHERE id = ?
		UNION ALL
		SELECT g.id, g.parent_id
		FROM groups g
		INNER JOIN group_chain gc ON gc.id = g.parent_id
	  )
	  SELECT id
	  FROM group_chain;
	`
	err := global.DB.Raw(sql, id).Scan(&ids)
	if err.Error != nil {
		return nil, err.Error
	}
	return ids, nil
}

func GetTopGroupNameExist(name string, tenantId string) (*model.Group, error) {
	g, err := query.Group.
		Where(query.Group.TenantID.Eq(tenantId)).
		Where(query.Group.ParentID.Eq("0")).
		Where(query.Group.Name.Eq(name)).
		First()
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return g, nil
}

func GetChildrenGroupNameExist(parentId string, name string, tenantId string) (*model.Group, error) {
	g, err := query.Group.
		Where(query.Group.TenantID.Eq(tenantId)).
		Where(query.Group.ParentID.Eq(parentId)).
		Where(query.Group.Name.Eq(name)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return g, nil
		}
		logrus.Error(err)
		return nil, err
	}
	return g, nil
}

// GetGroupNameExistByTenant 检查租户下是否存在指定名称的分组（无论层级）
func GetGroupNameExistByTenant(name string, tenantId string) (*model.Group, error) {
	g, err := query.Group.
		Where(query.Group.TenantID.Eq(tenantId)).
		Where(query.Group.Name.Eq(name)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		logrus.Error(err)
		return nil, err
	}
	return g, nil
}
