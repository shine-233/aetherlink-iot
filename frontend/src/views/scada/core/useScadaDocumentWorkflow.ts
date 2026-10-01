/**
 * SCADA project/document/version workflow (load, save, publish, rollback, archive, create).
 *
 * Extracted from the editor view so the persistence contract is testable without a DOM:
 *  - save always sends `expected_version` (optimistic concurrency); a version conflict is
 *    surfaced explicitly and never retried silently;
 *  - rollback reloads the editor from the response (it is a new draft on the server);
 *  - every `{ data, error }` response handles `error` so a failure never looks like "empty".
 */
import { computed, ref, watch } from 'vue'

import {
  archiveScadaDocument,
  createScadaDocument,
  createScadaProject,
  fetchScadaDocument,
  fetchScadaDocumentVersions,
  fetchScadaDocuments,
  fetchScadaProjects,
  publishScadaDocument,
  rollbackScadaDocument,
  saveScadaDocument,
  type ScadaDocument,
  type ScadaDocumentVersion,
  type ScadaProject
} from '@/service/api/scada'
import type { CanvasEditor } from './useCanvasEditor'

export interface ScadaWorkflowNotifier {
  error: (text: string) => void
  success: (text: string) => void
}

export function errorText(error: unknown): string {
  return String((error as { message?: string } | null)?.message ?? error ?? '')
}

export function isVersionConflict(error: unknown): boolean {
  return /version|conflict/i.test(errorText(error))
}

export function formatScadaDocumentLabel(doc: Pick<ScadaDocument, 'name' | 'current_version' | 'published_version'>) {
  const published = doc.published_version === null ? '' : ` / pub v${doc.published_version}`
  return `${doc.name} (v${doc.current_version}${published})`
}

export function useScadaDocumentWorkflow(editor: CanvasEditor, notify: ScadaWorkflowNotifier) {
  const projects = ref<ScadaProject[]>([])
  const documents = ref<ScadaDocument[]>([])
  const versions = ref<ScadaDocumentVersion[]>([])
  const projectId = ref<string | null>(null)
  const documentId = ref<string | null>(null)
  const currentDocument = ref<ScadaDocument | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const versionConflict = ref(false)
  const loadError = ref('')
  const tenantId = ref('')

  const tenant = () => tenantId.value || undefined

  const projectOptions = computed(() => projects.value.map((project) => ({ label: project.name, value: project.id })))
  const documentOptions = computed(() =>
    documents.value.map((doc) => ({ label: formatScadaDocumentLabel(doc), value: doc.id }))
  )
  const canSave = computed(() => !!currentDocument.value && editor.isDirty.value && !saving.value)
  // Publishing an unsaved draft would split "what is on screen" from "what is stored".
  const canPublish = computed(() => !!currentDocument.value && !editor.isDirty.value)

  function reportFailure(fallback: string, error: unknown) {
    notify.error(errorText(error) || fallback)
  }

  async function loadProjects() {
    loading.value = true
    loadError.value = ''
    try {
      const { data, error } = await fetchScadaProjects(tenant())
      if (error || !data) {
        loadError.value = '项目列表加载失败'
        return
      }
      projects.value = data
      if (!projectId.value && data.length > 0) projectId.value = data[0].id
    } finally {
      loading.value = false
    }
  }

  async function loadDocuments() {
    if (!projectId.value) {
      documents.value = []
      return
    }
    const { data, error } = await fetchScadaDocuments(projectId.value, tenant())
    if (error || !data) {
      reportFailure('画布列表加载失败', error)
      return
    }
    documents.value = data
    if (!data.some((doc) => doc.id === documentId.value)) documentId.value = data[0]?.id ?? null
  }

  async function loadVersions() {
    if (!documentId.value) {
      versions.value = []
      return
    }
    const { data, error } = await fetchScadaDocumentVersions(documentId.value, tenant())
    if (error || !data) {
      reportFailure('版本历史加载失败', error)
      return
    }
    versions.value = data
  }

  async function loadDocument() {
    if (!documentId.value) {
      currentDocument.value = null
      return
    }
    const { data, error } = await fetchScadaDocument(documentId.value, tenant())
    if (error || !data) {
      reportFailure('画布加载失败', error)
      return
    }
    currentDocument.value = data
    editor.load(data.json_data ?? '')
    versionConflict.value = false
    await loadVersions()
  }

  async function save() {
    const doc = currentDocument.value
    if (!doc) return
    saving.value = true
    try {
      const { data, error } = await saveScadaDocument(doc.id, {
        expected_version: doc.current_version,
        json_data: editor.serialize(),
        tenant_id: tenant()
      })
      if (error || !data) {
        if (isVersionConflict(error)) {
          versionConflict.value = true
          notify.error('版本冲突：画布已被他人修改，请刷新后重新编辑')
        } else {
          reportFailure('保存失败', error)
        }
        return
      }
      currentDocument.value = data
      editor.markSaved()
      versionConflict.value = false
      notify.success('已保存')
      await loadDocuments()
    } finally {
      saving.value = false
    }
  }

  async function publish() {
    const doc = currentDocument.value
    if (!doc) return
    const { data, error } = await publishScadaDocument(doc.id, tenant())
    if (error || !data) {
      reportFailure('发布失败', error)
      return
    }
    currentDocument.value = data
    editor.markSaved()
    notify.success('已发布')
    await loadDocuments()
    await loadVersions()
  }

  async function rollback(version: number) {
    const doc = currentDocument.value
    if (!doc) return
    const { data, error } = await rollbackScadaDocument(doc.id, version, tenant())
    if (error || !data) {
      reportFailure('回滚失败', error)
      return
    }
    currentDocument.value = data
    // Rollback creates a new draft; keeping the old in-memory canvas would overwrite it.
    editor.load(data.json_data ?? '')
    notify.success(`已回滚到 v${version}（生成为新草稿）`)
    await loadDocuments()
    await loadVersions()
  }

  async function archive() {
    const doc = currentDocument.value
    if (!doc) return
    const { error } = await archiveScadaDocument(doc.id, tenant())
    if (error) {
      reportFailure('归档失败', error)
      return
    }
    notify.success('已归档')
    await loadDocuments()
  }

  async function createProject(rawName: string): Promise<boolean> {
    const name = rawName.trim()
    if (!name) return false
    const { data, error } = await createScadaProject({ name, tenant_id: tenant() })
    if (error || !data) {
      reportFailure('新建项目失败', error)
      return false
    }
    await loadProjects()
    projectId.value = data.id
    return true
  }

  async function createDocument(rawName: string): Promise<boolean> {
    if (!projectId.value) return false
    const name = rawName.trim() || 'untitled-canvas'
    const { data, error } = await createScadaDocument(projectId.value, {
      name,
      json_data: editor.serialize(),
      tenant_id: tenant()
    })
    if (error || !data) {
      reportFailure('新建画布失败', error)
      return false
    }
    await loadDocuments()
    documentId.value = data.id
    return true
  }

  watch(projectId, () => {
    loadDocuments()
  })
  watch(documentId, () => {
    loadDocument()
  })

  return {
    projects,
    documents,
    versions,
    projectId,
    documentId,
    currentDocument,
    loading,
    saving,
    versionConflict,
    loadError,
    tenantId,
    projectOptions,
    documentOptions,
    canSave,
    canPublish,
    loadProjects,
    loadDocuments,
    loadDocument,
    loadVersions,
    save,
    publish,
    rollback,
    archive,
    createProject,
    createDocument
  }
}
