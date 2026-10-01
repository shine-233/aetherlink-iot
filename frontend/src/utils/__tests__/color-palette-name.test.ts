import { describe, expect, it } from 'vitest'
import colorNames from '../../../packages/color-palette/src/json/color-name.json'
import { getHex, getHsl, getRgb } from '../../../packages/color-palette/src/color'
import { getColorName } from '../../../packages/color-palette/src/name'
import { getColorByColorPaletteNumber, getColorPalette } from '../../../packages/color-palette/src/index'

/** 旧实现的最近邻搜索（每次逐条 colord 解析），仅用于对拍非精确命中的结果。 */
function referenceNearestName(color: string) {
  const rgb = getRgb(color)
  const hsl = getHsl(color)
  let cl = -1
  let df = -1
  ;(colorNames as [string, string][]).forEach(([hexValue], index) => {
    const { r, g, b } = getRgb(`#${hexValue}`)
    const { h, s, l } = getHsl(`#${hexValue}`)
    const ndf = (rgb.r - r) ** 2 + (rgb.g - g) ** 2 + (rgb.b - b) ** 2 + ((hsl.h - h) ** 2 + (hsl.s - s) ** 2 + (hsl.l - l) ** 2) * 2
    if (df < 0 || df > ndf) {
      df = ndf
      cl = index
    }
  })
  return cl < 0 ? 'Invalid Color' : (colorNames as [string, string][])[cl][1]
}

function seededColors(count: number) {
  let seed = 0x2f6e2b1
  const next = () => {
    seed = (seed * 1103515245 + 12345) & 0x7fffffff
    return seed
  }
  return Array.from({ length: count }, () => `#${(next() & 0xffffff).toString(16).padStart(6, '0')}`)
}

describe('color-palette getColorName (precomputed table)', () => {
  it('matches the reference nearest-name search for non-exact colors', () => {
    const exact = new Set((colorNames as [string, string][]).map(([hex]) => `#${hex.toLowerCase()}`))
    const samples = seededColors(120).filter((c) => !exact.has(getHex(c)))
    for (const color of samples) {
      expect(getColorName(color)).toBe(referenceNearestName(color))
    }
  })

  it('returns the exact table name on an exact hex hit', () => {
    const [firstHex, firstName] = (colorNames as [string, string][])[0]
    const [midHex, midName] = (colorNames as [string, string][])[700]
    expect(getColorName(`#${firstHex}`)).toBe(firstName)
    expect(getColorName(`#${midHex}`)).toBe(midName)
  })

  it('serves repeated lookups from cache with identical results', () => {
    expect(getColorName('#1677ff')).toBe(getColorName('#1677FF'))
  })
})

describe('color-palette lazy palette names', () => {
  it('keeps palette hexcodes and names stable and enumerable', () => {
    const palette = getColorPalette('#646cff', 'primary')
    expect(palette.main.hexcode).toBe(palette.colorMap.get(500)?.hexcode)
    expect(palette.palettes).toHaveLength(11)
    for (const item of palette.palettes) {
      expect(Object.keys(item)).toEqual(['hexcode', 'number', 'name'])
      expect(item.name).toBe(getColorName(item.hexcode))
      // spread/JSON 序列化也必须带上 name
      expect(JSON.parse(JSON.stringify(item)).name).toBe(item.name)
    }
    expect(getColorByColorPaletteNumber('#646cff', 700)).toBe(palette.colorMap.get(700)?.hexcode)
  })

  it('allows overriding name', () => {
    const item = getColorPalette('#52c41a', 'success').palettes[0]
    item.name = 'custom'
    expect(item.name).toBe('custom')
  })
})
