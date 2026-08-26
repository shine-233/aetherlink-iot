<!--
  文件用途: 配置导入导出视图组件。
  核心逻辑: 提供配置导出下拉、导入入口、处理状态和结果反馈。
  关键注意事项: 导入导出格式由工具层维护，组件不应复制持久化 schema 规则。
  重构建议: 将 UI 状态和导入导出动作封装为组合函数，便于复用和测试。
-->
<template>
  <n-space align="center">
    <!-- 导出按钮组 -->
    <n-dropdown :options="exportOptions" :disabled="isProcessing" @select="handleExportSelect">
      <n-button type="primary" size="small" :loading="isProcessing">
        <template #icon>
          <n-icon><CloudDownloadOutline /></n-icon>
        </template>
        {{ $t('common.export') }}
        <n-icon size="14"><ChevronDownOutline /></n-icon>
      </n-button>
    </n-dropdown>

    <!-- 导入按钮 -->
    <n-button type="info" size="small" :loading="isProcessing" @click="triggerFileInput">
      <template #icon>
        <n-icon><CloudUploadOutline /></n-icon>
      </template>
      {{ $t('common.import') }}
    </n-button>

    <!-- 隐藏的文件输入 -->
    <input ref="fileInputRef" type="file" accept=".json" style="display: none" @change="handleImportFileSelect" />

    <!-- 导入预览模态框 -->
    <n-modal
      v-model:show="showImportModal"
      preset="card"
      :title="$t('configuration.import.previewTitle')"
      size="large"
      :bordered="false"
      :segmented="false"
      style="width: 90%; max-width: 800px"
    >
      <div v-if="importPreview" class="import-preview">
        <n-alert type="info" :title="t('configuration.import.safetyGuideTitle')" style="margin-bottom: 16px">
          <ol class="import-safety-guide">
            <li>{{ t('configuration.import.safetyGuideCheckSource') }}</li>
            <li>{{ t('configuration.import.safetyGuideReviewStats') }}</li>
            <li>{{ t('configuration.import.safetyGuideResolveConflicts') }}</li>
          </ol>
        </n-alert>

        <!-- 基本信息 -->
        <n-card size="small" :title="$t('configuration.import.basicInfo')">
          <n-descriptions :column="2" size="small">
            <n-descriptions-item :label="$t('configuration.import.version')">
              {{ importPreview.basicInfo.version }}
            </n-descriptions-item>
            <n-descriptions-item :label="$t('configuration.import.exportTime')">
              {{ formatImportDateTime(importPreview.basicInfo.exportTime) }}
            </n-descriptions-item>
            <n-descriptions-item :label="$t('configuration.import.componentType')">
              {{ importPreview.basicInfo.componentType || $t('common.notSpecified') }}
            </n-descriptions-item>
            <n-descriptions-item :label="$t('configuration.import.source')">
              {{ importPreview.basicInfo.exportSource }}
            </n-descriptions-item>
          </n-descriptions>
        </n-card>

        <!-- 配置统计 -->
        <n-card size="small" :title="$t('configuration.import.statistics')">
          <n-space>
            <n-tag type="info">
              {{ $t('configuration.import.dataSourceCount') }}: {{ importPreview.statistics.dataSourceCount }}
            </n-tag>
            <n-tag type="success">
              {{ $t('configuration.import.interactionCount') }}: {{ importPreview.statistics.interactionCount }}
            </n-tag>
            <n-tag type="warning">
              {{ $t('configuration.import.httpConfigCount') }}: {{ importPreview.statistics.httpConfigCount }}
            </n-tag>
          </n-space>
        </n-card>

        <!-- 依赖分析 -->
        <n-card
          v-if="importPreview.dependencies && importPreview.dependencies.length > 0"
          size="small"
          :title="$t('configuration.import.dependencies')"
        >
          <n-space vertical size="small">
            <n-text depth="3">{{ $t('configuration.import.dependenciesHint') }}</n-text>
            <div class="dependency-list">
              <n-tag v-for="dep in importPreview.dependencies" :key="dep" type="info" size="small">
                {{ dep.substring(0, 8) }}...
              </n-tag>
            </div>
          </n-space>
        </n-card>

        <!-- 冲突检测 -->
        <n-alert
          v-if="importPreview.conflicts && importPreview.conflicts.length > 0"
          type="warning"
          :title="$t('configuration.import.conflictsFound')"
          style="margin: 16px 0"
        >
          <ul>
            <li v-for="conflict in importPreview.conflicts" :key="conflict">
              {{ conflict }}
            </li>
          </ul>
        </n-alert>

        <n-alert v-else type="success" :title="$t('configuration.import.noConflicts')" style="margin: 16px 0" />
      </div>

      <template #action>
        <n-space>
          <n-button @click="showImportModal = false">
            {{ $t('common.cancel') }}
          </n-button>
          <n-button
            type="primary"
            :loading="isProcessing"
            :disabled="(importPreview?.conflicts?.length ?? 0) > 0"
            @click="handleConfirmImport"
          >
            {{ $t('common.confirm') }}
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <!-- 单数据源导出选择模态框 -->
    <n-modal
      v-model:show="showSingleDataSourceModal"
      preset="card"
      :title="$t('configuration.export.selectDataSource')"
      size="medium"
      :bordered="false"
      :segmented="false"
      style="width: 90%; max-width: 600px"
    >
      <div v-if="availableDataSources.length > 0" class="datasource-selection">
        <n-text depth="3">{{ $t('configuration.export.selectDataSourceHint') }}</n-text>

        <div class="datasource-list" style="margin-top: 16px">
          <n-card
            v-for="source in availableDataSources"
            :key="source.sourceId"
            size="small"
            hoverable
            :class="['datasource-item', { 'has-data': source.hasData, 'empty-data': !source.hasData }]"
            style="margin-bottom: 8px; cursor: pointer"
            @click="() => handleSingleDataSourceExport(source.sourceId)"
          >
            <div class="datasource-info">
              <div class="datasource-header">
                <n-text strong>{{ source.sourceId }}</n-text>
                <n-tag :type="source.hasData ? 'success' : 'default'" size="small">
                  {{ source.hasData ? t('configuration.export.hasData') : t('configuration.export.noData') }}
                </n-tag>
              </div>
              <div class="datasource-details">
                <n-text depth="3">{{ t('configuration.export.dataItemCount') }}: {{ source.dataItemCount }}</n-text>
                <n-text depth="3">{{ t('configuration.export.position') }}: {{ source.sourceIndex + 1 }}</n-text>
              </div>
            </div>
          </n-card>
        </div>
      </div>

      <template #action>
        <n-space>
          <n-button @click="showSingleDataSourceModal = false">
            {{ $t('common.cancel') }}
          </n-button>
        </n-space>
      </template>
    </n-modal>

    <SingleDataSourceImportPreviewModal
      v-model:show="showSingleDataSourceImportModal"
      v-model:selected-target-slot="selectedTargetSlot"
      :preview="singleDataSourceImportPreview"
      :target-slot-options="targetSlotOptions"
      :is-processing="isProcessing"
      @confirm="handleSingleDataSourceImport"
    />
  </n-space>
