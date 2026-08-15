/**
 * 文件用途：聚合导出 AetherLink 前端共享 hooks。
 * 核心逻辑：统一暴露布尔状态、加载状态、上下文和 SVG 图标渲染组合式函数。
 * 关键注意事项：新增导出会成为包级公共 API，需要同步检查调用方命名冲突。
 * 重构建议：后续可按状态、请求和渲染类 hooks 分组导出。
 */
import useBoolean from './use-boolean'
import useLoading from './use-loading'
import useContext from './use-context'
import useSvgIconRender from './use-svg-icon-render'

export { useBoolean, useLoading, useContext, useSvgIconRender }
