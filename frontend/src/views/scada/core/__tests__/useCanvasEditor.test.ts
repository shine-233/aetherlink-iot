import { describe, expect, it } from 'vitest'

import { serializeScadaCanvas } from '../canvasDocument'
import { useCanvasEditor } from '../useCanvasEditor'

function editorWithCanvas(width = 1000, height = 800) {
  const editor = useCanvasEditor({ width, height })
  editor.markSaved()
  return editor
}

describe('canvas editor: add and remove', () => {
  it('adds a node and selects it', () => {
    const editor = editorWithCanvas()
    const created = editor.addNode({ kind: 'symbol', ref: 'valve.gate', x: 10, y: 20, width: 80, height: 60 })
    expect(editor.canvas.value.nodes).toHaveLength(1)
    expect(editor.selectedId.value).toBe(created.id)
  })

  it('generates a non-colliding id', () => {
    const editor = editorWithCanvas()
    const first = editor.addNode({ kind: 'widget', ref: 'gauge', x: 0, y: 0, width: 50, height: 50 })
    const second = editor.addNode({ kind: 'widget', ref: 'gauge', x: 0, y: 0, width: 50, height: 50 })
    expect(first.id).not.toBe(second.id)
  })

  it('refuses a duplicate explicit id', () => {
    const editor = editorWithCanvas()
    editor.addNode({ id: 'dup', kind: 'widget', ref: 'gauge', x: 0, y: 0, width: 50, height: 50 })
    expect(() =>
      editor.addNode({ id: 'dup', kind: 'widget', ref: 'gauge', x: 0, y: 0, width: 50, height: 50 })
    ).toThrow(/already contains/)
  })

  it('clears the selection when the selected node is removed', () => {
    const editor = editorWithCanvas()
    const node = editor.addNode({ kind: 'widget', ref: 'gauge', x: 0, y: 0, width: 50, height: 50 })
    editor.removeNode(node.id)
    expect(editor.selectedNode.value).toBeNull()
    expect(editor.canvas.value.nodes).toHaveLength(0)
  })
})

describe('canvas editor: bounds clamping', () => {
  it('clamps negative coordinates into the canvas', () => {
    const editor = editorWithCanvas(500, 400)
    const node = editor.addNode({ kind: 'widget', ref: 'gauge', x: -50, y: -30, width: 60, height: 40 })
    expect(node.x).toBe(0)
    expect(node.y).toBe(0)
  })

  // A node parked outside the canvas is unreachable and reads as a renderer bug.
  it('clamps a node that would sit past the right or bottom edge', () => {
    const editor = editorWithCanvas(200, 200)
    const node = editor.addNode({ kind: 'widget', ref: 'gauge', x: 190, y: 190, width: 50, height: 50 })
    expect(node.x).toBe(150)
    expect(node.y).toBe(150)
  })

  it('never allows zero or negative size', () => {
    const editor = editorWithCanvas()
    const node = editor.addNode({ kind: 'widget', ref: 'gauge', x: 0, y: 0, width: 0, height: -5 })
    expect(node.width).toBeGreaterThan(0)
    expect(node.height).toBeGreaterThan(0)
  })

  it('clamps on move as well as on add', () => {
    const editor = editorWithCanvas(300, 300)
    const node = editor.addNode({ kind: 'widget', ref: 'gauge', x: 0, y: 0, width: 50, height: 50 })
    editor.moveNode(node.id, 999, 999)
    const moved = editor.canvas.value.nodes[0]
    expect(moved.x).toBe(250)
    expect(moved.y).toBe(250)
  })
})

describe('canvas editor: dirty tracking', () => {
  it('is clean right after load', () => {
    const editor = useCanvasEditor()
    editor.load(
      serializeScadaCanvas({
        schemaVersion: 1,
        width: 800,
        height: 600,
        nodes: [{ id: 'a', kind: 'widget', ref: 'gauge', x: 0, y: 0, width: 10, height: 10 }]
      })
    )
    expect(editor.isDirty.value).toBe(false)
  })

  it('becomes dirty after an edit and clean after markSaved', () => {
    const editor = editorWithCanvas()
    expect(editor.isDirty.value).toBe(false)
    editor.addNode({ kind: 'widget', ref: 'gauge', x: 0, y: 0, width: 10, height: 10 })
    expect(editor.isDirty.value).toBe(true)
    editor.markSaved()
    expect(editor.isDirty.value).toBe(false)
  })

  // Comparing object identity instead of content would flag a no-op round trip as dirty,
  // producing a version bump for a save that changed nothing.
  it('stays clean when an edit is reverted', () => {
    const editor = editorWithCanvas()
    const node = editor.addNode({ kind: 'widget', ref: 'gauge', x: 40, y: 40, width: 60, height: 30 })
    editor.markSaved()
    editor.moveNode(node.id, 10, 10)
    expect(editor.isDirty.value).toBe(true)
    editor.moveNode(node.id, 40, 40)
    expect(editor.isDirty.value).toBe(false)
  })
})

describe('canvas editor: z ordering', () => {
  it('brings a node above every other node', () => {
    const editor = editorWithCanvas()
    const first = editor.addNode({ kind: 'widget', ref: 'gauge', x: 0, y: 0, width: 10, height: 10, z: 5 })
    const second = editor.addNode({ kind: 'widget', ref: 'gauge', x: 0, y: 0, width: 10, height: 10 })
    editor.bringToFront(second.id)
    const reloaded = editor.canvas.value.nodes
    expect(reloaded.find(node => node.id === second.id)?.z).toBeGreaterThan(
      reloaded.find(node => node.id === first.id)?.z ?? 0
    )
  })
})
