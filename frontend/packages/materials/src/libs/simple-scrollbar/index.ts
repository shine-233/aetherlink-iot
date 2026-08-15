/**
 * 文件用途：导出 SimpleScrollbar 滚动容器组件。
 * 核心逻辑：将同目录 Vue 组件作为默认导出，供 materials 包聚合使用。
 * 关键注意事项：当前是 simplebar-vue 的薄封装，不包含额外滚动业务逻辑。
 * 重构建议：如需扩展 props 或事件，应先明确与 simplebar-vue 的透传边界。
 */
import SimpleScrollbar from './index.vue'

export default SimpleScrollbar
