<script setup lang="ts">
defineProps<{
  selectedPackage: any
  showNoEligibleDeviceAlert: boolean
  fleetPreselectionResult: any
  isFleetFilterScope: boolean
  isFleetFilterRollout: boolean
  filterPreviewResult: any
  savedFleetFiltersLoading: boolean
  savedFleetFilterLoadFailed: boolean
  savedFleetFilterOptions: any[]
  selectedSavedFleetFilterId: string | null
  selectedSavedFleetFilter: any
}>()

const emit = defineEmits<{
  'update:selectedSavedFleetFilterId': [value: string | null]
}>()
</script>

<template>
  <NAlert v-if="selectedPackage?.device_config_name" type="info" class="mb-3" :show-icon="true">
    {{ $t('page.product.update-ota.packageDeviceConfigHint') }}: {{ selectedPackage.device_config_name }}
  </NAlert>
  <NAlert v-if="showNoEligibleDeviceAlert" type="warning" class="mb-3" :show-icon="true">
    {{ $t('page.product.update-ota.noEligibleDevice') }}
  </NAlert>
  <NAlert v-if="fleetPreselectionResult && !selectedSavedFleetFilter" type="info" class="mb-3" :show-icon="true">
    {{
      $t('page.product.update-ota.fleetPreselectionApplied')
        .replace('{requested}', String(fleetPreselectionResult.requestedCount))
        .replace('{selected}', String(fleetPreselectionResult.selectedCount))
        .replace('{excluded}', String(fleetPreselectionResult.excludedCount))
    }}
  </NAlert>
  <NAlert
    v-if="isFleetFilterScope && isFleetFilterRollout && !selectedSavedFleetFilter"
    type="success"
    class="mb-3"
    :show-icon="true"
  >
    {{
      $t('page.product.update-ota.fleetPreselectionFullFilter')
        .replace('{currentPage}', String(fleetPreselectionResult.currentPageCount ?? fleetPreselectionResult.requestedCount))
        .replace('{total}', String(fleetPreselectionResult.requestedTotal ?? '--'))
    }}
  </NAlert>
  <NAlert v-if="filterPreviewResult" type="success" class="mb-3" :show-icon="true">
    {{
      $t('page.product.update-ota.filterPreviewReadyDetail')
        .replace('{selected}', String(filterPreviewResult.selected_count ?? 0))
        .replace('{total}', String(filterPreviewResult.total_matched ?? 0))
        .replace('{max}', String(filterPreviewResult.max_devices ?? 0))
    }}
  </NAlert>
  <NAlert
    v-if="isFleetFilterScope && !isFleetFilterRollout"
    type="warning"
    class="mb-3"
    :show-icon="true"
  >
    {{
      $t('page.product.update-ota.fleetPreselectionCurrentPageOnly')
        .replace('{currentPage}', String(fleetPreselectionResult.currentPageCount ?? fleetPreselectionResult.requestedCount))
        .replace('{total}', String(fleetPreselectionResult.requestedTotal ?? '--'))
    }}
  </NAlert>
  <NFormItem
    v-if="savedFleetFiltersLoading || savedFleetFilterLoadFailed || savedFleetFilterOptions.length"
    :label="$t('page.product.update-ota.savedFleetFilterLabel')"
  >
    <NSpace vertical size="small" class="saved-filter-picker">
      <NSelect
        :value="selectedSavedFleetFilterId"
        clearable
        filterable
        :loading="savedFleetFiltersLoading"
        :options="savedFleetFilterOptions"
        :placeholder="$t('page.product.update-ota.savedFleetFilterPlaceholder')"
        @update:value="emit('update:selectedSavedFleetFilterId', $event)"
      />
      <NAlert v-if="savedFleetFilterLoadFailed" type="warning" :show-icon="false">
        {{ $t('page.product.update-ota.savedFleetFilterLoadFailed') }}
      </NAlert>
      <NAlert v-else-if="selectedSavedFleetFilter" type="success" :show-icon="false">
        {{
          $t('page.product.update-ota.savedFleetFilterApplied')
            .replace('{name}', selectedSavedFleetFilter.name)
            .replace('{total}', String(selectedSavedFleetFilter.previewTotal ?? '--'))
        }}
      </NAlert>
      <div v-else class="saved-filter-picker__hint">
        {{ $t('page.product.update-ota.savedFleetFilterHint') }}
      </div>
    </NSpace>
  </NFormItem>
</template>

<style scoped>
.saved-filter-picker {
  width: 100%;
}

.saved-filter-picker__hint {
  color: var(--text-color-3);
  font-size: 12px;
  line-height: 1.5;
}
</style>
