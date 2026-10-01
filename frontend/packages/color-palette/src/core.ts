/**
 * 文件用途：color-palette 的轻量入口（`@aetherlink/color-palette/core`）。
 * 核心逻辑：只包含色板派生，不引入色名表；色板项的 `.name` 在未注册解析器时回退为 hexcode。
 * 关键注意事项：需要真实色名的调用方使用包主入口（会注册 getColorName）。
 */
import { getColorPaletteFamily } from './palette'
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

  colorPaletteFamily.palettes.forEach((palette) => {
    colorMap.set(palette.number, palette)
  })

  const mainColor = colorMap.get(500) as ColorPaletteItem
  const matchColor = colorPaletteFamily.palettes.find((palette) => palette.hexcode === color) as ColorPaletteItem

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

/** The builtin color palettes */
export const colorPalettes = defaultPalettes as ColorPaletteFamily[]

export type { ColorPalette, ColorPaletteNumber, ColorPaletteItem, ColorPaletteFamily }
