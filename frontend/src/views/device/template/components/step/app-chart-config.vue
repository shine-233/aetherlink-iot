<!--
文件用途: App 图表配置步骤。
核心逻辑: 展示移动端图表预览，并打开 ThingsVis 编辑器维护图表配置。
关键注意事项: 图表配置会写回模板数据，编辑器入口和模板 ID 必须保持一致。
重构建议: 将 App/Web 图表配置的公共编辑器调用逻辑抽成共享组件。
-->
<script setup lang="tsx">
/**
 * App chart configuration step.
 * Shows a preview and opens the ThingsVis editor modal.
 */

import { ref, computed, onMounted, watch, defineAsyncComponent } from 'vue'
import { NButton, NModal, NCard, NEmpty, NSelect, NSpace, NSpin, NIcon } from 'naive-ui'
import { ExpandOutline, ContractOutline, CloseOutline } from '@vicons/ionicons5'
import { $t } from '@/locales'
import { getTemplat, putTemplat, telemetryApi, attributesApi } from '@/service/api'
import { smartDeepClone } from '@/utils/deep-clone'
import { extractPlatformFields, mergePlatformFieldsById } from '@/utils/thingsvis/platform-fields'
import type { PlatformField } from '@/utils/thingsvis/types'

const ThingsVisWidget = defineAsyncComponent({
  loader: () => import('@/components/thingsvis/ThingsVisWidget.vue'),
  suspensible: false
})

type ThingsVisWidgetExposed = {
  triggerSave: () => void
}

const emit = defineEmits(['update:stepCurrent', 'update:modalVisible'])

const props = defineProps({
  stepCurrent: {
    type: Number,
    required: true
  },
  modalVisible: {
    type: Boolean,
    required: true
  },
  deviceTemplateId: {
    type: String,
    required: true
  }
})

// Editor reference
const editorRef = ref<ThingsVisWidgetExposed | null>(null)

// State
// Wrap current template fields as a virtual device entry for the Field Picker.
const platformDevices = computed(() => {
  if (!platformFields.value.length) return []
  return [
    {
      deviceId: '__template__',
      deviceName: $t('device_template.currentThingModel'),
      groupId: '__template__',
      groupName: $t('device_template.thingModelFields'),
      fields: platformFields.value,
      presets: []
    }
  ]
})

const loading = ref(true)
const saving = ref(false)
const showEditorModal = ref(false)
const isEditorFullscreen = ref(false)
const initialConfig = ref<any>(null)
const platformFields = ref<PlatformField[]>([])
const hasConfig = ref(false)
const refreshInterval = ref(5000)

const unwrapApiList = (payload: unknown): any[] => {
  const data = payload as { data?: unknown }
  const body = data?.data as { list?: unknown } | unknown
  if (body && typeof body === 'object' && !Array.isArray(body) && Array.isArray((body as { list?: unknown }).list)) {
    return (body as { list: any[] }).list
  }
  return Array.isArray(body) ? body : []
}

const refreshOptions = [
  { label: $t('device_template.manualRefresh'), value: 0 },
  { label: $t('device_template.refreshEvery5Seconds'), value: 5000 },
  { label: $t('device_template.refreshEvery10Seconds'), value: 10000 },
  { label: $t('device_template.refreshEvery30Seconds'), value: 30000 },
  { label: $t('device_template.refreshEvery1Minute'), value: 60000 }
]

// Cancel
const cancellation: () => void = () => {
  emit('update:modalVisible')
}

// Previous step
const back: () => void = () => {
  emit('update:stepCurrent', 3)
}

// Open editor
const openEditor = () => {
  isEditorFullscreen.value = false
  showEditorModal.value = true
}

const toggleEditorFullscreen = () => {
  isEditorFullscreen.value = !isEditorFullscreen.value
}

const editorCardStyle = computed(() => ({
  width: isEditorFullscreen.value ? '100vw' : 'min(94vw, 1800px)',
  height: isEditorFullscreen.value ? '100vh' : 'min(92vh, 1120px)'
}))

const editorWidgetHeight = computed(() =>
  isEditorFullscreen.value ? 'calc(100vh - 170px)' : 'calc(min(92vh, 1120px) - 170px)'
)

