<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { NButton, NCard, NInput, NInputNumber, NSelect, NSpin, useMessage } from 'naive-ui'
import { LocalVisualizationViewer, type ChartWidgetConfig, type LocalWidgetType, type MetricWidgetConfig, type TextWidgetConfig } from '@/components/local-visualization-viewer'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
import { getDefaultVisualizationProviderFacade } from '@/service/visualization-provider/composition'
import type { VisualizationDashboardSchema } from '@/service/visualization-provider/contracts'
import { useAuthStore } from '@/store/modules/auth'
import {
  addWidget,
  loadEditorDashboard,
  removeWidget,
  serializeEditorDashboard,
  updateWidgetConfig,
  updateWidgetLayout,
  type EditorDashboard,
  type EditorModelResult,
  type EditorWidget
} from './editor-model'

const ADMIN_ROLES = new Set(['SYS_ADMIN', 'TENANT_ADMIN'])
const WIDGET_TYPES: { label: string; value: LocalWidgetType }[] = [
  { label: 'Text', value: 'text' },
  { label: 'Metric', value: 'metric' },
  { label: 'Line chart', value: 'line-chart' },
  { label: 'Bar chart', value: 'bar-chart' }
]

const route = useRoute()
const authStore = useAuthStore()
const { routerPushByKey } = useRouterPush()
const message = useMessage()
const providerFacade = getDefaultVisualizationProviderFacade()
const board = ref<VisualizationDashboardSchema | null>(null)
const dashboard = ref<EditorDashboard | null>(null)
const loading = ref(false)
const failed = ref(false)
const saving = ref(false)
const selectedType = ref<LocalWidgetType>('text')
const boardName = ref('')
const boardDescription = ref('')
let requestSequence = 0

const boardId = computed(() => {
  const id = route.query.id
  return typeof id === 'string' && id.trim() ? id.trim() : ''
})
const canEdit = computed(() => {
  const roles = new Set<string>()
  if (typeof authStore.userInfo.authority === 'string') roles.add(authStore.userInfo.authority)
  if (Array.isArray(authStore.userInfo.roles)) {
    authStore.userInfo.roles.forEach(role => {
      if (typeof role === 'string') roles.add(role)
    })
  }
  return [...roles].some(role => ADMIN_ROLES.has(role))
})

function isCurrentRequest(sequence: number, id: string) {
  return sequence === requestSequence && id === boardId.value
}

function applyResult(result: EditorModelResult) {
  if (!result.ok) {
    message.error(result.error)
    return false
  }
  dashboard.value = result.dashboard
  return true
}

async function loadBoard() {
  const id = boardId.value
  const sequence = ++requestSequence
  board.value = null
  dashboard.value = null
  boardName.value = ''
  boardDescription.value = ''
  failed.value = false
  loading.value = Boolean(id && canEdit.value)

  if (!id || !canEdit.value) {
    failed.value = true
    return
  }

  try {
    const result = await providerFacade.execute(provider => provider.getDashboard(id))
    if (!isCurrentRequest(sequence, id)) return
    if (!result.ok || result.data.id !== id || result.data.rendererData === undefined) {
      failed.value = true
      return
    }
    const normalized = loadEditorDashboard(result.data.rendererData)
    if (!normalized.ok) {
      failed.value = true
      return
    }
    board.value = result.data
    boardName.value = result.data.name
    boardDescription.value = result.data.description ?? ''
    dashboard.value = normalized.dashboard
  } catch {
    if (!isCurrentRequest(sequence, id)) return
    failed.value = true
  } finally {
    if (isCurrentRequest(sequence, id)) loading.value = false
  }
}

function handleAddWidget() {
  if (!dashboard.value || !canEdit.value) return
  applyResult(addWidget(dashboard.value, selectedType.value))
}

function handleRemoveWidget(id: string) {
  if (!dashboard.value || !canEdit.value) return
  applyResult(removeWidget(dashboard.value, id))
}

function handleLayout(widget: EditorWidget, key: 'x' | 'y' | 'w' | 'h', value: number | null) {
  if (!dashboard.value || value === null) return
  applyResult(updateWidgetLayout(dashboard.value, widget.id, { [key]: value }))
}

function handleConfig(widget: EditorWidget, key: string, value: unknown) {
  if (!dashboard.value) return
  applyResult(updateWidgetConfig(dashboard.value, widget.id, { ...widget.config, [key]: value }))
}

function handleChartCategories(widget: EditorWidget, value: string) {
  const categories = value.trim() ? value.split(',').map(item => item.trim()) : []
  handleConfig(widget, 'categories', categories)
}

function handleChartValues(widget: EditorWidget, value: string) {
  const values = value.trim() ? value.split(',').map(item => Number(item.trim())) : []
  handleConfig(widget, 'values', values)
}

async function handleSave() {
  if (!canEdit.value || saving.value || !board.value || !dashboard.value) return
  const id = boardId.value
  if (!id || board.value.id !== id) return
  const name = boardName.value.trim()
  if (!name || name.length > 255) {
    message.error($t('custom.nativeBoardEditor.nameInvalid'))
    return
  }
  if (boardDescription.value.length > 500) {
    message.error($t('custom.nativeBoardEditor.descriptionInvalid'))
    return
  }
  const serialized = serializeEditorDashboard(dashboard.value)
  if (!serialized.ok) {
    message.error(serialized.error)
    return
  }

  saving.value = true
  try {
    const result = await providerFacade.execute(provider =>
      provider.updateDashboard(id, {
        name,
        description: boardDescription.value,
        rendererData: serialized.dashboard
      })
    )
    if (!result.ok || result.data.id !== id) {
      message.error($t('custom.nativeBoardEditor.saveFailed'))
      return
    }
    message.success($t('custom.nativeBoardEditor.saveSuccess'))
    routerPushByKey('visualization_native-board', { query: { id } })
  } catch {
    message.error($t('custom.nativeBoardEditor.saveFailed'))
  } finally {
    saving.value = false
  }
}

