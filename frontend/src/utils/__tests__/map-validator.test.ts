/**
 * 文件用途：验证地图经纬度校验工具的范围与边界行为。
 * 核心逻辑：纬度 [-90, 90]、经度 [-180, 180]，且不允许 (0, 0) 同时为零；
 *   字符串重载先 Number() 再走数字校验，非法字符串与空串都被拒绝。
 * 关键注意事项：(0, 0) 约定视为"未标注"无效，而非真实的几内亚湾坐标。
 * 重构建议：若业务需要允许 (0, 0)，应通过参数开关而非直接放开断言。
 */
import { describe, expect, it } from 'vitest'
import {
  getCoordinateStringValidationError,
  getCoordinateValidationError,
  isValidCoordinate,
  isValidCoordinateString
} from '../common/map-validator'

describe('isValidCoordinate', () => {
  it('合法坐标返回 true', () => {
    expect(isValidCoordinate(39.9, 116.4)).toBe(true)
    expect(isValidCoordinate(-90, -180)).toBe(true)
    expect(isValidCoordinate(90, 180)).toBe(true)
  })

  it('超出范围返回 false', () => {
    expect(isValidCoordinate(90.1, 0)).toBe(false)
    expect(isValidCoordinate(-90.1, 0)).toBe(false)
    expect(isValidCoordinate(0, 180.1)).toBe(false)
    expect(isValidCoordinate(0, -180.1)).toBe(false)
  })

  it('(0, 0) 视为未标注，返回 false', () => {
    expect(isValidCoordinate(0, 0)).toBe(false)
  })

  it('NaN 返回 false', () => {
    expect(isValidCoordinate(Number.NaN, 10)).toBe(false)
    expect(isValidCoordinate(10, Number.NaN)).toBe(false)
  })
})

describe('isValidCoordinateString', () => {
  it('数字字符串通过校验', () => {
    expect(isValidCoordinateString('39.9', '116.4')).toBe(true)
    expect(isValidCoordinateString(' 39.9 ', '116.4')).toBe(true)
  })

  it('空串与非法字符串被拒绝', () => {
    expect(isValidCoordinateString('', '116.4')).toBe(false)
    expect(isValidCoordinateString('abc', '116.4')).toBe(false)
  })
})

describe('getCoordinateValidationError', () => {
  it('非法数字给出类型提示', () => {
    expect(getCoordinateValidationError(Number.NaN, 10)).toBe('经纬度必须是有效数字')
  })

  it('范围越界给出维度对应提示', () => {
    expect(getCoordinateValidationError(91, 0)).toContain('纬度')
    expect(getCoordinateValidationError(0, 181)).toContain('经度')
  })

  it('(0, 0) 给出零点提示', () => {
    expect(getCoordinateValidationError(0, 0)).toContain('不能同时为0')
  })

  it('合法坐标返回 null', () => {
    expect(getCoordinateValidationError(39.9, 116.4)).toBe(null)
  })
})

describe('getCoordinateStringValidationError', () => {
  it('与数字版语义一致', () => {
    expect(getCoordinateStringValidationError('91', '0')).toContain('纬度')
    expect(getCoordinateStringValidationError('abc', '0')).toBe('经纬度必须是有效数字')
    expect(getCoordinateStringValidationError('39.9', '116.4')).toBe(null)
  })
})
