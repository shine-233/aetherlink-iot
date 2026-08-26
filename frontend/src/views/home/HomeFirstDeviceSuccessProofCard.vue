<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'

const { readyProof } = defineProps<{
  facts: any[]
  chart: any
  readyProof: any
}>()

const emit = defineEmits<{
  copyChartProof: []
  downloadSuccessProof: []
}>()

const remainingProofCount = computed(() => readyProof.items?.filter((item: any) => !item.ok).length || 0)
const firstBlockingProofItem = computed(() => readyProof.items?.find((item: any) => !item.ok) || null)
</script>

<template>
  <div class="first-device-success-proof-grid">
    <div class="first-device-success-proof-card">
      <div class="font-600">{{ $t('custom.home.firstDevice.proof.latestTelemetryAndChart') }}</div>
      <div class="first-device-chart-generation-hint">
        <strong>
          {{
            chart.ready
              ? $t('custom.home.firstDevice.proof.chartGenerated')
              : $t('custom.home.firstDevice.proof.chartWillGenerate')
          }}
        </strong>
        <small>
          {{
            chart.ready
              ? $t('custom.home.firstDevice.proof.chartSourceSummary', {
                  source:
                    chart.generatedFrom === 'browser_test'
                      ? $t('custom.home.firstDevice.proof.chartSourceBrowserTest')
                      : $t('custom.home.firstDevice.proof.chartSourceLatestTelemetry'),
                  field: chart.primaryKey || 'telemetry',
                  points: chart.points?.length || 0
                })
              : $t('custom.home.firstDevice.proof.chartPendingHint')
          }}
        </small>
      </div>
      <div class="mt-8px grid gap-6px">
        <div v-for="fact in facts" :key="fact.key" class="first-device-operation-check">
          <div class="min-w-0">
            <strong>{{ fact.label }}</strong>
            <small>{{ fact.value }}</small>
          </div>
        </div>
      </div>
      <div v-if="chart.ready" class="first-device-chart">
        <div class="first-device-chart-summary">
          <div>
            <span>
              {{
                chart.generatedFrom === 'browser_test'
                  ? $t('custom.home.firstDevice.proof.chartSourceBrowserTest')
                  : $t('custom.home.firstDevice.proof.chartSummaryFromTelemetry')
              }}
            </span>
            <strong>{{ chart.primaryKey }} = {{ chart.primaryValue }}</strong>
          </div>
          <n-button size="tiny" type="primary" ghost @click="emit('copyChartProof')">
            {{ $t('custom.home.firstDevice.proof.copyChartProof') }}
          </n-button>
          <n-button size="tiny" secondary @click="emit('downloadSuccessProof')">
            {{ $t('custom.home.firstDevice.common.downloadSuccessProof') }}
          </n-button>
          <small>{{ chart.summary }}</small>
        </div>
        <div v-for="point in chart.points" :key="point.key" class="first-device-bar">
          <div class="min-w-0 flex-1">
            <span>{{ point.key }}</span>
            <div class="first-device-bar-track">
              <div class="first-device-bar-fill" :style="{ width: `${point.barPercent}%` }"></div>
            </div>
          </div>
          <strong>{{ point.value }}</strong>
        </div>
      </div>
      <div v-else class="mt-8px text-12px line-height-18px text-gray-500">
        {{ $t('custom.home.firstDevice.proof.firstTelemetryHint') }}
      </div>
    </div>

    <div class="first-device-success-proof-card">
      <div class="flex flex-col gap-8px sm:flex-row sm:items-start sm:justify-between">
        <div class="min-w-0">
          <div class="font-600">{{ readyProof.title }}</div>
          <div class="mt-4px text-12px line-height-18px text-gray-500">
            {{ readyProof.summary }}
          </div>
        </div>
        <n-tag size="small" round :bordered="false" :type="readyProof.ready ? 'success' : 'warning'">
          {{
            readyProof.ready
              ? $t('custom.home.firstDevice.proof.deliverable')
              : $t('custom.home.firstDevice.proof.keepGoing')
          }}
        </n-tag>
      </div>
      <div class="first-device-handoff-hint" :class="{ 'first-device-handoff-hint--ready': readyProof.ready }">
        <div class="min-w-0">
          <strong>
            {{
              readyProof.ready
                ? $t('custom.home.firstDevice.proof.readyToDeliver')
                : $t('custom.home.firstDevice.proof.remainingSteps', { count: remainingProofCount })
            }}
          </strong>
          <small>
            {{
              readyProof.ready
                ? $t('custom.home.firstDevice.proof.readyToDeliverDetail')
                : firstBlockingProofItem?.detail || $t('custom.home.firstDevice.proof.blockedFallbackDetail')
            }}
          </small>
        </div>
        <n-button v-if="readyProof.ready" size="tiny" type="primary" ghost @click="emit('downloadSuccessProof')">
          {{ $t('custom.home.firstDevice.proof.downloadDeliveryProof') }}
        </n-button>
        <n-tag v-else size="small" round :bordered="false" type="warning">
          {{ firstBlockingProofItem?.label || $t('custom.home.firstDevice.proof.waitingProofItem') }}
        </n-tag>
      </div>

      <div class="mt-10px grid gap-6px">
        <template v-if="readyProof.items?.length">
          <div v-for="item in readyProof.items" :key="item.key" class="first-device-proof-row">
            <div class="min-w-0">
              <span>{{ item.label }}</span>
              <small>{{ item.detail }}</small>
            </div>
            <strong :class="item.ok ? 'text-green-600' : 'text-orange-600'">
              {{ item.ok ? $t('custom.home.firstDevice.proof.passed') : $t('custom.home.firstDevice.proof.pending') }}
            </strong>
          </div>
        </template>
        <n-empty v-else :description="$t('common.noData')" />
      </div>
    </div>
  </div>
