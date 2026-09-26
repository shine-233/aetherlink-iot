import { describe, expect, it } from 'vitest'

import {
  SCADA_CANVAS_MAX_BYTES,
  SCADA_DISPLAY_MODE_FIXED_HEIGHT,
  SCADA_DISPLAY_MODE_FIXED_WIDTH,
  computeFitScale,
  createEmptyScadaCanvas,
  effectiveDisplayMode,
  nextScadaCanvasNodeId,
  parseScadaCanvas,
  scadaCanvasEqual,
  serializeScadaCanvas,
  type ScadaCanvas
} from '../canvasDocument'

function canvasWith(nodes: ScadaCanvas['nodes']): ScadaCanvas {
  return { schemaVersion: 1, width: 1280, height: 720, nodes }
}

const sampleNode = { id: 'n1', kind: 'widget' as const, ref: 'gauge', x: 10, y: 20, width: 100, height: 50 }

describe('scada canvas round trip', () => {
  it('serializes and parses back without loss', () => {
    const canvas = canvasWith([sampleNode])
    const reparsed = parseScadaCanvas(serializeScadaCanvas(canvas))
    expect(reparsed).toEqual(canvas)
  })

  it('is deterministic so an unchanged document does not bump the version', () => {
    const canvas = canvasWith([sampleNode, { ...sampleNode, id: 'n2', x: 5 }])
    // Key insertion order must not matter: the serializer fixes the order.
    const reordered = {
      height: canvas.height,
      nodes: canvas.nodes,
      schemaVersion: canvas.schemaVersion,
      width: canvas.width
    } as ScadaCanvas
    expect(serializeScadaCanvas(reordered)).toBe(serializeScadaCanvas(canvas))
    expect(scadaCanvasEqual(canvas, reordered)).toBe(true)
  })

  it('treats empty or blank json_data as an empty canvas', () => {
    expect(parseScadaCanvas('')).toEqual(createEmptyScadaCanvas())
    expect(parseScadaCanvas('   ')).toEqual(createEmptyScadaCanvas())
  })
})

describe('scada canvas validation', () => {
  it('rejects malformed json instead of silently repairing it', () => {
    expect(() => parseScadaCanvas('{"nodes":')).toThrow(/not valid JSON/)
  })

  it('rejects non-positive canvas size', () => {
    expect(() => parseScadaCanvas(JSON.stringify({ width: 0, height: 10, nodes: [] }))).toThrow()
    expect(() => parseScadaCanvas(JSON.stringify({ width: 10, height: -1, nodes: [] }))).toThrow()
  })

  it('rejects nodes without id or ref', () => {
    expect(() =>
      parseScadaCanvas(
        JSON.stringify({
          width: 10,
          height: 10,
          nodes: [{ id: '', kind: 'widget', ref: 'x', x: 0, y: 0, width: 1, height: 1 }]
        })
      )
    ).toThrow(/non-empty id/)
    expect(() =>
      parseScadaCanvas(
        JSON.stringify({
          width: 10,
          height: 10,
          nodes: [{ id: 'a', kind: 'widget', ref: '', x: 0, y: 0, width: 1, height: 1 }]
        })
      )
    ).toThrow(/non-empty ref/)
  })

  it('rejects unknown node kinds rather than defaulting to widget', () => {
    const raw = JSON.stringify({
      width: 10,
      height: 10,
      nodes: [{ id: 'a', kind: 'hologram', ref: 'r', x: 0, y: 0, width: 1, height: 1 }]
    })
    expect(() => parseScadaCanvas(raw)).toThrow(/unknown kind/)
  })

  it('rejects non-finite geometry', () => {
    const raw = JSON.stringify({
      width: 10,
      height: 10,
      nodes: [{ id: 'a', kind: 'widget', ref: 'r', x: Number.NaN, y: 0, width: 1, height: 1 }]
    })
    expect(() => parseScadaCanvas(raw)).toThrow(/finite number/)
  })

  it('rejects duplicate node ids', () => {
    const raw = JSON.stringify({ width: 10, height: 10, nodes: [sampleNode, sampleNode] })
    expect(() => parseScadaCanvas(raw)).toThrow(/duplicate canvas node id/)
  })

  it('rejects zero-sized nodes', () => {
    const raw = JSON.stringify({ width: 10, height: 10, nodes: [{ ...sampleNode, width: 0 }] })
    expect(() => parseScadaCanvas(raw)).toThrow(/positive width and height/)
  })
})

