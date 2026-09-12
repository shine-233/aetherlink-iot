<!--
  文件用途：SCADA 画布编辑器（ROADMAP P1.3）。
  核心逻辑：选择项目与文档 → 从符号/组件面板添加节点 → 在画布上拖动 →
    保存（带 expected_version 乐观并发）、发布、回滚与版本列表。
  关键注意事项：
    1. 保存必须带 expected_version。后端按它做条件更新，省略会让别人的保存被无声覆盖。
       版本冲突必须显式提示并引导刷新，绝不静默重试。
    2. 文档"脏"的判定基于序列化结果而非对象引用（见 core/useCanvasEditor），
       否则一次无变化的往返保存也会把版本 +1。
    3. API 返回 { data, error }，error 必须显式处理——忽略它会让失败看起来像"空数据"，
       用户以为画布本来就空，实际是请求挂了。
    4. 画布编辑规则在 core/useCanvasEditor.ts 与 core/canvasDocument.ts，本文件不重写。
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import {
  NAlert,
  NButton,
  NCollapse,
  NCollapseItem,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NSelect,
  NSpace,
  NTag,
  useMessage
} from 'naive-ui'

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
import {
  SCADA_SYMBOL_VIEWBOX,
  findScadaSymbol,
  listScadaSymbolCategories,
  listScadaSymbolsByCategory
} from './core/symbolLibrary'
import { useCanvasEditor } from './core/useCanvasEditor'

const message = useMessage()

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
const newProjectName = ref('')
const newDocumentName = ref('')
const tenantId = ref('')

const editor = useCanvasEditor()
const { canvas, selectedId, selectedNode, isDirty } = editor

const projectOptions = computed(() => projects.value.map(project => ({ label: project.name, value: project.id })))
const documentOptions = computed(() =>
  documents.value.map(doc => ({
    label: `${doc.name} (v${doc.current_version}${doc.published_version === null ? '' : ` / pub v${doc.published_version}`})`,
    value: doc.id
  }))
)
const palette = computed(() =>
  listScadaSymbolCategories().map(category => ({ category, symbols: listScadaSymbolsByCategory(category) }))
)
const canSave = computed(() => !!currentDocument.value && isDirty.value && !saving.value)
// 只允许在已同步状态发布：发布未保存的草稿会把"屏幕上看到的"和"库里的"割裂开。
const canPublish = computed(() => !!currentDocument.value && !isDirty.value)

function reportFailure(fallback: string, error: unknown) {
  const text = String((error as { message?: string })?.message ?? error ?? '')
  message.error(text || fallback)
}

async function loadProjects() {
  loading.value = true
  loadError.value = ''
  try {
    const { data, error } = await fetchScadaProjects(tenantId.value || undefined)
    if (error || !data) {
      loadError.value = '项目列表加载失败'
      return
    }
    projects.value = data
    if (!projectId.value && projects.value.length > 0) projectId.value = projects.value[0].id
  } finally {
    loading.value = false
  }
}

async function loadDocuments() {
  if (!projectId.value) {
    documents.value = []
    return
  }
  const { data, error } = await fetchScadaDocuments(projectId.value, tenantId.value || undefined)
  if (error || !data) {
    reportFailure('画布列表加载失败', error)
    return
  }
  documents.value = data
  if (!documents.value.some(doc => doc.id === documentId.value)) {
    documentId.value = documents.value[0]?.id ?? null
  }
}

async function loadVersions() {
  if (!documentId.value) {
    versions.value = []
    return
  }
  const { data, error } = await fetchScadaDocumentVersions(documentId.value, tenantId.value || undefined)
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
  const { data, error } = await fetchScadaDocument(documentId.value, tenantId.value || undefined)
  if (error || !data) {
    reportFailure('画布加载失败', error)
    return
  }
  currentDocument.value = data
  editor.load(data.json_data ?? '')
  versionConflict.value = false
  await loadVersions()
}

function addSymbol(ref: string, defaultWidth: number, defaultHeight: number) {
  try {
    editor.addNode({ kind: 'symbol', ref, x: 40, y: 40, width: defaultWidth, height: defaultHeight })
  } catch (error) {
    message.error((error as Error).message)
  }
}

function addWidget() {
  editor.addNode({ kind: 'widget', ref: 'timeseries', x: 40, y: 40, width: 320, height: 180 })
}

