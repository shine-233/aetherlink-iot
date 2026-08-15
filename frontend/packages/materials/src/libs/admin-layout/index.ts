/**
 * 文件用途：导出 AdminLayout 组件和布局相关常量。
 * 核心逻辑：将 Vue 组件作为默认导出，并转发滚动容器 ID 与最大层级常量。
 * 关键注意事项：这是组件包入口之一，导出名称变更会影响外部调用方。
 * 重构建议：后续可统一 materials 组件导出规范，减少各组件入口差异。
 */
import AdminLayout from './index.vue'
import { LAYOUT_MAX_Z_INDEX, LAYOUT_SCROLL_EL_ID } from './shared'

export default AdminLayout
export { LAYOUT_SCROLL_EL_ID, LAYOUT_MAX_Z_INDEX }
