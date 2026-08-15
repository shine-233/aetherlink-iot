/**
 * 文件用途：根据输入颜色计算最接近的颜色名称。
 * 核心逻辑：读取内置颜色名称数据，并综合 RGB 与 HSL 距离寻找最小差异项。
 * 关键注意事项：结果依赖静态 JSON 数据质量，不等同于设计系统中的语义色命名。
 * 重构建议：可把距离计算拆成纯函数，配合小型样例数据进行单元测试。
 */
import { getHex, getHsl, getRgb } from './color'
import colorNames from './json/color-name.json'

export function getColorName(color: string) {
  const hex = getHex(color)
  const rgb = getRgb(color)
  const hsl = getHsl(color)

  let ndf = 0
  let ndf1 = 0
  let ndf2 = 0
  let cl = -1
  let df = -1

  let name = ''

  colorNames.some((item, index) => {
    const [hexValue, colorName] = item

    const hexcode = `#${hexValue}`

    const match = hex === hexcode

    if (match) {
      name = colorName
    } else {
      const { r, g, b } = getRgb(hexcode)
      const { h, s, l } = getHsl(hexcode)

      ndf1 = (rgb.r - r) ** 2 + (rgb.g - g) ** 2 + (rgb.b - b) ** 2
      ndf2 = (hsl.h - h) ** 2 + (hsl.s - s) ** 2 + (hsl.l - l) ** 2

      ndf = ndf1 + ndf2 * 2
      if (df < 0 || df > ndf) {
        df = ndf
        cl = index
      }
    }

    return match
  })

  name = cl < 0 ? 'Invalid Color' : colorNames[cl][1]

  return name
}
