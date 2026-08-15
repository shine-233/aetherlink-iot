// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"
	"fmt"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"github.com/sirupsen/logrus"
	"gorm.io/gen"
)

// ALTER TABLE ota_upgrade_packages ALTER COLUMN additional_info SET DEFAULT '{}'::json;

func CreateOtaUpgradePackage(p *model.OtaUpgradePackage) error {
	return query.OtaUpgradePackage.Create(p)
}

func UpdateOtaUpgradePackage(p *model.OtaUpgradePackage) (gen.ResultInfo, error) {
	info, err := query.OtaUpgradePackage.Updates(p)
	return info, err
}

func DeleteOtaUpgradePackage(packageId string) error {
	info, err := query.OtaUpgradePackage.Where(query.OtaUpgradePackage.ID.Eq(packageId)).Delete()
	if err != nil {
		return err
	}
	if info.RowsAffected == 0 {
		return fmt.Errorf("no data deleted")
	}
	return nil
}

func GetOtaUpgradePackageByID(id string) (*model.OtaUpgradePackage, error) {
	ota, err := query.OtaUpgradePackage.Where(query.OtaUpgradePackage.ID.Eq(id)).First()
	if err != nil {
		logrus.Error(err)
	}
	return ota, err
}

func GetOtaUpgradePackageListByPage(p *model.GetOTAUpgradePackageLisyByPageReq, tenantId string) (int64, interface{}, error) {
	q := query.OtaUpgradePackage
	var count int64
	packageList := make([]model.GetOTAUpgradeTaskListByPageRsp, 0)
	queryBuilder := q.WithContext(context.Background())
	queryBuilder = queryBuilder.Where(q.TenantID.Eq(tenantId))
	if p.Name != "" {
		queryBuilder = queryBuilder.Where(q.Name.Like(fmt.Sprintf("%%%s%%", p.Name)))
	}

	if p.Version != "" {
		queryBuilder = queryBuilder.Where(q.Version.Like(fmt.Sprintf("%%%s%%", p.Version)))
	}

	if p.DeviceConfigID != "" {
		queryBuilder = queryBuilder.Where(q.DeviceConfigID.Eq(p.DeviceConfigID))
	}

	count, err := queryBuilder.Count()
	if err != nil {
		logrus.Error(err)
		return count, packageList, err
	}

	if p.Page != 0 && p.PageSize != 0 {
		queryBuilder = queryBuilder.Limit(p.PageSize)
		queryBuilder = queryBuilder.Offset((p.Page - 1) * p.PageSize)
	}

	d := query.DeviceConfig
	err = queryBuilder.Select(q.ALL, d.Name.As("device_config_name")).
		LeftJoin(d, d.ID.EqCol(q.DeviceConfigID)).
		Order(q.CreatedAt.Desc()).
		Scan(&packageList)
	if err != nil {
		logrus.Error(err)
		return count, packageList, err
	}
	return count, packageList, err
}
