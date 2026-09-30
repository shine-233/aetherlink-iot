<!--
  DeviceManageListShell: the device-manage list shell (search area, header actions,
  card / list / map views, footer pagination, empty state).

  Extracted from index.vue when the page moved onto `useListPage` (see
  useDeviceManageListPage.ts); it reproduces the surface the shared <data-table-page>
  wrapper used to render for this page. All list state stays owned by index.vue — the
  shell only forwards interactions as emits.
-->
<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import { GridOutline as CardIcon, ListOutline, MapOutline } from '@vicons/ionicons5'
import AdvancedListLayout from '@/components/list-page/index.vue'
import type { SearchConfig } from '@/components/data-table-page/types'
import type { DeviceManageRow } from './useDeviceManageListPage'
import DeviceManageSearchForm from './DeviceManageSearchForm.vue'
import DeviceManageCardView from './DeviceManageCardView.vue'
import DeviceManageEmptyState from './DeviceManageEmptyState.vue'

const TencentMap = defineAsyncComponent(() => import('@/components/data-table-page/modules/tencent-map.vue'))

const props = defineProps<{
  /** The device-management query contract (device-search-configs.ts). */
  configs: SearchConfig[]
  /** The list-page query object (useListPage `query`); mutated in place by the search form. */
  criteria: Record<string, any>
  topActions: { element: () => any }[]
  rows: DeviceManageRow[]
  loading?: boolean
  columns: any[]
  selectedRowKeys?: Array<string | number>
  rowKey: (row: DeviceManageRow) => string | number
  rowProps?: (row: DeviceManageRow) => Record<string, any>
  total?: number
  page?: number
  pageSize?: number
  firstDeviceOnboarding?: boolean
  rowClick?: (row: DeviceManageRow) => void
}>()

const emit = defineEmits<{
  search: []
  reset: []
  refresh: []
  'update:checkedRowKeys': [keys: Array<string | number>]
  'update:page': [next: number]
  'update:pageSize': [next: number]
  addDevice: []
  openServiceAccess: []
  backHome: []
}>()

// 与共享 data-table-page 传入的 availableViews 保持一致（卡片 / 列表 / 地图）。
const availableViews = [
  { key: 'card', icon: CardIcon, label: 'common.viewCard' },
  { key: 'list', icon: ListOutline, label: 'common.viewList' },
  { key: 'map', icon: MapOutline, label: 'common.viewMap' }
]

const showEmpty = computed(() => !props.loading && props.rows.length === 0)
</script>

<template>
  <AdvancedListLayout :initial-view="'card'" :available-views="availableViews" @query="emit('search')" @reset="emit('reset')" @refresh="emit('refresh')">
    <template #search-form-content>
      <DeviceManageSearchForm :configs="configs" :criteria="criteria" @search="emit('search')" @reset="emit('reset')" />
    </template>
    <template #header-left>
      <div class="flex gap-2">
        <component :is="action.element" v-for="(action, index) in topActions" :key="index" />
      </div>
    </template>
    <template #card-view>
      <DeviceManageCardView :rows="rows" :loading="loading" :row-click="rowClick">
        <template #empty>
          <DeviceManageEmptyState
            :search-criteria="criteria"
            :first-device-onboarding="firstDeviceOnboarding"
            @add-device="emit('addDevice')"
            @open-service-access="emit('openServiceAccess')"
            @clear-filters="emit('reset')"
            @back-home="emit('backHome')"
          />
        </template>
      </DeviceManageCardView>
    </template>
    <template #list-view>
      <n-scrollbar style="height: calc(100vh - 442px)" :size="1">
        <DeviceManageEmptyState
          v-if="showEmpty"
          :search-criteria="criteria"
          :first-device-onboarding="firstDeviceOnboarding"
          @add-device="emit('addDevice')"
          @open-service-access="emit('openServiceAccess')"
          @clear-filters="emit('reset')"
          @back-home="emit('backHome')"
        />
        <n-data-table
          v-else
          aria-label="data table"
          size="small"
          :row-props="rowProps"
          :row-key="rowKey"
          :loading="loading"
          :columns="columns"
          :data="rows"
          :checked-row-keys="selectedRowKeys"
          class="w-full"
          virtual-scroll
          :max-height="'calc(100vh - 442px)'"
          @update:checked-row-keys="emit('update:checkedRowKeys', $event)"
        />
      </n-scrollbar>
    </template>
    <template #map-view>
      <n-spin :show="loading">
        <DeviceManageEmptyState
          v-if="showEmpty"
          :search-criteria="criteria"
          :first-device-onboarding="firstDeviceOnboarding"
          @add-device="emit('addDevice')"
          @open-service-access="emit('openServiceAccess')"
          @clear-filters="emit('reset')"
          @back-home="emit('backHome')"
        />
        <div v-else class="map-view-container">
          <!-- rows 是设备列表的原始载荷（DeviceManageRow = Record<string, any>）；地图只读
               id / name / is_online / ts / location 这几个字段，与迁移前共享
               <data-table-page> 直接透传 dataList 的松散契约一致，故此处断言。
               tencent-map.vue 未导出 MapDevice，无法直接引用该类型。 -->
          <TencentMap :devices="rows as any" />
        </div>
      </n-spin>
    </template>
    <template #footer>
      <n-pagination
        class="justify-end"
        :page="page"
        :page-size="pageSize"
        :item-count="total"
        :page-sizes="[10, 20, 30, 40, 50]"
        show-size-picker
        @update:page="emit('update:page', $event)"
        @update:page-size="emit('update:pageSize', $event)"
      />
    </template>
  </AdvancedListLayout>
</template>

<style scoped lang="scss">
.map-view-container {
  height: calc(100vh - 442px);
  min-height: 360px;
}
</style>
