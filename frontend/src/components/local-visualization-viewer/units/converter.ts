import type { Dimension, UnitDefinition, UnitSystem } from './types'

const FAHRENHEIT_SCALE = 5.0 / 9.0
const FAHRENHEIT_OFFSET = 273.15 - 32.0 * FAHRENHEIT_SCALE

export const UNIT_REGISTRY: Record<string, UnitDefinition> = {
  // ---- 温度 (Base K) ----
  '°C': { symbol: '°C', dimension: 'temperature', scale: 1, offset: 273.15, system: 'metric' },
  '°F': { symbol: '°F', dimension: 'temperature', scale: FAHRENHEIT_SCALE, offset: FAHRENHEIT_OFFSET, system: 'imperial' },
  'K': { symbol: 'K', dimension: 'temperature', scale: 1, offset: 0, system: 'metric' },

  // ---- 长度 (Base m) ----
  'mm': { symbol: 'mm', dimension: 'length', scale: 0.001, system: 'metric' },
  'cm': { symbol: 'cm', dimension: 'length', scale: 0.01, system: 'metric' },
  'm': { symbol: 'm', dimension: 'length', scale: 1, system: 'metric' },
  'km': { symbol: 'km', dimension: 'length', scale: 1000, system: 'metric' },
  'in': { symbol: 'in', dimension: 'length', scale: 0.0254, system: 'imperial' },
  'ft': { symbol: 'ft', dimension: 'length', scale: 0.3048, system: 'imperial' },
  'yd': { symbol: 'yd', dimension: 'length', scale: 0.9144, system: 'imperial' },
  'mi': { symbol: 'mi', dimension: 'length', scale: 1609.344, system: 'imperial' },

  // ---- 质量 (Base kg) ----
  'g': { symbol: 'g', dimension: 'mass', scale: 0.001, system: 'metric' },
  'kg': { symbol: 'kg', dimension: 'mass', scale: 1, system: 'metric' },
  't': { symbol: 't', dimension: 'mass', scale: 1000, system: 'metric' },
  'oz': { symbol: 'oz', dimension: 'mass', scale: 0.028349523125, system: 'imperial' },
  'lb': { symbol: 'lb', dimension: 'mass', scale: 0.45359237, system: 'imperial' },

  // ---- 体积 (Base L) ----
  'mL': { symbol: 'mL', dimension: 'volume', scale: 0.001, system: 'metric' },
  'L': { symbol: 'L', dimension: 'volume', scale: 1, system: 'metric' },
  'm3': { symbol: 'm3', dimension: 'volume', scale: 1000, system: 'metric' },
  'fl_oz': { symbol: 'fl_oz', dimension: 'volume', scale: 0.0295735295625, system: 'imperial' },
  'qt': { symbol: 'qt', dimension: 'volume', scale: 0.946352946, system: 'imperial' },
  'gal': { symbol: 'gal', dimension: 'volume', scale: 3.785411784, system: 'imperial' },

  // ---- 面积 (Base m2) ----
  'cm2': { symbol: 'cm2', dimension: 'area', scale: 0.0001, system: 'metric' },
  'm2': { symbol: 'm2', dimension: 'area', scale: 1, system: 'metric' },
  'ha': { symbol: 'ha', dimension: 'area', scale: 10000, system: 'metric' },
  'km2': { symbol: 'km2', dimension: 'area', scale: 1000000, system: 'metric' },
  'in2': { symbol: 'in2', dimension: 'area', scale: 0.00064516, system: 'imperial' },
  'ft2': { symbol: 'ft2', dimension: 'area', scale: 0.09290304, system: 'imperial' },
  'acre': { symbol: 'acre', dimension: 'area', scale: 4046.8564224, system: 'imperial' },

  // ---- 速度 (Base m/s) ----
  'm/s': { symbol: 'm/s', dimension: 'speed', scale: 1, system: 'metric' },
  'km/h': { symbol: 'km/h', dimension: 'speed', scale: 1.0 / 3.6, system: 'metric' },
  'kn': { symbol: 'kn', dimension: 'speed', scale: 0.514444, system: 'imperial' },
  'mph': { symbol: 'mph', dimension: 'speed', scale: 0.44704, system: 'imperial' },

  // ---- 压力 (Base Pa) ----
  'Pa': { symbol: 'Pa', dimension: 'pressure', scale: 1, system: 'metric' },
  'mbar': { symbol: 'mbar', dimension: 'pressure', scale: 100, system: 'metric' },
  'kPa': { symbol: 'kPa', dimension: 'pressure', scale: 1000, system: 'metric' },
  'bar': { symbol: 'bar', dimension: 'pressure', scale: 100000, system: 'metric' },
  'MPa': { symbol: 'MPa', dimension: 'pressure', scale: 1000000, system: 'metric' },
  'psi': { symbol: 'psi', dimension: 'pressure', scale: 6894.757293168, system: 'imperial' },

  // ---- 能量 (Base J) ----
  'J': { symbol: 'J', dimension: 'energy', scale: 1, system: 'metric' },
  'Wh': { symbol: 'Wh', dimension: 'energy', scale: 3600, system: 'metric' },
  'kJ': { symbol: 'kJ', dimension: 'energy', scale: 1000, system: 'metric' },
  'kWh': { symbol: 'kWh', dimension: 'energy', scale: 3600000, system: 'metric' },
  'BTU': { symbol: 'BTU', dimension: 'energy', scale: 1055.05585262, system: 'imperial' },

  // ---- 功率 (Base W) ----
  'W': { symbol: 'W', dimension: 'power', scale: 1, system: 'metric' },
  'kW': { symbol: 'kW', dimension: 'power', scale: 1000, system: 'metric' },
  'MW': { symbol: 'MW', dimension: 'power', scale: 1000000, system: 'metric' },
  'hp': { symbol: 'hp', dimension: 'power', scale: 745.6998715823, system: 'imperial' },

  // ---- 流量 (Base m3/s) ----
  'L/s': { symbol: 'L/s', dimension: 'flow', scale: 0.001, system: 'metric' },
  'L/min': { symbol: 'L/min', dimension: 'flow', scale: 1.0 / 60000, system: 'metric' },
  'm3/h': { symbol: 'm3/h', dimension: 'flow', scale: 1.0 / 3600, system: 'metric' },
  'm3/s': { symbol: 'm3/s', dimension: 'flow', scale: 1, system: 'metric' },
  'cfm': { symbol: 'cfm', dimension: 'flow', scale: 0.0004719474432, system: 'imperial' },
  'gpm': { symbol: 'gpm', dimension: 'flow', scale: 6.30901964e-5, system: 'imperial' },

  // ---- 时间 (Base s) ----
  'ms': { symbol: 'ms', dimension: 'time', scale: 0.001, system: 'metric' },
  's': { symbol: 's', dimension: 'time', scale: 1, system: 'metric' },
  'min': { symbol: 'min', dimension: 'time', scale: 60, system: 'metric' },
  'h': { symbol: 'h', dimension: 'time', scale: 3600, system: 'metric' },

  // ---- 比例 (Base 1) ----
  'ppm': { symbol: 'ppm', dimension: 'ratio', scale: 1e-6, system: 'metric' },
  '%': { symbol: '%', dimension: 'ratio', scale: 0.01, system: 'metric' }
}

