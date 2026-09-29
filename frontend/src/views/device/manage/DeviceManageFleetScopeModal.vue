<!--
  Confirmation shown before a fleet action that only targets the current page of results.
-->
<script setup lang="ts">
import { $t } from '@/locales'
import './device-fleet-modal.css'

defineProps<{
  actionLabelKey?: string
  currentPageCount: number
  targetPreviewTotal?: number | null
  selectedCount: number
}>()

const visible = defineModel<boolean>('show', { required: true })

const emit = defineEmits<{ confirm: []; cancel: [] }>()
</script>

<template>
  <NModal v-model:show="visible" preset="card" class="max-w-560px">
    <template #header>{{ actionLabelKey ? $t(actionLabelKey) : '' }}</template>
    <NFlex vertical :size="12">
      <NAlert type="warning" :show-icon="false">
        {{ $t('custom.devicePage.fleetActionHint') }}
      </NAlert>
      <div class="selected-device-summary-grid">
        <div class="selected-device-summary-item">
          <span>{{ $t('custom.devicePage.fleetCurrentPageCount') }}</span>
          <strong>{{ currentPageCount }}</strong>
        </div>
        <div class="selected-device-summary-item">
          <span>{{ $t('custom.devicePage.fleetTargetPreviewCount') }}</span>
          <strong>{{ targetPreviewTotal ?? '--' }}</strong>
        </div>
        <div class="selected-device-summary-item">
          <span>{{ $t('custom.devicePage.fleetSelectedCount') }}</span>
          <strong>{{ selectedCount }}</strong>
        </div>
      </div>
      <NText depth="3">
        {{ $t('custom.devicePage.fleetCurrentPageOnly') }}
      </NText>
      <NFlex justify="end" :size="8">
        <NButton @click="emit('cancel')">{{ $t('common.cancel') }}</NButton>
        <NButton type="primary" @click="emit('confirm')">
          {{ actionLabelKey ? $t(actionLabelKey) : $t('common.confirm') }}
        </NButton>
      </NFlex>
    </NFlex>
  </NModal>
</template>
