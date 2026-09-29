<!--
  Paginated device list shown beside the equipment map.
-->
<script setup lang="ts">
import { $t } from '@/locales'
import type { DeviceRecord } from './equipment-map-model'

defineProps<{
  devices: DeviceRecord[]
  loading: boolean
  page: number
  pageCount: number
  selectedId?: string
}>()

const emit = defineEmits<{ select: [device: DeviceRecord]; 'update:page': [page: number] }>()
</script>

<template>
  <section class="device-map-list">
    <NSpin :show="loading">
      <NEmpty v-if="devices.length === 0" :description="$t('common.noData')" />
      <button
        v-for="device in devices"
        :key="device.id"
        type="button"
        class="device-map-list-item"
        :class="{ active: selectedId === device.id }"
        @click="emit('select', device)"
      >
        <span>
          <strong>{{ device.name || device.device_number || device.id }}</strong>
          <small>{{ device.pid_number || device.device_number || '-' }}</small>
        </span>
        <NTag :type="device.is_online === 1 ? 'success' : 'default'" size="small">
          {{ device.is_online === 1 ? $t('custom.devicePage.online') : $t('custom.devicePage.offline') }}
        </NTag>
      </button>
    </NSpin>
    <NPagination
      v-if="pageCount > 1"
      class="device-map-pagination"
      :page="page"
      :page-count="pageCount"
      @update:page="emit('update:page', $event)"
    />
  </section>
</template>

<style scoped>
.device-map-list {
  border-right: 1px solid var(--n-border-color);
  padding: 12px;
}

.device-map-list-item {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border: 1px solid transparent;
  border-radius: 8px;
  background: transparent;
  padding: 10px;
  color: var(--n-text-color);
  text-align: left;
  cursor: pointer;
}

.device-map-list-item + .device-map-list-item {
  margin-top: 8px;
}

.device-map-list-item.active,
.device-map-list-item:hover {
  border-color: var(--n-primary-color);
  background: var(--n-primary-color-suppl);
}

.device-map-list-item span {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
}

.device-map-list-item strong,
.device-map-list-item small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.device-map-list-item small {
  color: var(--n-text-color-3);
}

.device-map-pagination {
  margin-top: 12px;
  justify-content: center;
}

@media (max-width: 900px) {
  .device-map-list {
    border-right: 0;
    border-bottom: 1px solid var(--n-border-color);
  }
}
</style>
