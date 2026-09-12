/**
 * SCADA canvas document core (ROADMAP P1.3).
 *
 * The backend stores a canvas as an opaque `json_data` string and guards writes with
 * `expected_version` (optimistic concurrency). That means the frontend owns the document
 * shape: whatever this module serializes is what gets persisted, and whatever it refuses is
 * what can never reach the server.
 *
 * Design rules that must not be relaxed:
 *  1. Serialization is deterministic. Re-saving an unchanged document must produce the exact
 *     same string, otherwise every save bumps the version and "no changes" becomes
 *     unrepresentable.
 *  2. Oversized documents are rejected here, not truncated. A truncated canvas is
 *     permanently unparseable - that is worse than refusing to save.
 *  3. Unknown future fields survive a round trip instead of being silently dropped, so a
 *     newer editor's document is not destroyed by an older client.
 */

export const SCADA_CANVAS_SCHEMA_VERSION = 1

/** Hard ceiling for a serialized canvas. Matches the backend's refusal to store a doc it
 * cannot guarantee to parse back. */
export const SCADA_CANVAS_MAX_BYTES = 1024 * 1024

export type ScadaCanvasNodeKind = 'widget' | 'symbol' | 'text' | 'shape'

export interface ScadaCanvasNode {
  id: string
  kind: ScadaCanvasNodeKind
  /** Widget type for kind='widget'; symbol key for kind='symbol'. */
  ref: string
  x: number
  y: number
  width: number
  height: number
  rotation?: number
  /** Per-node props (data binding, label, thresholds...). Opaque to this module. */
  props?: Record<string, unknown>
  z?: number
}

export interface ScadaCanvas {
  schemaVersion: number
  width: number
  height: number
  background?: string
  nodes: ScadaCanvasNode[]
  /** Preserved but not interpreted: forward compatibility for newer editors. */
  extra?: Record<string, unknown>
}

export class ScadaCanvasError extends Error {}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value)
}

function validateNode(node: unknown, index: number): ScadaCanvasNode {
  if (!node || typeof node !== 'object') {
    throw new ScadaCanvasError(`canvas node at index ${index} must be an object`)
  }
  const candidate = node as Record<string, unknown>
  if (typeof candidate.id !== 'string' || candidate.id.trim() === '') {
    throw new ScadaCanvasError(`canvas node at index ${index} must have a non-empty id`)
  }
  const kind = candidate.kind
  if (kind !== 'widget' && kind !== 'symbol' && kind !== 'text' && kind !== 'shape') {
    throw new ScadaCanvasError(`canvas node at index ${index} has unknown kind: ${String(kind)}`)
  }
  if (typeof candidate.ref !== 'string' || candidate.ref.trim() === '') {
    throw new ScadaCanvasError(`canvas node at index ${index} must have a non-empty ref`)
  }
  for (const key of ['x', 'y', 'width', 'height'] as const) {
    if (!isFiniteNumber(candidate[key])) {
      throw new ScadaCanvasError(`canvas node ${candidate.id} field ${key} must be a finite number`)
    }
  }
  if ((candidate.width as number) <= 0 || (candidate.height as number) <= 0) {
    throw new ScadaCanvasError(`canvas node ${candidate.id} must have positive width and height`)
  }
  if (candidate.rotation !== undefined && !isFiniteNumber(candidate.rotation)) {
    throw new ScadaCanvasError(`canvas node ${candidate.id} rotation must be a finite number`)
  }
  if (candidate.z !== undefined && !isFiniteNumber(candidate.z)) {
    throw new ScadaCanvasError(`canvas node ${candidate.id} z must be a finite number`)
  }
  return candidate as unknown as ScadaCanvasNode
}

/**
 * Parse a canvas from its persisted string form.
 * Throws rather than returning a partially-repaired document: a silently repaired canvas
 * looks fine on screen and then saves a document the author never drew.
 */
