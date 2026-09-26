<!--
文件用途：统一调度日历（scheduler_events，ROADMAP TB-48）——月视图网格聚合展示三套存量调度与注册事件。
核心逻辑：
1. 月视图网格（纯 CSS Grid，7 列）：事件按 next_run_at 聚合到日单元格，来源类型着色
   （scene=绿 / report=蓝 / rpc=橙），窗口过滤由后端 from_ms/to_ms 承担；
2. 只读聚合面：GET /scheduler/events 一次拉取当月（page_size 取后端 500 上限），
   三套存量调度（场景定时/定时报表/舰队命令定时行）+ 注册行归一展示；
3. 注册面管理：注册列表弹窗内提供 scheduler_events 的创建/编辑/删除（scene 事件落到
   既有 scene automation timer 机制执行；report/rpc 注册行仅登记展示，执行仍归各存量系统）。
关键注意事项：
  - source_type 是着色与图例的依据，origin 区分同一来源类型下的存量行与注册行（tooltip）；
  - 表单按 event_type 条件显示字段：scene→ref_id+cron（后端按 UTC 算 next_run_at）、
    report→cron、rpc→next_run_at（一次性，cron 必须为空）；同场景仅允许一条启用定时触发，
    后端 201002 拒绝时如实提示，不做前端预判。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import {
  NButton,
  NCard,
  NDatePicker,
  NEmpty,
  NForm,
  NFormItem,
  NInput,
  NModal,
  NPopconfirm,
  NSelect,
  NSpace,
  NSpin,
  NSwitch,
  NTag,
  useMessage
} from 'naive-ui'
import {
  createSchedulerEvent,
  deleteSchedulerEvent,
  getSchedulerEvents,
  updateSchedulerEvent,
  type SchedulerEventItem,
  type SchedulerEventType
} from '@/service/api'
import { $t, i18n } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'

const message = useMessage()

// ---- 月视图状态 ----
const currentMonth = ref(startOfMonth(new Date()))
const loading = ref(false)
const monthEvents = ref<SchedulerEventItem[]>([])
const sourceFilter = ref<SchedulerEventType | ''>('')

function startOfMonth(date: Date) {
  return new Date(date.getFullYear(), date.getMonth(), 1)
}

/** 当月窗口（本地时区）的 epoch 毫秒边界，配合后端 from_ms/to_ms 过滤。 */
const monthWindow = computed(() => {
  const from = new Date(currentMonth.value.getFullYear(), currentMonth.value.getMonth(), 1, 0, 0, 0, 0)
  const to = new Date(currentMonth.value.getFullYear(), currentMonth.value.getMonth() + 1, 0, 23, 59, 59, 999)
  return { fromMs: from.getTime(), toMs: to.getTime() }
})

const monthLabel = computed(() =>
  new Intl.DateTimeFormat(i18n.global.locale.value, { year: 'numeric', month: 'long' }).format(currentMonth.value)
)

/** 周一为首列的星期短名（Intl 取本地化文案，避免自维护 7 个翻译键）。 */
const weekdayLabels = computed(() => {
  const formatter = new Intl.DateTimeFormat(i18n.global.locale.value, { weekday: 'short' })
  // 2023-01-02 是周一；循环 7 天得到周一→周日的表头。
  return Array.from({ length: 7 }, (_, index) => {
    const monday = new Date(2023, 0, 2 + index)
    return formatter.format(monday)
  })
})

interface CalendarCell {
  date: Date
  inMonth: boolean
  isToday: boolean
  /** 'YYYY-MM-DD'（本地时区），事件分桶键。 */
  key: string
}

/** 6 行 × 7 列的月网格单元格（纯 CSS Grid 渲染）。 */
const calendarCells = computed<CalendarCell[]>(() => {
  const first = currentMonth.value
  const firstWeekday = (first.getDay() + 6) % 7 // 周一=0
  const gridStart = new Date(first.getFullYear(), first.getMonth(), 1 - firstWeekday)
  const today = new Date()
  const todayKey = localDateKey(today)
  const cells: CalendarCell[] = []
  for (let index = 0; index < 42; index += 1) {
    const date = new Date(gridStart.getFullYear(), gridStart.getMonth(), gridStart.getDate() + index)
    cells.push({
      date,
      inMonth: date.getMonth() === first.getMonth(),
      isToday: localDateKey(date) === todayKey,
      key: localDateKey(date)
    })
  }
  return cells
})

