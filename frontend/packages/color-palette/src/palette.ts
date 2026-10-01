/**
 * 文件用途：计算输入颜色对应的最近色板族和派生色板。
 * 核心逻辑：使用 DeltaE 找到最近默认色板，再按 HSL 差值生成同族梯度。
 * 关键注意事项：派生色板依赖默认色板编号完整性，缺失编号会影响输出稳定性。
 * 重构建议：可把最近匹配和梯度派生拆成独立函数，并补充固定样例测试。
 */
import { getDeltaE, getHsl, isValidColor, transformHslToHex } from './color'
import { getColorName } from './name'
import type { ColorPaletteFamily, ColorPaletteFamilyWithNearestPalette } from './type'
import defaultPalettes from './json/palette.json'

export function getNearestColorPaletteFamily(color: string, families: ColorPaletteFamily[]) {
  const familyWithConfig = families.map((family) => {
    const palettes = family.palettes.map((palette) => {
      return {
        ...palette,
        delta: getDeltaE(color, palette.hexcode)
      }
    })

    const nearestPalette = palettes.reduce((prev, curr) => (prev.delta < curr.delta ? prev : curr))

    return {
      ...family,
      palettes,
      nearestPalette
    }
  })

  const nearestPaletteFamily = familyWithConfig.reduce((prev, curr) =>
    prev.nearestPalette.delta < curr.nearestPalette.delta ? prev : curr
  )

  const { l } = getHsl(color)

  const paletteFamily: ColorPaletteFamilyWithNearestPalette = {
    ...nearestPaletteFamily,
    nearestLightnessPalette: nearestPaletteFamily.palettes.reduce((prev, curr) => {
      const { l: prevLightness } = getHsl(prev.hexcode)
      const { l: currLightness } = getHsl(curr.hexcode)

      const deltaPrev = Math.abs(prevLightness - l)
      const deltaCurr = Math.abs(currLightness - l)

      return deltaPrev < deltaCurr ? prev : curr
    })
  }

  return paletteFamily
}

/**
 * 色名只用于展示（主题/CSS 变量构建只读 hexcode），而 getColorName 需要全表最近邻搜索。
 * 用可枚举的惰性 getter 推迟到真正读取 `.name` 时再算，并在首次读取后固化为普通值。
 */
function withLazyName(hexcode: string, number: ColorPaletteFamily['palettes'][number]['number']) {
  const item = { hexcode, number } as ColorPaletteFamily['palettes'][number]
  Object.defineProperty(item, 'name', {
    enumerable: true,
    configurable: true,
    get() {
      const name = getColorName(hexcode)
      Object.defineProperty(item, 'name', { value: name, enumerable: true, configurable: true, writable: true })
      return name
    },
    set(value: string) {
      Object.defineProperty(item, 'name', { value, enumerable: true, configurable: true, writable: true })
    }
  })
  return item
}

export function getColorPaletteFamily(color: string, colorName: string) {
  if (!isValidColor(color)) {
    throw new Error('Invalid color, please check color value!')
  }

  const { h: h1, s: s1 } = getHsl(color)

  const { nearestLightnessPalette, palettes } = getNearestColorPaletteFamily(
    color,
    defaultPalettes as ColorPaletteFamily[]
  )

  const { number, hexcode } = nearestLightnessPalette

  const { h: h2, s: s2 } = getHsl(hexcode)

  const deltaH = h1 - h2 || h2

  const sRatio = s1 / s2

  const colorPaletteFamily: ColorPaletteFamily = {
    key: colorName,
    palettes: palettes.map((palette) => {
      let hexValue = color

      const isSame = number === palette.number

      if (!isSame) {
        const { h: h3, s: s3, l } = getHsl(palette.hexcode)

        const newH = deltaH < 0 ? h3 + deltaH : deltaH
        const newS = s3 * sRatio

        hexValue = transformHslToHex({
          h: newH,
          s: newS,
          l
        })
      }

      return withLazyName(hexValue, palette.number)
    })
  }

  return colorPaletteFamily
}
