<script setup lang="ts">
import { computed, h, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NCheckbox,
  NDataTable,
  NDescriptions,
  NDescriptionsItem,
  NDrawer,
  NDrawerContent,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NModal,
  NPagination,
  NPopconfirm,
  NSpace,
  NSpin,
  NSwitch,
  NTag,
  useMessage,
  type DataTableColumns,
  type FormInst
} from 'naive-ui'
import { $t } from '@/locales'
import {
  createReportSchedule,
  deleteReportSchedule,
  getReportRun,
  getReportSchedule,
  listReportRuns,
  listReportSchedules,
  retryReportRun,
  runReportSchedule,
  updateReportSchedule,
  type ReportRun,
  type ReportSchedule,
  type ReportSchedulePayload,
  type UpdateReportSchedulePayload
} from '@/service/api/report'
import {
  canRetryReportRun,
  createReportIdempotencyKey,
  isUncertainReportTransportError,
  reportStatusTagType
} from './report-model'
import { useSelectedReportRunPoll } from './useSelectedReportRunPoll'

const PAGE_SIZE = 10
const RUN_PAGE_SIZE = 10

const message = useMessage()
const formRef = ref<FormInst | null>(null)
const schedules = ref<ReportSchedule[]>([])
const total = ref(0)
const page = ref(1)
const searchInput = ref('')
const search = ref('')
const listLoading = ref(false)
const listFailed = ref(false)
const showForm = ref(false)
const editing = ref<ReportSchedule | null>(null)
const saving = ref(false)
const deletingId = ref('')
const runningId = ref('')
const selectedSchedule = ref<ReportSchedule | null>(null)
const selectedScheduleId = computed(() => selectedSchedule.value?.id || '')
const historyVisible = ref(false)
const historyLoading = ref(false)
const runs = ref<ReportRun[]>([])
const runTotal = ref(0)
const runPage = ref(1)
const selectedRun = ref<ReportRun | null>(null)
const retryingRunId = ref('')
const pollFailed = ref(false)
let scheduleRequestSequence = 0
let historyRequestSequence = 0

const emptyForm = (): ReportSchedulePayload => ({
  name: '',
  cron_expr: '',
  timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
  recipients: '',
  device_ids: [],
  keys: [],
  lookback_hours: 24,
  format: 'csv',
  enabled: true
})
const form = reactive<ReportSchedulePayload>(emptyForm())
const deviceIdsText = ref('')
const keysText = ref('')

const copyForm = (schedule?: ReportSchedule) => {
  const source = schedule || emptyForm()
  Object.assign(form, {
    name: source.name,
    cron_expr: source.cron_expr,
    timezone: source.timezone || 'UTC',
    recipients: source.recipients,
    device_ids: [...source.device_ids],
    keys: [...source.keys],
    lookback_hours: source.lookback_hours,
    format: 'csv',
    enabled: source.enabled,
    ...(schedule ? { revision: schedule.revision } : {})
  })
  deviceIdsText.value = source.device_ids.join('\n')
  keysText.value = source.keys.join('\n')
}

const splitLines = (value: string) => [
  ...new Set(
    value
      .split(/[\n,]/)
      .map((item) => item.trim())
      .filter(Boolean)
  )
]
const formatTime = (value?: string | null) =>
  value ? new Date(value).toLocaleString() : $t('report.common.notAvailable')
const statusText = (value?: string | null) => (value ? $t(`report.status.${value}`, value) : $t('report.status.never'))
const runWindow = (run: ReportRun) => `${formatTime(run.window_start_at)} — ${formatTime(run.window_end_at)}`
const smtpAccepted = (run: ReportRun) => run.delivery_status === 'accepted'
const smtpAmbiguous = (run: ReportRun) => run.delivery_status === 'ambiguous'

