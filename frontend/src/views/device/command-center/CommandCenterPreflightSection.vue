<script setup lang="ts">
import CommandCenterPreflightPanel from './CommandCenterPreflightPanel.vue'

const props = defineProps<{
  /** Keep the parent-owned deferred-mount ref behind an explicit setter. */
  setPreflightViewportRef: (element: HTMLElement | null) => void
  shouldMountPreflightPanel: boolean
  contractRows: { label: string; value: string }[]
  currentPageCount: number | null
  filterSummaryItems: { key: string; label: string; value: string | number }[]
  hasCommandJobScope: boolean
  isDeviceFilterScope: boolean
  requestedTotal: number | null
}>()

const emit = defineEmits<{
  mountPanelNow: []
}>()

const bindPreflightViewportRef = (element: unknown) => {
  props.setPreflightViewportRef(element instanceof HTMLElement ? element : null)
}
</script>

<template>
  <div :ref="bindPreflightViewportRef">
    <CommandCenterPreflightPanel
      v-if="shouldMountPreflightPanel"
      :contract-rows="contractRows"
      :current-page-count="currentPageCount"
      :filter-summary-items="filterSummaryItems"
      :has-command-job-scope="hasCommandJobScope"
      :is-device-filter-scope="isDeviceFilterScope"
      :requested-total="requestedTotal"
    />
    <section v-else class="command-center-section command-preflight-deferred-placeholder">
      <div>
        <NTag type="info" size="small">{{ $t('custom.commandCenter.preflightTag') }}</NTag>
        <strong>{{ $t('custom.commandCenter.preflightDeferredTitle') }}</strong>
        <p>{{ $t('custom.commandCenter.preflightDeferredDesc') }}</p>
      </div>
      <NButton size="small" tertiary type="primary" @click="emit('mountPanelNow')">
        {{ $t('custom.commandCenter.loadPreflightPanel') }}
      </NButton>
    </section>
  </div>
</template>

<style scoped>
.command-center-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  padding: 16px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
}

.command-preflight-deferred-placeholder {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  border-style: dashed;
  background: linear-gradient(135deg, #f8fafc 0%, #eef6ff 100%);
}

.command-preflight-deferred-placeholder > div {
  display: grid;
  gap: 6px;
  min-width: 0;
}

.command-preflight-deferred-placeholder strong {
  color: #0f172a;
  font-size: 15px;
}

.command-preflight-deferred-placeholder p {
  margin: 0;
  color: #475569;
  font-size: 13px;
  line-height: 1.6;
}

@media (max-width: 900px) {
  .command-preflight-deferred-placeholder {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