</template>

<script setup lang="ts">
/**
 * 配置导入导出视图组件
 * 提供独立的配置导入导出UI功能
 */

import { ref, computed, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { useMessage } from 'naive-ui'
import {
  NSpace,
  NButton,
  NIcon,
  NModal,
  NCard,
  NDescriptions,
  NDescriptionsItem,
  NTag,
  NText,
  NAlert,
  NDropdown
} from 'naive-ui'
import { CloudDownloadOutline, CloudUploadOutline, ChevronDownOutline } from '@vicons/ionicons5'

import {
  configurationExporter,
  configurationImporter,
  singleDataSourceExporter,
  singleDataSourceImporter
} from '../../utils/ConfigurationImportExport'
import type {
  ImportPreview,
  SingleDataSourceImportPreview
} from '@/core/data-architecture/utils/ConfigurationImportExport'
import {
  buildConfigurationExportFileName,
  buildSingleDataSourceExportFileName,
  buildTargetSlotOptionsFromAvailableSources,
  buildTargetSlotOptionsFromPreviewSlots,
  downloadJsonFile,
  formatImportDateTime,
  isJsonFileName,
  isSingleDataSourceImportFile,
  readImportJsonFile,
  selectDefaultTargetSlot,
  type TargetSlotOption
} from './configurationImportExportViewHelpers'
import SingleDataSourceImportPreviewModal from './SingleDataSourceImportPreviewModal.vue'

// Props定义
interface Props {
  /** 当前配置数据 */
  configuration: Record<string, any>
  /** 组件ID */
  componentId: string
  /** 组件类型（可选） */
  componentType?: string
  /** 配置管理器实例 */
  configurationManager?: any
}

const props = withDefaults(defineProps<Props>(), {
  componentType: '',
  configurationManager: undefined
})

// Emits定义
const emit = defineEmits<{
  /** 导出成功事件 */
  exportSuccess: [data: any]
  /** 导入成功事件 */
  importSuccess: [data: any]
  /** 操作失败事件 */
  operationError: [error: Error]
}>()

const { t } = useI18n()
const message = useMessage()

// 响应式数据
const isProcessing = ref(false)
const showImportModal = ref(false)
const showSingleDataSourceModal = ref(false)
const showSingleDataSourceImportModal = ref(false)
const importFile = ref<File | null>(null)
const importPreview = ref<ImportPreview | null>(null)
const singleDataSourceImportPreview = ref<SingleDataSourceImportPreview | null>(null)
const fileInputRef = ref<HTMLInputElement>()
const availableDataSources = ref<
  Array<{
    sourceId: string
    sourceIndex: number
    hasData: boolean
    dataItemCount: number
  }>
>([])
const targetSlotOptions = ref<TargetSlotOption[]>([])
const selectedTargetSlot = ref<string>('')

const withProcessingState = async (task: () => Promise<void>): Promise<void> => {
  isProcessing.value = true
  try {
    await task()
  } finally {
    isProcessing.value = false
  }
}

const reportAsyncFlowError = (consoleLabel: string, messagePrefix: string, error: unknown): void => {
  const errorMessage = error instanceof Error ? error.message : String(error)
  console.error(consoleLabel, error)
  message.error(`${messagePrefix}: ${errorMessage}`)
  emit('operationError', error instanceof Error ? error : new Error(errorMessage))
}

const requireConfigurationManager = () => {
  if (!props.configurationManager) {
    throw new Error(t('configuration.export.noManagerError'))
  }

  return props.configurationManager
}

const openFullImportPreview = (preview: ImportPreview): void => {
  importPreview.value = preview
  showImportModal.value = true
}

const openSingleDataSourceImportPreview = async (preview: SingleDataSourceImportPreview): Promise<void> => {
  singleDataSourceImportPreview.value = preview
  await loadTargetSlotOptions()
  showSingleDataSourceImportModal.value = true
}

const finalizeFullImportSuccess = (importResult: unknown): void => {
  message.success(t('configuration.import.success'))
  emit('importSuccess', importResult)
  showImportModal.value = false
  importFile.value = null
  importPreview.value = null
}

const finalizeSingleDataSourceImportSuccess = (importData: unknown): void => {
  message.success(t('configuration.import.success'))
  emit('importSuccess', importData)
  showSingleDataSourceImportModal.value = false
  resetImportState()
}

// 导出选项
const exportOptions = computed(() => [
  {
    label: t('configuration.export.fullConfiguration'),
    key: 'full',
    icon: () => h(NIcon, null, { default: () => h(CloudDownloadOutline) })
  },
  {
    label: t('configuration.export.singleDataSource'),
    key: 'single',
    icon: () => h(NIcon, null, { default: () => h(CloudDownloadOutline) })
  }
])

/**
 * 处理配置导出
 */
const handleExportConfiguration = async (): Promise<void> => {
  if (isProcessing.value) return

  try {
    await withProcessingState(async () => {
      const configurationManager = requireConfigurationManager()

      if (process.env.NODE_ENV === 'development') {
        /* intentionally empty */
      }

      const exportResult = await configurationExporter.exportConfiguration(
        props.componentId,
        configurationManager,
        props.componentType
      )

      const fileName = buildConfigurationExportFileName(props.componentId)

      downloadJsonFile(exportResult, fileName)

      message.success(t('configuration.export.success'))
      emit('exportSuccess', exportResult)

      if (process.env.NODE_ENV === 'development') {
        /* intentionally empty */
      }
    })
  } catch (error) {
    reportAsyncFlowError('❌ [ConfigurationImportExportFlow] 配置导出失败:', t('configuration.export.error'), error)
  }
}

/**
 * 处理导出选择
 */
const handleExportSelect = (key: string): void => {
  if (key === 'full') {
    handleExportConfiguration()
  } else if (key === 'single') {
    handleShowSingleDataSourceExport()
  }
}

/**
 * 显示单数据源导出选择
 */
const handleShowSingleDataSourceExport = async (): Promise<void> => {
  if (!props.configurationManager) {
    message.error(t('configuration.export.noManagerError'))
    return
  }

  try {
    // 获取可用的数据源列表
    availableDataSources.value = singleDataSourceExporter.getAvailableDataSources(
      props.componentId,
      props.configurationManager
    )

    if (availableDataSources.value.length === 0) {
      message.warning(t('configuration.export.noDataSources'))
      return
    }

    showSingleDataSourceModal.value = true
  } catch (error) {
    console.error('❌ [ConfigurationImportExportFlow] 获取数据源列表失败:', error)
    message.error(t('configuration.export.getDataSourcesError'))
  }
}

/**
 * 处理单数据源导出
 */
const handleSingleDataSourceExport = async (sourceId: string): Promise<void> => {
  if (isProcessing.value) return

  try {
    isProcessing.value = true

    if (!props.configurationManager) {
      throw new Error(t('configuration.export.noManagerError'))
    }

    if (process.env.NODE_ENV === 'development') {
      /* intentionally empty */
    }

    // 执行单数据源导出
    const exportResult = await singleDataSourceExporter.exportSingleDataSource(
      props.componentId,
      sourceId,
      props.configurationManager,
      props.componentType
    )

    // 生成文件名
    const fileName = buildSingleDataSourceExportFileName(sourceId)

    // 下载文件
    downloadJsonFile(exportResult, fileName)

    message.success(t('configuration.export.success'))
    emit('exportSuccess', exportResult)

    if (process.env.NODE_ENV === 'development') {
      /* intentionally empty */
    }

    // 关闭模态框
    showSingleDataSourceModal.value = false
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error)
    console.error('❌ [ConfigurationImportExportFlow] 单数据源导出失败:', error)
    message.error(`${t('configuration.export.error')}: ${errorMessage}`)
    emit('operationError', error instanceof Error ? error : new Error(errorMessage))
  } finally {
    isProcessing.value = false
  }
}

