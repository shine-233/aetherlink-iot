// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"context"

	"github.com/sirupsen/logrus"
)

func DeleteServiceAccess(id string) error {
	q := query.ServiceAccess
	queryBuilder := q.WithContext(context.Background())
	_, err := queryBuilder.Where(q.ID.Eq(id)).Delete()
	return err
}

func UpdateServiceAccess(id string, updates map[string]interface{}) error {
	q := query.ServiceAccess
	queryBuilder := q.WithContext(context.Background())
	_, err := queryBuilder.Where(q.ID.Eq(id)).Updates(updates)
	return err
}

func GetServiceAccessListByPage(req *model.GetServiceAccessByPageReq, tenantID string) (int64, interface{}, error) {
	var count int64
	var serviceAccess = []model.ServiceAccess{}

	q := query.ServiceAccess
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.ServicePluginID.Eq(req.ServicePluginID))
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantID))

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, serviceAccess, err
	}
	queryBuilder = applyListPagination(queryBuilder, req.Page, req.PageSize)

	err = queryBuilder.Select().Order(q.CreateAt.Desc()).Scan(&serviceAccess)
	if err != nil {
		logrus.Error(err)
		return count, serviceAccess, err
	}
	return count, serviceAccess, err
}

// 通过凭证获取服务接入点信息
func GetServiceAccessByVoucher(voucher string, tenantID string) (*model.ServiceAccess, error) {
	// 使用first查询
	q := query.ServiceAccess
	queryBuilder := q.WithContext(context.Background())
	serviceAccess, err := queryBuilder.Where(q.Voucher.Eq(voucher)).Where(q.TenantID.Eq(tenantID)).First()
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return serviceAccess, nil
}

// 通过service_plugin_id获取服务接入点列表
func GetServiceAccessListByServicePluginID(servicePluginID string, tenantID string) ([]model.ServiceAccess, error) {
	q := query.ServiceAccess
	queryBuilder := q.WithContext(context.Background())
	var serviceAccess = []model.ServiceAccess{}
	queryBuilder = queryBuilder.Where(q.ServicePluginID.Eq(servicePluginID))
	if tenantID != "" {
		queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantID))
	}
	err := queryBuilder.Select().Scan(&serviceAccess)
	if err != nil {
		logrus.Error(err)
		return serviceAccess, err
	}
	return serviceAccess, nil
}

// 通过id获取服务接入点信息
// tenant-scope: no-tenant-column?2026-08-26 ?????
func GetServiceAccessByID(id string) (*model.ServiceAccess, error) {
	// 使用first查询
	q := query.ServiceAccess
	queryBuilder := q.WithContext(context.Background())
	serviceAccess, err := queryBuilder.Where(q.ID.Eq(id)).First()
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return serviceAccess, nil
}
