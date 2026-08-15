/**
 * 文件用途: visual-editor 类型 barrel export。
 * 核心逻辑: 汇总基础类型并向组件、store、configuration 模块提供稳定类型入口。
 * 关键注意事项: 导出名称变更会造成跨模块类型漂移，尤其影响配置导入导出和 data-architecture。
 * 重构建议: 保留兼容导出，新增类型时标注是否属于持久化 schema。
 */
export type { GraphData, CanvasState } from './base-types'

export type EditorMode = 'design' | 'preview' | 'edit'

export interface WidgetDefinition {
  id: string
  type?: string
  name?: string
  component?: any
  config?: Record<string, any>
  [key: string]: any
}

export interface ComponentDefinition {
  id?: string
  type?: string
  name?: string
  dataSourceKeys?: string[]
  [key: string]: any
}

export interface ReactiveDataBinding {
  source?: string
  target?: string
  path?: string
  [key: string]: any
}

export interface WidgetConfiguration {
  base?: Record<string, any>
  component?: Record<string, any>
  dataSource?: Record<string, any> | null
  interaction?: Record<string, any>
  metadata?: Record<string, any>
  customize?: Record<string, any>
  [key: string]: any
}

export interface VisualEditorWidget extends WidgetDefinition {
  configuration?: WidgetConfiguration
}
