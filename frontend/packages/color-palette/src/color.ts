/**
 * 文件用途：封装颜色校验、格式转换和色差计算工具。
 * 核心逻辑：基于 colord 及 lab 插件输出 HEX、RGB、HSL 和 DeltaE 结果。
 * 关键注意事项：输入色值必须先确认有效，否则转换结果可能无法代表真实颜色。
 * 重构建议：可为无效输入和边界色值增加统一错误策略与测试样例。
 */
import { colord, extend } from 'colord'
import type { HslColor } from 'colord'
import labPlugin from 'colord/plugins/lab'

extend([labPlugin])

export function isValidColor(color: string) {
  return colord(color).isValid()
}

export function getHex(color: string) {
  return colord(color).toHex()
}

export function getRgb(color: string) {
  return colord(color).toRgb()
}

export function getHsl(color: string) {
  return colord(color).toHsl()
}

export function getDeltaE(color1: string, color2: string) {
  return colord(color1).delta(color2)
}

export function transformHslToHex(color: HslColor) {
  return colord(color).toHex()
}
