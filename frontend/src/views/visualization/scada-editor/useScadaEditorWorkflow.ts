/**
 * 文件用途: SCADA 画布编辑器（/visualization/scada-editor）的文档工作流与控制命令流程。
 * 核心逻辑: 项目/文档加载 → 打开 → 本地 widget 编辑 → 带版本号保存 → 发布/回滚/归档；
 *           控制命令按注册表声明决定是否先申请确认令牌。
 * 关键注意事项:
 *  1. 保存携带 current_version 作为 expected_version；冲突与归档错误分开提示。
 *  2. 画布解析失败阻断保存——解析失败继续保存等于把真实画布覆盖成空。
 *  3. 陈旧判定时钟在 owner 卸载时清理（stop 也可手动调用）。
 */
import { computed, getCurrentInstance, onBeforeUnmount, ref, watch, type Ref } from 'vue'
import {
  archiveScadaDocument,
  createScadaDocument,
  createScadaProject,
  executeControl,
  fetchScadaDocument,
  fetchScadaDocumentVersions,
  fetchScadaDocuments,
  fetchScadaProjects,
  issueControlConfirmation,
  publishScadaDocument,
  rollbackScadaDocument,
  saveScadaDocument,
  type ScadaDocument,
  type ScadaDocumentVersion,
  type ScadaProject
} from '@/service/api/scada'
import {
  canEditDocument,
  commandRequiresConfirmation,
  isArchivedError,
  isTelemetryStale,
  isVersionConflictError,
  parseCanvas,
  resolveWidgets,
  serializeCanvas,
  type ScadaWidgetDefinition,
  type ScadaWidgetInstance,
  type TelemetryLinkState
} from './scada-model'

export interface ScadaEditorMessenger {
  success: (text: string) => void
  error: (text: string) => void
}

export interface UseScadaEditorWorkflowOptions {
  registry: ScadaWidgetDefinition[]
  message: ScadaEditorMessenger
  t: (key: string) => string
  /** Tenant override (admins only); undefined means "caller's tenant". */
  tenantQuery: Ref<string | undefined>
  /** Stale-clock period; exposed for tests. */
  clockIntervalMs?: number
}

/** Clamp a layout patch so grid positions never go negative and spans stay >= 1. */
export function normalizeLayoutPatch(patch: Partial<ScadaWidgetInstance['layout']>) {
  const out: Partial<ScadaWidgetInstance['layout']> = {}
  if (patch.x !== undefined) out.x = Math.max(0, patch.x)
  if (patch.y !== undefined) out.y = Math.max(0, patch.y)
  if (patch.w !== undefined) out.w = Math.max(1, patch.w)
  if (patch.h !== undefined) out.h = Math.max(1, patch.h)
  return out
}

