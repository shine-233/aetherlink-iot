<script setup lang="ts">
import { computed } from 'vue'
import { $t } from '@/locales'

type FilterSummaryItem = {
  key: string
  label: string
  value: string | number
}

type CommandJobHistorySummaryRow = {
  key: string
  label: string
  type?: 'default' | 'error' | 'info' | 'primary' | 'success' | 'warning'
  count: number
  filter?: string | null
}

const props = defineProps<{
  isDeviceFilterScope: boolean
  filterSummaryItems: FilterSummaryItem[]
  requestedTotal: number | null
  currentPageCount: number | null
  jobHistorySearch: string
  jobHistoryLoading: boolean
  jobHistoryStatus: string | number | null
  jobHistoryStatusOptions: any[]
  jobHistoryAttentionFilter: string | null
  jobHistoryAttentionOptions: any[]
  jobHistoryAttentionAggregateRows: CommandJobHistorySummaryRow[]
  jobHistoryInitialLoadQueued: boolean
  jobHistory: {
    list: any[]
    total: number
  }
  jobHistoryColumns: any[]
  previewLoading: boolean
  canPreviewCommandJobNow: boolean
  canLoadMoreJobHistory: boolean
}>()

const emit = defineEmits<{
  (e: 'update:jobHistorySearch', value: string): void
  (e: 'update:jobHistoryStatus', value: string | number | null): void
  (e: 'update:jobHistoryAttentionFilter', value: string | null): void
  (e: 'search'): void
  (e: 'clearSearch'): void
  (e: 'refresh'): void
  (e: 'openFleet'): void
  (e: 'preview'): void
  (e: 'loadMore'): void
}>()

const jobHistorySearchValue = computed({
  get: () => props.jobHistorySearch,
  set: (value: string) => emit('update:jobHistorySearch', value)
})

const jobHistoryStatusValue = computed({
  get: () => props.jobHistoryStatus,
  set: (value: string | number | null) => emit('update:jobHistoryStatus', value)
})

const jobHistoryAttentionFilterValue = computed({
  get: () => props.jobHistoryAttentionFilter,
  set: (value: string | null) => emit('update:jobHistoryAttentionFilter', value)
})

const updateJobHistoryStatus = (value: string | number | null) => {
  jobHistoryStatusValue.value = value
  emit('refresh')
}

const updateJobHistoryAttentionFilter = (value: string | null) => {
  jobHistoryAttentionFilterValue.value = value
}
</script>