async function loadSchedules() {
  const sequence = ++scheduleRequestSequence
  const snapshot = { page: page.value, search: search.value }
  listLoading.value = true
  listFailed.value = false
  try {
    const { data, error } = await listReportSchedules({
      page: snapshot.page,
      page_size: PAGE_SIZE,
      search: snapshot.search
    })
    if (sequence !== scheduleRequestSequence || snapshot.page !== page.value || snapshot.search !== search.value) return
    if (error || !data) throw error || new Error('missing data')
    schedules.value = data.list || []
    total.value = data.total || 0
  } catch {
    if (sequence !== scheduleRequestSequence) return
    schedules.value = []
    total.value = 0
    listFailed.value = true
    message.error($t('report.message.loadFailed'))
  } finally {
    if (sequence === scheduleRequestSequence) listLoading.value = false
  }
}

const REPORT_REVISION_CONFLICT_CODE = 201002

type ReportRequestError = {
  code?: number
  data?: { code?: number }
  response?: { data?: { code?: number } }
}

const reportErrorCode = (error: unknown) => {
  if (!error || typeof error !== 'object') return undefined
  const value = error as ReportRequestError
  return value.response?.data?.code ?? value.data?.code ?? value.code
}

const isReportOperationDenied = (error: unknown) => reportErrorCode(error) === REPORT_REVISION_CONFLICT_CODE

const refreshStaleSchedule = async (id: string) => {
  const { data, error } = await getReportSchedule(id)
  let refreshed: ReportSchedule | undefined = data || undefined
  if (error || !refreshed) {
    await loadSchedules()
    refreshed = schedules.value.find((schedule) => schedule.id === id)
  } else {
    const index = schedules.value.findIndex((schedule) => schedule.id === id)
    if (index >= 0) schedules.value.splice(index, 1, refreshed)
  }
  if (refreshed && editing.value?.id === id) {
    editing.value = refreshed
    copyForm(refreshed)
  }
  return refreshed
}

function searchSchedules() {
  search.value = searchInput.value.trim()
  page.value = 1
  void loadSchedules()
}
function changePage(next: number) {
  page.value = next
  void loadSchedules()
}
function openCreate() {
  editing.value = null
  copyForm()
  showForm.value = true
}
function openEdit(row: ReportSchedule) {
  editing.value = row
  copyForm(row)
  showForm.value = true
}

async function saveSchedule() {
  form.device_ids = splitLines(deviceIdsText.value)
  form.keys = splitLines(keysText.value)
  try {
    await formRef.value?.validate()
  } catch {
    return
  }
  if (!form.device_ids.length || !form.keys.length) {
    message.error($t('report.message.targetsRequired'))
    return
  }
  saving.value = true
  try {
    const result = editing.value
      ? await updateReportSchedule(editing.value.id, {
          ...form,
          revision: form.revision ?? editing.value.revision
        } satisfies UpdateReportSchedulePayload)
      : await createReportSchedule({ ...form })
    if (result.error) throw result.error
    message.success($t(editing.value ? 'report.message.updated' : 'report.message.created'))
    showForm.value = false
    await loadSchedules()
  } catch (error) {
    if (editing.value && isReportOperationDenied(error)) {
      const previousRevision = editing.value.revision
      const refreshed = await refreshStaleSchedule(editing.value.id)
      if (refreshed && refreshed.revision !== previousRevision) {
        message.warning($t('report.message.staleRevision'))
      } else {
        message.error($t('report.message.saveFailed'))
      }
    } else {
      message.error($t('report.message.saveFailed'))
    }
  } finally {
    saving.value = false
  }
}

async function removeSchedule(row: ReportSchedule) {
  deletingId.value = row.id
  try {
    const { error } = await deleteReportSchedule(row.id, row.revision)
    if (error) throw error
    message.success($t('report.message.deleted'))
    if (schedules.value.length === 1 && page.value > 1) page.value -= 1
    await loadSchedules()
  } catch (error) {
    if (isReportOperationDenied(error)) {
      const refreshed = await refreshStaleSchedule(row.id)
      if (refreshed && refreshed.revision !== row.revision) {
        message.warning($t('report.message.deleteStaleRevision'))
      } else {
        message.error($t('report.message.deleteFailed'))
      }
    } else {
      message.error($t('report.message.deleteFailed'))
    }
  } finally {
    deletingId.value = ''
  }
}

