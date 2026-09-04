// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"context"

	query "aetherlink-iot/backend/internal/query"
	"aetherlink-iot/backend/pkg/errcode"

	"github.com/sirupsen/logrus"
)

// UpdateLogo 按租户作用域更新品牌配置；仅能修改本租户（或系统全局 tenant_id 为空串）的行。
// 跨租户写入因 WHERE 同时约束 id 与 tenant_id 而命中 0 行，返回无权限错误，杜绝串扰。
//
// gorm-gen 预存缺陷与真实修复点（经真实后端复现确认，2026-09-03）：
//
//	`StructToMapAndVerifyJson` 会把非指针字段恒写入 map，因此 UpdateLogoReq 的 Id 总会进
//	logomap。`.Updates(logomap)` 在 map 含主键 "id" 时，gorm-gen 会把主键值回填到其共享的
//	*model.Logo（query.Logo 单例自 newLogo 起持有同一 model 实例，WithContext 只换 gorm.DB
//	不换 model），于是**上一次调用的 id 被 GORM 以未限定 `"id" = ...` 形式残留进本次 WHERE**，
//	与本次 id 叠加成 `id=A AND id=B` → 恒命中 0 行（自有租户写入被误拒）。
//	仅加 .WithContext() 不足以修复（model 仍共享）；根治 = 写入前把主键/租户列从 SET map 剥离，
//	id 只作 WHERE 定位、绝不重写，map 永不含主键 → 共享 model 的 PK 恒为空 → 无残留回显。
func UpdateLogo(tenantID string, logoID string, logomap map[string]interface{}) error {
	m := make(map[string]interface{}, len(logomap))
	for k, v := range logomap {
		if k == "id" || k == "tenant_id" {
			continue // 主键/租户列只允许作为 WHERE 定位，禁止写入 SET
		}
		m[k] = v
	}
	logo := query.Logo
	info, err := logo.WithContext(context.Background()).
		Where(logo.ID.Eq(logoID), logo.TenantID.Eq(tenantID)).
		Updates(m)
	if err != nil {
		logrus.Error(err)
		return err
	}
	if info.RowsAffected == 0 {
		return errcode.NewWithMessage(errcode.CodeNoPermission, "logo not found or not owned by tenant")
	}
	return nil
}

// GetLogoList 按租户作用域返回品牌配置列表（tenant_id 为空串时取系统全局行）。
// 每次查询均从 .WithContext() 取独立 DO，避免全局 query.Logo 的 statement 被复用/累加。
func GetLogoList(tenantID string) (int64, interface{}, error) {
	var count int64
	logoList, err := query.Logo.WithContext(context.Background()).
		Where(query.Logo.TenantID.Eq(tenantID)).Find()
	if err != nil {
		logrus.Error(err)
		return count, logoList, err
	}
	count, err = query.Logo.WithContext(context.Background()).
		Where(query.Logo.TenantID.Eq(tenantID)).Count()
	return count, logoList, err
}
