/**
 * 文件用途：定义 color-palette 包使用的色板类型。
 * 核心逻辑：声明色板编号、色板项、色板族以及带最近色板信息的扩展类型。
 * 关键注意事项：类型需要与 json 目录中的静态数据结构保持一致。
 * 重构建议：后续可增加 JSON 数据校验类型或生成脚本，减少手工维护偏差。
 */
/** The color palette number */
export type ColorPaletteNumber = 50 | 100 | 200 | 300 | 400 | 500 | 600 | 700 | 800 | 900 | 950

/** The color palette item */
export type ColorPaletteItem = {
  /** The color hexcode */
  hexcode: string
  /**
   * The color number
   *
   * @link {@link ColorPaletteNumber}
   */
  number: ColorPaletteNumber
  /** The color name */
  name: string
}

export type ColorPaletteFamily = {
  /** The color palette family key */
  key: string
  /** The color palette family's palettes */
  palettes: ColorPaletteItem[]
}

export type ColorPaletteWithDelta = ColorPaletteItem & {
  delta: number
}

export type ColorPaletteItemWithName = ColorPaletteItem & {
  name: string
}

export type ColorPaletteFamilyWithNearestPalette = ColorPaletteFamily & {
  nearestPalette: ColorPaletteWithDelta
  nearestLightnessPalette: ColorPaletteWithDelta
}

export type ColorPalette = ColorPaletteFamily & {
  /** The color map of the palette */
  colorMap: Map<ColorPaletteNumber, ColorPaletteItem>
  /**
   * The main color of the palette
   *
   * Which number is 500
   */
  main: ColorPaletteItemWithName
  /** The match color of the palette */
  match: ColorPaletteItemWithName
}
