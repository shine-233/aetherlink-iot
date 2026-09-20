/**
 * 文件用途：小部件动态表单配置（Dynamic Form Configurator）类型定义。
 *
 * 对标 ThingsBoard 4.0 动态表单体系：
 * 1. 动态绑定遥测字段与实体属性；
 * 2. 视觉样式微调（折线/平滑曲线/面积图、调色盘、Y 轴极值）；
 * 3. 告警/报警静态阈值线（Threshold Line）；
 * 4. 局部时间窗口覆盖（Widget-level Timewindow override）。
 */

import type { TimewindowConfig } from '../timewindow/types'
import type { EntityRelationSourceConfig } from '../entity-relation/types'
import type { UnitConversionConfig } from '../units/types'

export type ChartStyleType = 'line' | 'smooth' | 'area' | 'bar'

export interface ThresholdConfig {
  enabled: boolean
  value: number
  label?: string
  color?: string
}

export interface DynamicWidgetFormData {
  // 通用
  title?: string
  field?: string
  fallback?: string

  // HTML 小部件特定 (ROADMAP TB-11 对标 ThingsBoard 4.3.1.2)
  html?: string
  css?: string

  // 指标特定
  label?: string
  unit?: string
  decimals?: number

  // 图表特定
  seriesName?: string
  chartStyle?: ChartStyleType
  categoryField?: string
  valueField?: string
  categoriesText?: string
  valuesText?: string
  colorTheme?: string
  yMin?: number
  yMax?: number
  threshold?: ThresholdConfig

  // 时间窗口局部覆盖
  overrideTimewindow?: boolean
  timewindow?: TimewindowConfig

  // 实体关系数据源 (ROADMAP P1.1)
  entityRelation?: EntityRelationSourceConfig

  // 单位换算 (ROADMAP TB-9 对标 ThingsBoard Units Conversion)
  unitConversion?: UnitConversionConfig
}
