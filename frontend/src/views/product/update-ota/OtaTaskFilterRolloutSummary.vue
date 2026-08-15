<script setup lang="ts">
import type { DataTableColumns } from 'naive-ui'

defineProps<{
  selectedSavedFleetFilter: any
  fleetPreselectionResult: any
  fleetFilterSummaryItems: Array<{ key: string; label: string; value: string }>
  filterPreviewResult: any
  filterPreviewSubsetColumns: DataTableColumns<any>
  filterPreviewSubsetRows: any[]
}>()
</script>

<template>
  <section class="filter-rollout-summary">
    <div class="filter-rollout-summary__head">
      <div>
        <div class="filter-rollout-summary__title">{{ $t('page.product.update-ota.fullFilterSummaryTitle') }}</div>
        <div class="filter-rollout-summary__desc">{{ $t('page.product.update-ota.fullFilterSummaryDesc') }}</div>
      </div>
      <NTag type="info" round>
        {{
          $t('page.product.update-ota.fullFilterScopeCount')
            .replace('{currentPage}', String(selectedSavedFleetFilter ? 0 : (fleetPreselectionResult?.currentPageCount ?? fleetPreselectionResult?.requestedCount ?? 0)))
            .replace('{total}', String(fleetPreselectionResult?.requestedTotal ?? '--'))
        }}
      </NTag>
    </div>
    <div class="filter-summary-list">
      <NTag v-for="item in fleetFilterSummaryItems" :key="item.key" size="small">
        {{ item.label }}: {{ item.value }}
      </NTag>
      <NTag v-if="!fleetFilterSummaryItems.length" size="small" type="warning">
        {{ $t('page.product.update-ota.fullFilterNoFilter') }}
      </NTag>
    </div>
    <NAlert type="info" :show-icon="false">
      {{
        filterPreviewResult
          ? $t('page.product.update-ota.previewSubsetBackendHint')
          : $t('page.product.update-ota.previewSubsetCurrentPageHint')
      }}
    </NAlert>
    <NDataTable
      size="small"
      :columns="filterPreviewSubsetColumns"
      :data="filterPreviewSubsetRows"
      :pagination="false"
      :bordered="false"
    />
  </section>
</template>

<style scoped>
.filter-rollout-summary {
  display: grid;
  gap: 12px;
  margin-bottom: 12px;
  padding: 12px;
  border: 1px solid var(--border-color);
  border-radius: 8px;
  background: #f8fafc;
}

.filter-rollout-summary__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.filter-rollout-summary__title {
  font-weight: 600;
}

.filter-rollout-summary__desc {
  margin-top: 4px;
  color: var(--text-color-3);
  font-size: 12px;
  line-height: 1.5;
}

.filter-summary-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

@media (max-width: 768px) {
  .filter-rollout-summary__head {
    flex-direction: column;
  }
}
</style>
