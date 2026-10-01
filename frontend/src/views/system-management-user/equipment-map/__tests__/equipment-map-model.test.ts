/**
 * 文件用途: 覆盖设备地图的纯函数模型（位置解析、时间格式化、遥测渲染）。
 * 核心逻辑: 不依赖组件挂载，直接断言 parseLocation / formatTime / 遥测取值等纯逻辑。
 * 关键注意事项: 位置字段来自设备上报，JSON 与 "lat,lng" 两种形态都必须被识别。
 */
import { describe, expect, it } from 'vitest'
import {
  formatTime,
  parseLocation,
  telemetryLabel,
  telemetryValue,
  toLngLat,
  toMarkerStyle
} from '../equipment-map-model'

describe('equipment-map model', () => {
  it('parses location from a JSON string', () => {
    const result = parseLocation('{"lat": 22.5, "lng": 114.0}')
    expect(result?.lat).toBe(22.5)
    expect(result?.lng).toBe(114.0)
  })

  it('parses location from a comma-separated string', () => {
    const result = parseLocation('22.5, 114.0')
    expect(result?.lat).toBe(22.5)
    expect(result?.lng).toBe(114.0)
  })

  it('returns null for empty location', () => {
    expect(parseLocation('')).toBeNull()
    expect(parseLocation(undefined)).toBeNull()
  })

  it('keeps the raw text when no coordinates can be extracted', () => {
    expect(parseLocation('warehouse A')).toEqual({ raw: 'warehouse A' })
  })

  it('formats time with a locale fallback', () => {
    expect(formatTime(null)).toBe('-')
    expect(formatTime(undefined)).toBe('-')
    expect(formatTime('2024-01-01T00:00:00Z')).toBe(new Date('2024-01-01T00:00:00Z').toLocaleString())
  })

  it('renders telemetry labels and values', () => {
    expect(telemetryLabel({ key: 'temp', label: 'Temperature' })).toBe('Temperature')
    expect(telemetryLabel({ key: 'temp' })).toBe('temp')
    expect(telemetryValue({ key: 'temp', value: 25, unit: '°C' })).toBe('25 °C')
    expect(telemetryValue({ key: 'temp', value: null })).toBe('-')
  })

  it('converts a parsed location into AMap [lng, lat] order', () => {
    expect(toLngLat({ raw: '', lat: 22.5, lng: 114 })).toEqual([114, 22.5])
    expect(toLngLat(null)).toBeNull()
  })

  it('falls back to the grid centre when the location has no coordinates', () => {
    expect(toMarkerStyle(null)).toEqual({ left: '50%', top: '50%' })
    expect(toMarkerStyle({ raw: '' })).toEqual({ left: '50%', top: '50%' })
  })
})
