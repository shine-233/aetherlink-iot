import { describe, expect, it } from 'vitest'

import {
  SCADA_CANVAS_MAX_BYTES,
  createEmptyScadaCanvas,
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
    expect(() => parseScadaCanvas(JSON.stringify({ width: 10, height: 10, nodes: [{ id: '', kind: 'widget', ref: 'x', x: 0, y: 0, width: 1, height: 1 }] }))).toThrow(/non-empty id/)
    expect(() => parseScadaCanvas(JSON.stringify({ width: 10, height: 10, nodes: [{ id: 'a', kind: 'widget', ref: '', x: 0, y: 0, width: 1, height: 1 }] }))).toThrow(/non-empty ref/)
  })

  it('rejects unknown node kinds rather than defaulting to widget', () => {
    const raw = JSON.stringify({ width: 10, height: 10, nodes: [{ id: 'a', kind: 'hologram', ref: 'r', x: 0, y: 0, width: 1, height: 1 }] })
    expect(() => parseScadaCanvas(raw)).toThrow(/unknown kind/)
  })

  it('rejects non-finite geometry', () => {
    const raw = JSON.stringify({ width: 10, height: 10, nodes: [{ id: 'a', kind: 'widget', ref: 'r', x: Number.NaN, y: 0, width: 1, height: 1 }] })
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
    expect(nodes.some(node => node.id === id)).toBe(false)
  })
})
