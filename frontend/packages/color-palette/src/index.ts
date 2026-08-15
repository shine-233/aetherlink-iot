/**
 * 文件用途：提供 color-palette 包的公开色板查询入口。
 * 核心逻辑：按输入颜色和可选色名返回色板项、色板族或最近颜色名称。
 * 关键注意事项：默认色板来自本地 JSON，数据结构变更会影响所有公开查询结果。
 * 重构建议：可允许调用方注入色板数据，便于测试和多主题扩展。
 */
import { getColorPaletteFamily } from './palette'
import { getColorName } from './name'
import type { ColorPalette, ColorPaletteFamily, ColorPaletteItem, ColorPaletteNumber } from './type'
import defaultPalettes from './json/palette.json'

/**
 * Get color palette by provided color and color name
 *
 * @param color The provided color
 * @param colorName Color name
 */
export function getColorPalette(color: string, colorName: string) {
  const colorPaletteFamily = getColorPaletteFamily(color, colorName)

  const colorMap = new Map<ColorPaletteNumber, ColorPaletteItem>()

  colorPaletteFamily.palettes.forEach(palette => {
    colorMap.set(palette.number, palette)
  })

  const mainColor = colorMap.get(500) as ColorPaletteItem
  const matchColor = colorPaletteFamily.palettes.find(palette => palette.hexcode === color) as ColorPaletteItem

  const colorPalette: ColorPalette = {
    ...colorPaletteFamily,
    colorMap,
    main: mainColor,
    match: matchColor
  }

  return colorPalette
}

/**
 * Get color by color palette number
 *
 * @param color Color
 * @param num Color palette number
 * @returns Color hexcode
 */
export function getColorByColorPaletteNumber(color: string, num: ColorPaletteNumber) {
  const colorPalette = getColorPalette(color, color)

  const colorItem = colorPalette.colorMap.get(num) as ColorPaletteItem

  return colorItem.hexcode
}

export default getColorPalette

/** The builtin color palettes */
const colorPalettes = defaultPalettes as ColorPaletteFamily[]

export { getColorName, colorPalettes }

export type { ColorPalette, ColorPaletteNumber, ColorPaletteItem, ColorPaletteFamily }