async function runNow(row: ReportSchedule, idempotencyKey = createReportIdempotencyKey()) {
  if (runningId.value || !row.enabled) return
  runningId.value = row.id
  let retryUncertain = false
  try {
    const { data, error } = await runReportSchedule(row.id, { idempotencyKey })
    if (error || !data) throw error || new Error('missing data')
    message.success($t(data.idempotent_replay ? 'report.message.runReplay' : 'report.message.runStarted'))
    selectedSchedule.value = row
    selectedRun.value = null
    pollFailed.value = false
    historyVisible.value = true
    runPage.value = 1
    await Promise.all([loadRuns(), loadSchedules()])
    selectedRun.value = runs.value.find((item) => item.run_id === data.run_id) || null
    if (!selectedRun.value) {
      const detail = await getReportRun(row.id, data.run_id)
      if (!detail.error && detail.data) selectedRun.value = detail.data
    }
  } catch (error) {
    message.error($t('report.message.runFailed'))
    retryUncertain = isUncertainReportTransportError(error)
  } finally {
    runningId.value = ''
  }
  if (retryUncertain) await runNow(row, idempotencyKey)
}

async function loadRuns() {
  const scheduleId = selectedScheduleId.value
  if (!scheduleId) return
  const sequence = ++historyRequestSequence
  const snapshotPage = runPage.value
  historyLoading.value = true
  try {
    const { data, error } = await listReportRuns(scheduleId, { page: snapshotPage, page_size: RUN_PAGE_SIZE })
    if (
      sequence !== historyRequestSequence ||
      scheduleId !== selectedScheduleId.value ||
      snapshotPage !== runPage.value
    )
      return
    if (error || !data) throw error || new Error('missing data')
    runs.value = data.list || []
    runTotal.value = data.total || 0
    if (selectedRun.value)
      selectedRun.value = runs.value.find((run) => run.run_id === selectedRun.value?.run_id) || selectedRun.value
  } catch {
    if (sequence === historyRequestSequence) message.error($t('report.message.historyFailed'))
  } finally {
    if (sequence === historyRequestSequence) historyLoading.value = false
  }
}

function openHistory(row: ReportSchedule) {
  selectedSchedule.value = row
  selectedRun.value = null
  pollFailed.value = false
  runs.value = []
  runPage.value = 1
  historyVisible.value = true
  void loadRuns()
}
function changeRunPage(next: number) {
  runPage.value = next
  void loadRuns()
}
function selectRun(run: ReportRun) {
  pollFailed.value = false
  selectedRun.value = run
}

async function retryRun(run: ReportRun, idempotencyKey = createReportIdempotencyKey()) {
  const scheduleId = selectedScheduleId.value
  if (!scheduleId || !selectedSchedule.value?.enabled || retryingRunId.value) return
  retryingRunId.value = run.run_id
  let retryUncertain = false
  try {
    const { data, error } = await retryReportRun(scheduleId, run.run_id, { idempotencyKey })
    if (error || !data) throw error || new Error('missing data')
    message.success($t(data.idempotent_replay ? 'report.message.retryReplay' : 'report.message.retryStarted'))
    selectedRun.value = null
    pollFailed.value = false
    await loadRuns()
    selectedRun.value = runs.value.find((item) => item.run_id === data.run_id) || null
    if (!selectedRun.value) {
      const detail = await getReportRun(scheduleId, data.run_id)
      if (!detail.error && detail.data) selectedRun.value = detail.data
    }
  } catch (error) {
    message.error($t('report.message.retryFailed'))
    retryUncertain = isUncertainReportTransportError(error)
  } finally {
    retryingRunId.value = ''
  }
  if (retryUncertain) await retryRun(run, idempotencyKey)
}

useSelectedReportRunPoll({
  selectedScheduleId,
  selectedRun,
  visible: historyVisible,
  onUpdate: (run) => {
    pollFailed.value = false
    const index = runs.value.findIndex((item) => item.run_id === run.run_id)
    if (index >= 0) runs.value.splice(index, 1, run)
  },
  onFailure: ({ stopped }) => {
    if (stopped) pollFailed.value = true
  }
})

