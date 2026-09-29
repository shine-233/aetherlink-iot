<!--
  Read-only summary of the currently selected fleet devices (counts + copyable identifiers).
-->
<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'
import './device-fleet-modal.css'

export interface SelectedFleetSummary {
  total: number
  online: number
  offline: number
  alarmed: number
  missingVersion: number
}

const props = defineProps<{
  summary: SelectedFleetSummary
  deviceIdentifiers: string
}>()

const visible = defineModel<boolean>('show', { required: true })

const emit = defineEmits<{ copy: [] }>()

const hasRisk = computed(
  () => props.summary.offline > 0 || props.summary.alarmed > 0 || props.summary.missingVersion > 0
)
</script>

<template>
  <NModal v-model:show="visible" preset="card" class="max-w-640px">
    <template #header>{{ $t('custom.devicePage.selectedDeviceSummaryTitle') }}</template>
    <NFlex vertical :size="12">
      <NAlert type="info" :show-icon="false">
        {{ $t('custom.devicePage.selectedDeviceSummaryHint') }}
      </NAlert>
      <div class="selected-device-summary-grid">
        <div class="selected-device-summary-item">
          <span>{{ $t('custom.devicePage.selectedDeviceSummaryTotal') }}</span>
          <strong>{{ summary.total }}</strong>
        </div>
        <div class="selected-device-summary-item">
          <span>{{ $t('custom.devicePage.selectedDeviceSummaryOnline') }}</span>
          <strong>{{ summary.online }}</strong>
        </div>
        <div class="selected-device-summary-item">
          <span>{{ $t('custom.devicePage.selectedDeviceSummaryOffline') }}</span>
          <strong>{{ summary.offline }}</strong>
        </div>
        <div class="selected-device-summary-item">
          <span>{{ $t('custom.devicePage.selectedDeviceSummaryAlarmed') }}</span>
          <strong>{{ summary.alarmed }}</strong>
        </div>
        <div class="selected-device-summary-item">
          <span>{{ $t('custom.devicePage.selectedDeviceSummaryMissingVersion') }}</span>
          <strong>{{ summary.missingVersion }}</strong>
        </div>
      </div>
      <NAlert v-if="hasRisk" type="warning" :show-icon="false">
        {{ $t('custom.devicePage.selectedDeviceRiskHint') }}
      </NAlert>
      <NInput
        :value="deviceIdentifiers"
        type="textarea"
        :rows="5"
        readonly
        :placeholder="$t('custom.devicePage.selectedDeviceIdentifiersPlaceholder')"
      />
      <NFlex justify="end" :size="8">
        <NButton @click="visible = false">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" @click="emit('copy')">
          {{ $t('custom.devicePage.copySelectedDeviceIdentifiers') }}
        </NButton>
      </NFlex>
    </NFlex>
  </NModal>
</template>
