package dal

// 文件用途：设备分组的树形读取与重名校验。
// 核心逻辑：层级路径 / 子孙 ID 都用递归 CTE 在库内完成，避免在 Go 里逐层往返；
//   批量版带 root_id 分组，一次拿回整批分组的路径。
// 关键注意事项：
//   - 本文件的 SQL 都不带 tenant_id 谓词，属 caller-enforced：调用方（service 层）
//     必须先校验分组归属，否则会跨租户泄漏分组名。
//   - 重名校验里 ErrRecordNotFound 是正常分支（表示名字可用），不能当成错误抛出。

import (
	"errors"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	global "aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// tenant-scope: caller-enforced (service 层校验分组归属后调用，reviewed 2026-08-26)
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
// tenant-scope: caller-enforced (service 层校验分组归属后调用，reviewed 2026-08-26)
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
// tenant-scope: caller-enforced (service 层校验分组归属后调用，reviewed 2026-08-26)
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
