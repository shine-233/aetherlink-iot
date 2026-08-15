<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import dayjs from 'dayjs'
import { deviceAlarmHistory } from '@/service/api'
import { $t } from '@/locales'

type AlarmLevel = 'H' | 'M' | 'L' | 'N'

interface AlarmSummaryRecord {
  id?: string
  name?: string
  content?: string
  description?: string
  alarm_status?: AlarmLevel | string
  create_at?: string
  [key: string]: unknown
}

const props = defineProps<{
  deviceId: string
}>()

const emit = defineEmits<{
  (event: 'open', alarm: AlarmSummaryRecord): void
}>()

const loading = ref(false)
const loadFailed = ref(false)
const currentAlarm = ref<AlarmSummaryRecord | null>(null)
const recentAlarm = ref<AlarmSummaryRecord | null>(null)
let requestSeq = 0

function firstAlarm(response: any): AlarmSummaryRecord | null {
  const list = response?.data?.list
  return Array.isArray(list) && list.length > 0 ? (list[0] as AlarmSummaryRecord) : null
}

async function loadAlarmSummary() {
  const deviceId = props.deviceId.trim()
  if (!deviceId) {
    requestSeq += 1
    currentAlarm.value = null
    recentAlarm.value = null
    return
  }

  const currentRequestSeq = ++requestSeq
  loading.value = true
  loadFailed.value = false
  const baseQuery = { device_id: deviceId, page: 1, page_size: 1 }
  try {
    const [recentResponse, activeResponse] = await Promise.all([
      deviceAlarmHistory(baseQuery),
      deviceAlarmHistory({ ...baseQuery, alarm_status: 'ACTIVE' })
    ])
    if ([recentResponse, activeResponse].some((response) => response?.error)) {
      throw new Error('alarm summary request failed')
    }
    if (currentRequestSeq !== requestSeq) return
    recentAlarm.value = firstAlarm(recentResponse)
    currentAlarm.value = firstAlarm(activeResponse)
  } catch {
    if (currentRequestSeq !== requestSeq) return
    currentAlarm.value = null
    recentAlarm.value = null
    loadFailed.value = true
  } finally {
    if (currentRequestSeq === requestSeq) loading.value = false
  }
}

function alarmLevelLabel(status: unknown) {
  switch (status) {
    case 'H':
      return $t('common.highAlarm')
    case 'M':
      return $t('common.intermediateAlarm')
    case 'L':
      return $t('common.lowAlarm')
    case 'N':
      return $t('common.normal')
    default:
      return '-'
  }
}

function alarmTagType(status: unknown) {
  if (status === 'H') return 'error'
  if (status === 'M') return 'warning'
  if (status === 'L') return 'info'
  return 'success'
}

function alarmTitle(alarm: AlarmSummaryRecord) {
  return String(alarm.name || alarm.content || '-')
}

function alarmDescription(alarm: AlarmSummaryRecord) {
  return String(alarm.content || alarm.description || '-')
}

function formatAlarmTime(value: unknown) {
  if (!value) return '-'
  const parsed = dayjs(String(value))
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : '-'
}

watch(
  () => props.deviceId,
  () => {
    void loadAlarmSummary()
  }
)

onMounted(() => {
  void loadAlarmSummary()
})

defineExpose({ refresh: loadAlarmSummary })
</script>

<template>
  <div>
    <NAlert v-if="loadFailed" type="warning" :show-icon="false" class="mb-3">
      {{ $t('common.loadFailure') }}
    </NAlert>
    <div class="alarm-summary-grid">
      <NCard size="small" :title="$t('rdi.overview.activeAlarms')">
        <NSpin :show="loading">
          <template v-if="currentAlarm">
            <NFlex justify="space-between" align="center" :wrap="false">
              <strong>{{ alarmTitle(currentAlarm) }}</strong>
              <NTag :type="alarmTagType(currentAlarm.alarm_status)" size="small">
                {{ alarmLevelLabel(currentAlarm.alarm_status) }}
              </NTag>
            </NFlex>
            <p class="alarm-summary-description">{{ alarmDescription(currentAlarm) }}</p>
            <NFlex justify="space-between" align="center">
              <span class="alarm-summary-time">{{ formatAlarmTime(currentAlarm.create_at) }}</span>
              <NButton text type="primary" size="small" @click="emit('open', currentAlarm)">
                {{ $t('custom.devicePage.details') }}
              </NButton>
            </NFlex>
          </template>
          <NEmpty v-else size="small" :description="$t('card.alarmInfo.noAlarms')" />
        </NSpin>
      </NCard>

      <NCard size="small" :title="$t('rdi.overview.mostRecentAlert')">
        <NSpin :show="loading">
          <template v-if="recentAlarm">
            <NFlex justify="space-between" align="center" :wrap="false">
              <strong>{{ alarmTitle(recentAlarm) }}</strong>
              <NTag :type="alarmTagType(recentAlarm.alarm_status)" size="small">
                {{ alarmLevelLabel(recentAlarm.alarm_status) }}
              </NTag>
            </NFlex>
            <p class="alarm-summary-description">{{ alarmDescription(recentAlarm) }}</p>
            <NFlex justify="space-between" align="center">
              <span class="alarm-summary-time">{{ formatAlarmTime(recentAlarm.create_at) }}</span>
              <NButton text type="primary" size="small" @click="emit('open', recentAlarm)">
                {{ $t('custom.devicePage.details') }}
              </NButton>
            </NFlex>
          </template>
          <NEmpty v-else size="small" :description="$t('card.alarmInfo.noAlarms')" />
        </NSpin>
      </NCard>
    </div>
  </div>
</template>

<style scoped>
.alarm-summary-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin-bottom: 16px;
}

.alarm-summary-description {
  min-height: 20px;
  margin: 10px 0;
  color: var(--n-text-color-2);
}

.alarm-summary-time {
  color: var(--n-text-color-3);
  font-size: 12px;
}

@media (max-width: 800px) {
  .alarm-summary-grid {
    grid-template-columns: 1fr;
  }
}
</style>
