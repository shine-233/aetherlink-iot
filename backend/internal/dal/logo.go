// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"

	query "aetherlink-iot/backend/internal/query"

	"github.com/sirupsen/logrus"
)

func UpdateLogo(logoID string, logomap map[string]interface{}) error {
	p := query.Logo
	_, err := query.Logo.Where(p.ID.Eq(logoID)).Updates(logomap)
	if err != nil {
		logrus.Error(err)
	}
	return err
}

func GetLogoList() (int64, interface{}, error) {
	q := query.Logo
	var count int64
	queryBuilder := q.WithContext(context.Background())

	logoList, err := queryBuilder.Select().Find()
	if err != nil {
		logrus.Error(err)
		return count, logoList, err
	}
	count, err = queryBuilder.Count()
	return count, logoList, err
}
