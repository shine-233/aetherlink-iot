<script setup lang="ts">
import { $t } from '@/locales'
import type { OtaTaskPreflightSummary, OtaTaskRiskDevice } from './ota-task-state'
import type { OtaTaskPreflightItem } from './ota-task-preflight-view'

defineProps<{
  summary: OtaTaskPreflightSummary
  items: OtaTaskPreflightItem[]
  riskDevices: OtaTaskRiskDevice[]
}>()
</script>

<template>
  <NCard embedded size="small" class="ota-preflight-card">
    <NSpace vertical size="small">
      <NSpace align="center" justify="space-between" :wrap="true">
        <div class="font-600">{{ $t('page.product.update-ota.preflightTitle') }}</div>
        <NTag :type="summary.riskCount ? 'warning' : 'success'" round>
          {{ summary.riskCount ? $t('page.product.update-ota.preflightNeedsAttention') : $t('common.normal') }}
        </NTag>
      </NSpace>
      <NGrid x-gap="8" y-gap="8" cols="2 s:3 m:5" responsive="screen">
        <NGridItem v-for="item in items" :key="item.key">
          <div class="ota-preflight-card__metric">
            <span>{{ item.label }}</span>
            <NTag :type="item.type" round>{{ item.value }}</NTag>
          </div>
        </NGridItem>
      </NGrid>
      <NAlert v-if="summary.riskCount" type="warning" :show-icon="true">
        {{ $t('page.product.update-ota.preflightRiskHint') }}
      </NAlert>
      <div v-if="riskDevices.length" class="ota-risk-list">
        <div class="ota-risk-list__title">{{ $t('page.product.update-ota.preflightRiskDevices') }}</div>
        <div v-for="item in riskDevices.slice(0, 5)" :key="item.id" class="ota-risk-list__row">
          <div class="ota-risk-list__device">
            <span class="ota-risk-list__name">{{ item.label || item.id }}</span>
            <span class="ota-risk-list__version">
              {{ $t('page.product.update-ota.preflightCurrentVersion') }}:
              {{ item.currentVersion || '-' }}
            </span>
          </div>
          <NSpace size="small" :wrap="true">
            <NTag v-for="reasonKey in item.reasonKeys" :key="reasonKey" type="warning" round>
              {{ $t(reasonKey) }}
            </NTag>
          </NSpace>
        </div>
        <div v-if="riskDevices.length > 5" class="ota-risk-list__more">
          {{ $t('page.product.update-ota.preflightMoreRisks') }} {{ riskDevices.length - 5 }}
        </div>
      </div>
    </NSpace>
  </NCard>
</template>

<style scoped>
.ota-preflight-card__metric {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 8px;
  background: var(--card-color);
  color: var(--text-color-2);
  font-size: 12px;
}

.ota-risk-list {
  display: grid;
  gap: 8px;
}

.ota-risk-list__title {
  color: var(--text-color-2);
  font-size: 12px;
  font-weight: 600;
}

.ota-risk-list__row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
}

.ota-risk-list__device {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.ota-risk-list__name {
  overflow: hidden;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ota-risk-list__version,
.ota-risk-list__more {
  color: var(--text-color-3);
  font-size: 12px;
}
</style>
