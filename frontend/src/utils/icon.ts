/*
 * 文件用途：提供本地图标名称收集工具。
 * 核心逻辑：通过 Vite glob 读取本地 SVG 模块路径并转换为图标名称。
 * 关键注意事项：路径约定依赖构建工具和图标目录结构，移动资源时需要同步验证。
 * 重构建议：可把图标目录约定集中到常量中，降低路径散落风险。
 */
export function getLocalIcons() {
  const svgIcons = import.meta.glob('/src/assets/svg-icon/*.svg')

  const keys = Object.keys(svgIcons)
    .map(item => item.split('/').at(-1)?.replace('.svg', '') || '')
    .filter(Boolean)

  return keys
}