function onNodeMouseDown(event: MouseEvent, id: string) {
  editor.select(id)
  const node = canvas.value.nodes.find(item => item.id === id)
  if (!node) return
  const startX = event.clientX
  const startY = event.clientY
  const originX = node.x
  const originY = node.y
  const onMove = (moveEvent: MouseEvent) => {
    editor.moveNode(id, originX + (moveEvent.clientX - startX), originY + (moveEvent.clientY - startY))
  }
  const onUp = () => {
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

async function onSave() {
  const doc = currentDocument.value
  if (!doc) return
  saving.value = true
  try {
    const { data, error } = await saveScadaDocument(doc.id, {
      expected_version: doc.current_version,
      json_data: editor.serialize(),
      tenant_id: tenantId.value || undefined
    })
    if (error || !data) {
      const text = String((error as { message?: string })?.message ?? '')
      if (/version|conflict/i.test(text)) {
        // 冲突必须被识别：静默重试会把别人的画布覆盖掉。
        versionConflict.value = true
        message.error('版本冲突：画布已被他人修改，请刷新后重新编辑')
      } else {
        reportFailure('保存失败', error)
      }
      return
    }
    currentDocument.value = data
    editor.markSaved()
    versionConflict.value = false
    message.success('已保存')
    await loadDocuments()
  } finally {
    saving.value = false
  }
}

async function onPublish() {
  const doc = currentDocument.value
  if (!doc) return
  const { data, error } = await publishScadaDocument(doc.id, tenantId.value || undefined)
  if (error || !data) {
    reportFailure('发布失败', error)
    return
  }
  currentDocument.value = data
  editor.markSaved()
  message.success('已发布')
  await loadDocuments()
  await loadVersions()
}

async function onRollback(version: number) {
  const doc = currentDocument.value
  if (!doc) return
  const { data, error } = await rollbackScadaDocument(doc.id, version, tenantId.value || undefined)
  if (error || !data) {
    reportFailure('回滚失败', error)
    return
  }
  currentDocument.value = data
  // 回滚产生新草稿，必须重新载入：继续用旧内存态编辑会把回滚结果冲掉。
  editor.load(data.json_data ?? '')
  message.success(`已回滚到 v${version}（生成为新草稿）`)
  await loadDocuments()
  await loadVersions()
}

async function onArchive() {
  const doc = currentDocument.value
  if (!doc) return
  const { error } = await archiveScadaDocument(doc.id, tenantId.value || undefined)
  if (error) {
    reportFailure('归档失败', error)
    return
  }
  message.success('已归档')
  await loadDocuments()
}

async function onCreateProject() {
  const name = newProjectName.value.trim()
  if (!name) return
  const { data, error } = await createScadaProject({ name, tenant_id: tenantId.value || undefined })
  if (error || !data) {
    reportFailure('新建项目失败', error)
    return
  }
  newProjectName.value = ''
  await loadProjects()
  projectId.value = data.id
}

async function onCreateDocument() {
  if (!projectId.value) return
  const name = newDocumentName.value.trim() || 'untitled-canvas'
  const { data, error } = await createScadaDocument(projectId.value, {
    name,
    json_data: editor.serialize(),
    tenant_id: tenantId.value || undefined
  })
  if (error || !data) {
    reportFailure('新建画布失败', error)
    return
  }
  newDocumentName.value = ''
  await loadDocuments()
  documentId.value = data.id
}

watch(projectId, () => {
  loadDocuments()
})
watch(documentId, () => {
  loadDocument()
})

onMounted(() => {
  loadProjects()
})

// 显式暴露关键状态与动作：script setup 默认封闭，
// 而"保存是否带 expected_version""回滚后是否重置"这类契约必须在测试里可断言。
defineExpose({ isDirty, onSave, onRollback, onPublish })
</script>

<template>
  <div class="scada-editor">
    <NSpace vertical :size="12">
      <NAlert v-if="versionConflict" type="warning" title="版本冲突">
        画布已被他人修改。请刷新重新载入最新内容后再次保存，避免覆盖他人的改动。
      </NAlert>
      <NAlert v-if="loadError" type="error" :title="loadError" />

      <NSpace align="center" :wrap="true">
        <NSelect v-model:value="projectId" :options="projectOptions" :loading="loading" style="width: 220px" placeholder="select project" />
        <NInput v-model:value="newProjectName" style="width: 160px" placeholder="new project name" />
        <NButton size="small" @click="onCreateProject">new project</NButton>
      </NSpace>

      <NSpace align="center" :wrap="true">
        <NSelect v-model:value="documentId" :options="documentOptions" style="width: 280px" placeholder="select canvas" />
        <NInput v-model:value="newDocumentName" style="width: 160px" placeholder="new canvas name" />
        <NButton size="small" @click="onCreateDocument">new canvas</NButton>
      </NSpace>

      <NSpace align="center" :wrap="true">
        <NButton type="primary" :disabled="!canSave" :loading="saving" @click="onSave">save</NButton>
        <NButton :disabled="!canPublish" @click="onPublish">publish</NButton>
        <NButton :disabled="!currentDocument" @click="onArchive">archive</NButton>
        <NTag v-if="isDirty" type="warning">unsaved</NTag>
        <NTag v-else type="success">synced</NTag>
        <NTag v-if="currentDocument">v{{ currentDocument.current_version }}</NTag>
      </NSpace>

      <div class="scada-editor__body">
        <aside class="scada-editor__palette">
          <NCollapse>
            <NCollapseItem title="widgets" name="widgets">
              <NButton size="tiny" @click="addWidget">+ timeseries</NButton>
            </NCollapseItem>
            <NCollapseItem v-for="group in palette" :key="group.category" :title="group.category" :name="group.category">
              <NSpace vertical :size="4">
                <NButton
                  v-for="symbol in group.symbols"
                  :key="symbol.key"
                  size="tiny"
                  quaternary
                  @click="addSymbol(symbol.key, symbol.defaultWidth, symbol.defaultHeight)"
                >
                  {{ symbol.label }}
                </NButton>
              </NSpace>
            </NCollapseItem>
          </NCollapse>
        </aside>

        <section class="scada-editor__canvas-wrap">
          <div
            class="scada-editor__canvas"
            :style="{ width: `${canvas.width}px`, height: `${canvas.height}px`, background: canvas.background || '#fff' }"
          >
            <div
              v-for="node in canvas.nodes"
              :key="node.id"
              class="scada-editor__node"
              :class="{ 'is-selected': node.id === selectedId }"
              :style="{
                left: `${node.x}px`,
                top: `${node.y}px`,
                width: `${node.width}px`,
                height: `${node.height}px`,
                zIndex: node.z ?? 0
              }"
              @mousedown="onNodeMouseDown($event, node.id)"
            >
              <svg
                v-if="node.kind === 'symbol'"
                :viewBox="SCADA_SYMBOL_VIEWBOX"
                width="100%"
                height="100%"
                fill="none"
                stroke="currentColor"
                stroke-width="3"
              >
                <!-- 符号体来自受控符号库 core/symbolLibrary.ts，不含脚本或事件属性。 -->
                <!-- eslint-disable-next-line vue/no-v-html -->
                <g v-html="findScadaSymbol(node.ref)?.body ?? ''" />
              </svg>
              <span v-else class="scada-editor__node-label">{{ node.ref }}</span>
            </div>
            <NEmpty v-if="canvas.nodes.length === 0" description="add a symbol or widget from the left panel" />
          </div>
        </section>

        <aside class="scada-editor__inspector">
          <NForm v-if="selectedNode" label-placement="left" size="small">
            <NFormItem label="X">
              <NInputNumber
                :value="selectedNode.x"
                @update:value="v => v !== null && editor.updateNode(selectedNode!.id, { x: v })"
              />
            </NFormItem>
            <NFormItem label="Y">
              <NInputNumber
                :value="selectedNode.y"
                @update:value="v => v !== null && editor.updateNode(selectedNode!.id, { y: v })"
              />
            </NFormItem>
            <NFormItem label="W">
              <NInputNumber
                :value="selectedNode.width"
                @update:value="v => v !== null && editor.updateNode(selectedNode!.id, { width: v })"
              />
            </NFormItem>
            <NFormItem label="H">
              <NInputNumber
                :value="selectedNode.height"
                @update:value="v => v !== null && editor.updateNode(selectedNode!.id, { height: v })"
              />
            </NFormItem>
            <NSpace>
              <NButton size="tiny" @click="editor.bringToFront(selectedNode.id)">front</NButton>
              <NButton size="tiny" type="error" @click="editor.removeNode(selectedNode.id)">delete</NButton>
            </NSpace>
          </NForm>
          <NEmpty v-else description="select a node to edit" />

          <NCollapse style="margin-top: 12px">
            <NCollapseItem title="versions" name="versions">
              <NSpace vertical :size="4">
                <NSpace v-for="version in versions" :key="version.id" align="center">
                  <NTag size="small">v{{ version.version }}</NTag>
                  <NButton size="tiny" @click="onRollback(version.version)">rollback</NButton>
                </NSpace>
              </NSpace>
            </NCollapseItem>
          </NCollapse>
        </aside>
      </div>
    </NSpace>
  </div>
</template>

<style scoped>
.scada-editor {
  padding: 12px;
}

.scada-editor__body {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.scada-editor__palette,
.scada-editor__inspector {
  width: 200px;
  flex: 0 0 200px;
}

.scada-editor__canvas-wrap {
  flex: 1 1 auto;
  overflow: auto;
  border: 1px solid #e5e7eb;
}

.scada-editor__canvas {
  position: relative;
  transform-origin: top left;
}

.scada-editor__node {
  position: absolute;
  cursor: move;
  border: 1px dashed transparent;
  color: #1f2937;
}

.scada-editor__node.is-selected {
  border-color: #2563eb;
}

.scada-editor__node-label {
  display: block;
  padding: 4px;
  font-size: 12px;
}
</style>