describe('scada canvas size guard', () => {
  it('refuses an oversized canvas instead of truncating it', () => {
    const huge = canvasWith([{ ...sampleNode, props: { blob: 'x'.repeat(SCADA_CANVAS_MAX_BYTES + 10) } }])
    expect(() => serializeScadaCanvas(huge)).toThrow(/exceeds/)
  })
})

describe('scada canvas id allocation', () => {
  it('never reuses an existing id', () => {
    const nodes = [sampleNode, { ...sampleNode, id: 'node-2' }]
    const id = nextScadaCanvasNodeId(nodes)
    expect(nodes.some((node) => node.id === id)).toBe(false)
  })
})

// TP-22: display mode document contract, mirrored by backend/internal/scadadoc.
describe('scada canvas display mode (TP-22)', () => {
  const fixedCanvas = { ...createEmptyScadaCanvas(1920, 1080), displayMode: 'fixed1080' as const }

  it('serializes an authored display mode and parses it back', () => {
    const raw = serializeScadaCanvas(fixedCanvas)
    expect(raw).toContain('"displayMode":"fixed1080"')
    const reparsed = parseScadaCanvas(raw)
    expect(reparsed.displayMode).toBe('fixed1080')
    expect(reparsed.width).toBe(SCADA_DISPLAY_MODE_FIXED_WIDTH)
    expect(reparsed.height).toBe(SCADA_DISPLAY_MODE_FIXED_HEIGHT)
    // Round trip must stay deterministic: re-saving an unchanged fixed canvas is a no-op.
    expect(serializeScadaCanvas(reparsed)).toBe(raw)
  })

  it('keeps legacy documents byte-identical by omitting an absent mode', () => {
    const raw = serializeScadaCanvas(canvasWith([sampleNode]))
    expect(raw).not.toContain('displayMode')
    expect(parseScadaCanvas(raw).displayMode).toBeUndefined()
  })

  it('accepts the snake_case display_mode alias and rejects a dual-key conflict', () => {
    const alias = parseScadaCanvas(
      JSON.stringify({ display_mode: 'fixed1080', width: 1920, height: 1080, nodes: [] })
    )
    expect(alias.displayMode).toBe('fixed1080')
    expect(() =>
      parseScadaCanvas(
        JSON.stringify({ displayMode: 'responsive', display_mode: 'fixed1080', width: 1920, height: 1080, nodes: [] })
      )
    ).toThrow(/conflicting/)
  })

  it('refuses a fixed1080 canvas that is not 1920x1080', () => {
    expect(() =>
      parseScadaCanvas(JSON.stringify({ displayMode: 'fixed1080', width: 1280, height: 720, nodes: [] }))
    ).toThrow(/1920/)
    expect(() =>
      parseScadaCanvas(JSON.stringify({ displayMode: 'fixed4k', width: 3840, height: 2160, nodes: [] }))
    ).toThrow(/displayMode/)
  })

  it('defaults an absent mode to responsive without failing', () => {
    expect(effectiveDisplayMode(canvasWith([]))).toBe('responsive')
    expect(effectiveDisplayMode(null)).toBe('responsive')
    expect(effectiveDisplayMode(undefined)).toBe('responsive')
    expect(effectiveDisplayMode(parseScadaCanvas(''))).toBe('responsive')
  })
})

// TP-22: uniform fit scale for the fixed1080 container (transform: scale 等比适配).
describe('computeFitScale', () => {
  it('scales down by the tighter axis so the design box stays fully visible', () => {
    // 960×540 viewport against 1920×1080 design: uniform half scale.
    expect(computeFitScale(960, 540, 1920, 1080)).toBeCloseTo(0.5)
    // Wider viewport: height is the binding axis, no stretching.
    expect(computeFitScale(3840, 1080, 1920, 1080)).toBeCloseTo(1)
    // Narrower viewport: width is the binding axis.
    expect(computeFitScale(1280, 1024, 1920, 1080)).toBeCloseTo(1280 / 1920)
  })

  it('scales up small containers to fill a large TV wall', () => {
    expect(computeFitScale(3840, 2160, 1920, 1080)).toBeCloseTo(2)
  })

  it('falls back to 1 for non-positive inputs instead of collapsing to zero', () => {
    expect(computeFitScale(0, 540, 1920, 1080)).toBe(1)
    expect(computeFitScale(960, 0, 1920, 1080)).toBe(1)
    expect(computeFitScale(960, 540, 0, 1080)).toBe(1)
    expect(computeFitScale(960, 540, 1920, -1)).toBe(1)
    expect(computeFitScale(Number.NaN, 540, 1920, 1080)).toBe(1)
  })
})
