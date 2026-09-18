import { describe, expect, it } from 'vitest'
import {
  convertSeries,
  convertUnit,
  convertUnitToSystem,
  getCompatibleUnits,
  lookupUnit
} from './converter'

describe('Units Converter Engine', () => {
  describe('lookupUnit', () => {
    it('finds exact symbols', () => {
      const c = lookupUnit('°C')
      expect(c).toBeDefined()
      expect(c?.dimension).toBe('temperature')
      expect(c?.system).toBe('metric')

      const psi = lookupUnit('psi')
      expect(psi).toBeDefined()
      expect(psi?.dimension).toBe('pressure')
      expect(psi?.system).toBe('imperial')
    })

    it('finds aliases and unicode variants', () => {
      const c1 = lookupUnit('℃')
      expect(c1?.symbol).toBe('°C')
      const c2 = lookupUnit('degC')
      expect(c2?.symbol).toBe('°C')
      const f1 = lookupUnit('℉')
      expect(f1?.symbol).toBe('°F')
      const f2 = lookupUnit('fahrenheit')
      expect(f2?.symbol).toBe('°F')
    })

    it('returns undefined for unknown units', () => {
      expect(lookupUnit('unknown_unit')).toBeUndefined()
    })
  })

  describe('convertUnit', () => {
    it('converts temperature correctly (Celsius to Fahrenheit and Kelvin)', () => {
      expect(convertUnit(0, '°C', '°F')).toBeCloseTo(32, 5)
      expect(convertUnit(100, '°C', '°F')).toBeCloseTo(212, 5)
      expect(convertUnit(37, '°C', 'K')).toBeCloseTo(310.15, 5)
      expect(convertUnit(212, '°F', '°C')).toBeCloseTo(100, 5)
    })

    it('converts pressure correctly', () => {
      // 100 kPa = 1 bar = 14.5038 psi
      expect(convertUnit(100, 'kPa', 'bar')).toBeCloseTo(1, 4)
      expect(convertUnit(100, 'kPa', 'psi')).toBeCloseTo(14.50377, 4)
      expect(convertUnit(1, 'MPa', 'bar')).toBeCloseTo(10, 4)
    })

    it('converts length correctly', () => {
      expect(convertUnit(1, 'm', 'ft')).toBeCloseTo(3.28084, 4)
      expect(convertUnit(1, 'km', 'm')).toBeCloseTo(1000, 4)
      expect(convertUnit(1, 'in', 'cm')).toBeCloseTo(2.54, 4)
    })

    it('converts flow correctly', () => {
      expect(convertUnit(1, 'm3/h', 'L/min')).toBeCloseTo(16.66667, 4)
      expect(convertUnit(60, 'L/min', 'L/s')).toBeCloseTo(1, 4)
    })

    it('returns same value if from and to are identical', () => {
      expect(convertUnit(42.5, '°C', '°C')).toBe(42.5)
    })

    it('throws on dimension mismatch', () => {
      expect(() => convertUnit(10, 'm', 'kg')).toThrow(/dimension mismatch/)
    })

    it('throws on invalid non-finite value', () => {
      expect(() => convertUnit(NaN, '°C', '°F')).toThrow(/finite number/)
    })
  })

  describe('convertUnitToSystem', () => {
    it('converts to imperial canonical unit', () => {
      const res = convertUnitToSystem(20, '°C', 'imperial')
      expect(res.unit).toBe('°F')
      expect(res.value).toBeCloseTo(68, 5)

      const press = convertUnitToSystem(200, 'kPa', 'imperial')
      expect(press.unit).toBe('psi')
      expect(press.value).toBeCloseTo(29.0075, 3)
    })

    it('converts to metric canonical unit', () => {
      const res = convertUnitToSystem(68, '°F', 'metric')
      expect(res.unit).toBe('°C')
      expect(res.value).toBeCloseTo(20, 5)
    })
  })

  describe('convertSeries', () => {
    it('converts whole series atomically', () => {
      const res = convertSeries([0, 20, 100], '°C', '°F')
      expect(res[0]).toBeCloseTo(32, 5)
      expect(res[1]).toBeCloseTo(68, 5)
      expect(res[2]).toBeCloseTo(212, 5)
    })

    it('throws if any value is invalid', () => {
      expect(() => convertSeries([10, NaN, 30], '°C', '°F')).toThrow()
    })
  })

  describe('getCompatibleUnits', () => {
    it('returns all units in same dimension', () => {
      const compatible = getCompatibleUnits('°C')
      expect(compatible).toContain('°F')
      expect(compatible).toContain('K')
      expect(compatible).not.toContain('kg')
    })
  })
})