</template>

<style scoped>
.first-device-success-proof-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr);
  gap: 12px;
}

.first-device-success-proof-card {
  min-width: 0;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--card-color);
}

.first-device-operation-check {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--action-color);
}

.first-device-operation-check strong {
  display: block;
  overflow-wrap: anywhere;
  color: var(--text-color-1);
  font-size: var(--font-size-caption);
}

.first-device-operation-check small {
  display: block;
  margin-top: 3px;
  color: var(--text-color-3);
  font-size: 11px;
  line-height: 1.45;
}

.first-device-chart-generation-hint {
  display: grid;
  gap: 3px;
  margin-top: 8px;
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: rgb(var(--info-color) / 0.08);
}

.first-device-chart-generation-hint strong,
.first-device-chart-generation-hint small {
  display: block;
  overflow-wrap: anywhere;
}

.first-device-chart-generation-hint strong {
  color: var(--text-color-1);
  font-size: var(--font-size-caption);
}

.first-device-chart-generation-hint small {
  color: var(--text-color-2);
  font-size: 11px;
  line-height: 1.45;
}

.first-device-chart {
  display: grid;
  gap: 8px;
  margin-top: 8px;
}

.first-device-chart-summary {
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: rgb(var(--info-color) / 0.08);
}

.first-device-chart-summary span {
  display: block;
  color: rgb(var(----color));
  font-size: var(--font-size-caption);
}

.first-device-chart-summary strong {
  display: block;
  overflow-wrap: anywhere;
  color: var(--text-color-1);
  font-size: var(--font-size-base);
}

.first-device-chart-summary small {
  display: block;
  margin-top: 3px;
  color: var(--text-color-3);
  line-height: 1.4;
}

.first-device-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-width: 0;
  padding: 8px 10px;
  border-radius: 6px;
  background: var(--card-color);
}

.first-device-bar span {
  display: block;
  color: var(--text-color-3);
  font-size: var(--font-size-caption);
}

.first-device-bar strong {
  display: block;
  overflow-wrap: anywhere;
  color: var(--text-color-1);
}

.first-device-bar-track {
  width: 100%;
  height: 6px;
  margin-top: 5px;
  overflow: hidden;
  border-radius: var(--radius-pill);
  background: var(--action-color);
}

.first-device-bar-fill {
  height: 100%;
  border-radius: inherit;
  background: rgb(var(--success-color));
}

.first-device-handoff-hint {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  margin-top: 10px;
  padding: 10px;
  border: 1px solid rgb(var(--warning-color) / 0.6);
  border-radius: var(--radius-md);
  background: rgb(var(--warning-color) / 0.1);
}

.first-device-handoff-hint--ready {
  border-color: rgb(var(--success-color) / 0.5);
  background: rgb(var(--success-color) / 0.07);
}

.first-device-handoff-hint strong,
.first-device-handoff-hint small {
  display: block;
  overflow-wrap: anywhere;
}

.first-device-handoff-hint strong {
  color: var(--text-color-1);
  font-size: var(--font-size-caption);
}

.first-device-handoff-hint small {
  margin-top: 4px;
  color: var(--text-color-2);
  font-size: 11px;
  line-height: 1.45;
}

.first-device-proof-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  font-size: var(--font-size-caption);
}

.first-device-proof-row small {
  display: block;
  margin-top: 2px;
  color: var(--text-color-3);
  font-size: 11px;
  line-height: 1.4;
}

.first-device-proof-row strong {
  min-width: 0;
  overflow-wrap: anywhere;
  text-align: right;
}

@media (max-width: 900px) {
  .first-device-success-proof-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .first-device-handoff-hint {
    flex-direction: column;
  }
}
</style>