export function parseScadaCanvas(raw: string): ScadaCanvas {
  if (typeof raw !== 'string' || raw.trim() === '') {
    return createEmptyScadaCanvas()
  }
  let decoded: unknown
  try {
    decoded = JSON.parse(raw)
  } catch {
    throw new ScadaCanvasError('canvas json_data is not valid JSON')
  }
  if (!decoded || typeof decoded !== 'object') {
    throw new ScadaCanvasError('canvas json_data must be an object')
  }
  const candidate = decoded as Record<string, unknown>
  if (!isFiniteNumber(candidate.width) || !isFiniteNumber(candidate.height)) {
    throw new ScadaCanvasError('canvas must have numeric width and height')
  }
  if ((candidate.width as number) <= 0 || (candidate.height as number) <= 0) {
    throw new ScadaCanvasError('canvas must have positive width and height')
  }
  if (!Array.isArray(candidate.nodes)) {
    throw new ScadaCanvasError('canvas nodes must be an array')
  }
  const nodes = candidate.nodes.map((node, index) => validateNode(node, index))
  const seen = new Set<string>()
  for (const node of nodes) {
    if (seen.has(node.id)) {
      throw new ScadaCanvasError(`duplicate canvas node id: ${node.id}`)
    }
    seen.add(node.id)
  }
  return {
    schemaVersion: isFiniteNumber(candidate.schemaVersion) ? (candidate.schemaVersion as number) : SCADA_CANVAS_SCHEMA_VERSION,
    width: candidate.width as number,
    height: candidate.height as number,
    background: typeof candidate.background === 'string' ? candidate.background : undefined,
    nodes,
    extra:
      candidate.extra && typeof candidate.extra === 'object'
        ? (candidate.extra as Record<string, unknown>)
        : undefined
  }
}

export function createEmptyScadaCanvas(width = 1280, height = 720): ScadaCanvas {
  return { schemaVersion: SCADA_CANVAS_SCHEMA_VERSION, width, height, nodes: [] }
}

/** Key order is fixed so the same document always serializes to the same string. */
function canonicalizeNode(node: ScadaCanvasNode): Record<string, unknown> {
  const ordered: Record<string, unknown> = {
    id: node.id,
    kind: node.kind,
    ref: node.ref,
    x: node.x,
    y: node.y,
    width: node.width,
    height: node.height
  }
  if (node.rotation !== undefined) ordered.rotation = node.rotation
  if (node.z !== undefined) ordered.z = node.z
  if (node.props !== undefined) ordered.props = node.props
  return ordered
}

export function serializeScadaCanvas(canvas: ScadaCanvas): string {
  if (!canvas) {
    throw new ScadaCanvasError('canvas is required')
  }
  const ordered: Record<string, unknown> = {
    schemaVersion: canvas.schemaVersion ?? SCADA_CANVAS_SCHEMA_VERSION,
    width: canvas.width,
    height: canvas.height
  }
  if (canvas.background !== undefined) ordered.background = canvas.background
  ordered.nodes = (canvas.nodes ?? []).map(canonicalizeNode)
  if (canvas.extra !== undefined) ordered.extra = canvas.extra
  const encoded = JSON.stringify(ordered)
  const bytes = new TextEncoder().encode(encoded).length
  if (bytes > SCADA_CANVAS_MAX_BYTES) {
    // Refuse instead of truncating: a cut-off canvas can never be parsed back.
    throw new ScadaCanvasError(
      `canvas exceeds ${SCADA_CANVAS_MAX_BYTES} bytes (got ${bytes}); remove nodes or large props before saving`
    )
  }
  return encoded
}

/** Compare two canvases by their serialized form - the same test the version bump uses. */
export function scadaCanvasEqual(a: ScadaCanvas, b: ScadaCanvas): boolean {
  return serializeScadaCanvas(a) === serializeScadaCanvas(b)
}

export function nextScadaCanvasNodeId(existing: ScadaCanvasNode[], prefix = 'node'): string {
  const used = new Set(existing.map(node => node.id))
  let index = existing.length + 1
  let candidate = `${prefix}-${index}`
  while (used.has(candidate)) {
    index += 1
    candidate = `${prefix}-${index}`
  }
  return candidate
}
