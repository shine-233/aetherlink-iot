/**
 * 文件用途：根据输入颜色计算最接近的颜色名称。
 * 核心逻辑：读取内置颜色名称数据，并综合 RGB 与 HSL 距离寻找最小差异项。
 * 关键注意事项：结果依赖静态 JSON 数据质量，不等同于设计系统中的语义色命名。
 * 性能：1566 条色名的 RGB/HSL 在首次调用时一次性解析成扁平表（原实现每次调用都对
 *   每条色名重新 colord 解析两遍，~1ms/次）；结果按输入 hex 缓存。
 */
import { getHex, getHsl, getRgb } from './color'
import colorNames from './json/color-name.json'

type ColorNameEntry = {
  hex: string
  name: string
  r: number
  g: number
  b: number
  h: number
  s: number
  l: number
}

let table: ColorNameEntry[] | null = null
const nameCache = new Map<string, string>()
const NAME_CACHE_LIMIT = 512

function getTable(): ColorNameEntry[] {
  if (table) return table
  table = (colorNames as [string, string][]).map(([hexValue, name]) => {
    const hex = `#${hexValue}`
    const { r, g, b } = getRgb(hex)
    const { h, s, l } = getHsl(hex)
    return { hex, name, r, g, b, h, s, l }
  })
  return table
}

export function getColorName(color: string) {
  const hex = getHex(color)
  const cached = nameCache.get(hex)
  if (cached !== undefined) return cached

  const entries = getTable()
  const rgb = getRgb(color)
  const hsl = getHsl(color)

  let best = -1
  let bestDistance = -1
  for (let index = 0; index < entries.length; index += 1) {
    const entry = entries[index]
    // 精确命中直接返回该色名（旧实现会被循环后的赋值覆盖成"此前最近项"，属 bug）。
    if (entry.hex === hex) {
      best = index
      break
    }
    const rgbDistance = (rgb.r - entry.r) ** 2 + (rgb.g - entry.g) ** 2 + (rgb.b - entry.b) ** 2
    const hslDistance = (hsl.h - entry.h) ** 2 + (hsl.s - entry.s) ** 2 + (hsl.l - entry.l) ** 2
    const distance = rgbDistance + hslDistance * 2
    if (bestDistance < 0 || bestDistance > distance) {
      bestDistance = distance
      best = index
    }
  }

  const name = best < 0 ? 'Invalid Color' : entries[best].name
  if (nameCache.size >= NAME_CACHE_LIMIT) nameCache.clear()
  nameCache.set(hex, name)
  return name
}
