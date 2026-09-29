<!--
文件用途：告警消息管理页的告警历史列表（筛选、分诊、单条/批量处置、详情与证据导出）。
核心逻辑：查询/分页/实时刷新在 useAlarmHistoryQuery；单条与批量处置、证据导出分别在各自组合函数；
  分诊卡、空态引导、处置备注弹窗、详情弹窗拆为子组件，本文件只做编排。
关键注意事项：保持组件边界清晰，避免在子组件中绕过父页面的数据刷新与权限控制。
-->
<script setup lang="tsx">
import { computed, getCurrentInstance, ref } from 'vue'
import { NButton, NCard, NEmpty, NFlex, NInput, NTag } from 'naive-ui'
import dayjs from 'dayjs'
import { $t } from '@/locales'
import { deviceAlarmHistoryPut } from '@/service/api'
import type { FleetRolloutContext } from '../../../device/modules/fleet-rollout-context'
import {
  buildAlarmClosureEvidenceBundle,
  buildAlarmClosureEvidencePacket,
  buildAlarmClosureNextAction,
  buildAlarmEvidenceRow,
  buildAlarmResolutionTimeline,
  createAlarmStatusOptions,
  createAlarmTypeOptions,
  isAcknowledged,
  isReset
} from './alarm-configuration.helpers'
import { createAlarmConfigurationColumns, type AlarmConfigurationRow } from './alarmConfigurationColumns'
import AlarmBatchEvidenceCard from './AlarmBatchEvidenceCard.vue'
import AlarmActionNoteModal from './AlarmActionNoteModal.vue'
import AlarmDetailModal from './AlarmDetailModal.vue'
import AlarmEmptyGuide from './AlarmEmptyGuide.vue'
import AlarmTriagePanel from './AlarmTriagePanel.vue'
import { useAlarmBatchActions } from './useAlarmBatchActions'
import { useAlarmHistoryQuery } from './useAlarmHistoryQuery'
import { useAlarmSingleActions, type AlarmSingleActionRow } from './alarm-configuration.single-actions'
import { useAlarmClosureEvidenceExport } from './alarm-configuration.evidence-export'

const props = defineProps<{
  initialDeviceId?: string
  fleetContext?: FleetRolloutContext | null
}>()

const rowKey = (row: AlarmConfigurationRow) => row.id
const {
  loading,
  range,
  focusedDeviceId,
  hasRouteDeviceContext,
  fleetDeviceCount,
  queryData,
  tableData,
  selectedAlarmRowKeys,
  pagination,
  getAlarmHistory,
  handleSearch,
  resetData
} = useAlarmHistoryQuery(props)

const instance = getCurrentInstance()
const isMobilePlatform = computed(() => Boolean((instance?.proxy as any)?.getPlatform?.()))

const alarmStatusOptions = ref(createAlarmStatusOptions($t))
const alarmTypeOptions = ref(createAlarmTypeOptions($t))
const columns = createAlarmConfigurationColumns({
  getAlarmStatusOptions: () => alarmStatusOptions.value,
  onShowDetails: (row) => getInfo(row),
  onAcknowledge: (row) => acknowledgeAlarm(row),
  onReset: (row) => resetAlarm(row),
  onClear: (row) => clearAlarm(row),
  onMaintenance: (row) => maintenance(row)
})
const {
  selectedUnacknowledgedRows,
  selectedActiveRows,
  batchActionLoading,
  batchActionDialogVisible,
  batchActionNote,
  batchActionNoteMaxLength,
  batchActionDialogTitle,
  batchActionDialogHint,
  lastBatchActionEvidence,
  closeBatchActionDialog,
  runBatchAlarmAction,
  acknowledgeCurrentPage,
  resetCurrentPage
} = useAlarmBatchActions({ tableData, selectedAlarmRowKeys, refresh: getAlarmHistory })

