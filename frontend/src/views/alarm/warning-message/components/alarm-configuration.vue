<!--
文件用途：提供 告警消息管理 页面内的 alarm-configuration 子组件。
核心逻辑：封装局部表单、弹窗、列表或展示模块，通过 props、emit 与父页面协作。
关键注意事项：保持组件边界清晰，避免在子组件中绕过父页面的数据刷新与权限控制。
重构建议：后续可把重复表单规则、选项转换和弹窗状态管理抽成可复用组合函数。
-->
<script setup lang="tsx">
import { computed, getCurrentInstance, onMounted, reactive, ref, watch } from 'vue'
import { NButton, NCard, NFlex, NInput, NTag } from 'naive-ui'
import type { PaginationProps } from 'naive-ui'
import dayjs from 'dayjs'
import { useRouter } from 'vue-router'
import { alarmHistory } from '@/service/api/alarm'
import { $t } from '@/locales'
import { deviceAlarmHistoryPut } from '@/service/api'
import type { FleetRolloutContext } from '../../../device/modules/fleet-rollout-context'
import {
  alarmActionField,
  alarmSeverityLabel,
  alarmSeverityValue,
  alarmTypeLabel,
  buildAlarmClosureEvidenceBundle,
  buildAlarmClosureEvidenceFileName,
  buildAlarmClosureEvidencePacket,
  buildAlarmClosureNextAction,
  buildAlarmEvidenceRow,
  buildAlarmResolutionTimeline,
  buildAlarmTriageSummary,
  createAlarmStatusOptions,
  createAlarmTypeOptions,
  isAcknowledged,
  isReset
} from './alarm-configuration.helpers'
import {
  createAlarmConfigurationColumns,
  type AlarmConfigurationRow
} from './alarmConfigurationColumns'
import AlarmBatchEvidenceCard from './AlarmBatchEvidenceCard.vue'
import { useAlarmBatchActions } from './useAlarmBatchActions'
import {
  useAlarmSingleActions,
  type AlarmSingleActionRow
} from './alarm-configuration.single-actions'
import { useAlarmClosureEvidenceExport } from './alarm-configuration.evidence-export'

const props = defineProps<{
  initialDeviceId?: string
  fleetContext?: FleetRolloutContext | null
}>()

const router = useRouter()
const loading = ref(false)
const rowKey = (row: AlarmConfigurationRow) => row.id
let alarmHistoryRequestSeq = 0

const range = ref<[number, number]>([dayjs().subtract(1, 'month').valueOf(), dayjs().valueOf()])
const initialFocusedDeviceId = () => props.initialDeviceId || props.fleetContext?.deviceIds[0] || ''
const focusedDeviceId = ref(initialFocusedDeviceId())
const hasRouteDeviceContext = computed(() => Boolean(props.initialDeviceId || props.fleetContext?.deviceIds.length))
const fleetDeviceCount = computed(() => props.fleetContext?.deviceIds.length || 0)

const queryData = ref({
  alarm_status: '',
  alarm_type: '',
  device_id: focusedDeviceId.value,
  start_time: '',
  end_time: '',
  page: 1,
  page_size: 10
})
const tableData = ref<AlarmConfigurationRow[]>([])
const selectedAlarmRowKeys = ref<Array<string | number>>([])
const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 15, 20, 25, 30],
  onChange: (page: number) => {
    pagination.page = page
    selectedAlarmRowKeys.value = []
    // eslint-disable-next-line @typescript-eslint/no-use-before-define
    getAlarmHistory()
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
    selectedAlarmRowKeys.value = []
    // eslint-disable-next-line @typescript-eslint/no-use-before-define
    getAlarmHistory()
  }
})
const syncRangeQuery = () => {
  if (range.value && range.value.length > 0) {
    queryData.value.start_time = dayjs(range.value[0]).format('YYYY-MM-DDTHH:mm:ssZ')
    queryData.value.end_time = dayjs(range.value[1]).format('YYYY-MM-DDTHH:mm:ssZ')
  } else {
    queryData.value.start_time = ''
    queryData.value.end_time = ''
  }
}
const getAlarmHistory = async () => {
  const requestSeq = alarmHistoryRequestSeq + 1
  alarmHistoryRequestSeq = requestSeq
  loading.value = true
  syncRangeQuery()
  queryData.value.device_id = focusedDeviceId.value
  queryData.value.page = pagination.page as number
  queryData.value.page_size = pagination.pageSize as number
  const requestQuery = { ...queryData.value }
  try {
    const { data } = await alarmHistory(requestQuery)
    if (requestSeq !== alarmHistoryRequestSeq) return
    if (data) {
      pagination.itemCount = data.total
      tableData.value = data.list
    }
  } finally {
    if (requestSeq === alarmHistoryRequestSeq) {
      loading.value = false
    }
  }
}
const handleSearch = () => {
  pagination.page = 1
  selectedAlarmRowKeys.value = []
  getAlarmHistory()
}