const previewHeight = computed(() => 'min(68vh, 720px)')

// Next step.
const next = () => {
  emit('update:stepCurrent', 5)
}

// Save chart configuration.
const handleSave = async (payload: any) => {
  if (saving.value) return

  saving.value = true
  try {
    // Fetch current template data.
    const res = await getTemplat(props.deviceTemplateId)

    // Remove virtual device IDs from PLATFORM_FIELD data sources.
    // Runtime rendering injects the real device ID.
    const cleanedPayload = smartDeepClone(payload)
    if (cleanedPayload.dataSources && Array.isArray(cleanedPayload.dataSources)) {
      cleanedPayload.dataSources.forEach((ds: any) => {
        if (ds.type === 'PLATFORM_FIELD' && ds.config) {
          delete ds.config.deviceId
        }
      })
    }

    // Save only to app_chart_config and persist the refresh interval.
    const configToSave = {
      ...cleanedPayload,
      refreshInterval: refreshInterval.value
    }

    await putTemplat({
      ...res.data,
      app_chart_config: JSON.stringify(configToSave)
    })

    window.$message?.success($t('common.saveSuccess'))

    // Update state.
    initialConfig.value = configToSave
    hasConfig.value = true

    // Close modal.
    showEditorModal.value = false
  } catch (error) {
    console.error('Failed to save app chart configuration:', error)
    window.$message?.error($t('common.saveFailed'))
  } finally {
    saving.value = false
  }
}

// Load template data.
const loadTemplateData = async () => {
  loading.value = true
  try {
    const res = await getTemplat(props.deviceTemplateId)

    if (res.data) {
      // Extract platform fields, preferring thing model APIs.
      const [telemetryRes, attributesRes] = await Promise.all([
        telemetryApi({ page: 1, page_size: 1000, device_template_id: props.deviceTemplateId }),
        attributesApi({ page: 1, page_size: 1000, device_template_id: props.deviceTemplateId })
      ])

      const telemetryList = unwrapApiList(telemetryRes)
      const attributesList = unwrapApiList(attributesRes)

      const platformSource = {
        telemetry: telemetryList,
        attributes: attributesList
      }

      const extractedFields = extractPlatformFields(platformSource)
      const templateFallbackFields = extractPlatformFields(res.data)
      // Filter out command-type fields — only telemetry and attributes are relevant for charts.
      const filtered = mergePlatformFieldsById(extractedFields, templateFallbackFields).filter(
        (f: PlatformField) => f.dataType !== 'command'
      )
      platformFields.value = filtered

      // Load existing configuration.
      if (res.data.app_chart_config) {
        try {
          const config = JSON.parse(res.data.app_chart_config)
          initialConfig.value = config
          hasConfig.value = true
          // Restore refresh interval configuration.
          if (config.refreshInterval !== undefined) {
            refreshInterval.value = config.refreshInterval
          }
        } catch (e) {
          console.warn('Failed to parse app_chart_config', e)
          initialConfig.value = null
          hasConfig.value = false
        }
      }
    }
  } catch (error) {
    console.error('Failed to load template data:', error)
    window.$message?.error($t('common.fetchDataFailed'))
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadTemplateData()
})

watch(showEditorModal, (visible) => {
  if (!visible) {
    isEditorFullscreen.value = false
  }
})
</script>

