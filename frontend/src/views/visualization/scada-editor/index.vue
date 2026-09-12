<script setup lang="ts">
/**
 * 文件用途: SCADA 画布编辑器（ROADMAP P1.3）。
 * 核心逻辑: 项目管理 → 文档打开 → 画布编辑（增删 Widget、调整布局）→ 带版本号保存
 *           → 发布/回滚/归档，并显示遥测链路状态与控制命令确认流程。
 *
 * 关键注意事项:
 *  1. 保存必须携带 `current_version` 作为 expected_version；后端按它做条件更新。
 *     保存被拒时要能区分**版本冲突**（提示重新加载后重试）与**已归档**（重试无意义），
 *     混为一谈会让用户反复重试一个永远失败的操作。
 *  2. 画布解析失败必须**阻断保存**。解析失败却继续编辑并保存，
 *     等于把真实画布覆盖成空——是数据丢失，不是显示问题。
 *  3. 遥测非 connected 时显示"数据已过期"横幅，且不隐藏最后读数之外的提示：
 *     断线后把最后一帧当实时值展示，正是本项目要消灭的假成功。
 *  4. 已知未实现：画布拖拽（grid-layout-plus 尚无调用方证据，不盲接）、
 *     工业符号库、3D Widget 的实际渲染。这些在界面上明确标注，不做假入口。
 */
import { computed, ref, watch } from 'vue'
import { NButton, NCard, NInput, NInputNumber, NSelect, NSpin, NTag, useMessage } from 'naive-ui'
import { $t } from '@/locales'
import { useAuthStore } from '@/store/modules/auth'
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

const message = useMessage()
const authStore = useAuthStore()

// 内置 Widget 注册表。与后端 widget_registry.go 的注册保持一致；
// 真实接入时应改为从后端拉取，这里内置是为了让"能力降级"在前端可验证。
const WIDGET_REGISTRY: ScadaWidgetDefinition[] = [
  { type: 'gauge', version: '1', schema: '{}', capabilities: ['2d'], commands: [{ name: 'refresh', requires_confirmation: false }] },
  { type: 'chart', version: '1', schema: '{}', capabilities: ['2d'], commands: [{ name: 'refresh', requires_confirmation: false }] },
  { type: 'valve', version: '1', schema: '{}', capabilities: ['2d'], commands: [{ name: 'open_valve', requires_confirmation: true }] },
  { type: 'twin3d', version: '1', schema: '{}', capabilities: ['3d'], commands: [] }
]

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
// 遥测链路状态：真实接入应由数据通道驱动；默认 idle 即"陈旧"，
// 避免界面在链路尚未建立时把任何读数伪装成实时值。
const linkState = ref<TelemetryLinkState>('idle')
const lastMessageAt = ref<number | null>(null)
const now = ref(Date.now())
const newProjectName = ref('')
const newDocumentName = ref('')
const widgetTypeToAdd = ref('gauge')
const rollbackVersion = ref<number | null>(null)
const tenantFilter = ref('')

setInterval(() => { now.value = Date.now() }, 5000)

const isAdmin = computed(() => authStore.userInfo.authority === 'SYS_ADMIN')
const tenantQuery = computed(() => (isAdmin.value ? tenantFilter.value.trim() || undefined : undefined))
const editable = computed(() => canEditDocument(activeDocument.value?.status))
const stale = computed(() => isTelemetryStale(linkState.value, lastMessageAt.value, now.value))
const resolution = computed(() => resolveWidgets(widgets.value, WIDGET_REGISTRY, webglAvailable.value))
const canSave = computed(() => editable.value && parseError.value === '')

const versionOptions = computed(() =>
  versions.value.map(v => ({ label: `v${v.version}`, value: v.version }))
)

watch(activeProjectId, async id => {
  documents.value = []
  activeDocument.value = null
  widgets.value = []
  if (!id) return
  await loadDocuments(id)
})

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

async function handleCreateProject() {
  if (!newProjectName.value.trim()) return
  try {
    const { data } = await createScadaProject({
      name: newProjectName.value.trim(),
      tenant_id: tenantQuery.value
    })
    newProjectName.value = ''
    await loadProjects()
    if (data?.id) activeProjectId.value = data.id
    message.success($t('custom.scada.projectCreated'))
  } catch {
    message.error($t('custom.scada.createProjectFailed'))
  }
}

async function handleCreateDocument() {
  if (!activeProjectId.value || !newDocumentName.value.trim()) return
  try {
    const { data } = await createScadaDocument(activeProjectId.value, {
      name: newDocumentName.value.trim(),
      json_data: '{}',
      tenant_id: tenantQuery.value
    })
    newDocumentName.value = ''
    await loadDocuments(activeProjectId.value)
    if (data?.id) await openDocument(data.id)
  } catch {
    message.error($t('custom.scada.createDocumentFailed'))
  }
}

async function openDocument(id: string) {
  loading.value = true
  try {
    const { data } = await fetchScadaDocument(id, tenantQuery.value)
    applyDocument(data)
    const v = await fetchScadaDocumentVersions(id, tenantQuery.value)
    versions.value = v.data ?? []
  } finally {
    loading.value = false
  }
}

