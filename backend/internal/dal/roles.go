// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"fmt"
	"strings"
	"time"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/global"

	"github.com/sirupsen/logrus"
	"gorm.io/gen"
)

func CreateRole(data *model.Role) error {
	return query.Role.Create(data)
}

// tenant-scope: no-tenant-column?2026-08-26 ?????
func GetRoleByID(id string) (model.Role, error) {
	var data model.Role
	err := query.Role.Where(query.Role.ID.Eq(id)).Scan(&data)
	if err != nil {
		logrus.Error(err)
	}
	return data, err
}

func UpdateRole(data *model.Role, tenantID string) (gen.ResultInfo, error) {
	p := query.Role

	t := time.Now().UTC()
	data.UpdatedAt = &t

	queryBuilder := query.Role.Where(p.ID.Eq(data.ID))
	if strings.TrimSpace(tenantID) != "" {
		queryBuilder = queryBuilder.Where(p.TenantID.Eq(tenantID))
	}

	info, err := queryBuilder.Updates(data)
	if err != nil {
		logrus.Error(err)
	}
	return info, err
}

func DeleteRole(id string, tenantID string) error {
	queryBuilder := query.Role.Where(query.Role.ID.Eq(id))
	if strings.TrimSpace(tenantID) != "" {
		queryBuilder = queryBuilder.Where(query.Role.TenantID.Eq(tenantID))
	}
	_, err := queryBuilder.Delete()
	if err != nil {
		logrus.Error(err)
	}
	return err
}

func GetRoleListByPage(data *model.GetRoleListByPageReq, tenantID string) (int64, interface{}, error) {
	q := query.Role
	var count int64
	var dataList interface{}
	queryBuilder := q.WithContext(context.Background())

	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantID))
	if data.Name != nil && *data.Name != "" {
		queryBuilder = queryBuilder.Where(q.Name.Like(fmt.Sprintf("%%%s%%", *data.Name)))
	}

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, dataList, err
	}

	queryBuilder = applyListPagination(queryBuilder, data.Page, data.PageSize)

	dataList, err = queryBuilder.Select().Order(q.UpdatedAt).Find()
	if err != nil {
		logrus.Error(err)
		return count, dataList, err
	}

	return count, dataList, err
}

// 查询用户的角色
// GetRoleIDsByTenants 返回归属于任一指定租户的角色 ID 列表。
// 语义（供 C2 RBAC 角色集继承 service.RoleExpander 使用）：
//   - 空输入（nil/全空白）返回空切片、无错误，调用方无需特判；
//   - 仅返回 tenant_id 命中入参的角色；系统级角色（tenant_id 为 NULL/空串）一律排除，
//     避免把平台级角色误当作祖先租户角色继承，造成权限放大；
//   - 出错时如实上抛（调用方按"扩展失败即跳过、绝不放行"处理，不阻断鉴权主流程）。
// tenant-scope: caller-enforced（入参即租户白名单，等价于显式 IN 过滤）
func GetRoleIDsByTenants(tenantIDs []string) ([]string, error) {
	ids := make([]string, 0, len(tenantIDs))
	seen := make(map[string]struct{}, len(tenantIDs))
	for _, id := range tenantIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return []string{}, nil
	}

	var roles []*model.Role
	if err := query.Role.WithContext(context.Background()).
		Where(query.Role.TenantID.In(ids...)).
		Select(query.Role.ID).
		Scan(&roles); err != nil {
		logrus.WithError(err).Error("failed to load role ids by tenants")
		return nil, err
	}

	roleIDs := make([]string, 0, len(roles))
	for _, r := range roles {
		if r == nil || r.ID == "" {
			continue
		}
		roleIDs = append(roleIDs, r.ID)
	}
	return roleIDs, nil
}

// 查询用户的角色
// tenant-scope: no-tenant-column?2026-08-26 ?????
func GetRolesByUserId(userId string) ([]string, bool) {
	if global.CasbinEnforcer == nil {
		return nil, false
	}
	policys, err := global.CasbinEnforcer.GetFilteredNamedGroupingPolicy("g", 0, userId)
	if err != nil {
		logrus.WithError(err).Error("failed to load roles for user")
		return nil, false
	}
	var roles []string
	for _, policy := range policys {
		if len(policy) < 2 {
			continue
		}
		roles = append(roles, policy[1])
	}
	return roles, true
}

// tenant-scope: no-tenant-column?2026-08-26 ?????
func GetRolesByUserIds(userIds []string) map[string][]string {
	rolesByUserID := make(map[string][]string, len(userIds))
	if len(userIds) == 0 || global.CasbinEnforcer == nil {
		return rolesByUserID
	}

	wantedUserIDs := make(map[string]struct{}, len(userIds))
	for _, userID := range userIds {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			continue
		}
		wantedUserIDs[userID] = struct{}{}
		rolesByUserID[userID] = nil
	}
	if len(wantedUserIDs) == 0 {
		return rolesByUserID
	}

	policys, err := global.CasbinEnforcer.GetNamedGroupingPolicy("g")
	if err != nil {
		logrus.WithError(err).Error("failed to load role policies")
		return rolesByUserID
	}
	for _, policy := range policys {
		if len(policy) < 2 {
			continue
		}
		userID := strings.TrimSpace(policy[0])
		if _, ok := wantedUserIDs[userID]; !ok {
			continue
		}
		rolesByUserID[userID] = append(rolesByUserID[userID], policy[1])
	}

	return rolesByUserID
}
