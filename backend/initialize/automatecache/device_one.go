// 文件用途：定义“单一设备触发”场景使用的自动化缓存维度适配器。
// 核心逻辑：向上层缓存模块暴露固定的 key 前缀和触发条件类型，不直接处理 Redis IO。
// 关键注意事项：返回值参与缓存分流与条件匹配，任何调整都可能影响历史缓存命中结果。
// 重构建议：后续可将适配器注册集中化，避免维度扩展时散落在多个文件维护。

package automatecache

import "aetherlink-iot/backend/internal/model"

// 单一设备
type OneDeviceCache struct{}

func NewOneDeviceCache() *OneDeviceCache {
	return &OneDeviceCache{}
}

// GetAutomateCacheKeyPrefix 返回单一设备维度使用的缓存前缀。
func (*OneDeviceCache) GetAutomateCacheKeyPrefix() string {
	return "one"
}

// GetDeviceTriggerConditionType 返回与单一设备触发条件对应的模型常量。
func (*OneDeviceCache) GetDeviceTriggerConditionType() string {
	return model.DEVICE_TRIGGER_CONDITION_TYPE_ONE
}