/**
 * 触发文件选择
 */
const triggerFileInput = (): void => {
  fileInputRef.value?.click()
}

/**
 * 处理导入文件选择
 */
const handleImportFileSelect = (event: Event): void => {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]

  if (!file) return

  if (!isJsonFileName(file.name)) {
    message.error(t('configuration.import.invalidFileType'))
    return
  }

  importFile.value = file
  handlePreviewImport()
}

/**
 * 处理导入预览
 */
const handlePreviewImport = async (): Promise<void> => {
  if (!importFile.value) return

  try {
    await withProcessingState(async () => {
      const importData = await readImportJsonFile(importFile.value!, t('configuration.import.fileReadError'))

      if (process.env.NODE_ENV === 'development') {
        /* intentionally empty */
      }

      if (isSingleDataSourceImportFile(importData)) {
        await handleSingleDataSourceImportPreview(importData)
        return
      }

      await handleFullConfigurationImportPreview(importData)
    })
  } catch (error) {
    reportAsyncFlowError('❌ [ConfigurationImportExportFlow] 导入预览失败:', t('configuration.import.previewError'), error)
  }
}

/**
 * 处理完整配置导入预览
 */
const handleFullConfigurationImportPreview = async (importData: any): Promise<void> => {
  const preview = configurationImporter.generateImportPreview(
    importData,
    props.componentId,
    props.configurationManager || {},
    // 运行时契约：父组件传入的 configuration 在此场景承载可用组件列表（JSON 边界）。
    props.configuration as unknown as Array<Record<string, unknown>>
  )

  openFullImportPreview(preview)
}

