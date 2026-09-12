/**
 * Canvas editing state machine (ROADMAP P1.3).
 *
 * Kept out of the .vue file on purpose: move/add/delete and dirty tracking are the parts
 * that silently corrupt a document if they drift, and they are verifiable without a DOM.
 * The view only renders what this module decides.
 *
 * Rules:
 *  - Dirty is computed against the last *saved* serialization, not against a snapshot object.
 *    Comparing object identities would mark a document dirty after a no-op edit round trip.
 *  - Nodes are clamped inside the canvas bounds. A node parked at x=-9999 is unreachable in
 *    the UI and looks like a renderer bug, so it is refused at edit time instead.
 *  - Every mutation returns a new canvas: the backend bumps a version per save, and in-place
 *    mutation would make "undo" and dirty tracking impossible to reason about.
 */

import { computed, ref, type Ref } from 'vue'

import {
  createEmptyScadaCanvas,
  nextScadaCanvasNodeId,
  parseScadaCanvas,
  serializeScadaCanvas,
  type ScadaCanvas,
  type ScadaCanvasNode
} from './canvasDocument'

export interface UseCanvasEditorOptions {
  width?: number
  height?: number
}

export function useCanvasEditor(options: UseCanvasEditorOptions = {}) {
  const canvas: Ref<ScadaCanvas> = ref(createEmptyScadaCanvas(options.width ?? 1280, options.height ?? 720))
  const selectedId = ref<string | null>(null)
  /** Serialization of the last saved state; empty string means "never saved". */
  const savedSnapshot = ref('')

  const selectedNode = computed(() => canvas.value.nodes.find(node => node.id === selectedId.value) ?? null)

  const isDirty = computed(() => {
    if (savedSnapshot.value === '') return canvas.value.nodes.length > 0
    return serializeScadaCanvas(canvas.value) !== savedSnapshot.value
  })

  function clampNode(node: ScadaCanvasNode): ScadaCanvasNode {
    const maxWidth = canvas.value.width
    const maxHeight = canvas.value.height
    const width = Math.min(Math.max(1, node.width), maxWidth)
    const height = Math.min(Math.max(1, node.height), maxHeight)
    return {
      ...node,
      width,
      height,
      x: Math.min(Math.max(0, node.x), Math.max(0, maxWidth - width)),
      y: Math.min(Math.max(0, node.y), Math.max(0, maxHeight - height))
    }
  }

  function load(raw: string) {
    canvas.value = parseScadaCanvas(raw)
    selectedId.value = null
    savedSnapshot.value = serializeScadaCanvas(canvas.value)
  }

  function reset(width?: number, height?: number) {
    canvas.value = createEmptyScadaCanvas(width ?? canvas.value.width, height ?? canvas.value.height)
    selectedId.value = null
    savedSnapshot.value = serializeScadaCanvas(canvas.value)
  }

  function addNode(node: Omit<ScadaCanvasNode, 'id'> & { id?: string }): ScadaCanvasNode {
    const id = node.id && node.id.trim() !== '' ? node.id : nextScadaCanvasNodeId(canvas.value.nodes)
    if (canvas.value.nodes.some(existing => existing.id === id)) {
      throw new Error(`canvas already contains node ${id}`)
    }
    const created = clampNode({ ...node, id } as ScadaCanvasNode)
    canvas.value = { ...canvas.value, nodes: [...canvas.value.nodes, created] }
    selectedId.value = created.id
    return created
  }

  function updateNode(id: string, patch: Partial<Omit<ScadaCanvasNode, 'id'>>) {
    const nodes = canvas.value.nodes.map(node => (node.id === id ? clampNode({ ...node, ...patch }) : node))
    canvas.value = { ...canvas.value, nodes }
  }

  function moveNode(id: string, x: number, y: number) {
    updateNode(id, { x, y })
  }

  function removeNode(id: string) {
    canvas.value = { ...canvas.value, nodes: canvas.value.nodes.filter(node => node.id !== id) }
    if (selectedId.value === id) selectedId.value = null
  }

  function select(id: string | null) {
    selectedId.value = id
  }

  function bringToFront(id: string) {
    const maxZ = canvas.value.nodes.reduce((max, node) => Math.max(max, node.z ?? 0), 0)
    updateNode(id, { z: maxZ + 1 })
  }

  /** Marks the current state as saved; call only after the backend confirms the write. */
  function markSaved() {
    savedSnapshot.value = serializeScadaCanvas(canvas.value)
  }

  function serialize(): string {
    return serializeScadaCanvas(canvas.value)
  }

  return {
    canvas,
    selectedId,
    selectedNode,
    isDirty,
    load,
    reset,
    addNode,
    updateNode,
    moveNode,
    removeNode,
    select,
    bringToFront,
    markSaved,
    serialize
  }
}

export type CanvasEditor = ReturnType<typeof useCanvasEditor>
