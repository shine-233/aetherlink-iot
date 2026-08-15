// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"errors"

	model "aetherlink-iot/backend/internal/model"
	query "aetherlink-iot/backend/internal/query"

	"gorm.io/gorm"
)

// 根据类型获取配置（邮件/短信）
func GetNotificationServicesConfigByType(noticeType string) (*model.NotificationServicesConfig, error) {
	data, err := query.NotificationServicesConfig.Where(query.NotificationServicesConfig.NoticeType.Eq(noticeType)).Take()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, err
	}
	return data, nil
}

// 创建/保存配置
func SaveNotificationServicesConfig(data *model.NotificationServicesConfig) (*model.NotificationServicesConfig, error) {
	err := query.NotificationServicesConfig.Save(data)
	if err != nil {

		return nil, err
	}
	return data, nil
}
