<!--
文件用途：告警列表顶部的分诊概览（当前页计数卡片 + 批量处置/证据下载按钮）。
核心逻辑：由当前页行数据派生分诊计数；按钮只 emit，由父级组合函数执行动作。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NCard, NFlex, NTag } from 'naive-ui'
import { $t } from '@/locales'
import { buildAlarmTriageSummary } from './alarm-configuration.helpers'

const props = defineProps<{
  rows: Array<{ alarm_status?: string; remark?: unknown }>
  total: number
  batchLoading: boolean
  canAcknowledge: boolean
  canReset: boolean
}>()

defineEmits<{
  download: []
  acknowledge: []
  reset: []
}>()

type TagType = 'default' | 'info' | 'success' | 'warning' | 'error'

const cards = computed(() => {
  const summary = buildAlarmTriageSummary(props.rows)
  const card = (key: string, labelKey: string, value: number, type: TagType) => ({
    key,
    label: $t(labelKey as any),
    value,
    type
  })
  return [
    card('active', 'custom.alarmPage.activeAlarms', summary.active, summary.active > 0 ? 'error' : 'success'),
    card('high', 'custom.alarmPage.highSeverity', summary.high, summary.high > 0 ? 'error' : 'default'),
    card(
      'unacknowledged',
      'custom.alarmPage.unacknowledged',
      summary.unacknowledged,
      summary.unacknowledged > 0 ? 'warning' : 'success'
    ),
    card('reset', 'custom.alarmPage.resetAlarms', summary.reset, 'info')
  ]
})

const description = computed(() =>
  $t('custom.alarmPage.triageDesc').replace('{page}', String(props.rows.length)).replace('{total}', String(props.total))
)
</script>

<template>
  <NCard embedded size="small" class="alarm-triage">
    <div class="alarm-triage-header">
      <div>
        <div class="alarm-triage-title">{{ $t('custom.alarmPage.triageTitle') }}</div>
        <div class="alarm-triage-desc">{{ description }}</div>
      </div>
      <NFlex :size="8" align="center" wrap>
        <NButton size="small" secondary data-testid="alarm-download-current-page-evidence" @click="$emit('download')">
          {{ $t('custom.alarmPage.downloadEvidenceBundle') }}
        </NButton>
        <NButton
          size="small"
          type="success"
          secondary
          :loading="batchLoading"
          :disabled="!canAcknowledge"
          @click="$emit('acknowledge')"
        >
          {{ $t('custom.alarmPage.acknowledgeSelected') }}
        </NButton>
        <NButton
          size="small"
          type="error"
          secondary
          :loading="batchLoading"
          :disabled="!canReset"
          @click="$emit('reset')"
        >
          {{ $t('custom.alarmPage.resetSelected') }}
        </NButton>
      </NFlex>
    </div>
    <div class="alarm-triage-cards">
      <div v-for="item in cards" :key="item.key" class="alarm-triage-card">
        <span>{{ item.label }}</span>
        <NTag :type="item.type" size="small">{{ item.value }}</NTag>
      </div>
    </div>
  </NCard>
</template>

<style scoped lang="scss">
.alarm-triage {
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

@media (max-width: 900px) {
  .alarm-triage-header {
    flex-direction: column;
  }

  .alarm-triage-cards {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@include mobile {
  .alarm-triage-cards {
    grid-template-columns: 1fr;
  }
}
</style>