<template>
  <div class="step-app-chart">
    <!-- Preview area -->
    <NCard :title="$t('device_template.appChartConfiguration')" class="preview-card">
      <template #header-extra>
        <NSpace align="center">
          <span>{{ $t('device_template.refreshInterval') }}</span>

          <NSelect
            v-model:value="refreshInterval"
            :options="refreshOptions"
            size="small"
            style="width: 120px"
            :placeholder="$t('device_template.refreshInterval')"
          />
          <NButton type="primary" size="small" @click="openEditor">
            {{ hasConfig ? $t('device_template.editConfig') : $t('device_template.createConfig') }}
          </NButton>
        </NSpace>
      </template>

      <NSpin :show="loading" :description="$t('device_template.loading')">
        <!-- Preview existing configuration -->
        <div v-if="hasConfig && initialConfig" class="preview-area">
          <ThingsVisWidget
            mode="viewer"
            :config="initialConfig"
            :platform-fields="platformFields"
            :platform-devices="platformDevices"
            device-id="__template__"
            :height="previewHeight"
          />
        </div>

        <!-- Empty state -->
        <NEmpty v-else-if="!loading" :description="$t('device_template.emptyChartConfig')" />
        <div v-else style="min-height: 200px" />
      </NSpin>
    </NCard>

    <!-- Step actions -->
    <div class="actions-bar">
      <NButton type="primary" @click="next">
        {{ $t('device_template.nextStep') }}
      </NButton>
      <NButton class="m-r3" ghost type="primary" @click="back">
        {{ $t('device_template.back') }}
      </NButton>
      <NButton class="m-r3" @click="cancellation">
        {{ $t('generate.cancel') }}
      </NButton>
    </div>

    <!-- Editor modal -->
    <NModal v-model:show="showEditorModal" :mask-closable="false">
      <div class="chart-editor-shell" :class="{ 'chart-editor-shell--fullscreen': isEditorFullscreen }">
        <NCard
          :title="$t('device_template.editAppChartConfiguration')"
          :bordered="false"
          :class="['chart-editor-card', { 'chart-editor-card--fullscreen': isEditorFullscreen }]"
          :style="editorCardStyle"
        >
          <template #header-extra>
            <NSpace align="center" size="small">
              <NButton quaternary circle @click="toggleEditorFullscreen">
                <template #icon>
                  <NIcon>
                    <ContractOutline v-if="isEditorFullscreen" />
                    <ExpandOutline v-else />
                  </NIcon>
                </template>
              </NButton>
              <NButton quaternary circle @click="showEditorModal = false">
                <template #icon>
                  <NIcon>
                    <CloseOutline />
                  </NIcon>
                </template>
              </NButton>
            </NSpace>
          </template>

          <div class="editor-modal-content">
            <ThingsVisWidget
              ref="editorRef"
              mode="editor"
              :config="initialConfig"
              :platform-fields="platformFields"
              :platform-devices="platformDevices"
              device-id="__template__"
              :height="editorWidgetHeight"
              @save="handleSave"
            />
          </div>

          <template #footer>
            <div class="modal-footer">
              <NButton @click="showEditorModal = false">{{ $t('generate.cancel') }}</NButton>
              <NButton type="primary" :loading="saving" @click="editorRef?.triggerSave()">
                {{ $t('device_template.saveConfig') }}
              </NButton>
            </div>
          </template>
        </NCard>
      </div>
    </NModal>
  </div>
</template>

<style lang="scss" scoped>
.step-app-chart {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.preview-card {
  min-height: 400px;
}

.preview-area {
  width: 100%;
  min-height: min(68vh, 720px);
  border: 1px solid #e0e0e0;
  border-radius: 4px;
  overflow: hidden;
}

.actions-bar {
  display: flex;
  flex-direction: row-reverse;
  gap: 12px;
  margin-top: 16px;
}

.editor-modal-content {
  width: 100%;
  flex: 1 1 auto;
  min-height: 0;
  overflow: hidden;
  display: flex;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

:deep(.chart-editor-shell) {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  box-sizing: border-box;
}

:deep(.chart-editor-shell--fullscreen) {
  padding: 0;
}

:deep(.chart-editor-card) {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: 16px;
}

:deep(.chart-editor-card .n-card-header) {
  flex: 0 0 auto;
}

:deep(.chart-editor-card .n-card__content) {
  flex: 1 1 auto;
  min-height: 0;
  padding-top: 12px;
  display: flex;
  overflow: hidden;
}

:deep(.chart-editor-card .n-card__footer) {
  flex: 0 0 auto;
}

:deep(.chart-editor-card--fullscreen) {
  border-radius: 0;
}

:deep(.chart-editor-card--fullscreen .n-card__content) {
  padding-bottom: 12px;
}

:deep(.editor-modal-content .thingsvis-widget-container) {
  flex: 1 1 auto;
  min-height: 0;
}
</style>
