// 文件用途: 集中定义 REQ-58 设备列表搜索增强的筛选键契约(权威清单)。
// 核心逻辑: 导出 device/manage 页 searchConfigs 应当包含的全部筛选键,供契约测试锁定,
//   防止筛选键被误删/漏加而无测试守护(此前 17+1 个内联键零断言,属假覆盖)。
// 关键注意事项: 主清单历史提到的 `device-search-config.ts` 实际并不存在,筛选项内联在
//   `index.vue` 的 searchConfigs 数组;本模块即该契约的显式来源,契约测试读取 index.vue
//   源码提取实际 key 集与此清单比对。
// 重构建议: 若把 searchConfigs 完全提取为工厂函数,可让其 key 直接引用本常量以消除源码解析。

/**
 * device/manage 页设备列表筛选键的权威集合(REQ-58)。
 * 顺序不敏感;契约测试断言 index.vue 的 searchConfigs 键集与此完全一致。
 */
export const DEVICE_SEARCH_KEYS = [
  'group_id', // 设备分组
  'device_config_id', // 设备配置模板
  'is_online', // 在线状态
  'never_reported', // 上报历史(从未上报/至少一次)
  'lifecycle_status', // REQ-05b 生命周期状态(已激活/已安装/传输完成/全部)
  'last_reported_after', // 最近上报时间下界
  'last_reported_before', // 最近上报时间上界
  'warn_status', // 告警状态
  'device_type', // 接入类型(直连/网关/子设备)
  'service_identifier', // 服务标识
  'search', // 自由文本(名称/PID/固件/描述)
  'name', // 设备名称
  'device_number', // 设备编号
  'pid_number', // PID
  'firmware_version', // 固件版本
  'description', // 描述
  'shared_status', // 共享状态
  'label' // 标签
] as const

export type DeviceSearchKey = (typeof DEVICE_SEARCH_KEYS)[number]