export function useScadaEditorWorkflow(options: UseScadaEditorWorkflowOptions) {
  const { registry, message, t, tenantQuery } = options

  const projects = ref<ScadaProject[]>([])
  const documents = ref<ScadaDocument[]>([])
  const versions = ref<ScadaDocumentVersion[]>([])
  const activeProjectId = ref('')
  const activeDocument = ref<ScadaDocument | null>(null)
  const widgets = ref<ScadaWidgetInstance[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const parseError = ref('')
  const webglAvailable = ref(false)
  // 遥测链路状态：默认 idle 即"陈旧"，避免链路未建立时把任何读数伪装成实时值。
  const linkState = ref<TelemetryLinkState>('idle')
  const lastMessageAt = ref<number | null>(null)
  const now = ref(Date.now())

  // 陈旧判定时钟：卸载时必须清理，否则每次进出路由都会泄漏一个定时器。
  let clock: ReturnType<typeof setInterval> | null = setInterval(() => {
    now.value = Date.now()
  }, options.clockIntervalMs ?? 5000)
  function stop() {
    if (clock !== null) clearInterval(clock)
    clock = null
  }
  if (getCurrentInstance()) onBeforeUnmount(stop)

  const editable = computed(() => canEditDocument(activeDocument.value?.status))
  const stale = computed(() => isTelemetryStale(linkState.value, lastMessageAt.value, now.value))
  const resolution = computed(() => resolveWidgets(widgets.value, registry, webglAvailable.value))
  const canSave = computed(() => editable.value && parseError.value === '')
  const degradedTypes = computed(() => resolution.value.degraded.map((d) => d.type).join(', '))

  async function loadProjects() {
    loading.value = true
    try {
      const { data } = await fetchScadaProjects(tenantQuery.value)
      projects.value = data ?? []
    } catch {
      projects.value = []
    } finally {
      loading.value = false
    }
  }

  async function loadDocuments(projectId: string) {
    loading.value = true
    try {
      const { data } = await fetchScadaDocuments(projectId, tenantQuery.value)
      documents.value = data ?? []
    } catch {
      documents.value = []
    } finally {
      loading.value = false
    }
  }

  async function reloadVersions(documentId: string) {
    const v = await fetchScadaDocumentVersions(documentId, tenantQuery.value)
    versions.value = v.data ?? []
  }

  function applyDocument(doc: ScadaDocument | null | undefined) {
    activeDocument.value = doc ?? null
    const parsed = parseCanvas(doc?.json_data)
    if (!parsed.ok) {
      // 解析失败：清空编辑区并阻断保存，绝不用空画布顶替。
      widgets.value = []
      parseError.value = parsed.reason
      message.error(t('custom.scada.canvasParseFailed'))
      return
    }
    parseError.value = ''
    widgets.value = parsed.canvas.widgets
  }

  async function openDocument(id: string) {
    loading.value = true
    try {
      const { data } = await fetchScadaDocument(id, tenantQuery.value)
      applyDocument(data)
      await reloadVersions(id)
    } finally {
      loading.value = false
    }
  }

  async function createProject(rawName: string): Promise<boolean> {
    const name = rawName.trim()
    if (!name) return false
    try {
      const { data } = await createScadaProject({ name, tenant_id: tenantQuery.value })
      await loadProjects()
      if (data?.id) activeProjectId.value = data.id
      message.success(t('custom.scada.projectCreated'))
      return true
    } catch {
      message.error(t('custom.scada.createProjectFailed'))
      return false
    }
  }

  async function createDocument(rawName: string): Promise<boolean> {
    const name = rawName.trim()
    if (!activeProjectId.value || !name) return false
    try {
      const { data } = await createScadaDocument(activeProjectId.value, {
        name,
        json_data: '{}',
        tenant_id: tenantQuery.value
      })
      await loadDocuments(activeProjectId.value)
      if (data?.id) await openDocument(data.id)
      return true
    } catch {
      message.error(t('custom.scada.createDocumentFailed'))
      return false
    }
  }

  function addWidget(widgetType: string) {
    widgets.value = [
      ...widgets.value,
      {
        id: `w-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
        widget_type: widgetType,
        version: '1',
        layout: { x: 0, y: widgets.value.length * 2, w: 2, h: 2 }
      }
    ]
  }

  function removeWidget(id: string) {
    widgets.value = widgets.value.filter((w) => w.id !== id)
  }

  function updateLayout(id: string, patch: Partial<ScadaWidgetInstance['layout']>) {
    const safe = normalizeLayoutPatch(patch)
    widgets.value = widgets.value.map((w) => (w.id === id ? { ...w, layout: { ...w.layout, ...safe } } : w))
  }

  async function save() {
    const doc = activeDocument.value
    if (!doc) return
    if (parseError.value) {
      message.error(t('custom.scada.canvasParseFailed'))
      return
    }
    saving.value = true
    try {
      const { data } = await saveScadaDocument(doc.id, {
        expected_version: doc.current_version,
        json_data: serializeCanvas({ widgets: widgets.value }),
        tenant_id: tenantQuery.value
      })
      applyDocument(data)
      message.success(t('custom.scada.saved'))
    } catch (error) {
      // 冲突与归档必须分开提示：前者重载即可，后者重试无意义。
      if (isVersionConflictError(error)) message.error(t('custom.scada.versionConflict'))
      else if (isArchivedError(error)) message.error(t('custom.scada.archivedCannotEdit'))
      else message.error(t('custom.scada.saveFailed'))
    } finally {
      saving.value = false
    }
  }

  async function runDocumentAction(
    action: (id: string) => Promise<{ data?: ScadaDocument | null }>,
    successKey: string,
    failureKey: string,
    afterApply?: (id: string) => Promise<void>
  ) {
    const doc = activeDocument.value
    if (!doc) return
    try {
      const { data } = await action(doc.id)
      applyDocument(data)
      if (afterApply) await afterApply(doc.id)
      message.success(t(successKey))
    } catch {
      message.error(t(failureKey))
    }
  }

  const publish = () =>
    runDocumentAction(
      (id) => publishScadaDocument(id, tenantQuery.value),
      'custom.scada.published',
      'custom.scada.publishFailed'
    )

  const archive = () =>
    runDocumentAction(
      (id) => archiveScadaDocument(id, tenantQuery.value),
      'custom.scada.archived',
      'custom.scada.archiveFailed'
    )

  async function rollback(version: number | null) {
    if (version == null) return
    // 回滚产生新的草稿版本，需要重新加载版本列表才能看到可选版本的变化。
    await runDocumentAction(
      (id) => rollbackScadaDocument(id, version, tenantQuery.value),
      'custom.scada.rolledBack',
      'custom.scada.rollbackFailed',
      reloadVersions
    )
  }

  async function sendControl(widget: ScadaWidgetInstance, command: string, deviceId: string) {
    const doc = activeDocument.value
    if (!doc) return
    try {
      let token = ''
      // 是否需要确认由注册声明决定：这里不得为了省事跳过。
      if (commandRequiresConfirmation(registry, widget.widget_type, command)) {
        const issued = await issueControlConfirmation({
          document_id: doc.id,
          widget_id: widget.id,
          command,
          tenant_id: tenantQuery.value
        })
        token = issued.data?.confirmation_token ?? ''
        if (!token) {
          message.error(t('custom.scada.confirmationFailed'))
          return
        }
      }
      await executeControl({
        device_id: deviceId,
        document_id: doc.id,
        widget_id: widget.id,
        widget_type: widget.widget_type,
        version: widget.version,
        command,
        confirmation_token: token,
        tenant_id: tenantQuery.value
      })
      message.success(t('custom.scada.commandSent'))
    } catch {
      message.error(t('custom.scada.commandFailed'))
    }
  }

  watch(activeProjectId, async (id) => {
    documents.value = []
    activeDocument.value = null
    widgets.value = []
    if (!id) return
    await loadDocuments(id)
  })

  return {
    projects,
    documents,
    versions,
    activeProjectId,
    activeDocument,
    widgets,
    loading,
    saving,
    parseError,
    webglAvailable,
    linkState,
    lastMessageAt,
    now,
    editable,
    stale,
    resolution,
    canSave,
    degradedTypes,
    loadProjects,
    loadDocuments,
    openDocument,
    applyDocument,
    createProject,
    createDocument,
    addWidget,
    removeWidget,
    updateLayout,
    save,
    publish,
    archive,
    rollback,
    sendControl,
    stop
  }
}
