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
import { NButton, NCard, NEmpty, NSelect, NSpace, NSpin } from 'naive-ui'
import { getSchedulerEvents, type SchedulerEventItem, type SchedulerEventType } from '@/service/api'
import { $t, i18n } from '@/locales'
import { SOURCE_TYPES, monthWindow, shiftMonthDate, sourceLabelKey, startOfMonth } from './calendar-model'
import CalendarMonthGrid from './modules/CalendarMonthGrid.vue'
import SchedulerEventFormModal from './modules/SchedulerEventFormModal.vue'
import SchedulerRegistryModal from './modules/SchedulerRegistryModal.vue'

// ---- 月视图状态 ----
const currentMonth = ref(startOfMonth(new Date()))
const loading = ref(false)
const monthEvents = ref<SchedulerEventItem[]>([])
const sourceFilter = ref<SchedulerEventType | ''>('')

const monthLabel = computed(() =>
  new Intl.DateTimeFormat(i18n.global.locale.value, { year: 'numeric', month: 'long' }).format(currentMonth.value)
)

const sourceTypeOptions = SOURCE_TYPES.map((value) => ({ label: $t(sourceLabelKey(value)), value }))

// ---- 数据拉取 ----
// 快速连续翻月时旧请求可能晚到；序号守卫保证只有最新一次请求落地到网格。
let monthRequestSeq = 0
async function fetchMonthEvents() {
  const seq = ++monthRequestSeq
  loading.value = true
  try {
    const { fromMs, toMs } = monthWindow(currentMonth.value)
    const response = await getSchedulerEvents({
      from_ms: fromMs,
      to_ms: toMs,
      page_size: 500,
      source_type: sourceFilter.value || undefined
    })
    if (seq !== monthRequestSeq) return
    monthEvents.value = response.data?.list ?? []
  } finally {
    if (seq === monthRequestSeq) loading.value = false
  }
}

function shiftMonth(delta: number) {
  currentMonth.value = shiftMonthDate(currentMonth.value, delta)
  void fetchMonthEvents()
}

function goToday() {
  currentMonth.value = startOfMonth(new Date())
  void fetchMonthEvents()
}

// ---- 注册面管理（注册列表弹窗 + 创建/编辑弹窗） ----
const registryOpen = ref(false)
const registryRef = ref<InstanceType<typeof SchedulerRegistryModal> | null>(null)
const formOpen = ref(false)
const editingRow = ref<SchedulerEventItem | null>(null)

function openRegistry() {
  registryOpen.value = true
}

function openCreateForm() {
  editingRow.value = null
  formOpen.value = true
}

function openEditForm(row: SchedulerEventItem) {
  editingRow.value = row
  formOpen.value = true
}

async function onFormSaved() {
  // 保存后回到注册列表：已打开则刷新，未打开则打开（打开时弹窗自行拉取）。
  if (registryOpen.value) await registryRef.value?.reload()
  else registryOpen.value = true
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
            @update:value="fetchMonthEvents"
          />
          <span class="scheduler-legend">
            <span class="scheduler-dot scheduler-dot--scene" />{{ $t('page.schedulerCalendar.sourceScene') }}
            <span class="scheduler-dot scheduler-dot--report" />{{ $t('page.schedulerCalendar.sourceReport') }}
            <span class="scheduler-dot scheduler-dot--rpc" />{{ $t('page.schedulerCalendar.sourceRpc') }}
          </span>
        </NSpace>
      </NSpace>

      <NSpin :show="loading">
        <CalendarMonthGrid :month="currentMonth" :events="monthEvents" />
        <NEmpty v-if="!loading && monthEvents.length === 0" :description="$t('page.schedulerCalendar.noEvents')" />
      </NSpin>
    </NCard>

    <SchedulerRegistryModal
      ref="registryRef"
      v-model:show="registryOpen"
      @create="openCreateForm"
      @edit="openEditForm"
      @deleted="fetchMonthEvents"
    />

    <SchedulerEventFormModal
      v-model:show="formOpen"
      :row="editingRow"
      :source-type-options="sourceTypeOptions"
      @saved="onFormSaved"
    />
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
</style>
