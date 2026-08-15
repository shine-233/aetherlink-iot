// 文件用途: 集中定义 REQ-05b 设备生命周期状态筛选(lifecycle_status)的取值契约。
// 核心逻辑: 导出与后端 oneof 白名单一一对应的取值集合、下拉选项定义、以及默认值/合法性校验。
// 关键注意事项: 取值必须与后端 `GetDeviceListByPageReq.LifecycleStatus`(validate:"omitempty,oneof=activated inactive transmitted all")
//   逐字对齐——空串会被后端 oneof 拒绝(400),故默认选项禁止用空串。
// transmitted 由“至少成功上报过一次”的事实数据派生,不依赖可漂移的状态列。

/** 与后端 oneof=activated inactive transmitted all 逐字对齐的合法取值(顺序即下拉展示顺序)。 */
export const LIFECYCLE_STATUS_VALUES = ['activated', 'all', 'inactive', 'transmitted'] as const

export type LifecycleStatusValue = (typeof LIFECYCLE_STATUS_VALUES)[number]

/** 每个取值对应的 i18n label key。 */
export const LIFECYCLE_STATUS_LABEL_KEYS: Record<LifecycleStatusValue, string> = {
  activated: 'custom.devicePage.lifecycleActivatedOnly',
  all: 'custom.devicePage.lifecycleAll',
  inactive: 'custom.devicePage.lifecycleInactive',
  transmitted: 'custom.devicePage.lifecycleTransmitted'
}

/**
 * 下拉默认值。fresh load 时页面不主动发 lifecycle_status(交给后端 active-only 默认);
 * 用户显式选择时,首项为 'activated'(=仅已激活),绝不是会被后端拒绝的空串。
 */
export const LIFECYCLE_STATUS_DEFAULT: LifecycleStatusValue = 'activated'

/** 取值是否落在后端 oneof 白名单内(空串/未知值均为 false)。 */
export function isValidLifecycleStatus(value: unknown): value is LifecycleStatusValue {
  return typeof value === 'string' && (LIFECYCLE_STATUS_VALUES as readonly string[]).includes(value)
}

/** 构建 data-table-page 使用的 select options(label 为惰性 i18n 取值)。 */
export function buildLifecycleStatusOptions(t: (key: string) => string) {
  return LIFECYCLE_STATUS_VALUES.map((value) => ({
    value,
    label: () => t(LIFECYCLE_STATUS_LABEL_KEYS[value])
  }))
}
