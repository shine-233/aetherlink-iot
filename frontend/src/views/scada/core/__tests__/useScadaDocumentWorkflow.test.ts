/**
 * Persistence contract of the SCADA workflow composable, tested without mounting the view.
 */
import { flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const api = vi.hoisted(() => ({
  fetchScadaProjects: vi.fn(),
  fetchScadaDocuments: vi.fn(),
  fetchScadaDocument: vi.fn(),
  fetchScadaDocumentVersions: vi.fn(),
  createScadaProject: vi.fn(),
  createScadaDocument: vi.fn(),
  saveScadaDocument: vi.fn(),
  publishScadaDocument: vi.fn(),
  rollbackScadaDocument: vi.fn(),
  archiveScadaDocument: vi.fn()
}))

vi.mock('@/service/api/scada', () => api)

const { useCanvasEditor } = await import('../useCanvasEditor')
const { errorText, formatScadaDocumentLabel, isVersionConflict, useScadaDocumentWorkflow } = await import(
  '../useScadaDocumentWorkflow'
)

const emptyCanvas = JSON.stringify({ schemaVersion: 1, width: 1280, height: 720, nodes: [] })
const ok = <T>(data: T) => ({ data, error: null })
const doc = (overrides: Record<string, unknown> = {}) => ({
  id: 'doc-1',
  tenant_id: 't',
  project_id: 'proj-1',
  name: 'canvas',
  status: 'DRAFT',
  current_version: 3,
  published_version: null,
  json_data: emptyCanvas,
  ...overrides
})

function setup() {
  const notify = { error: vi.fn(), success: vi.fn() }
  const editor = useCanvasEditor()
  const workflow = useScadaDocumentWorkflow(editor, notify)
  return { notify, editor, workflow }
}

beforeEach(() => {
  vi.clearAllMocks()
  api.fetchScadaProjects.mockResolvedValue(ok([{ id: 'proj-1', name: 'p1' }]))
  api.fetchScadaDocuments.mockResolvedValue(ok([doc()]))
  api.fetchScadaDocument.mockResolvedValue(ok(doc()))
  api.fetchScadaDocumentVersions.mockResolvedValue(ok([]))
  api.saveScadaDocument.mockResolvedValue(ok(doc({ current_version: 4 })))
})

describe('helpers', () => {
  it('formats labels and detects conflicts', () => {
    expect(formatScadaDocumentLabel({ name: 'a', current_version: 2, published_version: 1 })).toBe('a (v2 / pub v1)')
    expect(formatScadaDocumentLabel({ name: 'a', current_version: 2, published_version: null })).toBe('a (v2)')
    expect(isVersionConflict({ message: 'Version mismatch' })).toBe(true)
    expect(isVersionConflict({ message: 'boom' })).toBe(false)
    expect(errorText(null)).toBe('')
  })
})

describe('useScadaDocumentWorkflow', () => {
  it('cascades project -> documents -> document on load', async () => {
    const { workflow } = setup()
    await workflow.loadProjects()
    await flushPromises()
    expect(workflow.projectId.value).toBe('proj-1')
    expect(workflow.documentId.value).toBe('doc-1')
    expect(workflow.currentDocument.value?.id).toBe('doc-1')
  })

  it('saves with expected_version and flags conflicts without retry', async () => {
    const { workflow, notify } = setup()
    await workflow.loadProjects()
    await flushPromises()
    api.saveScadaDocument.mockResolvedValueOnce({ data: null, error: { message: 'version conflict' } })
    await workflow.save()
    expect(api.saveScadaDocument).toHaveBeenCalledTimes(1)
    expect(api.saveScadaDocument.mock.calls[0][1].expected_version).toBe(3)
    expect(workflow.versionConflict.value).toBe(true)
    expect(notify.error).toHaveBeenCalledWith(expect.stringContaining('版本冲突'))
    expect(workflow.saving.value).toBe(false)
  })

  it('keeps the create-project name on failure', async () => {
    const { workflow, notify } = setup()
    api.createScadaProject.mockResolvedValue({ data: null, error: { message: '' } })
    expect(await workflow.createProject('  ')).toBe(false)
    expect(api.createScadaProject).not.toHaveBeenCalled()
    expect(await workflow.createProject('x')).toBe(false)
    expect(notify.error).toHaveBeenCalledWith('新建项目失败')
  })

  it('surfaces a project load failure', async () => {
    api.fetchScadaProjects.mockResolvedValue({ data: null, error: { message: 'boom' } })
    const { workflow } = setup()
    await workflow.loadProjects()
    expect(workflow.loadError.value).toBe('项目列表加载失败')
    expect(workflow.loading.value).toBe(false)
  })
})