const showAlarmEmptyGuide = computed(() => !loading.value && tableData.value.length === 0)
const lastBatchActionLabel = computed(() => {
  const action = lastBatchActionEvidence.value?.action
  if (action === 'acknowledge') return $t('custom.alarmPage.acknowledgeSelected')
  if (action === 'reset') return $t('custom.alarmPage.resetSelected')
  return action || '-'
})
const routeContextHint = computed(() =>
  fleetDeviceCount.value > 1
    ? $t('custom.alarmPage.fleetContextHint')
        .replace('{count}', String(fleetDeviceCount.value))
        .replace('{currentPage}', String(props.fleetContext?.currentPageCount ?? fleetDeviceCount.value))
        .replace('{total}', String(props.fleetContext?.requestedTotal ?? '--'))
    : $t('custom.alarmPage.singleDeviceContextHint')
)

const showDialog = ref(false)
const infoData = ref({} as any)
const formatAlarmTime = (value: unknown) => (value ? dayjs(value as any).format('YYYY-MM-DD HH:mm:ss') : '-')
const detailTimelineItems = computed(() => buildAlarmResolutionTimeline(infoData.value, $t, formatAlarmTime))
const detailClosureNextAction = computed(() => buildAlarmClosureNextAction(infoData.value, $t, formatAlarmTime))
const alarmClosureEvidencePacket = computed(() =>
  buildAlarmClosureEvidencePacket(infoData.value, detailTimelineItems.value, $t, formatAlarmTime)
)
const detailNeedsAcknowledge = computed(() => Boolean(infoData.value?.id && !isAcknowledged(infoData.value)))
const detailNeedsReset = computed(() => Boolean(infoData.value?.id && !isReset(infoData.value)))

function getInfo(data: any) {
  infoData.value = data
  showDialog.value = true
}
const closeModal = () => {
  showDialog.value = false
}

const alarmEvidenceBoundary = () => $t('custom.alarmPage.evidenceBundleBoundary')

const {
  singleActionDialogVisible,
  singleActionLoading,
  singleActionNote,
  singleActionNoteMaxLength,
  singleActionDialogTitle,
  singleActionDialogHint,
  lastSingleClosureEvidence,
  alarmAuditSummary,
  closeSingleActionDialog,
  openSingleAlarmAction,
  runSingleAlarmAction
} = useAlarmSingleActions({
  severityOptions: alarmStatusOptions,
  evidenceRowOf: (row) =>
    buildAlarmEvidenceRow({ row, severityOptions: alarmStatusOptions.value, t: $t, formatTime: formatAlarmTime }),
  evidenceBoundaryLabel: alarmEvidenceBoundary,
  closeDetailDialog: closeModal,
  refresh: getAlarmHistory
})

const acknowledgeAlarm = (row: any) => openSingleAlarmAction(row as AlarmSingleActionRow, 'acknowledge')
const resetAlarm = (row: any) => openSingleAlarmAction(row as AlarmSingleActionRow, 'reset')
const clearAlarm = (row: any) => openSingleAlarmAction(row as AlarmSingleActionRow, 'clear')

const { downloadAlarmClosureEvidenceBundle, copyAlarmClosureEvidence, copyLastBatchActionEvidence } =
  useAlarmClosureEvidenceExport({
    buildBundle: () =>
      buildAlarmClosureEvidenceBundle({
        tableData: tableData.value,
        queryData: queryData.value,
        pagination,
        selectedRowKeys: selectedAlarmRowKeys.value,
        infoData: infoData.value,
        detailClosureNextAction: detailClosureNextAction.value,
        detailTimelineItems: detailTimelineItems.value,
        alarmClosureEvidencePacket: alarmClosureEvidencePacket.value,
        lastSingleClosureEvidence: lastSingleClosureEvidence.value,
        lastBatchActionEvidence: lastBatchActionEvidence.value,
        focusedDeviceId: focusedDeviceId.value,
        hasRouteDeviceContext: hasRouteDeviceContext.value,
        fleetDeviceCount: fleetDeviceCount.value,
        currentFleetPageCount: props.fleetContext?.currentPageCount || 0,
        requestedFleetTotal: props.fleetContext?.requestedTotal || 0,
        boundary: alarmEvidenceBoundary(),
        severityOptions: alarmStatusOptions.value,
        t: $t,
        formatTime: formatAlarmTime
      }),
    resolvePrimaryAlarmId: () => infoData.value?.id || lastSingleClosureEvidence.value?.alarmId,
    closurePacketText: () => alarmClosureEvidencePacket.value,
    batchCopyText: () => lastBatchActionEvidence.value?.copyText
  })