function localDateKey(date: Date) {
  const month = `${date.getMonth() + 1}`.padStart(2, '0')
  const day = `${date.getDate()}`.padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

/** 事件按本地日分桶（next_run_at 缺失的不可调度条目不进网格）。 */
const eventsByDay = computed(() => {
  const buckets = new Map<string, SchedulerEventItem[]>()
  for (const event of monthEvents.value) {
    if (!event.next_run_at) continue
    const key = localDateKey(new Date(event.next_run_at))
    const bucket = buckets.get(key)
    if (bucket) {
      bucket.push(event)
    } else {
      buckets.set(key, [event])
    }
  }
  return buckets
})

function eventsOf(cell: CalendarCell) {
  return eventsByDay.value.get(cell.key) ?? []
}

function timeOf(event: SchedulerEventItem) {
  if (!event.next_run_at) return ''
  return new Intl.DateTimeFormat(i18n.global.locale.value, { hour: '2-digit', minute: '2-digit' }).format(
    new Date(event.next_run_at)
  )
}

const SOURCE_CLASS: Record<SchedulerEventType, string> = {
  scene: 'scheduler-event--scene',
  report: 'scheduler-event--report',
  rpc: 'scheduler-event--rpc'
}

// ---- 数据拉取 ----
async function fetchMonthEvents() {
  loading.value = true
  try {
    const { fromMs, toMs } = monthWindow.value
    const response = await getSchedulerEvents({
      from_ms: fromMs,
      to_ms: toMs,
      page_size: 500,
      source_type: sourceFilter.value || undefined
    })
    monthEvents.value = response.data?.list ?? []
  } finally {
    loading.value = false
  }
}

function shiftMonth(delta: number) {
  currentMonth.value = new Date(currentMonth.value.getFullYear(), currentMonth.value.getMonth() + delta, 1)
  void fetchMonthEvents()
}

function goToday() {
  currentMonth.value = startOfMonth(new Date())
  void fetchMonthEvents()
}

function onSourceFilterChange() {
  void fetchMonthEvents()
}

// ---- 注册面管理 ----
const registryOpen = ref(false)
const registryLoading = ref(false)
// 注册列表来自聚合面按 origin 过滤（聚合条目形状 SchedulerEventItem）。
const registryRows = ref<SchedulerEventItem[]>([])

const sourceTypeOptions = [
  { label: $t('page.schedulerCalendar.sourceScene'), value: 'scene' as const },
  { label: $t('page.schedulerCalendar.sourceReport'), value: 'report' as const },
  { label: $t('page.schedulerCalendar.sourceRpc'), value: 'rpc' as const }
]

async function openRegistry() {
  registryOpen.value = true
  registryLoading.value = true
  try {
    // 注册列表不带窗口：注册行不依赖当月窗口（含已停用/窗外登记行）。
    const response = await getSchedulerEvents({ page_size: 500 })
    registryRows.value = (response.data?.list ?? []).filter(item => item.origin === 'scheduler_registry')
  } finally {
    registryLoading.value = false
  }
}

const formOpen = ref(false)
const formSaving = ref(false)
const editingId = ref<string | null>(null)
const formModel = ref({
  name: '',
  event_type: 'scene' as SchedulerEventType,
  ref_id: '',
  cron: '',
  next_run_at: null as number | null,
  enabled: true
})

function openCreateForm() {
  editingId.value = null
  formModel.value = { name: '', event_type: 'scene', ref_id: '', cron: '', next_run_at: null, enabled: true }
  formOpen.value = true
}

function openEditForm(row: SchedulerEventItem) {
  editingId.value = row.id
  formModel.value = {
    name: row.name,
    event_type: row.source_type,
    ref_id: row.ref_id,
    cron: row.cron,
    next_run_at: row.next_run_at ? new Date(row.next_run_at).getTime() : null,
    enabled: row.enabled
  }
  formOpen.value = true
}

function validateForm() {
  if (!formModel.value.name.trim()) {
    message.error($t('page.schedulerCalendar.formNameRequired'))
    return false
  }
  if (formModel.value.event_type === 'scene' && !formModel.value.ref_id.trim()) {
    message.error($t('page.schedulerCalendar.formRefIdRequired'))
    return false
  }
  if (formModel.value.event_type !== 'rpc' && !formModel.value.cron.trim()) {
    message.error($t('page.schedulerCalendar.formCronRequired'))
    return false
  }
  if (formModel.value.event_type === 'rpc' && !formModel.value.next_run_at) {
    message.error($t('page.schedulerCalendar.formRunAtRequired'))
    return false
  }
  return true
}

async function submitForm() {
  if (!validateForm()) return
  formSaving.value = true
  try {
    const isRpc = formModel.value.event_type === 'rpc'
    if (editingId.value) {
      await updateSchedulerEvent(editingId.value, {
        name: formModel.value.name.trim(),
        ref_id: formModel.value.ref_id.trim() || undefined,
        cron: isRpc ? undefined : formModel.value.cron.trim(),
        next_run_at: isRpc && formModel.value.next_run_at ? new Date(formModel.value.next_run_at).toISOString() : undefined,
        enabled: formModel.value.enabled
      })
      message.success($t('common.updateSuccess'))
    } else {
      await createSchedulerEvent({
        name: formModel.value.name.trim(),
        event_type: formModel.value.event_type,
        ref_id: formModel.value.ref_id.trim() || undefined,
        cron: isRpc ? undefined : formModel.value.cron.trim(),
        next_run_at: isRpc && formModel.value.next_run_at ? new Date(formModel.value.next_run_at).toISOString() : undefined,
        enabled: formModel.value.enabled
      })
      message.success($t('page.schedulerCalendar.createSuccess'))
    }
    formOpen.value = false
    await openRegistry()
    await fetchMonthEvents()
  } finally {
    formSaving.value = false
  }
}

async function removeEvent(id: string) {
  await deleteSchedulerEvent(id)
  message.success($t('common.deleteSuccess'))
  await openRegistry()
  await fetchMonthEvents()
}

onMounted(() => {
  void fetchMonthEvents()
})
</script>

<template>
  <div class="scheduler-calendar">
    <NCard :title="$t('page.schedulerCalendar.heading')" :bordered="false">
      <template #header-extra>
        <NSpace align="center">
          <NButton size="small" @click="openRegistry">{{ $t('page.schedulerCalendar.registry') }}</NButton>
          <NButton size="small" @click="openCreateForm">{{ $t('page.schedulerCalendar.createEvent') }}</NButton>
        </NSpace>
      </template>

      <NSpace align="center" class="scheduler-toolbar" justify="space-between">
        <NSpace align="center">
          <NButton size="small" @click="shiftMonth(-1)">{{ $t('page.schedulerCalendar.prevMonth') }}</NButton>
          <NButton size="small" @click="goToday">{{ $t('page.schedulerCalendar.today') }}</NButton>
          <NButton size="small" @click="shiftMonth(1)">{{ $t('page.schedulerCalendar.nextMonth') }}</NButton>
          <span class="scheduler-month-label">{{ monthLabel }}</span>
        </NSpace>
        <NSpace align="center">
          <NSelect
            v-model:value="sourceFilter"
            size="small"
            clearable
            :options="sourceTypeOptions"
            :placeholder="$t('page.schedulerCalendar.sourceFilterPlaceholder')"
            class="scheduler-source-select"
            @update:value="onSourceFilterChange"
          />
          <span class="scheduler-legend">
            <span class="scheduler-dot scheduler-dot--scene" />{{ $t('page.schedulerCalendar.sourceScene') }}
            <span class="scheduler-dot scheduler-dot--report" />{{ $t('page.schedulerCalendar.sourceReport') }}
            <span class="scheduler-dot scheduler-dot--rpc" />{{ $t('page.schedulerCalendar.sourceRpc') }}
          </span>
        </NSpace>
      </NSpace>

      <NSpin :show="loading">
        <div class="scheduler-grid" role="grid">
          <div v-for="label in weekdayLabels" :key="label" class="scheduler-grid__weekday" role="columnheader">
            {{ label }}
          </div>
          <div
            v-for="cell in calendarCells"
            :key="cell.key"
            class="scheduler-grid__cell"
            :class="{ 'scheduler-grid__cell--outside': !cell.inMonth, 'scheduler-grid__cell--today': cell.isToday }"
            role="gridcell"
          >
            <div class="scheduler-grid__date">
              {{ cell.date.getDate() }}
              <span v-if="cell.isToday" class="scheduler-grid__today-badge">{{ $t('page.schedulerCalendar.today') }}</span>
            </div>
            <div
              v-for="event in eventsOf(cell)"
              :key="`${event.id}:${event.origin}`"
              class="scheduler-event"
              :class="SOURCE_CLASS[event.source_type]"
              :title="`${$t(`page.schedulerCalendar.source${event.source_type.charAt(0).toUpperCase()}${event.source_type.slice(1)}`)} · ${event.origin}${event.enabled ? '' : ' · ' + $t('page.schedulerCalendar.disabled')} · ${event.name} @ ${timeOf(event)}`"
            >
              <span class="scheduler-event__time">{{ timeOf(event) }}</span>
              <span class="scheduler-event__name">{{ event.name }}</span>
            </div>
          </div>
        </div>
        <NEmpty v-if="!loading && monthEvents.length === 0" :description="$t('page.schedulerCalendar.noEvents')" />
      </NSpin>
    </NCard>

    <NModal
      v-model:show="registryOpen"
      preset="card"
      :title="$t('page.schedulerCalendar.registryTitle')"
      class="scheduler-registry-modal"
    >
      <NSpace justify="end">
        <NButton size="small" type="primary" @click="openCreateForm">
          {{ $t('page.schedulerCalendar.createEvent') }}
        </NButton>
      </NSpace>
      <NSpin :show="registryLoading">
        <div v-if="registryRows.length === 0 && !registryLoading">
          <NEmpty :description="$t('page.schedulerCalendar.noRegistryRows')" />
        </div>
        <table v-else class="scheduler-registry-table">
          <thead>
            <tr>
              <th>{{ $t('page.schedulerCalendar.formName') }}</th>
              <th>{{ $t('page.schedulerCalendar.formType') }}</th>
              <th>{{ $t('page.schedulerCalendar.formRefId') }}</th>
              <th>{{ $t('page.schedulerCalendar.formCron') }}</th>
              <th>{{ $t('page.schedulerCalendar.nextRun') }}</th>
              <th>{{ $t('page.schedulerCalendar.enabled') }}</th>
              <th>{{ $t('page.schedulerCalendar.actions') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="row in registryRows" :key="row.id">
              <td>{{ row.name }}</td>
              <td>
                <NTag size="small" :type="row.source_type === 'scene' ? 'success' : row.source_type === 'report' ? 'info' : 'warning'">
                  {{ $t(`page.schedulerCalendar.source${row.source_type.charAt(0).toUpperCase()}${row.source_type.slice(1)}`) }}
                </NTag>
              </td>
              <td class="scheduler-registry-table__mono">{{ row.ref_id || '-' }}</td>
              <td class="scheduler-registry-table__mono">{{ row.cron || '-' }}</td>
              <td>{{ row.next_run_at ? formatDateTime(row.next_run_at) : '-' }}</td>
              <td>{{ row.enabled ? $t('page.schedulerCalendar.enabledOn') : $t('page.schedulerCalendar.disabled') }}</td>
              <td>
                <NSpace size="small">
                  <NButton size="tiny" @click="openEditForm(row)">{{ $t('page.schedulerCalendar.edit') }}</NButton>
                  <NPopconfirm @positive-click="removeEvent(row.id)">
                    <template #trigger>
                      <NButton size="tiny" type="error">{{ $t('page.schedulerCalendar.delete') }}</NButton>
                    </template>
                    {{ $t('page.schedulerCalendar.confirmDelete') }}
                  </NPopconfirm>
                </NSpace>
              </td>
            </tr>
          </tbody>
        </table>
      </NSpin>
    </NModal>

    <NModal
      v-model:show="formOpen"
      preset="card"
      :title="editingId ? $t('page.schedulerCalendar.editEvent') : $t('page.schedulerCalendar.createEvent')"
      class="scheduler-form-modal"
    >
      <NForm label-placement="left" label-width="110">
        <NFormItem :label="$t('page.schedulerCalendar.formName')" required>
          <NInput v-model:value="formModel.name" :maxlength="128" />
        </NFormItem>
        <NFormItem :label="$t('page.schedulerCalendar.formType')" required>
          <NSelect
            v-model:value="formModel.event_type"
            :options="sourceTypeOptions"
            :disabled="!!editingId"
          />
        </NFormItem>
        <NFormItem
          v-if="formModel.event_type !== 'report'"
          :label="$t('page.schedulerCalendar.formRefId')"
          :required="formModel.event_type === 'scene'"
        >
          <NInput v-model:value="formModel.ref_id" :maxlength="64" :placeholder="$t('page.schedulerCalendar.formRefIdHint')" />
        </NFormItem>
        <NFormItem
          v-if="formModel.event_type !== 'rpc'"
          :label="$t('page.schedulerCalendar.formCron')"
          required
        >
          <NInput v-model:value="formModel.cron" :maxlength="64" :placeholder="$t('page.schedulerCalendar.formCronHint')" />
        </NFormItem>
        <NFormItem v-if="formModel.event_type === 'rpc'" :label="$t('page.schedulerCalendar.formRunAt')" required>
          <NDatePicker v-model:value="formModel.next_run_at" type="datetime" clearable />
        </NFormItem>
        <NFormItem :label="$t('page.schedulerCalendar.enabled')">
          <NSwitch v-model:value="formModel.enabled" />
        </NFormItem>
      </NForm>
      <template #footer>
        <NSpace justify="end">
          <NButton @click="formOpen = false">{{ $t('page.schedulerCalendar.cancel') }}</NButton>
          <NButton type="primary" :loading="formSaving" @click="submitForm">
            {{ $t('page.schedulerCalendar.save') }}
          </NButton>
        </NSpace>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.scheduler-calendar {
  padding: 12px;
}

.scheduler-toolbar {
  margin-bottom: 12px;
}

.scheduler-month-label {
  font-size: 16px;
  font-weight: 600;
  min-width: 140px;
  text-align: center;
}

.scheduler-source-select {
  width: 160px;
}

.scheduler-legend {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
}

.scheduler-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-left: 8px;
}

.scheduler-dot--scene {
  background: rgb(var(--success-color));
}

.scheduler-dot--report {
  background: rgb(var(--info-color));
}

.scheduler-dot--rpc {
  background: rgb(var(--warning-color));
}

/* 月视图网格：纯 CSS Grid，7 列等宽，周一为首列。 */
.scheduler-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 4px;
}

.scheduler-grid__weekday {
  text-align: center;
  font-size: 12px;
  font-weight: 600;
  padding: 4px 0;
}

.scheduler-grid__cell {
  min-height: 96px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 4px;
  overflow: hidden;
}

.scheduler-grid__cell--outside {
  opacity: 0.45;
  background: var(--card-color);
}

.scheduler-grid__cell--today {
  border-color: rgb(var(--info-color));
  box-shadow: inset 0 0 0 1px rgb(var(--info-color));
}

.scheduler-grid__date {
  font-size: 12px;
  font-weight: 600;
  margin-bottom: 4px;
  display: flex;
  align-items: center;
  gap: 4px;
}

.scheduler-grid__today-badge {
  font-size: 10px;
  color: rgb(var(--info-color));
  border: 1px solid rgb(var(--info-color));
  border-radius: 3px;
  padding: 0 3px;
}

.scheduler-event {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 11px;
  line-height: 1.4;
  border-left: 3px solid transparent;
  border-radius: 2px;
  padding: 1px 4px;
  margin-bottom: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.scheduler-event__name {
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 来源类型着色（与图例 dot 同色系）。 */
.scheduler-event--scene {
  border-left-color: rgb(var(--success-color));
  background: rgb(var(--success-color) / 0.12);
}

.scheduler-event--report {
  border-left-color: rgb(var(--info-color));
  background: rgb(var(--info-color) / 0.12);
}

.scheduler-event--rpc {
  border-left-color: rgb(var(--warning-color));
  background: rgb(var(--warning-color) / 0.12);
}

.scheduler-registry-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.scheduler-registry-table th,
.scheduler-registry-table td {
  border-bottom: 1px solid var(--border-color);
  padding: 6px 8px;
  text-align: left;
}

.scheduler-registry-table__mono {
  font-family: monospace;
}

.scheduler-form-modal,
.scheduler-registry-modal {
  width: 720px;
}
</style>