export const ALIAS_MAP: Record<string, string> = {
  'C': '°C',
  'degC': '°C',
  'celsius': '°C',
  '℃': '°C',
  'F': '°F',
  'degF': '°F',
  'fahrenheit': '°F',
  '℉': '°F',
  'kelvin': 'K',
  'meter': 'm',
  'meters': 'm',
  'metre': 'm',
  'kilometer': 'km',
  'kilometers': 'km',
  'foot': 'ft',
  'feet': 'ft',
  'inch': 'in',
  'inches': 'in',
  'mile': 'mi',
  'miles': 'mi',
  'gram': 'g',
  'grams': 'g',
  'kilogram': 'kg',
  'kilograms': 'kg',
  'pound': 'lb',
  'pounds': 'lb',
  'lbs': 'lb',
  'ounce': 'oz',
  'liter': 'L',
  'liters': 'L',
  'litre': 'L',
  'milliliter': 'mL',
  'gallon': 'gal',
  'gallons': 'gal',
  'hour': 'h',
  'hours': 'h',
  'minute': 'min',
  'minutes': 'min',
  'second': 's',
  'seconds': 's',
  'percent': '%',
  'mps': 'm/s',
  'kph': 'km/h',
  'degrees_celsius': '°C',
  'degrees_fahrenheit': '°F'
}

