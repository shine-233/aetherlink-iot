<!--
文件用途: OTA 升级任务的创建弹窗（启动上下文 + 设备/筛选范围 + 预检 + 名称描述）。
核心逻辑: 表单与筛选选项全部由父级注入，弹窗只负责布局与把搜索/保存动作回抛。
关键注意事项: 车队筛选投放时设备下拉被筛选预览表替代，两条链路的必填项不同，不可混用。
-->
<script setup lang="ts">
import { $t } from '@/locales'
import OtaTaskFilterRolloutSummary from './OtaTaskFilterRolloutSummary.vue'
import OtaTaskLaunchContext from './OtaTaskLaunchContext.vue'
import OtaTaskPreflightCard from './OtaTaskPreflightCard.vue'

defineProps<{
  selectedPackage: any
  showNoEligibleDeviceAlert: boolean
  fleetPreselectionResult: any
  isFleetFilterScope: boolean
  isFleetFilterRollout: boolean
  filterPreviewResult: any
  savedFleetFiltersLoading: boolean
  savedFleetFilterLoadFailed: boolean
  savedFleetFilterOptions: any[]
  selectedSavedFleetFilter: any
  fleetFilterSummaryItems: any[]
  filterPreviewSubsetColumns: any[]
  filterPreviewSubsetRows: any[]
  deviceLoading: boolean
  deviceOptions: any[]
  preflight: any
  preflightItems: any[]
  riskDevices: any[]
  saving: boolean
  canSaveTask: boolean
  primaryActionLabel: string
}>()

const show = defineModel<boolean>('show', { required: true })
const taskForm = defineModel<any>('taskForm', { required: true })
const selectedSavedFleetFilterId = defineModel<string | null>('selectedSavedFleetFilterId', { required: true })

const emit = defineEmits<{ 'search-devices': [query: string]; save: [] }>()
</script>

<template>
  <NModal v-model:show="show" preset="card" class="task-modal" :title="$t('page.product.update-ota.updateTask')">
    <NForm label-placement="top">
      <OtaTaskLaunchContext
        :selected-package="selectedPackage"
        :show-no-eligible-device-alert="showNoEligibleDeviceAlert"
        :fleet-preselection-result="fleetPreselectionResult"
        :is-fleet-filter-scope="isFleetFilterScope"
        :is-fleet-filter-rollout="isFleetFilterRollout"
        :filter-preview-result="filterPreviewResult"
        :saved-fleet-filters-loading="savedFleetFiltersLoading"
        :saved-fleet-filter-load-failed="savedFleetFilterLoadFailed"
        :saved-fleet-filter-options="savedFleetFilterOptions"
        :selected-saved-fleet-filter-id="selectedSavedFleetFilterId"
        :selected-saved-fleet-filter="selectedSavedFleetFilter"
        @update:selected-saved-fleet-filter-id="selectedSavedFleetFilterId = $event"
      />
      <NFormItem :label="$t('page.product.update-ota.taskName')" required>
        <NInput v-model:value="taskForm.name" />
      </NFormItem>
      <OtaTaskFilterRolloutSummary
        v-if="isFleetFilterRollout"
        :selected-saved-fleet-filter="selectedSavedFleetFilter"
        :fleet-preselection-result="fleetPreselectionResult"
        :fleet-filter-summary-items="fleetFilterSummaryItems"
        :filter-preview-result="filterPreviewResult"
        :filter-preview-subset-columns="filterPreviewSubsetColumns"
        :filter-preview-subset-rows="filterPreviewSubsetRows"
      />
      <NFormItem v-else :label="$t('page.product.update-ota.selectDevice')" required>
        <NSelect
          v-model:value="taskForm.device_id_list"
          multiple
          filterable
          remote
          :loading="deviceLoading"
          :disabled="deviceLoading"
          :options="deviceOptions"
          :virtual-scroll="true"
          max-tag-count="responsive"
          :placeholder="deviceLoading ? $t('common.loading') : $t('page.product.update-ota.selectDevice')"
          @search="emit('search-devices', $event)"
        />
      </NFormItem>
      <OtaTaskPreflightCard class="mb-3" :summary="preflight" :items="preflightItems" :risk-devices="riskDevices" />
      <NFormItem :label="$t('page.product.update-ota.desc')">
        <NInput v-model:value="taskForm.description" type="textarea" :autosize="{ minRows: 2, maxRows: 4 }" />
      </NFormItem>
    </NForm>
    <template #footer>
      <NSpace justify="end">
        <NButton @click="show = false">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" :loading="saving" :disabled="!canSaveTask" @click="emit('save')">
          {{ primaryActionLabel }}
        </NButton>
      </NSpace>
    </template>
  </NModal>
</template>

<style scoped>
.task-modal {
  width: min(620px, calc(100vw - 32px));
}
</style>