// 维护备注弹窗
const showModal = ref(false)
const description = ref('')
const maintenance = (row: any) => {
  infoData.value = row
  description.value = row.description
  showModal.value = true
}
const cancelCallback = () => {
  description.value = ''
  showModal.value = false
}
const submitCallback = async () => {
  if (description.value === '') {
    window.$message?.error($t('common.enterAlarmDesc'))
    return
  }
  await deviceAlarmHistoryPut({ id: infoData.value.id, description: description.value })
  cancelCallback()
  await getAlarmHistory()
}
</script>

<template>
  <div class="h-full flex-col">
    <NAlert v-if="hasRouteDeviceContext" type="info" :show-icon="false" class="mb-12px">
      <div class="alarm-route-context">
        <div>{{ routeContextHint }}</div>
        <NFlex :size="8" align="center" wrap>
          <NTag size="small" type="info">{{ $t('custom.alarmPage.focusDevice') }}: {{ focusedDeviceId || '-' }}</NTag>
          <NInput
            v-if="fleetDeviceCount > 1"
            v-model:value="focusedDeviceId"
            size="small"
            class="max-w-320px"
            :placeholder="$t('custom.alarmPage.focusDevicePlaceholder')"
          />
          <NButton v-if="fleetDeviceCount > 1" size="small" secondary @click="handleSearch">
            {{ $t('custom.alarmPage.applyFocusDevice') }}
          </NButton>
        </NFlex>
      </div>
    </NAlert>
    <NForm class="alarm-query-form" :inline="!isMobilePlatform" label-placement="left" :model="queryData">
      <NFormItem path="range">
        <n-date-picker v-model:value="range" type="datetimerange" :clearable="false" separator="-" />
      </NFormItem>
      <NFormItem :label="$t('generate.alarm-level')" path="alarm_status">
        <NSelect
          v-model:value="queryData.alarm_status"
          :clearable="false"
          class="w-200px"
          :options="alarmStatusOptions"
        />
      </NFormItem>
      <NFormItem :label="$t('rdi.overview.alarmType')" path="alarm_type">
        <NSelect v-model:value="queryData.alarm_type" :clearable="false" class="w-200px" :options="alarmTypeOptions" />
      </NFormItem>
      <NFormItem>
        <NButton type="primary" @click="handleSearch">{{ $t('common.search') }}</NButton>
        <NButton class="ml-12px" @click="resetData">{{ $t('common.reset') }}</NButton>
      </NFormItem>
    </NForm>
    <AlarmTriagePanel
      :rows="tableData"
      :total="pagination.itemCount || 0"
      :batch-loading="batchActionLoading"
      :can-acknowledge="selectedUnacknowledgedRows.length > 0"
      :can-reset="selectedActiveRows.length > 0"
      @download="downloadAlarmClosureEvidenceBundle"
      @acknowledge="acknowledgeCurrentPage"
      @reset="resetCurrentPage"
    />
    <AlarmBatchEvidenceCard
      v-if="lastBatchActionEvidence"
      :evidence="lastBatchActionEvidence"
      :action-label="lastBatchActionLabel"
      @copy="copyLastBatchActionEvidence"
      @download="downloadAlarmClosureEvidenceBundle"
    />
    <AlarmEmptyGuide v-if="showAlarmEmptyGuide" @reset="resetData" />
    <div class="w-100% flex-1-hidden alarm-table-scroll">
      <n-data-table
        v-model:checked-row-keys="selectedAlarmRowKeys"
        remote
        :loading="loading"
        :columns="columns"
        :data="tableData"
        :pagination="pagination"
        :row-key="rowKey"
        class="w-100%"
      >
        <template #empty>
          <NEmpty :description="$t('common.noData')" class="py-24px" />
        </template>
      </n-data-table>
    </div>
    <AlarmActionNoteModal
      v-model:show="batchActionDialogVisible"
      v-model:note="batchActionNote"
      :title="batchActionDialogTitle"
      :hint="batchActionDialogHint"
      :loading="batchActionLoading"
      :max-length="batchActionNoteMaxLength"
      :placeholder="$t('custom.alarmPage.batchActionNotePlaceholder')"
      @cancel="closeBatchActionDialog"
      @confirm="runBatchAlarmAction"
    />
    <AlarmActionNoteModal
      v-model:show="singleActionDialogVisible"
      v-model:note="singleActionNote"
      :title="singleActionDialogTitle"
      :hint="singleActionDialogHint"
      :loading="singleActionLoading"
      :max-length="singleActionNoteMaxLength"
      :placeholder="$t('custom.alarmPage.singleActionNotePlaceholder')"
      @cancel="closeSingleActionDialog"
      @confirm="runSingleAlarmAction"
    />
    <AlarmDetailModal
      v-model:show="showDialog"
      :alarm="infoData"
      :status-options="alarmStatusOptions"
      :next-action="detailClosureNextAction"
      :timeline-items="detailTimelineItems"
      :needs-acknowledge="detailNeedsAcknowledge"
      :needs-reset="detailNeedsReset"
      @acknowledge="acknowledgeAlarm(infoData)"
      @reset="resetAlarm(infoData)"
      @maintenance="maintenance(infoData)"
      @copy-evidence="copyAlarmClosureEvidence"
      @download-evidence="downloadAlarmClosureEvidenceBundle"
    />
    <n-modal v-model:show="showModal" class="max-w-[600px]">
      <NCard class="alarm-action-modal-card">
        <NCard embedded size="small" class="mb-4 whitespace-pre-line">
          {{ alarmAuditSummary(infoData) }}
        </NCard>
        <n-form-item :show-feedback="false" :label="$t('rdi.overview.maintenanceNote')">
          <NInput v-model:value="description" type="textarea" />
        </n-form-item>
        <NFlex justify="flex-end" class="mt-4">
          <NButton @click="cancelCallback">{{ $t('generate.cancel') }}</NButton>
          <NButton @click="submitCallback">{{ $t('common.save') }}</NButton>
        </NFlex>
      </NCard>
    </n-modal>
  </div>
</template>

<style scoped lang="scss">
.alarm-route-context {
  display: grid;
  gap: 8px;
  line-height: 1.5;
}

/* alarmConfigurationColumns.tsx 渲染的闭环下一步单元格 */
:deep(.alarm-closure-next-action) {
  display: grid;
  gap: 5px;
  align-items: flex-start;
  max-width: 260px;
  line-height: 1.45;
}

:deep(.alarm-closure-next-action__step) {
  color: #334155;
  font-size: 12px;
  overflow-wrap: anywhere;
}

:deep(.alarm-closure-next-action__evidence) {
  color: #64748b;
  font-size: 12px;
  overflow-wrap: anywhere;
}

.alarm-action-modal-card {
  width: min(96vw, 600px);
}

/* ≤640px 现场手机查告警最小保障；断点统一取 _mixins.scss 的 mobile mixin（=--breakpoint-sm=640px）。 */
@include mobile {
  .alarm-query-form :deep(.n-form-item) {
    width: 100%;
    margin-right: 0;
  }

  .alarm-query-form :deep(.n-date-picker) {
    width: 100%;
  }

  .alarm-table-scroll {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;

    > .n-data-table {
      min-width: 720px;
    }
  }
}
</style>