watch([boardId, canEdit], () => void loadBoard(), { immediate: true })
onBeforeUnmount(() => {
  requestSequence += 1
})
</script>

<template>
  <div class="h-full">
    <NSpin :show="loading">
      <div v-if="dashboard && board" class="grid gap-4 xl:grid-cols-2">
        <NCard :title="$t('custom.nativeBoardEditor.title')">
          <div class="mb-4 grid gap-3">
            <NInput
              v-model:value="boardName"
              :maxlength="255"
              show-count
              :placeholder="$t('custom.nativeBoardEditor.name')"
              data-testid="board-name"
            />
            <NInput
              v-model:value="boardDescription"
              type="textarea"
              :maxlength="500"
              show-count
              :placeholder="$t('custom.nativeBoardEditor.description')"
              data-testid="board-description"
            />
          </div>
          <div class="mb-4 flex flex-wrap items-center gap-2">
            <NSelect v-model:value="selectedType" :options="WIDGET_TYPES" class="w-48" data-testid="widget-type" />
            <NButton type="primary" data-testid="add-widget" @click="handleAddWidget">
              {{ $t('custom.nativeBoardEditor.addWidget') }}
            </NButton>
            <NButton type="success" :loading="saving" data-testid="save-board" @click="handleSave">
              {{ $t('custom.nativeBoardEditor.save') }}
            </NButton>
          </div>

          <NCard v-for="widget in dashboard.widgets" :key="widget.id" size="small" class="mb-3" data-testid="widget-editor">
            <div class="mb-3 flex items-center justify-between">
              <strong>{{ widget.type }} · {{ widget.id }}</strong>
              <NButton type="error" size="small" @click="handleRemoveWidget(widget.id)">
                {{ $t('custom.nativeBoardEditor.remove') }}
              </NButton>
            </div>
            <div class="mb-3 grid grid-cols-4 gap-2">
              <NInputNumber :value="widget.x" :min="0" :max="dashboard.columns - 1" @update:value="handleLayout(widget, 'x', $event)" />
              <NInputNumber :value="widget.y" :min="0" :max="199" @update:value="handleLayout(widget, 'y', $event)" />
              <NInputNumber :value="widget.w" :min="1" :max="dashboard.columns" @update:value="handleLayout(widget, 'w', $event)" />
              <NInputNumber :value="widget.h" :min="1" :max="200" @update:value="handleLayout(widget, 'h', $event)" />
            </div>

            <template v-if="widget.type === 'text'">
              <NInput :value="(widget.config as TextWidgetConfig).text" placeholder="Text" @update:value="handleConfig(widget, 'text', $event)" />
              <NInput :value="(widget.config as TextWidgetConfig).field" class="mt-2" placeholder="Safe field name (optional)" @update:value="handleConfig(widget, 'field', $event || undefined)" />
              <NInput :value="(widget.config as TextWidgetConfig).fallback" class="mt-2" placeholder="Fallback (optional)" @update:value="handleConfig(widget, 'fallback', $event || undefined)" />
            </template>
            <template v-else-if="widget.type === 'metric'">
              <NInput :value="(widget.config as MetricWidgetConfig).label" placeholder="Label" @update:value="handleConfig(widget, 'label', $event)" />
              <NInput :value="(widget.config as MetricWidgetConfig).field" class="mt-2" placeholder="Safe field name" @update:value="handleConfig(widget, 'field', $event)" />
              <NInput :value="(widget.config as MetricWidgetConfig).unit" class="mt-2" placeholder="Unit (optional)" @update:value="handleConfig(widget, 'unit', $event || undefined)" />
              <NInputNumber :value="(widget.config as MetricWidgetConfig).decimals" class="mt-2" :min="0" :max="6" placeholder="Decimals" @update:value="handleConfig(widget, 'decimals', $event === null ? undefined : $event)" />
            </template>
            <template v-else>
              <NInput :value="(widget.config as ChartWidgetConfig).title" placeholder="Chart title" @update:value="handleConfig(widget, 'title', $event || undefined)" />
              <NInput :value="(widget.config as ChartWidgetConfig).categories?.join(', ')" class="mt-2" placeholder="Categories, comma separated" @update:value="handleChartCategories(widget, $event)" />
              <NInput :value="(widget.config as ChartWidgetConfig).values?.join(', ')" class="mt-2" placeholder="Numeric values, comma separated" @update:value="handleChartValues(widget, $event)" />
              <NInput :value="(widget.config as ChartWidgetConfig).seriesName" class="mt-2" placeholder="Series name (optional)" @update:value="handleConfig(widget, 'seriesName', $event || undefined)" />
            </template>
          </NCard>
        </NCard>

        <NCard :title="$t('custom.nativeBoardEditor.preview')">
          <LocalVisualizationViewer :dashboard="dashboard" :fields="{}" data-testid="native-board-preview" />
        </NCard>
      </div>
      <div v-else class="flex min-h-80 items-center justify-center text-gray-400" role="status">
        {{ loading ? $t('custom.nativeBoardEditor.loading') : failed ? $t('custom.nativeBoardEditor.loadFailed') : '' }}
      </div>
    </NSpin>
  </div>
</template>
