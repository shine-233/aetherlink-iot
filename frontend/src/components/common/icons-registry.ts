/**
 * 文件用途：集中导出通用图标组件注册表。
 * 核心逻辑：从图标库导入菜单、按钮、表格和功能面板常用图标，并以稳定名称向外暴露。
 * 关键注意事项：导出名称可能被路由元信息或共享组件间接引用，删除或改名需先完成调用方迁移。
 * 重构建议：可按业务域或图标来源拆分注册表，再通过当前入口维持兼容导出。
 */
import { coreIcons } from './icons-registry-core'
import { deviceIcons } from './icons-registry-device'
import { visualizationIcons } from './icons-registry-visualization'

export const icons = {
  ...coreIcons,
  ...deviceIcons,
  ...visualizationIcons
}
