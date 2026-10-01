<!--
  文件用途：调度日历月视图网格（纯 CSS Grid，7 列，周一为首列）。
  核心逻辑：接收当月与事件列表，本组件内完成网格计算与按日分桶；来源类型着色。
-->
<script setup lang="ts">
import { computed } from 'vue'
import type { SchedulerEventItem } from '@/service/api'
import { $t, i18n } from '@/locales'
import { SOURCE_CLASS, bucketEventsByDay, buildCalendarCells, sourceLabelKey, weekdayLabels } from '../calendar-model'

const props = defineProps<{
  month: Date
  events: SchedulerEventItem[]
}>()

const locale = computed(() => i18n.global.locale.value)
const labels = computed(() => weekdayLabels(locale.value))
const cells = computed(() => buildCalendarCells(props.month))
const eventsByDay = computed(() => bucketEventsByDay(props.events))
// 同一 locale 下复用 formatter：每个事件每次渲染都 new Intl.DateTimeFormat 代价不小。
const timeFormatter = computed(() => new Intl.DateTimeFormat(locale.value, { hour: '2-digit', minute: '2-digit' }))

function timeOf(event: SchedulerEventItem) {
  return event.next_run_at ? timeFormatter.value.format(new Date(event.next_run_at)) : ''
}

function titleOf(event: SchedulerEventItem) {
  const disabled = event.enabled ? '' : ` · ${$t('page.schedulerCalendar.disabled')}`
  return `${$t(sourceLabelKey(event.source_type))} · ${event.origin}${disabled} · ${event.name} @ ${timeOf(event)}`
}
</script>

<template>
  <div class="scheduler-grid" role="grid">
    <div v-for="label in labels" :key="label" class="scheduler-grid__weekday" role="columnheader">
      {{ label }}
    </div>
    <div
      v-for="cell in cells"
      :key="cell.key"
      class="scheduler-grid__cell"
      :class="{ 'scheduler-grid__cell--outside': !cell.inMonth, 'scheduler-grid__cell--today': cell.isToday }"
      role="gridcell"
      :aria-current="cell.isToday ? 'date' : undefined"
    >
      <div class="scheduler-grid__date">
        {{ cell.date.getDate() }}
        <span v-if="cell.isToday" class="scheduler-grid__today-badge">{{ $t('page.schedulerCalendar.today') }}</span>
      </div>
      <div
        v-for="event in eventsByDay.get(cell.key) ?? []"
        :key="`${event.id}:${event.origin}`"
        class="scheduler-event"
        :class="SOURCE_CLASS[event.source_type]"
        :title="titleOf(event)"
      >
        <span class="scheduler-event__time">{{ timeOf(event) }}</span>
        <span class="scheduler-event__name">{{ event.name }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
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
</style>