export const CANONICAL_UNITS: Record<Dimension, Record<UnitSystem, string>> = {
  temperature: { metric: '°C', imperial: '°F' },
  length: { metric: 'm', imperial: 'ft' },
  mass: { metric: 'kg', imperial: 'lb' },
  volume: { metric: 'L', imperial: 'gal' },
  area: { metric: 'm2', imperial: 'ft2' },
  speed: { metric: 'm/s', imperial: 'mph' },
  pressure: { metric: 'kPa', imperial: 'psi' },
  energy: { metric: 'kWh', imperial: 'BTU' },
  power: { metric: 'kW', imperial: 'hp' },
  flow: { metric: 'm3/h', imperial: 'gpm' },
  time: { metric: 's', imperial: 's' },
  ratio: { metric: '%', imperial: '%' }
}

export function lookupUnit(symbol: string): UnitDefinition | undefined {
  const trimmed = symbol.trim()
  if (UNIT_REGISTRY[trimmed]) {
    return UNIT_REGISTRY[trimmed]
  }
  const aliased = ALIAS_MAP[trimmed]
  if (aliased && UNIT_REGISTRY[aliased]) {
    return UNIT_REGISTRY[aliased]
  }
  return undefined
}

export function getCanonicalUnit(dimension: Dimension, system: UnitSystem): string {
  return CANONICAL_UNITS[dimension]?.[system] ?? ''
}

export function getCompatibleUnits(symbol: string): string[] {
  const u = lookupUnit(symbol)
  if (!u) return []
  return Object.values(UNIT_REGISTRY)
    .filter(item => item.dimension === u.dimension)
    .map(item => item.symbol)
}

export function convertUnit(value: number, from: string, to: string): number {
  if (!Number.isFinite(value)) {
    throw new Error('units: value must be a finite number')
  }
  const fromUnit = lookupUnit(from)
  if (!fromUnit) {
    throw new Error(`units: unknown unit symbol "${from}"`)
  }
  const toUnit = lookupUnit(to)
  if (!toUnit) {
    throw new Error(`units: unknown unit symbol "${to}"`)
  }
  if (fromUnit.dimension !== toUnit.dimension) {
    throw new Error(`units: dimension mismatch between "${from}" and "${to}"`)
  }
  if (fromUnit.symbol === toUnit.symbol) {
    return value
  }

  // To base -> from base
  const fromOffset = fromUnit.offset ?? 0
  const toOffset = toUnit.offset ?? 0
  const baseValue = value * fromUnit.scale + fromOffset
  const converted = (baseValue - toOffset) / toUnit.scale
  if (!Number.isFinite(converted)) {
    throw new Error('units: converted value is not finite')
  }
  return Number(converted.toPrecision(12))
}

export function convertUnitToSystem(value: number, from: string, system: UnitSystem): { value: number; unit: string } {
  const fromUnit = lookupUnit(from)
  if (!fromUnit) {
    throw new Error(`units: unknown unit symbol "${from}"`)
  }
  const target = getCanonicalUnit(fromUnit.dimension, system)
  if (!target) {
    throw new Error(`units: no canonical unit for dimension "${fromUnit.dimension}" in system "${system}"`)
  }
  return {
    value: convertUnit(value, from, target),
    unit: target
  }
}

export function convertSeries(values: number[], from: string, to: string): number[] {
  // Check units first
  convertUnit(0, from, to)
  return values.map((v, i) => {
    try {
      return convertUnit(v, from, to)
    } catch (err: any) {
      throw new Error(`Index ${i} conversion failed: ${err?.message || err}`)
    }
  })
}
