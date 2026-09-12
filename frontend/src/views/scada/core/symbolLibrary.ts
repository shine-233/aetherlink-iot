/**
 * Industrial symbol library for the SCADA canvas (ROADMAP P1.3).
 *
 * Symbols are plain SVG bodies drawn in a fixed 0..100 viewBox, so a canvas node only needs
 * to store a symbol key plus its box; rendering scales by width/height without per-symbol
 * math. Keeping them as data (not components) means the palette, the canvas renderer and the
 * export path all read the same source.
 *
 * Rules:
 *  - `key` is stable and is what gets persisted in canvas nodes (ref). Renaming a key
 *    orphans saved documents, so keys are append-only.
 *  - Every symbol declares a category so a palette can group them without hardcoding lists.
 *  - `stroke`-based bodies only: fill is left to the renderer so theme/state colors
 *    (running / fault / offline) can be applied without a second asset set.
 */

export type ScadaSymbolCategory = 'valve' | 'pump' | 'vessel' | 'motor' | 'sensor' | 'pipe' | 'electrical'

export interface ScadaSymbol {
  key: string
  label: string
  category: ScadaSymbolCategory
  /** SVG markup drawn inside a 0 0 100 100 viewBox. Rendered with stroke=currentColor. */
  body: string
  /** Default canvas size in px when dropped from the palette. */
  defaultWidth: number
  defaultHeight: number
  /** Bindable points, e.g. which telemetry key this symbol usually shows. */
  bindingHints?: string[]
}

export const SCADA_SYMBOL_VIEWBOX = '0 0 100 100'

