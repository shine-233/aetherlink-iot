/**
 * 文件用途：PageTab 组件对外导出入口。
 * 核心逻辑：默认导出 index.vue，方便材料库按目录导入页签组件。
 * 主要逻辑：保持轻量转发，不在入口层引入额外副作用。
 * 关键注意事项：如果未来增加命名导出，应同步更新材料库统一导出清单。
 * 重构建议：建议在包级 README 中记录新增导出的兼容策略。
 */
import PageTab from './index.vue'

export default PageTab
