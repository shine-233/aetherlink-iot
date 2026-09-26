// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"github.com/sirupsen/logrus"
)

func UpdateDataPolicy(datapolicy *model.DataPolicy) error {
	p := query.DataPolicy
	_, err := query.DataPolicy.Where(p.ID.Eq(datapolicy.ID)).Updates(datapolicy)
	if err != nil {
		logrus.Error(err)
	}
	return err
}

// CreateDataPolicy 新增数据策略行（TB-15R：行级租户/档案粒度保留）。
// 行级唯一性由 138.sql 的 uq_data_policy_row_level 部分唯一索引兜底，重复创建在此报错。
// tenant-scope: caller-enforced —— 行级策略是平台管理面数据，创建由服务层
// requireDataPolicyAdmin（SYS_ADMIN）前置校验；租户/档案取值即策略语义本身，不做范围过滤。
func CreateDataPolicy(datapolicy *model.DataPolicy) error {
	if err := query.DataPolicy.Create(datapolicy); err != nil {
		logrus.Error(err)
		return err
	}
	return nil
}

// GetDataPolicyByID 按主键取单条策略行（删除入口的行级/全局判定用）。
// tenant-scope: caller-enforced —— 平台管理面读取，服务层 requireDataPolicyAdmin 前置校验。
func GetDataPolicyByID(id string) (*model.DataPolicy, error) {
	p := query.DataPolicy
	row, err := p.Where(p.ID.Eq(id)).First()
	if err != nil {
		logrus.Error(err)
		return nil, err
	}
	return row, nil
}

func DeleteDataPolicy(id string) error {
	_, err := query.DataPolicy.Where(query.DataPolicy.ID.Eq(id)).Delete()
	if err != nil {
		logrus.Error(err)
	}
	return err
}

// tenant-scope: system-table?2026-08-26 ?????
func GetDataPolicyListByPage(datapolicy *model.GetDataPolicyListByPageReq) (int64, interface{}, error) {
	q := query.DataPolicy
	var count int64
	var datapolicyList interface{}
	queryBuilder := q.WithContext(context.Background())

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, datapolicyList, err
	}

	queryBuilder = applyListPagination(queryBuilder, datapolicy.Page, datapolicy.PageSize)

	datapolicyList, err = queryBuilder.Select().Order(q.ID.Asc()).Find()
	if err != nil {
		logrus.Error(err)
		return count, datapolicyList, err
	}

	return count, datapolicyList, err
}

// tenant-scope: system-table?2026-08-26 ?????
func GetDataPolicy() ([]*model.DataPolicy, error) {
	p := query.DataPolicy
	datapolicyList, err := p.Select().Find()
	return datapolicyList, err
}
