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

func DeleteDataPolicy(id string) error {
	_, err := query.DataPolicy.Where(query.DataPolicy.ID.Eq(id)).Delete()
	if err != nil {
		logrus.Error(err)
	}
	return err
}

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

func GetDataPolicy() ([]*model.DataPolicy, error) {
	p := query.DataPolicy
	datapolicyList, err := p.Select().Find()
	return datapolicyList, err
}
