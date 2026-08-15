<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import type { CommandJobResultActions, CommandJobResultViewModel } from './commandCenterJobResultViewModel'

const COMMAND_JOB_RESULT_PAGE_SIZE = 50
const commandJobResultPagination = {
  pageSize: COMMAND_JOB_RESULT_PAGE_SIZE,
  showSizePicker: true,
  pageSizes: [25, 50, 100],
  simple: false
}

const props = defineProps<{
  jobResult: CommandJobResultViewModel
  jobActions: CommandJobResultActions
}>()

const state = computed(() => props.jobResult)
const actions = computed(() => props.jobActions)
const rowSearchDraft = ref('')

watch(
  () => state.value.commandJobRowsSearch,
  value => {
    rowSearchDraft.value = value
  },
  { immediate: true }
)

const activeRowsFilterLabel = computed(() => {
  return (
    state.value.commandJobRowsStatusFilterOptions.find(option => option.value === state.value.commandJobRowsStatusFilter)?.label ||
    state.value.commandJobRowsStatusFilter
  )
})

const rowsSearchLabel = computed(() => state.value.commandJobRowsSearch || '-')
const hasActiveRowsSearch = computed(() => Boolean(state.value.commandJobRowsSearch))
const hasActiveRowsFilter = computed(() => state.value.commandJobRowsStatusFilter !== 'all')
const rowsNoMatch = computed(() => {
  return !state.value.commandJobRowsLoading && state.value.submitRowsForCustomer.length === 0
})

const submitRowsSearch = () => {
  actions.value.setCommandJobRowsSearch(rowSearchDraft.value)
}

const clearRowsSearch = () => {
  rowSearchDraft.value = ''
  actions.value.clearCommandJobRowsSearch()
}

const resetRowsFilter = () => {
  actions.value.setCommandJobRowsStatusFilter('all')
}
</script>

<template>
  <div class="command-job-result-filter">
    <div class="command-job-result-filter__group">
      <span>{{ $t('custom.commandCenter.rowsFilterLabel') }}</span>
      <NSelect
        size="small"
        class="command-job-result-filter__select"
        :value="state.commandJobRowsStatusFilter"
        :options="state.commandJobRowsStatusFilterOptions"
        :disabled="state.commandJobRowsLoading"
        @update:value="actions.setCommandJobRowsStatusFilter"
      />
    </div>
    <div class="command-job-result-filter__search">
      <span>{{ $t('custom.commandCenter.rowsSearchLabel') }}</span>
      <NInput
        v-model:value="rowSearchDraft"
        size="small"
        clearable
        :placeholder="$t('custom.commandCenter.rowsSearchPlaceholder')"
        :disabled="state.commandJobRowsLoading"
        @keyup.enter="submitRowsSearch"
        @clear="clearRowsSearch"
      />
      <NButton size="small" secondary :loading="state.commandJobRowsLoading" @click="submitRowsSearch">
        {{ $t('custom.commandCenter.rowsSearchAction') }}
      </NButton>
    </div>
  </div>

  <NAlert type="info" :show-icon="false">
    {{
      $t('custom.commandCenter.rowsScopeSummary')
        .replace('{shown}', String(state.submitRowsForCustomer.length))
        .replace('{total}', String(state.submitResult?.rows_total ?? state.submitRowsForCustomer.length))
        .replace('{filter}', activeRowsFilterLabel)
        .replace('{search}', rowsSearchLabel)
    }}
  </NAlert>

  <NAlert v-if="rowsNoMatch" type="warning" :show-icon="false">
    <div class="command-job-empty-rows">
      <div>
        <strong>{{ $t('custom.commandCenter.rowsNoMatchTitle') }}</strong>
        <span>
          {{
            $t('custom.commandCenter.rowsNoMatchDesc')
              .replace('{filter}', activeRowsFilterLabel)
              .replace('{search}', rowsSearchLabel)
          }}
        </span>
      </div>
      <NSpace>
        <NButton v-if="hasActiveRowsSearch" size="small" secondary @click="clearRowsSearch">
          {{ $t('custom.commandCenter.rowsNoMatchClearSearch') }}
        </NButton>
        <NButton v-if="hasActiveRowsFilter" size="small" secondary @click="resetRowsFilter">
          {{ $t('custom.commandCenter.rowsNoMatchShowAll') }}
        </NButton>
        <NButton
          size="small"
          type="primary"
          secondary
          :loading="state.supportBundleLoading"
          @click="actions.loadCommandJobSupportBundle"
        >
          {{ $t('custom.commandCenter.rowsNoMatchSupportBundle') }}
        </NButton>
      </NSpace>
    </div>
  </NAlert>

  <NDataTable
    size="small"
    :columns="state.submitColumns"
    :data="state.submitRowsForCustomer"
    :loading="state.commandJobRowsLoading"
    :pagination="commandJobResultPagination"
    :bordered="false"
    flex-height
    class="command-job-result-table"
  />
  <NAlert v-if="state.submitRowsHiddenCount > 0" type="info" :show-icon="false">
    <div class="command-job-rows-more">
      <span>
        {{
          $t('custom.commandCenter.resultRowsDisplayLimit')
            .replace('{shown}', String(state.submitRowsForCustomer.length))
            .replace('{hidden}', String(state.submitRowsHiddenCount))
        }}
      </span>
      <NButton
        v-if="state.canLoadMoreCommandJobRows"
        size="small"
        secondary
        :loading="state.commandJobRowsLoading"
        @click="actions.loadMoreCommandJobRows"
      >
        {{ $t('custom.commandCenter.loadMoreResultRows') }}
      </NButton>
    </div>
  </NAlert>
</template>

<style scoped>
.command-job-result-table {
  min-height: 240px;
  max-height: min(520px, 60vh);
}

.command-job-result-filter {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.command-job-result-filter span {
  color: #475569;
  font-size: 12px;
  white-space: nowrap;
}

.command-job-result-filter__group,
.command-job-result-filter__search {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.command-job-result-filter__search {
  flex: 1;
  justify-content: flex-end;
}

.command-job-result-filter__search :deep(.n-input) {
  max-width: 320px;
}

.command-job-result-filter__select {
  width: min(220px, 100%);
}

.command-job-empty-rows {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.command-job-empty-rows > div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.command-job-empty-rows strong {
  color: #0f172a;
  font-size: 13px;
}

.command-job-empty-rows span {
  color: #475569;
  font-size: 12px;
  overflow-wrap: anywhere;
}

.command-job-rows-more {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.command-job-rows-more span {
  min-width: 0;
  overflow-wrap: anywhere;
}

@media (max-width: 900px) {
  .command-job-result-filter,
  .command-job-result-filter__group,
  .command-job-result-filter__search {
    align-items: stretch;
    flex-direction: column;
  }

  .command-job-result-filter__search {
    justify-content: flex-start;
  }

  .command-job-result-filter__search :deep(.n-input),
  .command-job-result-filter__select {
    max-width: none;
    width: 100%;
  }

  .command-job-empty-rows,
  .command-job-rows-more {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
