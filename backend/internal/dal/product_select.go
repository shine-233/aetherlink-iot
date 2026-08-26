// 文件用途：产品选择列表的数据访问——租户过滤 + 名称模糊搜索的最小读取面。
// 核心逻辑：为预注册/设备建档等场景提供产品下拉数据源，仅暴露 id 与名称。
// 关键注意事项：必须强制 tenant 过滤，禁止跨租户产品出现在任何下拉面。
package dal

import (
	"context"
	"strings"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"
)

func GetProductSelectListByPage(req *model.GetProductSelectListReq, tenantID string) (int64, []model.ProductSelectItem, error) {
	items := make([]model.ProductSelectItem, 0)
	q := query.Product
	builder := q.WithContext(context.Background()).Where(q.TenantID.Eq(tenantID))
	if name := strings.TrimSpace(req.Name); name != "" {
		builder = builder.Where(q.Name.Like(ContainsLikePattern(name)))
	}

	count, err := builder.Count()
	if err != nil {
		return 0, items, err
	}
	if req.Page > 0 && req.PageSize > 0 {
		builder = builder.Order(q.CreatedAt.Desc()).Limit(req.PageSize).Offset((req.Page - 1) * req.PageSize)
	}
	err = builder.Select(q.ID, q.Name).Scan(&items)
	return count, items, err
}
