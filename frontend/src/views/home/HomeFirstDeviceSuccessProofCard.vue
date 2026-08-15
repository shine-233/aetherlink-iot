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
        <n-button
          v-if="readyProof.ready"
          size="tiny"
          type="primary"
          ghost
          @click="emit('downloadSuccessProof')"
        >
          {{ $t('custom.home.firstDevice.proof.downloadDeliveryProof') }}
        </n-button>
        <n-tag v-else size="small" round :bordered="false" type="warning">
          {{ firstBlockingProofItem?.label || $t('custom.home.firstDevice.proof.waitingProofItem') }}
        </n-tag>
      </div>

      <div class="mt-10px grid gap-6px">
        <div v-for="item in readyProof.items" :key="item.key" class="first-device-proof-row">
          <div class="min-w-0">
            <span>{{ item.label }}</span>
            <small>{{ item.detail }}</small>
          </div>
          <strong :class="item.ok ? 'text-green-600' : 'text-orange-600'">
            {{
              item.ok
                ? $t('custom.home.firstDevice.proof.passed')
                : $t('custom.home.firstDevice.proof.pending')
            }}
          </strong>
        </div>
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
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #fff;
}

.first-device-operation-check {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  padding: 8px 10px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  background: #f8fafc;
}

.first-device-operation-check strong {
  display: block;
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 12px;
}

.first-device-operation-check small {
  display: block;
  margin-top: 3px;
  color: #64748b;
  font-size: 11px;
  line-height: 1.45;
}

.first-device-chart-generation-hint {
  display: grid;
  gap: 3px;
  margin-top: 8px;
  padding: 8px 10px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #eff6ff;
}

.first-device-chart-generation-hint strong,
.first-device-chart-generation-hint small {
  display: block;
  overflow-wrap: anywhere;
}

.first-device-chart-generation-hint strong {
  color: #0f172a;
  font-size: 12px;
}

.first-device-chart-generation-hint small {
  color: #475569;
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
  border: 1px solid #bfdbfe;
  border-radius: 6px;
  background: #eff6ff;
}

.first-device-chart-summary span {
  display: block;
  color: #2563eb;
  font-size: 12px;
}

.first-device-chart-summary strong {
  display: block;
  overflow-wrap: anywhere;
  color: #0f172a;
  font-size: 14px;
}

.first-device-chart-summary small {
  display: block;
  margin-top: 3px;
  color: #64748b;
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
  background: #fff;
}

.first-device-bar span {
  display: block;
  color: #64748b;
  font-size: 12px;
}

.first-device-bar strong {
  display: block;
  overflow-wrap: anywhere;
  color: #0f172a;
}

.first-device-bar-track {
  width: 100%;
  height: 6px;
  margin-top: 5px;
  overflow: hidden;
  border-radius: 999px;
  background: #e2e8f0;
}

.first-device-bar-fill {
  height: 100%;
  border-radius: inherit;
  background: #22c55e;
}

.first-device-handoff-hint {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  margin-top: 10px;
  padding: 10px;
  border: 1px solid #fed7aa;
  border-radius: 8px;
  background: #fff7ed;
}

.first-device-handoff-hint--ready {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.first-device-handoff-hint strong,
.first-device-handoff-hint small {
  display: block;
  overflow-wrap: anywhere;
}

.first-device-handoff-hint strong {
  color: #0f172a;
  font-size: 12px;
}

.first-device-handoff-hint small {
  margin-top: 4px;
  color: #475569;
  font-size: 11px;
  line-height: 1.45;
}

.first-device-proof-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  font-size: 12px;
}

.first-device-proof-row small {
  display: block;
  margin-top: 2px;
  color: #94a3b8;
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