function applyDocument(doc: ScadaDocument | null | undefined) {
  activeDocument.value = doc ?? null
  const parsed = parseCanvas(doc?.json_data)
  if (!parsed.ok) {
    // 解析失败：清空编辑区并阻断保存，绝不用空画布顶替。
    widgets.value = []
    parseError.value = parsed.reason
    message.error($t('custom.scada.canvasParseFailed'))
    return
  }
  parseError.value = ''
  widgets.value = parsed.canvas.widgets
}

function addWidget() {
  widgets.value = [
    ...widgets.value,
    {
      id: `w-${Date.now()}`,
      widget_type: widgetTypeToAdd.value,
      version: '1',
      layout: { x: 0, y: widgets.value.length * 2, w: 2, h: 2 }
    }
  ]
}

function removeWidget(id: string) {
  widgets.value = widgets.value.filter(w => w.id !== id)
}

function updateLayout(id: string, patch: Partial<ScadaWidgetInstance['layout']>) {
  widgets.value = widgets.value.map(w => (w.id === id ? { ...w, layout: { ...w.layout, ...patch } } : w))
}

async function handleSave() {
  const doc = activeDocument.value
  if (!doc) return
  if (parseError.value) {
    message.error($t('custom.scada.canvasParseFailed'))
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
    message.success($t('custom.scada.saved'))
  } catch (error) {
    // 冲突与归档必须分开提示：前者重载即可，后者重试无意义。
    if (isVersionConflictError(error)) {
      message.error($t('custom.scada.versionConflict'))
    } else if (isArchivedError(error)) {
      message.error($t('custom.scada.archivedCannotEdit'))
    } else {
      message.error($t('custom.scada.saveFailed'))
    }
  } finally {
    saving.value = false
  }
}

async function handlePublish() {
  if (!activeDocument.value) return
  try {
    const { data } = await publishScadaDocument(activeDocument.value.id, tenantQuery.value)
    applyDocument(data)
    message.success($t('custom.scada.published'))
  } catch {
    message.error($t('custom.scada.publishFailed'))
  }
}

async function handleRollback() {
  if (!activeDocument.value || rollbackVersion.value == null) return
  try {
    const { data } = await rollbackScadaDocument(activeDocument.value.id, rollbackVersion.value, tenantQuery.value)
    applyDocument(data)
    // 回滚产生新的草稿版本，需要重新加载版本列表才能看到可选版本的变化。
    const v = await fetchScadaDocumentVersions(activeDocument.value.id, tenantQuery.value)
    versions.value = v.data ?? []
    message.success($t('custom.scada.rolledBack'))
  } catch {
    message.error($t('custom.scada.rollbackFailed'))
  }
}

async function handleArchive() {
  if (!activeDocument.value) return
  try {
    const { data } = await archiveScadaDocument(activeDocument.value.id, tenantQuery.value)
    applyDocument(data)
    message.success($t('custom.scada.archived'))
  } catch {
    message.error($t('custom.scada.archiveFailed'))
  }
}

