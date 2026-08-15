// 文件用途: 提供 DAL 层手写数据访问方法，封装业务对象对应的查询、写入、缓存或聚合读取职责。
// 核心逻辑: 组合 GORM Gen query、事务句柄和模型转换，向 service 层暴露稳定的持久化操作边界。
// 关键注意事项: 新增或修改查询时必须保持租户隔离、权限前置校验结果、事务原子性和缓存一致性，避免跨租户泄漏或半提交。
// 重构建议: 将复杂筛选、分页和事务步骤拆成可测试 helper，补齐 focused DAL 测试后再调整查询组合。

package dal

import (
	"aetherlink-iot/backend/internal/model"
	"aetherlink-iot/backend/internal/query"
	"errors"

	"gorm.io/gorm"
)

// ErrRecordNotFound 记录未找到错误
var ErrRecordNotFound = gorm.ErrRecordNotFound

// GetDeviceConfigByTemplateSecret 通过设备配置密钥获取设备配置
func GetDeviceConfigByTemplateSecret(templateSecret string) (*model.DeviceConfig, error) {
	deviceConfig, err := query.DeviceConfig.Where(query.DeviceConfig.TemplateSecret.Eq(templateSecret)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return deviceConfig, nil
}

// GetProductByProductKey 通过产品密钥获取产品信息
func GetProductByProductKey(productKey string) (*model.Product, error) {
	product, err := query.Product.Where(query.Product.ProductKey.Eq(productKey)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return product, nil
}
