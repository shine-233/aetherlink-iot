/**
 * 文件用途：小部件动态表单的模式定义、校验与模型转换器。
 */

import type {
  ChartWidgetConfig,
  HtmlWidgetConfig,
  LocalWidgetConfig,
  LocalWidgetType,
  MetricWidgetConfig,
  TextWidgetConfig
} from '../types'
import { generateRelationFieldKey } from '../entity-relation/resolver'
import type { DynamicWidgetFormData } from './types'

export function validateWidgetForm(
  type: LocalWidgetType,
  data: DynamicWidgetFormData
): { valid: boolean; errors: string[] } {
  const errors: string[] = []

  if (data.entityRelation?.enabled) {
    if (!data.entityRelation.rootId?.trim()) {
      errors.push('实体关系数据源：起点实体 ID 不能为空')
    }
    if (!data.entityRelation.relationType?.trim()) {
      errors.push('实体关系数据源：关系类型不能为空')
    }
    if (!data.entityRelation.targetKey?.trim()) {
      errors.push('实体关系数据源：目标遥测 Key 不能为空')
    }
  }

  if (data.unitConversion?.enabled) {
    if (!data.unitConversion.unitSystem && !data.unitConversion.targetUnit?.trim()) {
      errors.push('单位换算配置：请选择目标单位制式或输入目标单位符号')
    }
  }

  if (type === 'text') {
    if (!data.title && !data.field && !data.entityRelation?.enabled) {
      errors.push('文本小部件至少需要指定静态文本内容或绑定动态字段')
    }
  } else if (type === 'metric') {
    if (!data.label?.trim()) {
      errors.push('指标标题 (Label) 不能为空')
    }
    if (!data.field?.trim() && !data.entityRelation?.enabled) {
      errors.push('指标绑定字段 (Field) 不能为空')
    }
    if (data.decimals !== undefined && (data.decimals < 0 || data.decimals > 10)) {
      errors.push('小数保留位数必须在 0 到 10 之间')
    }
  } else if (type === 'html') {
    if (!data.html?.trim()) {
      errors.push('HTML 内容不能为空')
    }
  } else if (type === 'line-chart' || type === 'bar-chart') {
    if (data.yMin !== undefined && data.yMax !== undefined && data.yMin >= data.yMax) {
      errors.push('Y 轴最小值必须小于最大值')
    }
  }

  return {
    valid: errors.length === 0,
    errors
  }
}

export function convertFormToWidgetConfig(
  type: LocalWidgetType,
  data: DynamicWidgetFormData
): LocalWidgetConfig {
  const entityRelation = data.entityRelation?.enabled ? data.entityRelation : undefined

  if (type === 'html') {
    const derivedField = entityRelation ? generateRelationFieldKey(entityRelation) : undefined
    const cfg: HtmlWidgetConfig = {
      html: data.html?.trim() || '',
      css: data.css?.trim() || undefined,
      field: data.field?.trim() || derivedField,
      fallback: data.fallback?.trim() || undefined,
      ...(entityRelation ? { entityRelation } : {})
    }
    return cfg
  }

  if (type === 'text') {
    const derivedField = entityRelation ? generateRelationFieldKey(entityRelation) : undefined
    const cfg: TextWidgetConfig = {
      text: data.title || '',
      field: data.field?.trim() || derivedField,
      fallback: data.fallback?.trim() || undefined,
      ...(entityRelation ? { entityRelation } : {})
    }
    return cfg
  }

  if (type === 'metric') {
    const derivedField = entityRelation ? generateRelationFieldKey(entityRelation) : ''
    const cfg: MetricWidgetConfig = {
      label: data.label || data.title || '',
      field: data.field?.trim() || derivedField,
      unit: data.unit?.trim() || undefined,
      decimals: typeof data.decimals === 'number' ? data.decimals : undefined,
      fallback: data.fallback?.trim() || undefined,
      ...(entityRelation ? { entityRelation } : {}),
      ...(data.unitConversion?.enabled ? { unitConversion: data.unitConversion } : {})
    }
    return cfg
  }

  // 图表
  let categories: string[] | undefined
  let values: number[] | undefined

  if (data.categoriesText) {
    categories = data.categoriesText
      .split(/[,，\n]/)
      .map(s => s.trim())
      .filter(Boolean)
  }
  if (data.valuesText) {
    values = data.valuesText
      .split(/[,，\n]/)
      .map(s => s.trim())
      .filter(Boolean)
      .map(Number)
      .filter(n => !Number.isNaN(n))
  }

  const derivedCategoryField = entityRelation ? `${generateRelationFieldKey(entityRelation)}_cats` : undefined
  const derivedValueField = entityRelation ? generateRelationFieldKey(entityRelation) : undefined

  const cfg: ChartWidgetConfig = {
    title: data.title?.trim() || undefined,
    seriesName: data.seriesName?.trim() || undefined,
    categoryField: data.categoryField?.trim() || derivedCategoryField,
    valueField: data.valueField?.trim() || derivedValueField,
    categories: categories && categories.length > 0 ? categories : undefined,
    values: values && values.length > 0 ? values : undefined,
    chartStyle: data.chartStyle,
    colorTheme: data.colorTheme,
    yMin: data.yMin,
    yMax: data.yMax,
    unit: data.unit?.trim() || undefined,
    threshold: data.threshold?.enabled ? data.threshold : undefined,
    timewindow: data.overrideTimewindow ? data.timewindow : undefined,
    ...(entityRelation ? { entityRelation } : {}),
    ...(data.unitConversion?.enabled ? { unitConversion: data.unitConversion } : {})
  }

  return cfg
}

export function convertWidgetConfigToForm(
  type: LocalWidgetType,
  config?: LocalWidgetConfig | null
): DynamicWidgetFormData {
  if (!config) return {}

  if (type === 'html') {
    const c = config as HtmlWidgetConfig
    return {
      html: c.html,
      css: c.css,
      title: c.field,
      field: c.field,
      fallback: c.fallback,
      entityRelation: c.entityRelation ? structuredClone(c.entityRelation) : undefined
    }
  }

  if (type === 'text') {
    const c = config as TextWidgetConfig
    return {
      title: c.text,
      field: c.field,
      fallback: c.fallback,
      entityRelation: c.entityRelation ? structuredClone(c.entityRelation) : undefined
    }
  }

  if (type === 'metric') {
    const c = config as MetricWidgetConfig
    return {
      label: c.label,
      title: c.label,
      field: c.field,
      unit: c.unit,
      decimals: c.decimals,
      fallback: c.fallback,
      entityRelation: c.entityRelation ? structuredClone(c.entityRelation) : undefined,
      unitConversion: c.unitConversion ? structuredClone(c.unitConversion) : undefined
    }
  }

  // 图表
  const c = config as ChartWidgetConfig
  return {
    title: c.title,
    seriesName: c.seriesName,
    categoryField: c.categoryField,
    valueField: c.valueField,
    categoriesText: c.categories?.join(', '),
    valuesText: c.values?.join(', '),
    chartStyle: c.chartStyle || 'line',
    colorTheme: c.colorTheme,
    yMin: c.yMin,
    yMax: c.yMax,
    unit: c.unit,
    threshold: c.threshold || { enabled: false, value: 0 },
    overrideTimewindow: Boolean(c.timewindow),
    timewindow: c.timewindow,
    entityRelation: c.entityRelation ? structuredClone(c.entityRelation) : undefined,
    unitConversion: c.unitConversion ? structuredClone(c.unitConversion) : undefined
  }
}
