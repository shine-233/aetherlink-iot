<script setup lang="ts">
import { $t } from '@/locales'

type ContractRow = {
  label: string
  value: string
}

type FilterSummaryItem = {
  key: string
  label: string
  value: string | number
}

defineProps<{
  contractRows: ContractRow[]
  currentPageCount: number | null
  filterSummaryItems: FilterSummaryItem[]
  hasCommandJobScope: boolean
  isDeviceFilterScope: boolean
  requestedTotal: number | null
}>()
</script>

<template>
  <section class="command-center-section">
    <div class="command-center-section__head">
      <NTag type="info" size="small">{{ $t('custom.commandCenter.preflightTag') }}</NTag>
      <h2>{{ $t('custom.commandCenter.preflightTitle') }}</h2>
    </div>
    <NDescriptions bordered :column="2" size="small">
      <NDescriptionsItem v-for="row in contractRows" :key="row.label" :label="row.label">
        {{ row.value }}
      </NDescriptionsItem>
    </NDescriptions>
    <div v-if="filterSummaryItems.length" class="command-filter-summary mt-3">
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
    <NAlert v-if="!hasCommandJobScope" class="mt-3" type="warning" :show-icon="false">
      {{ $t('custom.commandCenter.noSelection') }}
    </NAlert>
    <NAlert v-else class="mt-3" type="success" :show-icon="false">
      {{ isDeviceFilterScope ? $t('custom.commandCenter.filterScopeReady') : $t('custom.commandCenter.selectionReady') }}
    </NAlert>
  </section>
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

.command-center-section__head {
  display: flex;
  align-items: center;
  gap: 8px;
}

.command-center-section h2 {
  margin: 0;
  color: #0f172a;
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

.mt-3 {
  margin-top: 12px;
}
</style>
