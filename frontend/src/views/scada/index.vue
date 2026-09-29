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
import { computed, onMounted, ref } from 'vue'
import { NAlert, NButton, NEmpty, NInput, NSelect, NSpace, NTag, useMessage } from 'naive-ui'

import { computeFitScale } from './core/canvasDocument'
import { SCADA_SYMBOL_VIEWBOX, findScadaSymbol, type ScadaSymbol } from './core/symbolLibrary'
import { useCanvasEditor } from './core/useCanvasEditor'
import { useElementSize } from './core/useElementSize'
import { usePointerDrag } from './core/usePointerDrag'
import { useScadaDocumentWorkflow } from './core/useScadaDocumentWorkflow'
import ScadaInspector from './modules/ScadaInspector.vue'
import ScadaPalette from './modules/ScadaPalette.vue'

const message = useMessage()

const editor = useCanvasEditor()
const { canvas, selectedId, selectedNode, isDirty, displayMode } = editor

const workflow = useScadaDocumentWorkflow(editor, {
  error: (text) => message.error(text),
  success: (text) => message.success(text)
})
const {
  versions,
  projectId,
  documentId,
  currentDocument,
  loading,
  saving,
  versionConflict,
  loadError,
  projectOptions,
  documentOptions,
  canSave,
  canPublish
} = workflow

const newProjectName = ref('')
const newDocumentName = ref('')

// ---------------------------------------------------------------------------
// TP-22 fixed1080 等比适配：fixed 模式下容器按 computeFitScale 缩放画布，
// responsive 恒为 1（旧画布行为零变化）。拖拽位移按 scale 反除，缩放不影响编辑精度。
// 观察器与拖拽监听都在卸载时释放（见 useElementSize / usePointerDrag）。
// ---------------------------------------------------------------------------
const canvasWrapRef = ref<HTMLElement | null>(null)
const { size: wrapSize, start: observeCanvasWrap } = useElementSize(canvasWrapRef)
const drag = usePointerDrag()

const fitScale = computed(() =>
  displayMode.value === 'fixed1080'
    ? computeFitScale(wrapSize.value.width, wrapSize.value.height, canvas.value.width, canvas.value.height)
    : 1
)

// 用户可见文案沿用本页既有惯例（save/publish 等均为组件内常量，不进 locale）。
const displayModeOptions = [
  { label: 'responsive (free size)', value: 'responsive' },
  { label: 'fixed 1920x1080 (TV wall)', value: 'fixed1080' }
]

function onDisplayModeChange(value: string) {
  editor.setDisplayMode(value === 'fixed1080' ? 'fixed1080' : 'responsive')
}

function addSymbol(symbol: ScadaSymbol) {
  try {
    editor.addNode({ kind: 'symbol', ref: symbol.key, x: 40, y: 40, width: symbol.defaultWidth, height: symbol.defaultHeight })
  } catch (error) {
    message.error((error as Error).message)
  }
}

function addWidget() {
  editor.addNode({ kind: 'widget', ref: 'timeseries', x: 40, y: 40, width: 320, height: 180 })
}

function onNodeMouseDown(event: MouseEvent, id: string) {
  editor.select(id)
  const node = canvas.value.nodes.find((item) => item.id === id)
  if (!node) return
  const originX = node.x
  const originY = node.y
  // fixed1080 缩放后屏幕位移与画布坐标差 scale 倍，反除保持 1:1 编辑手感。
  drag.begin(event, ({ dx, dy }) => editor.moveNode(id, originX + dx / fitScale.value, originY + dy / fitScale.value))
}

const onSave = workflow.save
const onPublish = workflow.publish
const onRollback = workflow.rollback
const onArchive = workflow.archive

async function onCreateProject() {
  if (await workflow.createProject(newProjectName.value)) newProjectName.value = ''
}

async function onCreateDocument() {
  if (await workflow.createDocument(newDocumentName.value)) newDocumentName.value = ''
}

onMounted(() => {
  workflow.loadProjects()
  observeCanvasWrap()
})

// 显式暴露关键状态与动作：script setup 默认封闭，
// 而"保存是否带 expected_version""回滚后是否重置"这类契约必须在测试里可断言。
defineExpose({ isDirty, onSave, onRollback, onPublish, displayMode, fitScale, setDisplayMode: onDisplayModeChange })
</script>

<template>
  <div class="scada-editor">
    <NSpace vertical :size="12">
      <NAlert v-if="versionConflict" type="warning" title="版本冲突">
        画布已被他人修改。请刷新重新载入最新内容后再次保存，避免覆盖他人的改动。
      </NAlert>
      <NAlert v-if="loadError" type="error" :title="loadError" />

      <NSpace align="center" :wrap="true">
        <NSelect
          v-model:value="projectId"
          :options="projectOptions"
          :loading="loading"
          style="width: 220px"
          placeholder="select project"
        />
        <NInput v-model:value="newProjectName" style="width: 160px" placeholder="new project name" />
        <NButton size="small" @click="onCreateProject">new project</NButton>
      </NSpace>

      <NSpace align="center" :wrap="true">
        <NSelect
          v-model:value="documentId"
          :options="documentOptions"
          style="width: 280px"
          placeholder="select canvas"
        />
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
        <NSelect
          :value="displayMode"
          :options="displayModeOptions"
          style="width: 200px"
          data-testid="scada-display-mode"
          @update:value="onDisplayModeChange"
        />
        <NTag v-if="displayMode === 'fixed1080'" size="small">x{{ fitScale.toFixed(2) }}</NTag>
      </NSpace>

      <div class="scada-editor__body">
        <ScadaPalette @add-symbol="addSymbol" @add-widget="addWidget" />

        <section ref="canvasWrapRef" class="scada-editor__canvas-wrap">
          <!-- fixed1080：外层占位盒按缩放后尺寸撑开滚动区，画布本体 transform scale 等比适配。 -->
          <div
            class="scada-editor__canvas-scaler"
            :style="
              fitScale === 1
                ? undefined
                : { width: `${canvas.width * fitScale}px`, height: `${canvas.height * fitScale}px` }
            "
          >
            <div
              class="scada-editor__canvas"
              :style="{
                width: `${canvas.width}px`,
                height: `${canvas.height}px`,
                background: canvas.background || '#fff',
                transform: fitScale === 1 ? undefined : `scale(${fitScale})`
              }"
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
          </div>
        </section>

        <ScadaInspector
          :node="selectedNode"
          :versions="versions"
          @update-node="editor.updateNode"
          @bring-to-front="editor.bringToFront"
          @remove-node="editor.removeNode"
          @rollback="onRollback"
        />
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

.scada-editor__canvas-wrap {
  flex: 1 1 auto;
  overflow: auto;
  border: 1px solid var(--border-color);
}

.scada-editor__canvas-scaler {
  /* fixed1080 缩放占位：尺寸由内联样式按 scale 计算，responsive 模式不生效。 */
}

.scada-editor__canvas {
  position: relative;
  transform-origin: top left;
}

.scada-editor__node {
  position: absolute;
  cursor: move;
  border: 1px dashed transparent;
  color: var(--text-color-1);
}

.scada-editor__node.is-selected {
  border-color: rgb(var(--primary-color));
}

.scada-editor__node-label {
  display: block;
  padding: 4px;
  font-size: 12px;
}
</style>
