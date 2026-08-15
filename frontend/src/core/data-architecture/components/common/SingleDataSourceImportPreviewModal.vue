<template>
  <n-modal
    :show="show"
    preset="dialog"
    :title="t('configuration.import.singleDataSourcePreview')"
    style="width: 600px"
    :show-icon="false"
    @update:show="emit('update:show', $event)"
  >
    <div v-if="preview">
      <n-space vertical>
        <n-alert type="info" :title="t('configuration.import.safetyGuideTitle')">
          <ol class="single-datasource-import-guide">
            <li>{{ t('configuration.import.safetyGuideCheckSource') }}</li>
            <li>{{ t('configuration.import.safetyGuideSelectSlot') }}</li>
            <li>{{ t('configuration.import.safetyGuideResolveConflicts') }}</li>
          </ol>
        </n-alert>

        <n-card :title="t('configuration.import.sourceInfo')" size="small">
          <n-descriptions :column="2" size="small">
            <n-descriptions-item :label="t('configuration.export.dataSource')">
              {{ preview.basicInfo.originalSourceId }}
            </n-descriptions-item>
            <n-descriptions-item :label="t('configuration.import.version')">
              {{ preview.basicInfo.version }}
            </n-descriptions-item>
            <n-descriptions-item :label="t('configuration.import.exportTime')">
              {{ new Date(preview.basicInfo.exportTime).toLocaleString() }}
            </n-descriptions-item>
            <n-descriptions-item :label="t('configuration.export.dataItems')">
              {{ preview.configSummary.dataItemCount }}
            </n-descriptions-item>
          </n-descriptions>
        </n-card>

        <n-card :title="t('configuration.import.basicInfo')" size="small">
          <n-descriptions :column="2" size="small">
            <n-descriptions-item :label="t('configuration.import.source')">
              {{ preview.basicInfo.exportSource }}
            </n-descriptions-item>
            <n-descriptions-item :label="t('configuration.export.mergeStrategy')">
              {{ preview.configSummary.mergeStrategy }}
            </n-descriptions-item>
            <n-descriptions-item :label="t('configuration.import.interactionCount')">
              {{ preview.relatedConfig.interactionCount }}
            </n-descriptions-item>
            <n-descriptions-item :label="t('configuration.import.httpConfigCount')">
              {{ preview.relatedConfig.httpBindingCount }}
            </n-descriptions-item>
          </n-descriptions>

          <div v-if="preview.dependencies.length > 0" style="margin-top: 12px">
            <n-text depth="2" style="font-size: 12px">{{ t('configuration.import.dependencies') }}</n-text>
            <n-space size="small" style="margin-top: 4px">
              <n-tag v-for="dep in preview.dependencies" :key="dep" type="warning" size="small">
                {{ dep }}
              </n-tag>
            </n-space>
          </div>

          <n-alert
            v-if="preview.conflicts.length > 0"
            type="warning"
            :title="t('configuration.import.conflictsFound')"
            style="margin-top: 12px"
          >
            <ul style="margin: 4px 0; padding-left: 20px">
              <li v-for="conflict in preview.conflicts" :key="conflict">
                {{ conflict }}
              </li>
            </ul>
          </n-alert>
        </n-card>

        <n-card :title="t('configuration.import.targetInfo')" size="small">
          <n-form-item :label="t('configuration.import.selectTargetSlot')">
            <n-select
              :value="selectedTargetSlot"
              :options="targetSlotOptions"
              :placeholder="t('configuration.import.selectTargetSlot')"
              @update:value="emit('update:selectedTargetSlot', $event)"
            >
              <template #option="{ option }">
                <div style="display: flex; justify-content: space-between; align-items: center">
                  <span>{{ option.label }}</span>
                  <n-tag :type="option.occupied ? 'warning' : 'success'" size="small">
                    {{ option.occupied ? t('configuration.import.slotOccupied') : t('configuration.import.slotEmpty') }}
                  </n-tag>
                </div>
              </template>
            </n-select>
          </n-form-item>

          <n-alert
            v-if="selectedTargetSlot && targetSlotOptions.find((slot) => slot.value === selectedTargetSlot)?.occupied"
            type="warning"
            :title="t('configuration.import.slotOccupied')"
            style="margin-top: 8px"
          >
            {{ t('configuration.import.slotOverwriteWarning') }}
          </n-alert>
        </n-card>
      </n-space>
    </div>

    <template #action>
      <n-space>
        <n-button @click="emit('update:show', false)">
          {{ t('common.cancel') }}
        </n-button>
        <n-button
          type="primary"
          :disabled="!selectedTargetSlot || isProcessing || (preview?.conflicts.length || 0) > 0"
          :loading="isProcessing"
          @click="emit('confirm')"
        >
          {{ t('configuration.import.importToSlot') }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { SingleDataSourceImportPreview } from '@/core/data-architecture/utils/ConfigurationImportExport'
import type { TargetSlotOption } from './configurationImportExportViewHelpers'

defineProps<{
  show: boolean
  preview: SingleDataSourceImportPreview | null
  selectedTargetSlot: string
  targetSlotOptions: TargetSlotOption[]
  isProcessing: boolean
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
  'update:selectedTargetSlot': [value: string]
  confirm: []
}>()

const { t } = useI18n()
</script>

<style scoped>
.single-datasource-import-guide {
  margin: 0;
  padding-left: 18px;
  line-height: 1.6;
}
</style>
