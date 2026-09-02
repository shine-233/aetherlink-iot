<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'

interface DeploymentHealthRow {
  key: string
  ok: boolean
  label: string
  description?: string
  sourceLabel?: string
  nextAction?: string
  error?: string
  latency?: number | string
}

const props = defineProps<{
  deploymentHealthLoading: boolean
  deploymentHealthOk: boolean
  deploymentHealthRows: DeploymentHealthRow[]
}>()

const emit = defineEmits<{
  refreshDeploymentHealth: []
}>()

const firstFailedDeploymentHealthRow = computed(() => props.deploymentHealthRows.find((row) => !row.ok) || null)
const deploymentHealthPassedCount = computed(() => props.deploymentHealthRows.filter((row) => row.ok).length)
const deploymentHealthFailedCount = computed(() =>
  Math.max(props.deploymentHealthRows.length - deploymentHealthPassedCount.value, 0)
)
const deploymentHealthOverview = computed(() => {
  if (!props.deploymentHealthRows.length) {
    return {
      title: $t('custom.home.firstDevice.health.overview.waitingTitle'),
      detail: $t('custom.home.firstDevice.health.overview.waitingDetail'),
      type: 'default'
    }
  }
  if (props.deploymentHealthOk) {
    return {
      title: $t('custom.home.firstDevice.health.overview.readyTitle'),
      detail: $t('custom.home.firstDevice.health.overview.readyDetail'),
      type: 'success'
    }
  }
  return {
    title: $t('custom.home.firstDevice.health.overview.failedTitle', {
      count: deploymentHealthFailedCount.value
    }),
    detail:
      firstFailedDeploymentHealthRow.value?.nextAction ||
      firstFailedDeploymentHealthRow.value?.error ||
      $t('custom.home.firstDevice.health.overview.failedFallback'),
    type: 'warning'
  }
})
</script>

<template>
  <div class="rounded-6px bg-gray-50 px-12px py-10px">
    <div class="flex items-center justify-between gap-8px">
      <div>
        <div class="text-12px text-gray-500">{{ $t('custom.home.firstDevice.health.prerequisite') }}</div>
        <div class="mt-2px font-600">{{ $t('custom.home.firstDevice.health.title') }}</div>
      </div>
      <n-button size="tiny" :loading="deploymentHealthLoading" @click="emit('refreshDeploymentHealth')">
        {{ $t('custom.home.firstDevice.health.check') }}
      </n-button>
    </div>
    <div class="mt-4px text-gray-500">
      {{
        deploymentHealthOk
          ? $t('custom.home.firstDevice.health.readyDesc')
          : $t('custom.home.firstDevice.health.pendingDesc')
      }}
    </div>
    <div class="first-device-health-overview" :class="`first-device-health-overview--${deploymentHealthOverview.type}`">
      <div class="min-w-0">
        <strong>{{ deploymentHealthOverview.title }}</strong>
        <small>{{ deploymentHealthOverview.detail }}</small>
      </div>
      <div class="first-device-health-overview__score">
        <span>{{ $t('custom.home.firstDevice.health.passed') }}</span>
        <strong>{{ deploymentHealthPassedCount }}/{{ deploymentHealthRows.length || 5 }}</strong>
        <small v-if="deploymentHealthFailedCount">
          {{ $t('custom.home.firstDevice.health.failedCount', { count: deploymentHealthFailedCount }) }}
        </small>
      </div>
    </div>

    <div v-if="firstFailedDeploymentHealthRow" class="mt-6px text-12px line-height-18px text-red-600">
      {{ $t('custom.home.firstDevice.health.priorityPrefix', { label: firstFailedDeploymentHealthRow.label }) }} -
      {{
        firstFailedDeploymentHealthRow.error ||
        firstFailedDeploymentHealthRow.description ||
        $t('custom.home.firstDevice.health.checkFailed')
      }}
      <div v-if="firstFailedDeploymentHealthRow.nextAction" class="mt-2px text-orange-600">
        {{ $t('custom.home.firstDevice.common.nextStepPrefix', { label: firstFailedDeploymentHealthRow.nextAction }) }}
      </div>
    </div>
    <div v-if="deploymentHealthRows.length" class="mt-8px grid gap-6px">
      <div v-for="row in deploymentHealthRows" :key="row.key" class="first-device-health-row">
        <div class="min-w-0">
          <span>{{ row.label }}</span>
          <small>{{ row.description }}</small>
          <small v-if="row.sourceLabel">
            {{ $t('custom.home.firstDevice.health.sourcePrefix', { label: row.sourceLabel }) }}
          </small>
          <small v-if="!row.ok && row.nextAction" class="first-device-health-next-action">
            {{ $t('custom.home.firstDevice.common.nextStepPrefix', { label: row.nextAction }) }}
          </small>
        </div>
        <strong :class="row.ok ? 'text-green-600' : 'text-red-600'">
          {{
            row.ok
              ? $t('custom.home.firstDevice.health.normalLatency', { latency: row.latency })
              : row.error || $t('custom.home.firstDevice.health.abnormal')
          }}
        </strong>
      </div>
    </div>
    <n-empty v-else size="small" class="mt-8px justify-center">
      {{ $t('custom.home.firstDevice.health.noResult') }}
    </n-empty>
  </div>
</template>

<style scoped>
.first-device-health-overview {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  margin-top: 8px;
  padding: 10px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
}

.first-device-health-overview--success {
  border-color: #bbf7d0;
  background: #f0fdf4;
}

.first-device-health-overview--warning {
  border-color: #fed7aa;
  background: #fff7ed;
}

.first-device-health-overview strong,
.first-device-health-overview small,
.first-device-health-overview span {
  display: block;
  overflow-wrap: anywhere;
}

.first-device-health-overview strong {
  color: #0f172a;
  font-size: 12px;
}

.first-device-health-overview small,
.first-device-health-overview span {
  color: #64748b;
  font-size: 11px;
  line-height: 1.45;
}

.first-device-health-overview__score {
  min-width: 72px;
  text-align: right;
}

.first-device-health-overview__score strong {
  font-size: 16px;
}

.first-device-health-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  font-size: 12px;
}

.first-device-health-row small {
  display: block;
  margin-top: 2px;
  color: #94a3b8;
  font-size: 11px;
  line-height: 1.4;
}

.first-device-health-row .first-device-health-next-action {
  color: #c2410c;
}

.first-device-health-row strong {
  min-width: 0;
  overflow-wrap: anywhere;
  text-align: right;
}

@media (max-width: 900px) {
  .first-device-health-overview {
    flex-direction: column;
  }
}
</style>
