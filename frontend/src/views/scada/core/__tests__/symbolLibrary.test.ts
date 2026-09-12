import { describe, expect, it } from 'vitest'

import {
  SCADA_SYMBOL_VIEWBOX,
  findScadaSymbol,
  listScadaSymbolCategories,
  listScadaSymbols,
  listScadaSymbolsByCategory
} from '../symbolLibrary'

describe('scada symbol library', () => {
  it('exposes a non-empty catalogue', () => {
    expect(listScadaSymbols().length).toBeGreaterThan(0)
  })

  it('keeps symbol keys unique and non-empty', () => {
    const keys = listScadaSymbols().map(symbol => symbol.key)
    expect(new Set(keys).size).toBe(keys.length)
    for (const key of keys) {
      expect(key.trim()).not.toBe('')
    }
  })

  // Keys are persisted inside canvas node `ref`. A rename orphans every saved document,
  // so uniqueness alone is not enough - lookups must be stable.
  it('resolves every listed symbol by key', () => {
    for (const symbol of listScadaSymbols()) {
      expect(findScadaSymbol(symbol.key)?.key).toBe(symbol.key)
    }
  })

  it('returns undefined for unknown keys instead of a placeholder', () => {
    expect(findScadaSymbol('does.not.exist')).toBeUndefined()
  })

  it('gives every symbol a positive default size', () => {
    for (const symbol of listScadaSymbols()) {
      expect(symbol.defaultWidth).toBeGreaterThan(0)
      expect(symbol.defaultHeight).toBeGreaterThan(0)
    }
  })

  it('groups the catalogue without losing symbols', () => {
    const categories = listScadaSymbolCategories()
    expect(categories.length).toBeGreaterThan(0)
    const total = categories.reduce(
      (sum, category) => sum + listScadaSymbolsByCategory(category).length,
      0
    )
    expect(total).toBe(listScadaSymbols().length)
  })

  it('uses a single fixed viewBox so nodes can scale without per-symbol math', () => {
    expect(SCADA_SYMBOL_VIEWBOX).toBe('0 0 100 100')
  })

  // Symbols are stored as SVG markup and injected into the DOM; unescaped script payloads
  // in the catalogue would be an injection vector, so keep the invariant explicit.
  it('contains no script or event-handler payloads', () => {
    for (const symbol of listScadaSymbols()) {
      expect(symbol.body.toLowerCase()).not.toContain('<script')
      expect(symbol.body.toLowerCase()).not.toContain('onerror')
      expect(symbol.body.toLowerCase()).not.toContain('onload')
    }
  })

  it('hands out copies so callers cannot mutate the catalogue', () => {
    const first = findScadaSymbol('valve.gate')
    if (!first) throw new Error('fixture symbol missing')
    first.label = 'mutated'
    expect(findScadaSymbol('valve.gate')?.label).not.toBe('mutated')
  })
})