/**
 * 处理单数据源导入预览
 */
const handleSingleDataSourceImportPreview = async (importData: any): Promise<void> => {
  const configurationManager = requireConfigurationManager()
  const preview = singleDataSourceImporter.generateImportPreview(importData, props.componentId, configurationManager)

  await openSingleDataSourceImportPreview(preview)
}

/**
 * 加载目标槽位选项
 */
const loadTargetSlotOptions = async (): Promise<void> => {
  if (!props.configurationManager) {
    return
  }

  try {
    if (singleDataSourceImportPreview.value?.availableSlots?.length) {
      targetSlotOptions.value = buildTargetSlotOptionsFromPreviewSlots(
        singleDataSourceImportPreview.value.availableSlots,
        t
      )
      selectedTargetSlot.value = selectDefaultTargetSlot(targetSlotOptions.value)
      return
    }

    // 获取当前组件的可用数据源槽位
    const availableSources = singleDataSourceExporter.getAvailableDataSources(
      props.componentId,
      props.configurationManager
    )

    targetSlotOptions.value = buildTargetSlotOptionsFromAvailableSources(availableSources, t)
    selectedTargetSlot.value = selectDefaultTargetSlot(targetSlotOptions.value)
  } catch (error) {
    console.error('❌ [ConfigurationImportExportFlow] 加载槽位选项失败:', error)
  }
}

