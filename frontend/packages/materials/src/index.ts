// 文件用途：统一导出前端 materials 包中的布局、页签、滚动条组件和公共类型。
// 核心逻辑：从 libs 子目录聚合 AdminLayout、PageTab、SimpleScrollbar 以及布局常量，并透传 types。
// 关键注意事项：这是包级公开入口，修改导出名称会影响前端工作区内所有依赖 materials 的调用方。
// 重构建议：建议为新增物料建立独立子目录 README 和导出兼容说明，避免入口文件变成无说明的杂项聚合。
import AdminLayout, { LAYOUT_MAX_Z_INDEX, LAYOUT_SCROLL_EL_ID } from './libs/admin-layout'
import PageTab from './libs/page-tab'
import SimpleScrollbar from './libs/simple-scrollbar'

export { AdminLayout, LAYOUT_SCROLL_EL_ID, LAYOUT_MAX_Z_INDEX, PageTab, SimpleScrollbar }
export * from './types'