const scheduleColumns = computed<DataTableColumns<ReportSchedule>>(() => [
  {
    title: $t('report.table.name'),
    key: 'name',
    minWidth: 180,
    render: (row) =>
      h('div', [h('strong', row.name), h('div', { class: 'report-muted' }, `${row.cron_expr} · ${row.timezone}`)])
  },
  {
    title: $t('report.table.delivery'),
    key: 'delivery',
    minWidth: 180,
    render: (row) =>
      h('div', [
        h('div', row.recipients),
        h(
          'div',
          { class: 'report-muted' },
          $t('report.table.targetSummary', { devices: row.device_ids.length, keys: row.keys.length })
        )
      ])
  },
  { title: $t('report.table.nextRun'), key: 'next_run_at', width: 165, render: (row) => formatTime(row.next_run_at) },
  {
    title: $t('report.table.lastRun'),
    key: 'last_status',
    width: 150,
    render: (row) =>
      h('div', [
        h(
          NTag,
          { size: 'small', type: reportStatusTagType(row.last_status) },
          { default: () => statusText(row.last_status) }
        ),
        h('div', { class: 'report-muted' }, formatTime(row.last_run_at))
      ])
  },
  {
    title: $t('report.table.enabled'),
    key: 'enabled',
    width: 90,
    render: (row) =>
      h(
        NTag,
        { type: row.enabled ? 'success' : 'default', size: 'small' },
        { default: () => $t(row.enabled ? 'report.common.enabled' : 'report.common.disabled') }
      )
  },
  {
    title: $t('report.table.actions'),
    key: 'actions',
    width: 310,
    fixed: 'right',
    render: (row) =>
      h(
        NSpace,
        { size: 6, wrap: true },
        {
          default: () => [
            h(
              NButton,
              {
                size: 'small',
                type: 'primary',
                loading: runningId.value === row.id,
                disabled: Boolean(runningId.value) || !row.enabled,
                onClick: () => runNow(row)
              },
              { default: () => $t('report.action.runNow') }
            ),
            h(
              NButton,
              { size: 'small', onClick: () => openHistory(row) },
              { default: () => $t('report.action.history') }
            ),
            h(NButton, { size: 'small', onClick: () => openEdit(row) }, { default: () => $t('report.action.edit') }),
            h(
              NPopconfirm,
              { onPositiveClick: () => removeSchedule(row) },
              {
                trigger: () =>
                  h(
                    NButton,
                    { size: 'small', type: 'error', loading: deletingId.value === row.id },
                    { default: () => $t('report.action.delete') }
                  ),
                default: () => $t('report.message.deleteConfirm')
              }
            )
          ]
        }
      )
  }
])

const runColumns = computed<DataTableColumns<ReportRun>>(() => [
  { title: $t('report.history.started'), key: 'created_at', width: 170, render: (row) => formatTime(row.created_at) },
  { title: $t('report.history.window'), key: 'window', minWidth: 220, render: runWindow },
  {
    title: $t('report.history.overall'),
    key: 'overall_status',
    width: 120,
    render: (row) =>
      h(
        NTag,
        { size: 'small', type: reportStatusTagType(row.overall_status) },
        { default: () => statusText(row.overall_status) }
      )
  },
  {
    title: $t('report.history.generation'),
    key: 'generation_status',
    width: 130,
    render: (row) => statusText(row.generation_status)
  },
  {
    title: $t('report.history.delivery'),
    key: 'delivery_status',
    width: 130,
    render: (row) => statusText(row.delivery_status)
  },
  {
    title: $t('report.history.risk'),
    key: 'duplicate_delivery_risk',
    width: 120,
    render: (row) =>
      row.duplicate_delivery_risk
        ? h(NTag, { type: 'warning', size: 'small' }, { default: () => $t('report.risk.duplicate') })
        : $t('report.risk.none')
  },
  {
    title: $t('report.table.actions'),
    key: 'actions',
    width: 170,
    render: (row) =>
      h(
        NSpace,
        { size: 6 },
        {
          default: () => [
            h(
              NButton,
              { size: 'small', onClick: () => selectRun(row) },
              { default: () => $t('report.action.details') }
            ),
            canRetryReportRun(row, selectedSchedule.value?.enabled)
              ? h(
                  NButton,
                  {
                    size: 'small',
                    type: 'warning',
                    loading: retryingRunId.value === row.run_id,
                    onClick: () => retryRun(row)
                  },
                  { default: () => $t('report.action.retry') }
                )
              : null
          ]
        }
      )
  }
])