/**
 * 执行单数据源导入
 */
const handleSingleDataSourceImport = async (): Promise<void> => {
  if (!importFile.value || !singleDataSourceImportPreview.value || !selectedTargetSlot.value) {
    return
  }

  try {
    await withProcessingState(async () => {
      const importData = await readImportJsonFile(importFile.value!, t('configuration.import.fileReadError'))

      if (process.env.NODE_ENV === 'development') {
        /* intentionally empty */
      }

      await singleDataSourceImporter.importSingleDataSource(
        importData,
        props.componentId,
        selectedTargetSlot.value,
        props.configurationManager
      )

      finalizeSingleDataSourceImportSuccess(importData)

      if (process.env.NODE_ENV === 'development') {
        /* intentionally empty */
      }
    })
  } catch (error) {
    reportAsyncFlowError('❌ [ConfigurationImportExportFlow] 单数据源导入失败:', t('configuration.import.error'), error)
  }
}

/**
 * 重置导入状态
 */
const resetImportState = (): void => {
  importFile.value = null
  importPreview.value = null
  singleDataSourceImportPreview.value = null
  selectedTargetSlot.value = ''
  targetSlotOptions.value = []
}

/**
 * 确认导入配置
 */
const handleConfirmImport = async (): Promise<void> => {
  if (!importFile.value || !importPreview.value) return

  try {
    await withProcessingState(async () => {
      const importData = await readImportJsonFile(importFile.value!, t('configuration.import.fileReadError'))

      if (process.env.NODE_ENV === 'development') {
        /* intentionally empty */
      }

      const importResult = configurationImporter.importConfiguration(
        importData,
        props.componentId,
        props.configurationManager
      )

      finalizeFullImportSuccess(importResult)

      if (process.env.NODE_ENV === 'development') {
        /* intentionally empty */
      }
    })
  } catch (error) {
    reportAsyncFlowError('❌ [ConfigurationImportExportFlow] 配置导入失败:', t('configuration.import.error'), error)
  }
}
</script>

<style scoped>
.import-preview {
  max-height: 500px;
  overflow-y: auto;
}

.import-preview > .n-card {
  margin-bottom: 16px;
}

.import-preview > .n-card:last-child {
  margin-bottom: 0;
}

.dependency-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.import-safety-guide {
  margin: 0;
  padding-left: 18px;
  line-height: 1.6;
}

/* 响应式设计 */
@media (max-width: 768px) {
  .import-preview {
    max-height: 400px;
  }
}
</style>