const resetData = () => {
  range.value = [dayjs().subtract(1, 'month').valueOf(), dayjs().valueOf()]
  queryData.value.alarm_status = ''
  queryData.value.alarm_type = ''
  focusedDeviceId.value = initialFocusedDeviceId()
  handleSearch()
}

onMounted(() => {
  getAlarmHistory()
})

watch(
  () => [props.initialDeviceId, props.fleetContext?.deviceIds.join(',')],
  () => {
    focusedDeviceId.value = initialFocusedDeviceId()
    handleSearch()
  }
)

const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})

const alarmStatusOptions = ref(createAlarmStatusOptions($t))
const alarmTypeOptions = ref(createAlarmTypeOptions($t))
const columns = createAlarmConfigurationColumns({
  getAlarmStatusOptions: () => alarmStatusOptions.value,
  onShowDetails: row => getInfo(row),
  onAcknowledge: row => acknowledgeAlarm(row),
  onReset: row => resetAlarm(row),
  onMaintenance: row => maintenance(row)
})
const alarmTriageSummary = computed(() => buildAlarmTriageSummary(tableData.value))
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
} = useAlarmBatchActions({
  tableData,
  selectedAlarmRowKeys,
  refresh: getAlarmHistory
})
const alarmTriageCards = computed(() => [
  {
    key: 'active',
    label: $t('custom.alarmPage.activeAlarms'),
    value: alarmTriageSummary.value.active,
    type: alarmTriageSummary.value.active > 0 ? ('error' as const) : ('success' as const)
  },
  {
    key: 'high',
    label: $t('custom.alarmPage.highSeverity'),
    value: alarmTriageSummary.value.high,
    type: alarmTriageSummary.value.high > 0 ? ('error' as const) : ('default' as const)
  },
  {
    key: 'unacknowledged',
    label: $t('custom.alarmPage.unacknowledged'),
    value: alarmTriageSummary.value.unacknowledged,
    type: alarmTriageSummary.value.unacknowledged > 0 ? ('warning' as const) : ('success' as const)
  },
  {
    key: 'reset',
    label: $t('custom.alarmPage.resetAlarms'),
    value: alarmTriageSummary.value.reset,
    type: 'info' as const
  }
])
const showAlarmEmptyGuide = computed(() => !loading.value && tableData.value.length === 0)
const lastBatchActionLabel = computed(() => {
  if (lastBatchActionEvidence.value?.action === 'acknowledge') {
    return $t('custom.alarmPage.acknowledgeSelected')
  }
  if (lastBatchActionEvidence.value?.action === 'reset') {
    return $t('custom.alarmPage.resetSelected')
  }
  return lastBatchActionEvidence.value?.action || '-'
})
const alarmEmptyGuideItems = computed(() => [
  {
    key: 'rule',
    title: $t('custom.alarmPage.emptyGuideRuleTitle'),
    description: $t('custom.alarmPage.emptyGuideRuleDesc'),
    type: 'info' as const
  },
  {
    key: 'filter',
    title: $t('custom.alarmPage.emptyGuideFilterTitle'),
    description: $t('custom.alarmPage.emptyGuideFilterDesc'),
    type: 'warning' as const
  },
  {
    key: 'closure',
    title: $t('custom.alarmPage.emptyGuideClosureTitle'),
    description: $t('custom.alarmPage.emptyGuideClosureDesc'),
    type: 'success' as const
  }
])
const showDialog = ref(false)
const infoData = ref({} as any)
const formatAlarmTime = (value: unknown) => (value ? dayjs(value as any).format('YYYY-MM-DD HH:mm:ss') : '-')
const detailTimelineItems = computed(() => buildAlarmResolutionTimeline(infoData.value, $t, formatAlarmTime))
const detailClosureNextAction = computed(() => buildAlarmClosureNextAction(infoData.value, $t, formatAlarmTime))
const alarmClosureEvidencePacket = computed(() =>
  buildAlarmClosureEvidencePacket(infoData.value, detailTimelineItems.value, $t, formatAlarmTime)
)
const detailNeedsAcknowledge = computed(() => infoData.value?.id && !isAcknowledged(infoData.value))
const detailNeedsReset = computed(() => infoData.value?.id && !isReset(infoData.value))

