/**
 * 文件用途：提供 color-palette 包的公开色板查询入口。
 * 核心逻辑：复用 ./core 的色板派生，并注册色名解析器，使色板项 `.name` 返回最近颜色名称。
 * 关键注意事项：本入口会把 41KB 色名表打进调用方 chunk；只需要 hexcode 的场景请改用
 *   `@aetherlink/color-palette/core`。
 */
import { getColorPalette } from './core'
import { setColorNameResolver } from './palette'
import { getColorName } from './name'

setColorNameResolver(getColorName)

export { getColorByColorPaletteNumber, colorPalettes } from './core'
export { getColorName, getColorPalette }

export default getColorPalette

export type { ColorPalette, ColorPaletteNumber, ColorPaletteItem, ColorPaletteFamily } from './type'
