/**
 * 文件用途: visual-editor 组件侧 editor store facade。
 * 核心逻辑: 透传 `store/modules/editor` 的 useEditorStore，保持组件导入路径稳定。
 * 关键注意事项: 这里是兼容层，不应引入新的 store 实例或状态分叉。
 * 重构建议: 若 editor store 迁移，优先保持本文件 re-export 并用 rg 校验调用点。
 */
export { useEditorStore } from '@/store/modules/editor'
