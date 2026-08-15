<script setup lang="ts">
import { computed } from 'vue'
import type { TelemetryControlItem } from './telemetryControlState'

const props = defineProps<{
  controlList: TelemetryControlItem[]
  controlListLoaded: boolean
  controlListLoading: boolean
  showLog: boolean
}>()

const emit = defineEmits<{
  (e: 'publish'): void
  (e: 'simulate'): void
  (e: 'load-controls'): void
  (e: 'control-change', item: TelemetryControlItem): void
}>()

const enabledControlCount = computed(() => props.controlList.length)
const controlCountText = computed(() => (props.controlListLoaded ? String(enabledControlCount.value) : '--'))
const simulationStatusType = computed(() => (props.showLog ? 'success' : 'default'))
const simulationStatusText = computed(() => (props.showLog ? 'MQTT' : '--'))

const getControlTitle = (item: TelemetryControlItem) => item.name || item.key || '--'
const getControlPayloadPreview = (item: TelemetryControlItem) => item.content || item.key || '--'
</script>

<template>
  <div class="telemetry-operations mb-4">
    <NFlex justify="space-between" align="center" :wrap="true" class="mb-3 telemetry-operations__header">
      <NFlex align="center" :wrap="true" :size="8">
        <div class="text-15px font-600">{{ $t('custom.device_details.commandDelivery') }}</div>
        <n-tag size="small" round type="info">
          {{ controlCountText }} {{ $t('custom.device_details.command') }}
        </n-tag>
        <n-tag size="small" round :type="simulationStatusType">
          {{ $t('generate.simulate-report-data') }}: {{ simulationStatusText }}
        </n-tag>
      </NFlex>

      <NFlex :wrap="true">
        <n-button type="primary" @click="emit('publish')">
          {{ $t('generate.issue-control') }}
        </n-button>
        <n-button secondary :loading="controlListLoading" @click="emit('load-controls')">
          {{ $t('custom.device_details.command') }}
        </n-button>
        <n-button v-if="showLog" type="primary" secondary @click="emit('simulate')">
          {{ $t('generate.simulate-report-data') }}
        </n-button>
      </NFlex>
    </NFlex>

    <NGrid v-if="controlList.length" x-gap="12" y-gap="12" cols="1 s:2 m:3 l:4" responsive="screen">
      <NGridItem v-for="item in controlList" :key="item.id">
        <NCard hoverable size="small" class="telemetry-operations__control-card">
          <NFlex justify="space-between" align="center" :wrap="false">
            <div class="min-w-0">
              <div class="ellipsis-text text-15px font-600" :title="getControlTitle(item)">
                {{ getControlTitle(item) }}
              </div>
              <div class="ellipsis-text text-12px text-gray-400" :title="getControlPayloadPreview(item)">
                {{ getControlPayloadPreview(item) }}
              </div>
            </div>
            <n-button size="small" tertiary type="primary" @click="emit('control-change', item)">
              {{ $t('custom.device_details.command') }}
            </n-button>
          </NFlex>
        </NCard>
      </NGridItem>
    </NGrid>

    <n-card v-else-if="controlListLoaded" embedded>
      <n-empty size="small" :description="$t('common.noData')">
        <template #extra>
          <n-button size="small" secondary type="primary" @click="emit('publish')">
            {{ $t('generate.issue-control') }}
          </n-button>
        </template>
      </n-empty>
    </n-card>
    <n-card v-else embedded>
      <n-empty size="small" :description="$t('common.noData')">
        <template #extra>
          <n-button size="small" secondary type="primary" :loading="controlListLoading" @click="emit('load-controls')">
            {{ $t('custom.device_details.command') }}
          </n-button>
        </template>
      </n-empty>
    </n-card>
  </div>
</template>

<style scoped>
.telemetry-operations__header {
  gap: 12px;
}

.telemetry-operations__control-card :deep(.n-card__content) {
  padding: 12px;
}
</style>