const SYMBOLS: ScadaSymbol[] = [
  {
    key: 'valve.gate',
    label: '闸阀',
    category: 'valve',
    body: '<path d="M10 50 L40 50 L50 30 L60 50 L90 50" /><path d="M50 30 L50 70" /><path d="M38 70 L62 70 L50 50 Z" />',
    defaultWidth: 80,
    defaultHeight: 60,
    bindingHints: ['valve_open', 'position']
  },
  {
    key: 'valve.ball',
    label: '球阀',
    category: 'valve',
    body: '<path d="M10 50 L40 50 L60 50 L90 50" /><circle cx="50" cy="50" r="14" /><path d="M38 42 L62 58" />',
    defaultWidth: 80,
    defaultHeight: 60,
    bindingHints: ['valve_open']
  },
  {
    key: 'valve.control',
    label: '调节阀',
    category: 'valve',
    body: '<path d="M10 50 L38 50 L62 50 L90 50" /><path d="M38 34 L62 34 L50 50 Z" /><path d="M38 66 L62 66 L50 50 Z" /><path d="M50 20 L50 34" />',
    defaultWidth: 80,
    defaultHeight: 60,
    bindingHints: ['valve_open', 'setpoint']
  },
  {
    key: 'pump.centrifugal',
    label: '离心泵',
    category: 'pump',
    body: '<circle cx="50" cy="50" r="28" /><path d="M22 36 L78 64" /><path d="M78 36 L22 64" /><path d="M50 22 L50 10" />',
    defaultWidth: 80,
    defaultHeight: 80,
    bindingHints: ['running', 'speed', 'current']
  },
  {
    key: 'pump.positive',
    label: '容积泵',
    category: 'pump',
    body: '<circle cx="50" cy="50" r="28" /><path d="M32 32 L68 68" /><path d="M50 22 L50 10" />',
    defaultWidth: 80,
    defaultHeight: 80,
    bindingHints: ['running', 'pressure']
  },
  {
    key: 'vessel.tank',
    label: '储罐',
    category: 'vessel',
    body: '<rect x="22" y="18" width="56" height="64" rx="8" /><path d="M22 46 L78 46" />',
    defaultWidth: 90,
    defaultHeight: 100,
    bindingHints: ['level', 'volume']
  },
  {
    key: 'vessel.reactor',
    label: '反应釜',
    category: 'vessel',
    body: '<path d="M24 20 L76 20 L70 78 Q50 92 30 78 Z" /><path d="M50 8 L50 20" />',
    defaultWidth: 90,
    defaultHeight: 100,
    bindingHints: ['temperature', 'pressure']
  },
  {
    key: 'motor.electric',
    label: '电动机',
    category: 'motor',
    body: '<circle cx="50" cy="50" r="28" /><text x="50" y="56" text-anchor="middle" font-size="22" fill="currentColor" stroke="none">M</text>',
    defaultWidth: 80,
    defaultHeight: 80,
    bindingHints: ['running', 'current', 'temperature']
  },
  {
    key: 'sensor.temperature',
    label: '温度变送器',
    category: 'sensor',
    body: '<circle cx="50" cy="50" r="22" /><path d="M50 72 L50 90" /><text x="50" y="56" text-anchor="middle" font-size="20" fill="currentColor" stroke="none">T</text>',
    defaultWidth: 60,
    defaultHeight: 80,
    bindingHints: ['temperature']
  },
  {
    key: 'sensor.pressure',
    label: '压力变送器',
    category: 'sensor',
    body: '<circle cx="50" cy="44" r="22" /><path d="M50 66 L50 90" /><text x="50" y="50" text-anchor="middle" font-size="20" fill="currentColor" stroke="none">P</text>',
    defaultWidth: 60,
    defaultHeight: 80,
    bindingHints: ['pressure']
  },
  {
    key: 'sensor.flow',
    label: '流量变送器',
    category: 'sensor',
    body: '<circle cx="50" cy="44" r="22" /><path d="M50 66 L50 90" /><text x="50" y="50" text-anchor="middle" font-size="20" fill="currentColor" stroke="none">F</text>',
    defaultWidth: 60,
    defaultHeight: 80,
    bindingHints: ['flow']
  },
  {
    key: 'pipe.straight',
    label: '直管',
    category: 'pipe',
    body: '<path d="M8 50 L92 50" /><path d="M8 44 L8 56" /><path d="M92 44 L92 56" />',
    defaultWidth: 100,
    defaultHeight: 24,
    bindingHints: []
  },
  {
    key: 'pipe.elbow',
    label: '弯头',
    category: 'pipe',
    body: '<path d="M8 50 L50 50 Q78 50 78 22 L78 8" />',
    defaultWidth: 80,
    defaultHeight: 80,
    bindingHints: []
  },
  {
    key: 'electrical.breaker',
    label: '断路器',
    category: 'electrical',
    body: '<rect x="30" y="26" width="40" height="48" /><path d="M50 74 L50 58" /><path d="M38 58 L62 44" />',
    defaultWidth: 60,
    defaultHeight: 80,
    bindingHints: ['closed', 'current']
  },
  {
    key: 'electrical.transformer',
    label: '变压器',
    category: 'electrical',
    body: '<circle cx="38" cy="50" r="20" /><circle cx="62" cy="50" r="20" />',
    defaultWidth: 90,
    defaultHeight: 70,
    bindingHints: ['voltage', 'temperature']
  }
]

const BY_KEY = new Map(SYMBOLS.map(symbol => [symbol.key, symbol]))

export function listScadaSymbols(): ScadaSymbol[] {
  return SYMBOLS.map(symbol => ({ ...symbol }))
}

export function findScadaSymbol(key: string): ScadaSymbol | undefined {
  const found = BY_KEY.get(key)
  return found ? { ...found } : undefined
}

export function listScadaSymbolCategories(): ScadaSymbolCategory[] {
  const seen = new Set<ScadaSymbolCategory>()
  for (const symbol of SYMBOLS) seen.add(symbol.category)
  return [...seen]
}

export function listScadaSymbolsByCategory(category: ScadaSymbolCategory): ScadaSymbol[] {
  return SYMBOLS.filter(symbol => symbol.category === category).map(symbol => ({ ...symbol }))
}
