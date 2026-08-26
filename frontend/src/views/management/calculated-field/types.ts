/**
 * 文件用途：计算字段页面的本地视图类型。
 * 核心逻辑：声明设备模板下拉选项的最小形状（id/name），供模板映射与下拉渲染复用。
 * 关键注意事项：仅描述视图层消费的字段，不承担接口契约职责；接口类型见 service/api/calculated_field.ts。
 */
export interface DeviceTemplateOption {
  id: string
  name: string
  [key: string]: unknown
}
