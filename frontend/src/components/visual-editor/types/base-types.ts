/**
 * 文件用途: visual-editor 基础类型定义。
 * 核心逻辑: 描述图形节点、画布状态和编辑器基础实体，供配置、store 和运行时组件共享。
 * 关键注意事项: 字段会进入持久化 dashboard 或导入导出数据，删除/改名需保留迁移兼容。
 * 重构建议: 将持久化字段与运行时临时字段分层，并为 schema 变化补迁移测试。
 */
export interface GraphData {
  id: string
  type?: string
  componentType?: string
  name?: string
  label?: string
  x?: number
  y?: number
  w?: number
  h?: number
  properties?: Record<string, any>
  config?: Record<string, any>
  metadata?: Record<string, any>
  [key: string]: any
}

export interface CanvasState {
  zoom: number
  offsetX: number
  offsetY: number
  [key: string]: any
}
