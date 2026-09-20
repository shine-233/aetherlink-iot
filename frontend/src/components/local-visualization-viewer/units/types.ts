export type Dimension =
  | 'temperature'
  | 'length'
  | 'mass'
  | 'volume'
  | 'area'
  | 'speed'
  | 'pressure'
  | 'energy'
  | 'power'
  | 'flow'
  | 'time'
  | 'ratio'

export type UnitSystem = 'metric' | 'imperial'

export interface UnitDefinition {
  symbol: string
  dimension: Dimension
  scale: number
  offset?: number
  system: UnitSystem
}

export interface UnitConversionConfig {
  enabled: boolean
  targetUnit?: string
  unitSystem?: UnitSystem
}
