<!--
  文件用途：设备诊断概览（纯展示）：三张成功率卡片 + 最近失败记录表。
  数据来源：父组件通过 `useDeviceDiagnosticsStats` 注入；本组件不发请求，只负责渲染与发出 `refresh`。
  状态反馈：loading 时刷新按钮转圈；error 时展示加载失败提示（保留上一次成功数据，不闪空）。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { Refresh } from '@vicons/ionicons5'

import { $t } from '@/locales'
import { createFailureRecordColumns } from './useDeviceDiagnosticsStats'
import type { DiagnosticsLoadStatus, FailureRecord, Statistics, StatisticsItem } from './useDeviceDiagnosticsStats'

const props = defineProps<{
  statistics: Statistics
  failureRecords: FailureRecord[]
  status: DiagnosticsLoadStatus
  error?: unknown
  nextSteps: string[]
}>()

const emit = defineEmits<{
  refresh: []
}>()

const columns = createFailureRecordColumns()

const statCards = computed<
  Array<{ key: string; title: string; type: 'info' | 'success' | 'warning'; item: StatisticsItem }>
>(() => [
  {
    key: 'uplink',
    title: $t('custom.device_details.uplinkSuccessRate'),
    type: 'info',
    item: props.statistics.uplink
  },
  {
    key: 'downlink',
    title: $t('custom.device_details.downlinkSuccessRate'),
    type: 'success',
    item: props.statistics.downlink
  },
  {
    key: 'storage',
    title: $t('custom.device_details.storageSuccessRate'),
    type: 'warning',
    item: props.statistics.storage
  }
])

const loading = computed(() => props.status === 'loading')

const errorDetail = computed(() => {
  const err = props.error as { message?: unknown } | string | null | undefined
  if (!err) return ''
  if (typeof err === 'string') return err
  return typeof err.message === 'string' ? err.message : ''
})
</script>

<template>
  <div>
    <!-- 统计概览 -->
    <div class="mb-4">
      <div class="flex items-center justify-between">
        <div class="text-18px">{{ $t('custom.device_details.statisticsOverview') }}</div>
        <NButton :bordered="false" :loading="loading" data-testid="diagnostics-refresh" @click="emit('refresh')">
          <NIcon size="18">
            <Refresh />
          </NIcon>
          {{ $t('common.refresh') }}
        </NButton>
      </div>
      <NAlert
        v-if="status === 'error'"
        type="error"
        class="mt-3"
        :title="$t('common.loadFailed')"
        data-testid="diagnostics-error"
      >
        <div class="text-13px leading-6">
          <div v-if="errorDetail">{{ errorDetail }}</div>
          <div v-for="step in nextSteps" :key="step">- {{ step }}</div>
        </div>
      </NAlert>
      <NFlex :gap="16" class="mt-4">
        <NCard v-for="card in statCards" :key="card.key" class="flex-1" :title="card.title">
          <NFlex vertical :gap="8">
            <NText :type="card.type" class="text-28px font-bold">
              <NNumberAnimation :from="0" :to="card.item.rate" :precision="1" />
              <span>%</span>
            </NText>
            <NText :depth="2" class="text-14px">
              {{ card.item.success }}/{{ card.item.total }}{{ $t('custom.device_details.diagnosisCountUnit') }}
            </NText>
          </NFlex>
        </NCard>
      </NFlex>
    </div>

    <!-- 最近失败记录 -->
    <div>
      <div class="flex items-center justify-between mb-4">
        <div class="text-18px">
          {{ $t('custom.device_details.recentFailureRecords') }}
        </div>
      </div>

      <NAlert
        v-if="failureRecords.length === 0 && status !== 'error'"
        type="info"
        class="mb-3"
        :title="$t('custom.device_details.diagnosisNoFailureRecords')"
      >
        <div class="text-13px leading-6">
          <div v-for="step in nextSteps" :key="step">- {{ step }}</div>
        </div>
      </NAlert>
      <NDataTable :columns="columns" :data="failureRecords" :loading="loading" :max-height="350" remote />
    </div>
  </div>
</template>

<style scoped lang="scss">
:deep(.n-card-header) {
  font-size: 16px;
}
</style>
