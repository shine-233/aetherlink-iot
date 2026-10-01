<!--
文件用途：RDI 概览系统快照的筛选面板（关键字、状态、高级筛选：告警级别/分组，及已生效筛选标签）。
核心逻辑：状态全部来自父级 useRdiSnapshotFilters 返回的对象，本组件只做双向绑定与展示。
-->
<script setup lang="ts">
import { NButton, NInput, NSelect, NSpace, NTag, NTreeSelect } from 'naive-ui'
import { $t } from '@/locales'
import type { useRdiSnapshotFilters } from './useRdiSnapshotFilters'

defineProps<{ filters: ReturnType<typeof useRdiSnapshotFilters> }>()
</script>

<template>
  <div class="snapshot-filter-panel">
    <NSpace align="center" :wrap="true" class="snapshot-filter-bar">
      <NInput
        v-model:value="filters.snapshotFilterKeyword.value"
        :placeholder="$t('rdi.overview.snapshotFilterPlaceholder')"
        clearable
        class="snapshot-filter-keyword"
      />
      <NSelect
        v-model:value="filters.snapshotFilterStatus.value"
        :options="filters.snapshotStatusOptions.value"
        :placeholder="$t('rdi.overview.snapshotFilterStatus')"
        class="snapshot-filter-status"
      />
      <NButton size="small" tertiary @click="filters.toggleSnapshotAdvancedFilters">
        {{
          filters.snapshotFilterAdvancedVisible.value
            ? $t('rdi.overview.snapshotFilterHideAdvanced')
            : $t('rdi.overview.snapshotFilterShowAdvanced')
        }}
      </NButton>
      <NButton
        v-if="filters.snapshotHasActiveFilters.value"
        size="small"
        type="warning"
        secondary
        @click="filters.resetSnapshotFilters"
      >
        {{ $t('rdi.overview.snapshotFilterClear') }}
      </NButton>
    </NSpace>
    <div v-if="filters.snapshotFilterAdvancedVisible.value" class="snapshot-filter-advanced">
      <div class="snapshot-filter-advanced-item">
        <span class="snapshot-filter-advanced-label">{{ $t('common.alarm_level') }}</span>
        <NSelect
          v-model:value="filters.snapshotFilterAlarmLevel.value"
          :options="filters.snapshotAlarmLevelOptions.value"
          class="snapshot-filter-advanced-control"
        />
      </div>
      <div class="snapshot-filter-advanced-item">
        <span class="snapshot-filter-advanced-label">{{ $t('rdi.overview.snapshotFilterGroup') }}</span>
        <NTreeSelect
          v-model:value="filters.snapshotFilterGroupId.value"
          :options="filters.snapshotGroupOptions.value"
          :placeholder="$t('rdi.overview.snapshotFilterGroupPlaceholder')"
          clearable
          filterable
          class="snapshot-filter-advanced-control"
        />
      </div>
    </div>
    <div v-if="filters.snapshotActiveFilterChips.value.length" class="snapshot-filter-chips">
      <NTag
        v-for="chip in filters.snapshotActiveFilterChips.value"
        :key="chip.key"
        closable
        type="info"
        size="small"
        @close="filters.removeSnapshotFilter(chip.key)"
      >
        {{ chip.label }}
      </NTag>
    </div>
  </div>
</template>

<style scoped>
.snapshot-filter-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 16px;
}

.snapshot-filter-keyword {
  width: 260px;
  max-width: 100%;
}

.snapshot-filter-status {
  width: 160px;
}

.snapshot-filter-advanced {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  padding: 12px 14px;
  border: 1px solid #e5e7eb;
  border-radius: 8px;
  background: #f9fafb;
}

.snapshot-filter-advanced-item {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.snapshot-filter-advanced-label {
  color: #6b7280;
  font-size: 13px;
  white-space: nowrap;
}

.snapshot-filter-advanced-control {
  width: 200px;
}

.snapshot-filter-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