onMounted(loadSchedules)
onBeforeUnmount(() => {
  scheduleRequestSequence += 1
  historyRequestSequence += 1
})
</script>

<template>
  <main class="report-workspace">
    <header class="report-header">
      <div>
        <p class="report-kicker">{{ $t('report.page.kicker') }}</p>
        <h1>{{ $t('report.page.title') }}</h1>
        <p class="report-subtitle">{{ $t('report.page.subtitle') }}</p>
      </div>
      <NButton type="primary" size="large" data-testid="report-create" @click="openCreate">
        {{ $t('report.action.create') }}
      </NButton>
    </header>

    <section class="report-command-bar">
      <div class="report-cadence">
        <span>{{ $t('report.page.cadenceLabel') }}</span>
        <strong>{{ $t('report.page.cadenceValue') }}</strong>
      </div>
      <NInput
        v-model:value="searchInput"
        clearable
        :placeholder="$t('report.page.search')"
        @keyup.enter="searchSchedules"
      />
      <NButton @click="searchSchedules">{{ $t('report.action.search') }}</NButton>
      <NButton :loading="listLoading" @click="loadSchedules">{{ $t('report.action.refresh') }}</NButton>
    </section>

    <NAlert v-if="listFailed" type="error" class="mb-4">{{ $t('report.message.loadFailed') }}</NAlert>
    <NCard class="report-ledger" :bordered="false">
      <NSpin :show="listLoading">
        <NDataTable
          v-if="schedules.length"
          :columns="scheduleColumns"
          :data="schedules"
          :row-key="(row: ReportSchedule) => row.id"
          :scroll-x="1200"
          data-testid="report-schedule-table"
        />
        <NEmpty v-else-if="!listLoading" :description="$t('report.page.empty')" class="py-20">
          <template #extra>
            <NButton type="primary" @click="openCreate">{{ $t('report.action.createFirst') }}</NButton>
          </template>
        </NEmpty>
        <div v-if="total > PAGE_SIZE" class="report-pagination">
          <NPagination :page="page" :page-size="PAGE_SIZE" :item-count="total" @update:page="changePage" />
        </div>
      </NSpin>
    </NCard>

    <NModal
      v-model:show="showForm"
      preset="card"
      :title="$t(editing ? 'report.form.editTitle' : 'report.form.createTitle')"
      class="report-form-modal"
    >
      <NForm ref="formRef" :model="form" label-placement="top">
        <div class="report-form-grid">
          <NFormItem
            :label="$t('report.form.name')"
            path="name"
            :rule="{ required: true, message: $t('report.form.required') }"
          >
            <NInput v-model:value="form.name" :maxlength="128" />
          </NFormItem>
          <NFormItem
            :label="$t('report.form.cron')"
            path="cron_expr"
            :rule="{ required: true, message: $t('report.form.required') }"
          >
            <NInput v-model:value="form.cron_expr" placeholder="0 8 * * 1-5" />
          </NFormItem>
          <NFormItem
            :label="$t('report.form.timezone')"
            path="timezone"
            :rule="{ required: true, message: $t('report.form.required') }"
          >
            <NInput v-model:value="form.timezone" placeholder="Europe/Paris" />
          </NFormItem>
          <NFormItem :label="$t('report.form.lookback')" path="lookback_hours">
            <NInputNumber v-model:value="form.lookback_hours" :min="1" :max="8760" class="w-full" />
          </NFormItem>
        </div>
        <NFormItem
          :label="$t('report.form.recipients')"
          path="recipients"
          :rule="{ required: true, message: $t('report.form.required') }"
        >
          <NInput v-model:value="form.recipients" :placeholder="$t('report.form.recipientsHint')" />
        </NFormItem>
        <div class="report-form-grid">
          <NFormItem :label="$t('report.form.devices')">
            <NInput
              v-model:value="deviceIdsText"
              type="textarea"
              :rows="5"
              :placeholder="$t('report.form.linesHint')"
            />
          </NFormItem>
          <NFormItem :label="$t('report.form.keys')">
            <NInput v-model:value="keysText" type="textarea" :rows="5" :placeholder="$t('report.form.linesHint')" />
          </NFormItem>
        </div>
        <div class="report-form-footer-row">
          <NCheckbox :checked="form.format === 'csv'" disabled>CSV</NCheckbox>
          <label>
            <span>{{ $t('report.form.enabled') }}</span>
            <NSwitch v-model:value="form.enabled" />
          </label>
        </div>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton :disabled="saving" @click="showForm = false">{{ $t('report.action.cancel') }}</NButton>
          <NButton type="primary" :loading="saving" @click="saveSchedule">{{ $t('report.action.save') }}</NButton>
        </NSpace>
      </template>
    </NModal>

    <NDrawer v-model:show="historyVisible" width="min(1080px, 94vw)" placement="right">
      <NDrawerContent :title="selectedSchedule?.name || $t('report.history.title')" closable>
        <div class="report-history-header">
          <div>
            <p>{{ $t('report.history.subtitle') }}</p>
            <strong>{{ selectedSchedule?.cron_expr }} · {{ selectedSchedule?.timezone }}</strong>
          </div>
          <NButton :loading="historyLoading" @click="loadRuns">{{ $t('report.action.refresh') }}</NButton>
        </div>
        <NAlert v-if="!selectedSchedule?.enabled" type="info" class="mb-4">
          {{ $t('report.message.scheduleDisabled') }}
        </NAlert>
        <NAlert v-if="pollFailed" type="error" class="mb-4" data-testid="report-poll-failed">
          {{ $t('report.message.pollFailed') }}
        </NAlert>
        <NSpin :show="historyLoading">
          <NDataTable
            v-if="runs.length"
            :columns="runColumns"
            :data="runs"
            :row-key="(row: ReportRun) => row.run_id"
            :scroll-x="1100"
          />
          <NEmpty v-else-if="!historyLoading" :description="$t('report.history.empty')" class="py-16" />
          <div v-if="runTotal > RUN_PAGE_SIZE" class="report-pagination">
            <NPagination
              :page="runPage"
              :page-size="RUN_PAGE_SIZE"
              :item-count="runTotal"
              @update:page="changeRunPage"
            />
          </div>
        </NSpin>

        <section v-if="selectedRun" class="report-run-detail" data-testid="report-run-detail">
          <div class="report-detail-heading">
            <div>
              <p class="report-kicker">{{ $t('report.detail.kicker') }}</p>
              <h2>{{ selectedRun.run_id }}</h2>
            </div>
            <NTag :type="reportStatusTagType(selectedRun.overall_status)">
              {{ statusText(selectedRun.overall_status) }}
            </NTag>
          </div>
          <NAlert v-if="smtpAmbiguous(selectedRun) || selectedRun.duplicate_delivery_risk" type="warning" class="mb-4">
            {{ $t('report.risk.warning') }}
          </NAlert>
          <NDescriptions bordered :column="2" label-placement="left">
            <NDescriptionsItem :label="$t('report.history.window')">{{ runWindow(selectedRun) }}</NDescriptionsItem>
            <NDescriptionsItem :label="$t('report.detail.updated')">
              {{
                formatTime(
                  selectedRun.delivery_completed_at || selectedRun.generation_completed_at || selectedRun.created_at
                )
              }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('report.history.generation')">
              {{ statusText(selectedRun.generation_status) }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('report.history.delivery')">
              {{ statusText(selectedRun.delivery_status) }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('report.detail.smtpAccepted')">
              {{ smtpAccepted(selectedRun) ? $t('report.common.yes') : $t('report.common.no') }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('report.detail.smtpAmbiguous')">
              {{ smtpAmbiguous(selectedRun) ? $t('report.common.yes') : $t('report.common.no') }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('report.detail.generationAttempts')">
              {{ selectedRun.generation_attempts }}
            </NDescriptionsItem>
            <NDescriptionsItem :label="$t('report.detail.deliveryAttempts')">
              {{ selectedRun.delivery_attempts }}
            </NDescriptionsItem>
          </NDescriptions>
          <NAlert v-if="smtpAccepted(selectedRun)" type="info" class="mt-4">
            {{ $t('report.detail.smtpAcceptanceCaveat') }}
          </NAlert>
          <h3>{{ $t('report.detail.errors') }}</h3>
          <NAlert v-if="selectedRun.generation_error_code" type="error" class="mb-2">
            {{ $t('report.history.generation') }}: {{ selectedRun.generation_error_code }}
          </NAlert>
          <NAlert v-if="selectedRun.delivery_error_code" type="error" class="mb-2">
            {{ $t('report.history.delivery') }}: {{ selectedRun.delivery_error_code }}
          </NAlert>
          <p v-if="!selectedRun.generation_error_code && !selectedRun.delivery_error_code" class="report-muted">
            {{ $t('report.detail.noErrors') }}
          </p>
          <NSpace justify="end">
            <NButton
              v-if="canRetryReportRun(selectedRun, selectedSchedule?.enabled)"
              type="warning"
              :loading="retryingRunId === selectedRun.run_id"
              @click="retryRun(selectedRun)"
            >
              {{ $t('report.action.retry') }}
            </NButton>
          </NSpace>
        </section>
      </NDrawerContent>
    </NDrawer>
  </main>
</template>

<style scoped>
.report-workspace {
  min-height: 100%;
  padding: clamp(18px, 3vw, 36px);
  background: linear-gradient(135deg, rgba(34, 197, 94, 0.05), transparent 36%), var(--n-color);
  color: var(--n-text-color);
}
.report-header {
  display: flex;
  align-items: end;
  justify-content: space-between;
  gap: 24px;
  max-width: 1440px;
  margin: 0 auto 20px;
  padding: 24px 4px 4px;
  border-bottom: 1px solid rgba(125, 140, 132, 0.25);
}
.report-header h1 {
  margin: 2px 0 8px;
  font:
    700 clamp(30px, 4vw, 52px)/1.05 Georgia,
    serif;
  letter-spacing: -0.035em;
}
.report-kicker {
  margin: 0;
  color: rgb(var(--success-800-color));
  font:
    700 12px/1.4 ui-monospace,
    monospace;
  letter-spacing: 0.14em;
  text-transform: uppercase;
}
.report-subtitle,
.report-history-header p {
  max-width: 720px;
  margin: 0;
  color: var(--text-color-2);
}
.report-command-bar {
  display: grid;
  grid-template-columns: minmax(220px, 1fr) minmax(260px, 2fr) auto auto;
  gap: 10px;
  align-items: center;
  max-width: 1440px;
  margin: 0 auto 14px;
}
.report-cadence {
  display: flex;
  flex-direction: column;
  padding-left: 14px;
  border-left: 3px solid rgb(var(--success-color));
  color: var(--text-color-2);
  font-size: 12px;
}
.report-cadence strong {
  color: var(--n-text-color);
  font:
    600 14px ui-monospace,
    monospace;
}
.report-ledger {
  max-width: 1440px;
  margin: 0 auto;
  box-shadow: 0 16px 46px rgba(23, 54, 39, 0.08);
}
.report-muted {
  margin-top: 4px;
  color: var(--text-color-3);
  font-size: 12px;
}
.report-pagination {
  display: flex;
  justify-content: flex-end;
  padding-top: 18px;
}
.report-form-modal {
  width: min(760px, 92vw);
}
.report-form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 18px;
}
.report-form-footer-row,
.report-form-footer-row label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.report-history-header,
.report-detail-heading {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  margin-bottom: 18px;
}
.report-run-detail {
  margin-top: 26px;
  padding: 22px;
  border: 1px solid rgba(125, 140, 132, 0.28);
  border-radius: 10px;
  background: rgba(32, 166, 106, 0.035);
}
.report-run-detail h2 {
  margin: 3px 0 0;
  font:
    650 21px ui-monospace,
    monospace;
  overflow-wrap: anywhere;
}
.report-run-detail h3 {
  margin: 24px 0 12px;
  font-size: 15px;
}
@media (max-width: 760px) {
  .report-header {
    align-items: start;
    flex-direction: column;
  }
  .report-command-bar {
    grid-template-columns: 1fr 1fr;
  }
  .report-cadence,
  .report-command-bar :deep(.n-input) {
    grid-column: 1 / -1;
  }
  .report-form-grid {
    grid-template-columns: 1fr;
  }
}
</style>
