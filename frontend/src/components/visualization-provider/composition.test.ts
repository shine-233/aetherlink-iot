import { describe, expect, it } from 'vitest'
import {
  getDefaultVisualizationRendererRegistry,
  registerDefaultVisualizationRenderers,
  registerLocalVisualizationRenderer
} from './composition'

describe('default visualization renderer composition', () => {
  it('registers native and legacy renderers once in a stable registry', () => {
    const first = registerDefaultVisualizationRenderers()
    const nativeRenderer = first.get('native-board')
    const legacyRenderer = first.get('legacy-thingsvis')

    expect(first).toBe(getDefaultVisualizationRendererRegistry())
    expect(nativeRenderer).toBeDefined()
    expect(legacyRenderer).toBeDefined()
    expect(registerDefaultVisualizationRenderers().get('native-board')).toBe(nativeRenderer)
    expect(registerDefaultVisualizationRenderers().get('legacy-thingsvis')).toBe(legacyRenderer)
  })

  it('registers the real local renderer under an explicit provider id without replacing it', () => {
    const nativeRenderer = getDefaultVisualizationRendererRegistry().get('native-board')

    expect(registerLocalVisualizationRenderer('test-local')).toBe(true)
    expect(registerLocalVisualizationRenderer('test-local')).toBe(false)
    expect(getDefaultVisualizationRendererRegistry().get('test-local')).toBe(nativeRenderer)
  })
})
