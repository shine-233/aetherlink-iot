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
import { computed, ref } from 'vue'
import { NButton, NCard, NInput, NSelect, NSpin, NTag, useMessage } from 'naive-ui'
import { $t } from '@/locales'
import { useAuthStore } from '@/store/modules/auth'
import type { ScadaWidgetDefinition } from './scada-model'
import ScadaWidgetCard from './ScadaWidgetCard.vue'
import { useScadaEditorWorkflow } from './useScadaEditorWorkflow'

const message = useMessage()
const authStore = useAuthStore()

// 内置 Widget 注册表。与后端 widget_registry.go 的注册保持一致；
// 真实接入时应改为从后端拉取，这里内置是为了让"能力降级"在前端可验证。
// 与后端 scada_mobile_wiring.go 的 builtinWidgetDefinitions 逐字段一致
// （parity 测试 TestBuiltinWidgetRegistryMatchesFrontend 按路径读取本文件，勿移走）。
// schema 全部字段可选、只做类型/取值约束：存量画布不受影响，新画布错误配置在保存时被拒。
// prettier-ignore
const WIDGET_REGISTRY: ScadaWidgetDefinition[] = [
  { type: 'gauge', version: '1', schema: '{"type":"object","properties":{"title":{"type":"string","maxLength":64},"unit":{"type":"string","maxLength":16},"telemetry_key":{"type":"string","maxLength":128},"min":{"type":"number"},"max":{"type":"number"}}}', capabilities: ['2d'], commands: [{ name: 'refresh', requires_confirmation: false }] },
  { type: 'chart', version: '1', schema: '{"type":"object","properties":{"title":{"type":"string","maxLength":64},"telemetry_keys":{"type":"array","items":{"type":"string","maxLength":128},"maxItems":8},"time_window_seconds":{"type":"integer","minimum":60,"maximum":2592000}}}', capabilities: ['2d'], commands: [{ name: 'refresh', requires_confirmation: false }] },
  { type: 'valve', version: '1', schema: '{"type":"object","properties":{"title":{"type":"string","maxLength":64},"telemetry_key":{"type":"string","maxLength":128},"device_id":{"type":"string","maxLength":64},"open_command":{"type":"string","maxLength":64},"close_command":{"type":"string","maxLength":64}}}', capabilities: ['2d'], commands: [{ name: 'open_valve', requires_confirmation: true }] },
  { type: 'twin3d', version: '1', schema: '{"type":"object","properties":{"title":{"type":"string","maxLength":64},"model_url":{"type":"string","maxLength":512},"camera_initial":{"type":"string","enum":["orbit","front","top","side"]}}}', capabilities: ['3d'], commands: [] }
]
const newProjectName = ref('')
const newDocumentName = ref('')
const widgetTypeToAdd = ref('gauge')
const rollbackVersion = ref<number | null>(null)
const tenantFilter = ref('')

const isAdmin = computed(() => authStore.userInfo.authority === 'SYS_ADMIN')
const tenantQuery = computed(() => (isAdmin.value ? tenantFilter.value.trim() || undefined : undefined))

// 文档工作流、陈旧时钟（卸载自动清理）与控制命令确认流程见 useScadaEditorWorkflow。
const workflow = useScadaEditorWorkflow({ registry: WIDGET_REGISTRY, message, t: $t, tenantQuery })
const {
  documents,
  versions,
  activeProjectId,
  activeDocument,
  widgets,
  loading,
  saving,
  parseError,
  editable,
  stale,
  resolution,
  canSave,
  degradedTypes,
  loadProjects,
  openDocument
} = workflow

const versionOptions = computed(() => versions.value.map((v) => ({ label: `v${v.version}`, value: v.version })))
const projectOptions = computed(() => workflow.projects.value.map((p) => ({ label: p.name, value: p.id })))
// 注册表是静态常量：选项与命令索引只需构建一次，避免模板每次渲染（含 5s 时钟触发）重复 map/find。
const widgetTypeOptions = WIDGET_REGISTRY.map((w) => ({ label: `${w.type}@${w.version}`, value: w.type }))
const commandsByType = new Map(WIDGET_REGISTRY.map((d) => [d.type, d.commands ?? []]))

async function handleCreateProject() {
  if (await workflow.createProject(newProjectName.value)) newProjectName.value = ''
}

async function handleCreateDocument() {
  if (await workflow.createDocument(newDocumentName.value)) newDocumentName.value = ''
}

const handleSave = workflow.save
const handlePublish = workflow.publish
const handleArchive = workflow.archive
const handleRollback = () => workflow.rollback(rollbackVersion.value)

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
          :options="projectOptions"
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
            {{ $t('custom.scada.degradedWidgets') }}: {{ degradedTypes }}
          </NTag>
        </div>

        <div v-if="parseError" class="mt-2">
          <NTag type="error">{{ $t('custom.scada.canvasParseFailed') }} ({{ parseError }})</NTag>
        </div>

        <div class="row mt-2">
          <NSelect v-model:value="widgetTypeToAdd" :options="widgetTypeOptions" class="w-48" />
          <NButton :disabled="!editable" @click="workflow.addWidget(widgetTypeToAdd)">{{ $t('custom.scada.addWidget') }}</NButton>
          <NButton type="primary" :disabled="!canSave" :loading="saving" @click="handleSave">
            {{ $t('custom.scada.save') }}
          </NButton>
          <NButton :disabled="!editable" @click="handlePublish">{{ $t('custom.scada.publish') }}</NButton>
          <NButton :disabled="!editable" @click="handleArchive">{{ $t('custom.scada.archive') }}</NButton>
        </div>

        <div class="row mt-2">
          <NSelect
            v-model:value="rollbackVersion"
            :options="versionOptions"
            :placeholder="$t('custom.scada.selectVersion')"
            class="w-48"
          />
          <NButton :disabled="!editable || rollbackVersion === null" @click="handleRollback">
            {{ $t('custom.scada.rollback') }}
          </NButton>
        </div>

        <!-- 画布：当前为 CSS 网格预览 + 布局数值编辑，拖拽尚未接线（不做假入口） -->
        <div class="canvas mt-3">
          <ScadaWidgetCard
            v-for="w in widgets"
            :key="w.id"
            :widget="w"
            :commands="commandsByType.get(w.widget_type) ?? []"
            :editable="editable"
            @remove="workflow.removeWidget"
            @update-layout="workflow.updateLayout"
            @control="(widget, cmd) => workflow.sendControl(widget, cmd, '')"
          />
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
  border: 1px dashed var(--border-color);
  border-radius: 6px;
}
.hint {
  margin-top: 8px;
  color: var(--text-color-3);
}
</style>