async function handleControl(widget: ScadaWidgetInstance, command: string, deviceId: string) {
  const doc = activeDocument.value
  if (!doc) return
  try {
    let token = ''
    // 是否需要确认由注册声明决定：这里不得为了省事跳过。
    if (commandRequiresConfirmation(WIDGET_REGISTRY, widget.widget_type, command)) {
      const issued = await issueControlConfirmation({
        document_id: doc.id,
        widget_id: widget.id,
        command,
        tenant_id: tenantQuery.value
      })
      token = issued.data?.confirmation_token ?? ''
      if (!token) {
        message.error($t('custom.scada.confirmationFailed'))
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
    message.success($t('custom.scada.commandSent'))
  } catch {
    message.error($t('custom.scada.commandFailed'))
  }
}

loadProjects()
</script>

<template>
  <NSpin :show="loading">
    <div class="scada-editor">
      <NCard :title="$t('custom.scada.projects')">
        <div class="row">
          <NInput v-if="isAdmin" v-model:value="tenantFilter" :placeholder="$t('custom.scada.tenantId')" class="w-48" />
          <NInput v-model:value="newProjectName" :placeholder="$t('custom.scada.newProjectName')" class="w-48" />
          <NButton type="primary" @click="handleCreateProject">{{ $t('custom.scada.createProject') }}</NButton>
          <NButton @click="loadProjects">{{ $t('common.refresh') }}</NButton>
        </div>
        <NSelect
          v-model:value="activeProjectId"
          :options="projects.map(p => ({ label: p.name, value: p.id }))"
          :placeholder="$t('custom.scada.selectProject')"
          class="mt-2"
        />
      </NCard>

      <NCard v-if="activeProjectId" :title="$t('custom.scada.documents')" class="mt-3">
        <div class="row">
          <NInput v-model:value="newDocumentName" :placeholder="$t('custom.scada.newDocumentName')" class="w-48" />
          <NButton type="primary" @click="handleCreateDocument">{{ $t('custom.scada.createDocument') }}</NButton>
        </div>
        <div class="row mt-2">
          <NButton
            v-for="doc in documents"
            :key="doc.id"
            size="small"
            :type="activeDocument?.id === doc.id ? 'primary' : 'default'"
            @click="openDocument(doc.id)"
          >
            {{ doc.name }} (v{{ doc.current_version }})
          </NButton>
        </div>
      </NCard>

      <NCard v-if="activeDocument" :title="activeDocument.name" class="mt-3">
        <div class="row">
          <NTag :type="activeDocument.status === 'PUBLISHED' ? 'success' : 'default'">
            {{ activeDocument.status }}
          </NTag>
          <NTag>v{{ activeDocument.current_version }}</NTag>
          <NTag v-if="activeDocument.published_version">published v{{ activeDocument.published_version }}</NTag>
          <!-- 陈旧横幅：断线后把最后一帧当实时值是典型的假成功 -->
          <NTag v-if="stale" type="warning">{{ $t('custom.scada.dataStale') }}</NTag>
          <NTag v-if="resolution.degraded.length" type="warning">
            {{ $t('custom.scada.degradedWidgets') }}: {{ resolution.degraded.map(d => d.type).join(', ') }}
          </NTag>
        </div>

        <div v-if="parseError" class="mt-2">
          <NTag type="error">{{ $t('custom.scada.canvasParseFailed') }} ({{ parseError }})</NTag>
        </div>

        <div class="row mt-2">
          <NSelect v-model:value="widgetTypeToAdd" :options="WIDGET_REGISTRY.map(w => ({ label: `${w.type}@${w.version}`, value: w.type }))" class="w-48" />
          <NButton :disabled="!editable" @click="addWidget">{{ $t('custom.scada.addWidget') }}</NButton>
          <NButton type="primary" :disabled="!canSave" :loading="saving" @click="handleSave">
            {{ $t('custom.scada.save') }}
          </NButton>
          <NButton :disabled="!editable" @click="handlePublish">{{ $t('custom.scada.publish') }}</NButton>
          <NButton :disabled="!editable" @click="handleArchive">{{ $t('custom.scada.archive') }}</NButton>
        </div>

        <div class="row mt-2">
          <NSelect v-model:value="rollbackVersion" :options="versionOptions" :placeholder="$t('custom.scada.selectVersion')" class="w-48" />
          <NButton :disabled="!editable || rollbackVersion === null" @click="handleRollback">
            {{ $t('custom.scada.rollback') }}
          </NButton>
        </div>

        <!-- 画布：当前为 CSS 网格预览 + 布局数值编辑，拖拽尚未接线（不做假入口） -->
        <div class="canvas mt-3">
          <div
            v-for="w in widgets"
            :key="w.id"
            class="canvas-item"
            :style="{ gridColumn: `${w.layout.x + 1} / span ${w.layout.w}`, gridRow: `${w.layout.y + 1} / span ${w.layout.h}` }"
          >
            <div class="row">
              <strong>{{ w.widget_type }}@{{ w.version }}</strong>
              <NButton size="tiny" :disabled="!editable" @click="removeWidget(w.id)">{{ $t('common.delete') }}</NButton>
            </div>
            <div class="row">
              <NInputNumber size="tiny" :value="w.layout.x" @update:value="v => updateLayout(w.id, { x: v ?? 0 })" />
              <NInputNumber size="tiny" :value="w.layout.y" @update:value="v => updateLayout(w.id, { y: v ?? 0 })" />
              <NInputNumber size="tiny" :value="w.layout.w" @update:value="v => updateLayout(w.id, { w: v ?? 1 })" />
              <NInputNumber size="tiny" :value="w.layout.h" @update:value="v => updateLayout(w.id, { h: v ?? 1 })" />
            </div>
            <div class="row">
              <NButton
                v-for="cmd in (WIDGET_REGISTRY.find(d => d.type === w.widget_type)?.commands ?? [])"
                :key="cmd.name"
                size="tiny"
                :type="cmd.requires_confirmation ? 'warning' : 'default'"
                @click="handleControl(w, cmd.name, '')"
              >
                {{ cmd.name }}{{ cmd.requires_confirmation ? ' ⚠' : '' }}
              </NButton>
            </div>
          </div>
        </div>
        <p v-if="!widgets.length" class="hint">{{ $t('custom.scada.emptyCanvas') }}</p>
      </NCard>
    </div>
  </NSpin>
</template>

<style scoped>
.scada-editor {
  padding: 12px;
}
.row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
}
.mt-2 {
  margin-top: 8px;
}
.mt-3 {
  margin-top: 12px;
}
.w-48 {
  width: 220px;
}
.canvas {
  display: grid;
  grid-auto-rows: 40px;
  grid-template-columns: repeat(12, 1fr);
  gap: 6px;
  min-height: 160px;
  padding: 8px;
  border: 1px dashed #c0c4cc;
  border-radius: 6px;
}
.canvas-item {
  padding: 6px;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  background: #fafafa;
  overflow: hidden;
}
.hint {
  margin-top: 8px;
  color: #909399;
}
</style>
