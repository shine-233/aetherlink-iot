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

      <NAlert
        v-if="!jobHistoryInitialLoadQueued && !jobHistoryLoading && !jobHistory.list.length"
        type="info"
        :show-icon="false"
      >
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
      >
        <template #empty>
          <NEmpty :description="$t('common.noData')" class="py-24px" />
        </template>
      </NDataTable>
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
  border: 1px solid rgb(var(--info-color) / 0.35);
  border-radius: var(--radius-md);
  background: rgb(var(--info-color) / 0.06);
}

.command-filter-summary__head {
  display: grid;
  gap: 4px;
}

.command-filter-summary__head strong {
  color: var(--text-color-1);
  font-size: var(--font-size-base);
}

.command-filter-summary__head span {
  color: var(--text-color-2);
  font-size: var(--font-size-caption);
}

.command-job-history {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--action-color);
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
  color: var(--text-color-1);
  font-size: var(--font-size-base);
}

.command-job-history__head span,
.command-job-history__total {
  color: var(--text-color-3);
  font-size: var(--font-size-caption);
}

.command-job-attention-summary {
  display: grid;
  gap: 10px;
  padding: 10px;
  border: 1px solid rgb(var(--info-color) / 0.2);
  border-radius: var(--radius-md);
  background: rgb(var(--info-color) / 0.07);
}

.command-job-attention-summary__head {
  display: grid;
  gap: 3px;
}

.command-job-attention-summary__head strong {
  color: var(--text-color-1);
  font-size: var(--font-size-secondary);
}

.command-job-attention-summary__head span {
  color: var(--text-color-2);
  font-size: var(--font-size-caption);
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
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  background: var(--card-color);
}

.command-job-attention-summary__item span {
  min-width: 0;
  overflow: hidden;
  color: var(--text-color-2);
  font-size: var(--font-size-caption);
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
  color: rgb(var(--info-800-color));
}

.command-job-history-empty span {
  color: rgb(var(--info-700-color));
  font-size: var(--font-size-caption);
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
  color: var(--text-color-3);
  font-size: var(--font-size-caption);
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