<template>
  <div class="command-job-history-panel">
    <div v-if="isDeviceFilterScope && filterSummaryItems.length" class="command-filter-summary">
      <div class="command-filter-summary__head">
        <strong>{{ $t('custom.commandCenter.filterScopeSummaryTitle') }}</strong>
        <span>
          {{
            $t('custom.commandCenter.filterScopeSummary')
              .replace('{filters}', String(filterSummaryItems.length))
              .replace('{total}', requestedTotal === null ? '--' : String(requestedTotal))
              .replace('{currentPage}', currentPageCount === null ? '--' : String(currentPageCount))
          }}
        </span>
      </div>
      <NSpace :size="[8, 8]">
        <NTag v-for="item in filterSummaryItems" :key="item.key" size="small" type="info">
          {{ item.label }}: {{ item.value }}
        </NTag>
      </NSpace>
    </div>

    <div class="command-job-history">
      <div class="command-job-history__head">
        <div>
          <strong>{{ $t('custom.commandCenter.jobHistoryTitle') }}</strong>
          <span>{{ $t('custom.commandCenter.jobHistoryDesc') }}</span>
        </div>
        <NSpace>
          <NInput
            v-model:value="jobHistorySearchValue"
            class="w-240px"
            clearable
            size="small"
            :disabled="jobHistoryLoading"
            :placeholder="$t('custom.commandCenter.jobHistorySearchPlaceholder')"
            @keyup.enter="emit('search')"
            @clear="emit('clearSearch')"
          />
          <NButton size="small" secondary :loading="jobHistoryLoading" @click="emit('search')">
            {{ $t('custom.commandCenter.jobHistorySearchAction') }}
          </NButton>
          <NSelect
            v-model:value="jobHistoryStatusValue"
            class="w-180px"
            clearable
            :options="jobHistoryStatusOptions"
            :placeholder="$t('custom.commandCenter.jobHistoryStatusFilter')"
            @update:value="updateJobHistoryStatus"
          />
          <NSelect
            v-model:value="jobHistoryAttentionFilterValue"
            class="w-220px"
            clearable
            :options="jobHistoryAttentionOptions"
            :placeholder="$t('custom.commandCenter.jobHistoryAttentionFilter')"
            @update:value="updateJobHistoryAttentionFilter"
          />
          <NButton size="small" secondary :loading="jobHistoryLoading" @click="emit('refresh')">
            {{ $t('custom.commandCenter.refreshJob') }}
          </NButton>
        </NSpace>
      </div>

      <div class="command-job-attention-summary">
        <div class="command-job-attention-summary__head">
          <strong>{{ $t('custom.commandCenter.jobHistoryAttentionSummaryTitle') }}</strong>
          <span>{{ $t('custom.commandCenter.jobHistoryAttentionSummaryDesc') }}</span>
        </div>
        <div class="command-job-attention-summary__grid">
          <div
            v-for="row in jobHistoryAttentionAggregateRows"
            :key="row.key"
            class="command-job-attention-summary__item"
          >
            <span>{{ row.label }}</span>
            <NButton
              size="tiny"
              secondary
              :type="row.type"
              :disabled="row.count <= 0 || !row.filter"
              @click="emit('update:jobHistoryAttentionFilter', row.filter || null)"
            >
              {{ row.count }}
            </NButton>
          </div>
        </div>
      </div>

      <NAlert v-if="!jobHistoryInitialLoadQueued && !jobHistoryLoading && !jobHistory.list.length" type="info" :show-icon="false">
        <div class="command-job-history-empty">
          <div>
            <strong>{{ $t('custom.commandCenter.jobHistoryEmptyTitle') }}</strong>
            <span>{{ $t('custom.commandCenter.jobHistoryEmptyDesc') }}</span>
          </div>
          <NSpace :size="[8, 8]">
            <NButton size="small" type="primary" @click="emit('openFleet')">
              {{ $t('custom.commandCenter.jobHistoryEmptyOpenFleet') }}
            </NButton>
            <NButton
              size="small"
              secondary
              :loading="previewLoading"
              :disabled="!canPreviewCommandJobNow"
              @click="emit('preview')"
            >
              {{ $t('custom.commandCenter.jobHistoryEmptyPreview') }}
            </NButton>
          </NSpace>
        </div>
      </NAlert>
      <NAlert v-else-if="jobHistoryInitialLoadQueued" type="info" :show-icon="false">
        {{ $t('common.loading') }}
      </NAlert>
      <NDataTable
        v-if="!jobHistoryInitialLoadQueued && jobHistory.list.length"
        size="small"
        :loading="jobHistoryLoading"
        :columns="jobHistoryColumns"
        :data="jobHistory.list"
        :pagination="false"
        :bordered="false"
      />
      <span class="command-job-history__total">
        {{ $t('custom.commandCenter.jobHistoryTotal').replace('{total}', String(jobHistory.total)) }}
      </span>
      <div v-if="canLoadMoreJobHistory" class="command-job-history__more">
        <span>
          {{
            $t('custom.commandCenter.jobHistoryLoaded')
              .replace('{shown}', String(jobHistory.list.length))
              .replace('{total}', String(jobHistory.total))
          }}
        </span>
        <NButton size="small" secondary :loading="jobHistoryLoading" @click="emit('loadMore')">
          {{ $t('custom.commandCenter.jobHistoryLoadMore') }}
        </NButton>
      </div>
    </div>
  </div>
</template>

<style scoped>
.command-job-history-panel {
  display: grid;
  gap: 10px;
}

.command-filter-summary {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #bae6fd;
  border-radius: 8px;
  background: #f0f9ff;
}

.command-filter-summary__head {
  display: grid;
  gap: 4px;
}

.command-filter-summary__head strong {
  color: #0f172a;
  font-size: 14px;
}

.command-filter-summary__head span {
  color: #475569;
  font-size: 12px;
}

.command-job-history {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #f8fafc;
}

.command-job-history__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-job-history__head > div {
  display: grid;
  gap: 4px;
}

.command-job-history__head strong {
  color: #0f172a;
  font-size: 14px;
}

.command-job-history__head span,
.command-job-history__total {
  color: #64748b;
  font-size: 12px;
}

.command-job-attention-summary {
  display: grid;
  gap: 10px;
  padding: 10px;
  border: 1px solid #dbeafe;
  border-radius: 8px;
  background: #eff6ff;
}

.command-job-attention-summary__head {
  display: grid;
  gap: 3px;
}

.command-job-attention-summary__head strong {
  color: #0f172a;
  font-size: 13px;
}

.command-job-attention-summary__head span {
  color: #475569;
  font-size: 12px;
}

.command-job-attention-summary__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.command-job-attention-summary__item {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  background: #fff;
}

.command-job-attention-summary__item span {
  min-width: 0;
  overflow: hidden;
  color: #334155;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.command-job-history-empty {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-job-history-empty strong {
  display: block;
  margin-bottom: 4px;
  color: #075985;
}

.command-job-history-empty span {
  color: #0369a1;
  font-size: 12px;
}

.command-job-history__more {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding-top: 4px;
}

.command-job-history__more span {
  min-width: 0;
  color: #64748b;
  font-size: 12px;
}

@media (max-width: 900px) {
  .command-job-history__head {
    flex-direction: column;
  }

  .command-job-attention-summary__grid {
    grid-template-columns: 1fr;
  }

  .command-job-history__more,
  .command-job-history-empty {
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