function getInfo(data: any) {
  infoData.value = data
  showDialog.value = true
}
const closeModal = () => {
  showDialog.value = false
}

const alarmEvidenceBoundary = () => $t('custom.alarmPage.evidenceBundleBoundary')
const alarmEvidenceRow = (row: any) =>
  buildAlarmEvidenceRow({
    row,
    severityOptions: alarmStatusOptions.value,
    t: $t,
    formatTime: formatAlarmTime
  })

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
  evidenceRowOf: row => alarmEvidenceRow(row),
  evidenceBoundaryLabel: alarmEvidenceBoundary,
  closeDetailDialog: closeModal,
  refresh: getAlarmHistory
})

const acknowledgeAlarm = (row: any) => {
  openSingleAlarmAction(row as AlarmSingleActionRow, 'acknowledge')
}

const resetAlarm = (row: any) => {
  openSingleAlarmAction(row as AlarmSingleActionRow, 'reset')
}


const buildCurrentAlarmClosureEvidenceBundle = () =>
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
  })

const { downloadAlarmClosureEvidenceBundle, copyAlarmClosureEvidence, copyLastBatchActionEvidence } =
  useAlarmClosureEvidenceExport({
    buildBundle: buildCurrentAlarmClosureEvidenceBundle,
    resolvePrimaryAlarmId: () => infoData.value?.id || lastSingleClosureEvidence.value?.alarmId,
    closurePacketText: () => alarmClosureEvidencePacket.value,
    batchCopyText: () => lastBatchActionEvidence.value?.copyText
  })

const openAlarmAuditLog = () => {
  const createdAt = infoData.value?.create_at ? dayjs(infoData.value.create_at) : dayjs().subtract(1, 'day')
  const start = createdAt.isValid() ? createdAt.subtract(1, 'hour') : dayjs().subtract(1, 'day')
  const end = dayjs().add(1, 'hour')

  router.push({
    name: 'system-management-user_system-log',
    query: {
      method: 'PUT',
      path: '/api/v1/alarm/info/history',
      start_time: start.format('YYYY-MM-DDTHH:mm:ssZ'),
      end_time: end.format('YYYY-MM-DDTHH:mm:ssZ')
    }
  })
}

const alarmDeviceId = (device: any) => String(device?.id || device?.device_id || device?.deviceId || '').trim()

const openAlarmDeviceReadyCheck = (device: any) => {
  const deviceId = alarmDeviceId(device)
  if (!deviceId) return

  router.push({
    path: '/device/details',
    query: {
      d_id: deviceId,
      tab: 'ready-check',
      source: 'alarm',
      alarm_history_id: String(infoData.value?.id || '')
    }
  })
}

const showModal = ref(false)
const description = ref('')
const maintenance = (row) => {
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
  const putData = {
    id: infoData.value.id,
    description: description.value
  }
  await deviceAlarmHistoryPut(putData)
  cancelCallback()
  await getAlarmHistory()
}
</script>

