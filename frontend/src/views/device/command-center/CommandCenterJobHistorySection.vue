<script setup lang="ts">
import CommandJobHistoryPanel from './CommandJobHistoryPanel.vue'

const props = defineProps<{
  /**
   * The parent owns the deferred-mount ref. Passing a setter keeps the
   * ownership explicit because template ref unwrapping would otherwise pass
   * the current `null` value instead of the Ref object itself.
   */
  setHistoryViewportRef: (element: HTMLElement | null) => void
  shouldMountJobHistoryPanel: boolean
  isDeviceFilterScope: boolean
  filterSummaryItems: any[]
  requestedTotal: number | null
  currentPageCount: number | null
  jobHistorySearch: string
  jobHistoryLoading: boolean
  jobHistoryStatus: string | null
  jobHistoryStatusOptions: unknown[]
  jobHistoryAttentionFilter: string | null
  jobHistoryAttentionOptions: unknown[]
  jobHistoryAttentionAggregateRows: any[]
  jobHistoryInitialLoadQueued: boolean
  jobHistory: {
    list: any[]
    total: number
  }
  jobHistoryColumns: unknown[]
  previewLoading: boolean
  canPreviewCommandJobNow: boolean
  canLoadMoreJobHistory: boolean
}>()

const emit = defineEmits<{
  'update:jobHistorySearch': [value: string]
  'update:jobHistoryStatus': [value: string | null]
  'update:jobHistoryAttentionFilter': [value: string | null]
  search: []
  clearSearch: []
  refresh: []
  openFleet: []
  preview: []
  loadMore: []
  mountPanelNow: []
}>()

const bindHistoryViewportRef = (element: unknown) => {
  props.setHistoryViewportRef(element instanceof HTMLElement ? element : null)
}
</script>

<template>
  <div :ref="bindHistoryViewportRef">
    <CommandJobHistoryPanel
      v-if="shouldMountJobHistoryPanel"
      :is-device-filter-scope="isDeviceFilterScope"
      :filter-summary-items="filterSummaryItems"
      :requested-total="requestedTotal"
      :current-page-count="currentPageCount"
      :job-history-search="jobHistorySearch"
      :job-history-loading="jobHistoryLoading"
      :job-history-status="jobHistoryStatus"
      :job-history-status-options="jobHistoryStatusOptions"
      :job-history-attention-filter="jobHistoryAttentionFilter"
      :job-history-attention-options="jobHistoryAttentionOptions"
      :job-history-attention-aggregate-rows="jobHistoryAttentionAggregateRows"
      :job-history-initial-load-queued="jobHistoryInitialLoadQueued"
      :job-history="jobHistory"
      :job-history-columns="jobHistoryColumns"
      :preview-loading="previewLoading"
      :can-preview-command-job-now="canPreviewCommandJobNow"
      :can-load-more-job-history="canLoadMoreJobHistory"
      @update:job-history-search="emit('update:jobHistorySearch', $event)"
      @update:job-history-status="emit('update:jobHistoryStatus', $event)"
      @update:job-history-attention-filter="emit('update:jobHistoryAttentionFilter', $event)"
      @search="emit('search')"
      @clear-search="emit('clearSearch')"
      @refresh="emit('refresh')"
      @open-fleet="emit('openFleet')"
      @preview="emit('preview')"
      @load-more="emit('loadMore')"
    />
    <div v-else class="command-history-deferred-placeholder">
      <div class="command-history-deferred-placeholder__copy">
        <span>{{ $t('custom.commandCenter.jobHistoryTitle') }}</span>
        <strong>{{ $t('custom.commandCenter.jobHistoryDeferredTitle') }}</strong>
        <p>{{ $t('custom.commandCenter.jobHistoryDeferredDesc') }}</p>
      </div>
      <NButton size="small" secondary type="primary" @click="emit('mountPanelNow')">
        {{ $t('custom.commandCenter.loadJobHistoryPanel') }}
      </NButton>
    </div>
  </div>
</template>

<style scoped>
.command-history-deferred-placeholder {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
  border: 1px dashed #cbd5e1;
  border-radius: 14px;
  background: linear-gradient(135deg, #f8fafc 0%, #fff7ed 100%);
  padding: 16px;
}

.command-history-deferred-placeholder__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-history-deferred-placeholder__copy span {
  color: #b45309;
  font-size: 12px;
  font-weight: 700;
}

.command-history-deferred-placeholder__copy strong {
  color: #0f172a;
  font-size: 15px;
}

.command-history-deferred-placeholder__copy p {
  margin: 0;
  color: #475569;
  font-size: 13px;
  line-height: 1.6;
}

@media (max-width: 900px) {
  .command-history-deferred-placeholder {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
