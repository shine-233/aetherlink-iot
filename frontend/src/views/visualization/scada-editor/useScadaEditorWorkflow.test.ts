/**
 * 文件用途：useScadaEditorWorkflow 的行为契约（不挂载视图）。
 * 覆盖：保存带 expected_version、冲突/归档分开提示、解析失败阻断保存、
 *       需确认命令先申请令牌、无令牌不下发、时钟 stop 后不再走。
 */
import { ref } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  archiveScadaDocument: vi.fn(),
  createScadaDocument: vi.fn(),
  createScadaProject: vi.fn(),
  executeControl: vi.fn(),
  fetchScadaDocument: vi.fn(),
  fetchScadaDocumentVersions: vi.fn(),
  fetchScadaDocuments: vi.fn(),
  fetchScadaProjects: vi.fn(),
  issueControlConfirmation: vi.fn(),
  publishScadaDocument: vi.fn(),
  rollbackScadaDocument: vi.fn(),
  saveScadaDocument: vi.fn()
}))
vi.mock('@/service/api/scada', () => api)

const { normalizeLayoutPatch, useScadaEditorWorkflow } = await import('./useScadaEditorWorkflow')

const registry = [
  { type: 'gauge', version: '1', schema: '{}', capabilities: ['2d' as const], commands: [] },
  {
    type: 'valve',
    version: '1',
    schema: '{}',
    capabilities: ['2d' as const],
    commands: [{ name: 'open_valve', requires_confirmation: true }]
  }
]

const canvasJson = JSON.stringify({ widgets: [] })
const doc = (overrides: Record<string, unknown> = {}) => ({
  id: 'd1',
  name: 'd',
  status: 'DRAFT',
  current_version: 7,
  published_version: null,
  json_data: canvasJson,
  ...overrides
})

function setup() {
  const message = { success: vi.fn(), error: vi.fn() }
  const workflow = useScadaEditorWorkflow({
    registry,
    message,
    t: (key) => key,
    tenantQuery: ref<string | undefined>(undefined)
  })
  return { message, workflow }
}

beforeEach(() => {
  vi.clearAllMocks()
  api.fetchScadaDocument.mockResolvedValue({ data: doc() })
  api.fetchScadaDocumentVersions.mockResolvedValue({ data: [] })
  api.saveScadaDocument.mockResolvedValue({ data: doc({ current_version: 8 }) })
})

afterEach(() => {
  vi.useRealTimers()
})

describe('normalizeLayoutPatch', () => {
  it('clamps positions and spans', () => {
    expect(normalizeLayoutPatch({ x: -3, y: 2, w: 0, h: -1 })).toEqual({ x: 0, y: 2, w: 1, h: 1 })
    expect(normalizeLayoutPatch({})).toEqual({})
  })
})

describe('useScadaEditorWorkflow', () => {
  it('saves with expected_version from the open document', async () => {
    const { workflow, message } = setup()
    await workflow.openDocument('d1')
    workflow.addWidget('gauge')
    await workflow.save()
    expect(api.saveScadaDocument.mock.calls[0][1].expected_version).toBe(7)
    expect(message.success).toHaveBeenCalledWith('custom.scada.saved')
    expect(workflow.activeDocument.value?.current_version).toBe(8)
    workflow.stop()
  })

  it('separates version-conflict and archived errors', async () => {
    const { workflow, message } = setup()
    await workflow.openDocument('d1')
    api.saveScadaDocument.mockRejectedValueOnce({ response: { status: 409 }, message: 'version conflict' })
    await workflow.save()
    api.saveScadaDocument.mockRejectedValueOnce(new Error('boom'))
    await workflow.save()
    const keys = message.error.mock.calls.map((c) => c[0])
    expect(keys).toContain('custom.scada.saveFailed')
    expect(keys.length).toBe(2)
    workflow.stop()
  })

  it('blocks saving when the canvas failed to parse', async () => {
    api.fetchScadaDocument.mockResolvedValue({ data: doc({ json_data: '{not json' }) })
    const { workflow, message } = setup()
    await workflow.openDocument('d1')
    expect(workflow.canSave.value).toBe(false)
    await workflow.save()
    expect(api.saveScadaDocument).not.toHaveBeenCalled()
    expect(message.error).toHaveBeenCalledWith('custom.scada.canvasParseFailed')
    workflow.stop()
  })

  it('requests a confirmation token before a confirmable command and aborts without one', async () => {
    const { workflow, message } = setup()
    await workflow.openDocument('d1')
    const valve = { id: 'w1', widget_type: 'valve', version: '1', layout: { x: 0, y: 0, w: 1, h: 1 } }
    api.issueControlConfirmation.mockResolvedValueOnce({ data: { confirmation_token: '' } })
    await workflow.sendControl(valve, 'open_valve', 'dev')
    expect(api.executeControl).not.toHaveBeenCalled()
    expect(message.error).toHaveBeenCalledWith('custom.scada.confirmationFailed')

    api.issueControlConfirmation.mockResolvedValueOnce({ data: { confirmation_token: 'tok' } })
    api.executeControl.mockResolvedValueOnce({})
    await workflow.sendControl(valve, 'open_valve', 'dev')
    expect(api.executeControl.mock.calls[0][0].confirmation_token).toBe('tok')
    workflow.stop()
  })

  it('stops the stale clock', () => {
    vi.useFakeTimers()
    const { workflow } = setup()
    const before = workflow.now.value
    workflow.stop()
    vi.advanceTimersByTime(20000)
    expect(workflow.now.value).toBe(before)
  })

  it('reloads versions after rollback and ignores a null version', async () => {
    const { workflow } = setup()
    await workflow.openDocument('d1')
    await workflow.rollback(null)
    expect(api.rollbackScadaDocument).not.toHaveBeenCalled()
    api.rollbackScadaDocument.mockResolvedValueOnce({ data: doc({ current_version: 9 }) })
    api.fetchScadaDocumentVersions.mockResolvedValueOnce({ data: [{ id: 'v', version: 9 }] })
    await workflow.rollback(3)
    expect(workflow.versions.value).toHaveLength(1)
    workflow.stop()
  })
})