<template>
  <div class="h-full flex-col">
    <NAlert v-if="hasRouteDeviceContext" type="info" :show-icon="false" class="mb-12px">
      <div class="alarm-route-context">
        <div>
          {{
            fleetDeviceCount > 1
              ? $t('custom.alarmPage.fleetContextHint')
                  .replace('{count}', String(fleetDeviceCount))
                  .replace('{currentPage}', String(props.fleetContext?.currentPageCount ?? fleetDeviceCount))
                  .replace('{total}', String(props.fleetContext?.requestedTotal ?? '--'))
              : $t('custom.alarmPage.singleDeviceContextHint')
          }}
        </div>
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
    <NForm
      ref="queryFormRef"
      class="alarm-query-form"
      :inline="!getPlatform"
      label-placement="left"
      :model="queryData"
    >
      <NFormItem path="status">
        <n-date-picker v-model:value="range" type="datetimerange" :clearable="false" separator="-" />
      </NFormItem>
      <NFormItem :label="$t('generate.alarm-level')" path="status">
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
    <NCard embedded size="small" class="alarm-triage-card">
      <div class="alarm-triage-header">
        <div>
          <div class="alarm-triage-title">{{ $t('custom.alarmPage.triageTitle') }}</div>
          <div class="alarm-triage-desc">
            {{
              $t('custom.alarmPage.triageDesc')
                .replace('{page}', String(tableData.length))
                .replace('{total}', String(pagination.itemCount || 0))
            }}
          </div>
        </div>
        <NFlex :size="8" align="center" wrap>
          <NButton
            size="small"
            secondary
            data-testid="alarm-download-current-page-evidence"
            @click="downloadAlarmClosureEvidenceBundle"
          >
            {{ $t('custom.alarmPage.downloadEvidenceBundle') }}
          </NButton>
          <NButton
            size="small"
            type="success"
            secondary
            :loading="batchActionLoading"
            :disabled="selectedUnacknowledgedRows.length === 0"
            @click="acknowledgeCurrentPage"
          >
            {{ $t('custom.alarmPage.acknowledgeSelected') }}
          </NButton>
          <NButton
            size="small"
            type="error"
            secondary
            :loading="batchActionLoading"
            :disabled="selectedActiveRows.length === 0"
            @click="resetCurrentPage"
          >
            {{ $t('custom.alarmPage.resetSelected') }}
          </NButton>
        </NFlex>
      </div>
      <div class="alarm-triage-cards">
        <div v-for="card in alarmTriageCards" :key="card.key" class="alarm-triage-card">
          <span>{{ card.label }}</span>
          <NTag :type="card.type" size="small">{{ card.value }}</NTag>
        </div>
      </div>
    </NCard>
    <AlarmBatchEvidenceCard
      v-if="lastBatchActionEvidence"
      :evidence="lastBatchActionEvidence"
      :action-label="lastBatchActionLabel"
      @copy="copyLastBatchActionEvidence"
      @download="downloadAlarmClosureEvidenceBundle"
    />
    <NCard v-if="showAlarmEmptyGuide" embedded size="small" class="alarm-empty-guide">
      <div class="alarm-empty-guide__head">
        <div>
          <div class="alarm-empty-guide__title">{{ $t('custom.alarmPage.emptyGuideTitle') }}</div>
          <div class="alarm-empty-guide__desc">{{ $t('custom.alarmPage.emptyGuideDesc') }}</div>
        </div>
        <NButton size="small" secondary @click="resetData">{{ $t('custom.alarmPage.emptyGuideResetAction') }}</NButton>
      </div>
      <div class="alarm-empty-guide__items">
        <div v-for="item in alarmEmptyGuideItems" :key="item.key" class="alarm-empty-guide__item">
          <NTag :type="item.type" size="small">{{ item.title }}</NTag>
          <span>{{ item.description }}</span>
        </div>
      </div>
    </NCard>
    <div class="w-100% flex-1-hidden alarm-table-scroll">
      <n-data-table
        remote
        :loading="loading"
        :columns="columns"
        :data="tableData"
        :pagination="pagination"
        :row-key="rowKey"
        v-model:checked-row-keys="selectedAlarmRowKeys"
        class="w-100%"
      />
    </div>
    <!--    <div class="flex gap-20px">-->
    <!--      <NButton @click="handleBatch">{{ $t('generate.batch-process') }}</NButton>-->
    <!--      <NButton @click="handleIgnore">{{ $t('generate.batch-ignore') }}</NButton>-->
    <!--    </div>-->
    <n-modal v-model:show="batchActionDialogVisible" class="max-w-[600px]" :mask-closable="!batchActionLoading">
      <NCard :title="batchActionDialogTitle" class="alarm-action-modal-card">
        <div class="batch-action-hint">
          <div class="whitespace-pre-line">{{ batchActionDialogHint }}</div>
        </div>
        <n-form-item :label="$t('custom.alarmPage.batchActionNoteLabel')">
          <NInput
            v-model:value="batchActionNote"
            type="textarea"
            :maxlength="batchActionNoteMaxLength"
            show-count
            :autosize="{ minRows: 3, maxRows: 5 }"
            :placeholder="$t('custom.alarmPage.batchActionNotePlaceholder')"
          />
        </n-form-item>
        <NFlex justify="flex-end" class="mt-4">
          <NButton :disabled="batchActionLoading" @click="closeBatchActionDialog">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="batchActionLoading" @click="runBatchAlarmAction">
            {{ $t('common.confirm') }}
          </NButton>
        </NFlex>
      </NCard>
    </n-modal>
    <n-modal v-model:show="singleActionDialogVisible" class="max-w-[600px]" :mask-closable="!singleActionLoading">
      <NCard :title="singleActionDialogTitle" class="alarm-action-modal-card">
        <div class="batch-action-hint">
          <div class="whitespace-pre-line">{{ singleActionDialogHint }}</div>
        </div>
        <n-form-item :label="$t('custom.alarmPage.batchActionNoteLabel')">
          <NInput
            v-model:value="singleActionNote"
            type="textarea"
            :maxlength="singleActionNoteMaxLength"
            show-count
            :autosize="{ minRows: 3, maxRows: 5 }"
            :placeholder="$t('custom.alarmPage.singleActionNotePlaceholder')"
          />
        </n-form-item>
        <NFlex justify="flex-end" class="mt-4">
          <NButton :disabled="singleActionLoading" @click="closeSingleActionDialog">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="singleActionLoading" @click="runSingleAlarmAction">
            {{ $t('common.confirm') }}
          </NButton>
        </NFlex>
      </NCard>
    </n-modal>
    <n-modal v-model:show="showDialog" :title="$t('generate.alarm-info')" class="max-w-[800px]">
      <NCard class="alarm-detail-modal-card">
        <div>
          <NH3>{{ $t('generate.alarm-info') }}</NH3>
        </div>
        <NAlert :type="detailClosureNextAction.type" :show-icon="false" class="alarm-closure-next-alert">
          <div class="alarm-closure-next-alert__head">
            <NTag :type="detailClosureNextAction.type" size="small">{{ detailClosureNextAction.status }}</NTag>
            <strong>{{ detailClosureNextAction.nextStep }}</strong>
          </div>
          <div class="alarm-closure-next-alert__evidence">{{ detailClosureNextAction.evidence }}</div>
        </NAlert>
        <n-form-item label-placement="left" :show-feedback="false" :label="$t('generate.alarmConfugName') + ':'">
          {{ infoData.name }}
        </n-form-item>
        <n-form-item label-placement="left" :show-feedback="false" :label="$t('generate.sceneLinkageName') + ':'">
          {{ infoData['alarm_config_name'] }}
        </n-form-item>
        <n-form-item label-placement="left" :show-feedback="false" :label="$t('common.alarm_time') + ':'">
          {{ dayjs(infoData['create_at']).format('YYYY-MM-DD HH:mm:ss') }}
        </n-form-item>
        <n-form-item label-placement="left" :show-feedback="false" :label="$t('generate.alarm-status') + ':'">
          {{ alarmStatusOptions.find((data) => data.value === infoData['alarm_status'])?.label || '' }}
        </n-form-item>
        <n-form-item label-placement="left" :show-feedback="false" :label="$t('common.alarm_level') + ':'">
          {{ alarmSeverityLabel(alarmSeverityValue(infoData), alarmStatusOptions) }}
        </n-form-item>
        <n-form-item label-placement="left" :show-feedback="false" :label="$t('rdi.overview.alarmType') + ':'">
          {{ alarmTypeLabel(infoData, $t) }}
        </n-form-item>
        <n-form-item label-placement="left" :show-feedback="false" :label="$t('generate.alarmReason') + ':'">
          {{ infoData.content }}
        </n-form-item>
        <n-form-item label-placement="left" :show-feedback="false" :label="$t('generate.alarm-description') + ':'">
          {{ infoData.description }}
        </n-form-item>
        <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('rdi.overview.acknowledgedBy')}:`">
          {{ alarmActionField(infoData, 'acknowledged_by') }}
        </n-form-item>
        <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('rdi.overview.acknowledgedAt')}:`">
          {{ alarmActionField(infoData, 'acknowledged_at') }}
        </n-form-item>
        <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('rdi.overview.resetBy')}:`">
          {{ alarmActionField(infoData, 'reset_by') }}
        </n-form-item>
        <n-form-item label-placement="left" :show-feedback="false" :label="`${$t('rdi.overview.resetAt')}:`">
          {{ alarmActionField(infoData, 'reset_at') }}
        </n-form-item>
        <NCard embedded size="small" class="alarm-resolution-card">
          <div class="alarm-resolution-header">
            <div>
              <div class="alarm-resolution-title">{{ $t('custom.alarmPage.timelineTitle') }}</div>
              <div class="alarm-resolution-desc">{{ $t('custom.alarmPage.timelineDesc') }}</div>
            </div>
            <NFlex :size="8" wrap justify="end">
              <NButton
                size="small"
                secondary
                data-testid="alarm-copy-closure-evidence"
                @click="copyAlarmClosureEvidence"
              >
                {{ $t('custom.alarmPage.copyClosureEvidence') }}
              </NButton>
              <NButton
                size="small"
                secondary
                data-testid="alarm-download-detail-evidence"
                @click="downloadAlarmClosureEvidenceBundle"
              >
                {{ $t('custom.alarmPage.downloadEvidenceBundle') }}
              </NButton>
              <NButton size="small" secondary @click="openAlarmAuditLog">
                {{ $t('custom.alarmPage.viewAuditLog') }}
              </NButton>
            </NFlex>
          </div>
          <div class="alarm-resolution-timeline">
            <div v-for="item in detailTimelineItems" :key="item.key" class="alarm-resolution-item">
              <NTag :type="item.type" size="small">{{ item.title }}</NTag>
              <div class="alarm-resolution-item__body">
                <strong>{{ item.time }}</strong>
                <span>{{ item.description }}</span>
              </div>
            </div>
          </div>
          <NAlert type="info" :show-icon="false" class="mt-3">
            {{ $t('custom.alarmPage.auditBoundaryHint') }}
          </NAlert>
          <NFlex class="mt-3" :size="8" wrap>
            <NButton v-if="detailNeedsAcknowledge" size="small" type="success" secondary @click="acknowledgeAlarm(infoData)">
              {{ $t('rdi.overview.acknowledgeAlarm') }}
            </NButton>
            <NButton v-if="detailNeedsReset" size="small" type="error" secondary @click="resetAlarm(infoData)">
              {{ $t('common.reset') }}
            </NButton>
            <NButton size="small" secondary @click="maintenance(infoData)">
              {{ $t('rdi.overview.maintenanceNote') }}
            </NButton>
          </NFlex>
        </NCard>
        <n-form-item label-placement="top" :show-feedback="false" :label="$t('generate.alarmDevices') + ':'">
          <NTable size="small" :bordered="false" :single-line="false" class="mb-6">
            <thead>
              <tr>
                <th>{{ $t('common.index') }}</th>
                <th class="min-w-180px">{{ $t('generate.device-code') }}</th>
                <th>{{ $t('custom.devicePage.deviceName') }}</th>
                <th>{{ $t('custom.device_details.readyCheck') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(device, index) in infoData.alarm_device_list" :key="index">
                <td class="min-w-100px">{{ Number(index) + 1 }}</td>
                <td>{{ device.id }}</td>
                <td>{{ device['name'] }}</td>
                <td>
                  <NButton
                    size="small"
                    secondary
                    type="primary"
                    :disabled="!alarmDeviceId(device)"
                    data-testid="alarm-device-ready-check"
                    @click="openAlarmDeviceReadyCheck(device)"
                  >
                    {{ $t('custom.commandCenter.openDeviceDiagnosis') }}
                  </NButton>
                </td>
              </tr>
            </tbody>
          </NTable>
        </n-form-item>
        <NFlex justify="flex-end">
          <NButton @click="closeModal">{{ $t('custom.devicePage.close') }}</NButton>
        </NFlex>
      </NCard>
    </n-modal>
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
.pop-up {
  display: flex;
}

.pop-up-content {
  height: 200px;
  padding: 10px;
  border: 1px solid rgb(215, 213, 213);
  border-radius: 10px;
}

.alarm-route-context {
  display: grid;
  gap: 8px;
  line-height: 1.5;
}

.alarm-triage-card {
  margin-bottom: 12px;
}

.alarm-triage-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.alarm-triage-title {
  color: #0f172a;
  font-weight: 600;
}

.alarm-triage-desc {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.alarm-triage-cards {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  margin-top: 12px;
}

.alarm-triage-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
  color: #475569;
  font-size: 13px;
}

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

.alarm-empty-guide {
  margin-bottom: 12px;
  border: 1px solid #dbeafe;
  background: linear-gradient(135deg, #f8fbff 0%, #eff6ff 100%);
}

.alarm-empty-guide__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.alarm-empty-guide__title {
  color: #0f172a;
  font-weight: 700;
}

.alarm-empty-guide__desc {
  margin-top: 4px;
  color: #475569;
  font-size: 12px;
  line-height: 1.6;
}

.alarm-empty-guide__items {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  margin-top: 12px;
}

.alarm-empty-guide__item {
  display: grid;
  gap: 8px;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.72);
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
}

.alarm-resolution-card {
  margin: 12px 0 18px;
}

.alarm-closure-next-alert {
  margin-bottom: 12px;
}

.alarm-closure-next-alert__head {
  display: flex;
  gap: 8px;
  align-items: center;
  flex-wrap: wrap;
}

.alarm-closure-next-alert__evidence {
  margin-top: 6px;
  color: #475569;
  font-size: 12px;
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.alarm-resolution-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.alarm-resolution-title {
  font-weight: 600;
}

.alarm-resolution-desc {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.alarm-resolution-timeline {
  display: grid;
  gap: 10px;
}

.alarm-resolution-item {
  display: grid;
  grid-template-columns: 148px minmax(0, 1fr);
  gap: 10px;
  align-items: flex-start;
  padding: 10px 12px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}

.alarm-resolution-item__body {
  min-width: 0;
}

.alarm-resolution-item__body strong,
.alarm-resolution-item__body span {
  display: block;
}

.alarm-resolution-item__body span {
  margin-top: 4px;
  color: #475569;
  overflow-wrap: anywhere;
}

.batch-action-hint {
  margin-bottom: 16px;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
  color: #475569;
  font-size: 13px;
  line-height: 1.5;
}

@media (max-width: 900px) {
  .alarm-triage-header {
    flex-direction: column;
  }

  .alarm-triage-cards,
  .alarm-empty-guide__items {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .alarm-resolution-header {
    flex-direction: column;
  }

  .alarm-resolution-item {
    grid-template-columns: 1fr;
  }
}

/* 弹窗宽度收敛：min(96vw, 原宽)，桌面端取原宽、手机端不溢出。 */
.alarm-action-modal-card {
  width: min(96vw, 600px);
}

.alarm-detail-modal-card {
  width: min(96vw, 800px);
}

/* ≤640px 现场手机查告警最小保障；断点统一取 _mixins.scss 的 mobile mixin（=--breakpoint-sm=640px）。 */
@include mobile {
  /* 查询表单控件换行铺满，按钮组随表单流式换行不溢出 */
  .alarm-query-form :deep(.n-form-item) {
    width: 100%;
    margin-right: 0;
  }

  .alarm-query-form :deep(.n-date-picker) {
    width: 100%;
  }

  /* 表格横向滚动容器化：外层容器横向滑动，表格给足最小宽度 */
  .alarm-table-scroll {
    overflow-x: auto;
    -webkit-overflow-scrolling: touch;

    > .n-data-table {
      min-width: 720px;
    }
  }

  .alarm-triage-cards,
  .alarm-empty-guide__items {
    grid-template-columns: 1fr;
  }
}

</style>
