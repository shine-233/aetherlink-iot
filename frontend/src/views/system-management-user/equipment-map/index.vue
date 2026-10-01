<!--
文件用途: 承载设备地图相关的系统管理用户侧页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { deviceList } from '@/service/api/device'
import { $t } from '@/locales'
import { normalizeListResponse, useListPage } from '@/components/data-table-page/useListPage'
import { parseLocation, toLngLat, toMarkerStyle, type DeviceRecord } from './equipment-map-model'
import { useEquipmentTelemetry } from './useEquipmentTelemetry'
import EquipmentMapDeviceList from './EquipmentMapDeviceList.vue'
import EquipmentMapSurface from './EquipmentMapSurface.vue'
import EquipmentMapTelemetryPanel from './EquipmentMapTelemetryPanel.vue'

const router = useRouter()
const selectedDevice = ref<DeviceRecord | null>(null)

// 设备列表：分页、加载态与过期请求丢弃交给 useListPage；
// 成功回包后在 onLoaded 里维护“当前预览设备”的选中语义。
const {
  query,
  page,
  total,
  pageCount,
  rows: devices,
  loading,
  load: fetchDevices,
  search: searchDevices,
  setPage: changePage
} = useListPage<DeviceRecord, { search: string }>({
  initialQuery: () => ({ search: '' }),
  initialPageSize: 12,
  serialize: q => ({ search: q.search.trim() || undefined }),
  fetcher: async params => {
    const { data, error } = await deviceList(params)
    return error ? { list: [], total: 0 } : normalizeListResponse<DeviceRecord>(data)
  },
  onLoaded: ({ list }) => {
    if (list.length === 0) {
      clearSelectedDevice()
      return
    }
    const selectedStillVisible = list.some(device => device.id === selectedDevice.value?.id)
    if (!selectedDevice.value || !selectedStillVisible) previewDevice(list[0])
  }
})

const { telemetryLoading, telemetryRequested, selectedTelemetry, load: loadTelemetry, reset: resetTelemetry } =
  useEquipmentTelemetry()

const onlineCount = computed(() => devices.value.filter(item => item.is_online === 1).length)
const alarmCount = computed(() => devices.value.filter(item => item.warn_status === 'Y').length)
const selectedLocation = computed(() => parseLocation(selectedDevice.value?.location))
const selectedPosition = computed(() => toLngLat(selectedLocation.value))
const markerStyle = computed(() => toMarkerStyle(selectedLocation.value))
const selectedDeviceLabel = computed(
  () => selectedDevice.value?.name || selectedDevice.value?.device_number || ''
)

function clearSelectedDevice() {
  selectedDevice.value = null
  resetTelemetry()
}

function previewDevice(device: DeviceRecord) {
  selectedDevice.value = device
  resetTelemetry()
}

async function selectDevice(device: DeviceRecord) {
  previewDevice(device)
  await loadTelemetry(device.id)
}

async function reloadSelectedTelemetry() {
  if (!selectedDevice.value?.id) return
  await loadTelemetry(selectedDevice.value.id)
}

function openDeviceDetails() {
  if (!selectedDevice.value?.id) return
  router.push({ name: 'device_details', query: { d_id: selectedDevice.value.id } })
}

onMounted(() => {
  void fetchDevices()
})
</script>

<template>
  <div class="device-map-page">
    <div class="device-map-header">
      <div>
        <h2>{{ $t('rdi.map.title') }}</h2>
        <p>{{ $t('rdi.map.subtitle') }}</p>
      </div>
      <NSpace>
        <NButton @click="router.back()">{{ $t('common.back') }}</NButton>
        <NButton type="primary" :loading="loading" @click="fetchDevices">{{ $t('common.refresh') }}</NButton>
      </NSpace>
    </div>

    <div class="device-map-toolbar">
      <NInput
        v-model:value="query.search"
        clearable
        :placeholder="$t('rdi.map.searchPlaceholder')"
        @keyup.enter="searchDevices"
      />
      <NButton type="primary" @click="searchDevices">{{ $t('common.search') }}</NButton>
    </div>

    <div class="device-map-metrics">
      <div>
        <span>{{ $t('rdi.map.totalDevices') }}</span>
        <strong>{{ total }}</strong>
      </div>
      <div>
        <span>{{ $t('rdi.map.currentPageOnline') }}</span>
        <strong>{{ onlineCount }}</strong>
      </div>
      <div>
        <span>{{ $t('rdi.map.currentPageAlarms') }}</span>
        <strong>{{ alarmCount }}</strong>
      </div>
    </div>

    <div class="device-map-layout">
      <EquipmentMapDeviceList
        :devices="devices"
        :loading="loading"
        :page="page"
        :page-count="pageCount"
        :selected-id="selectedDevice?.id"
        @select="selectDevice"
        @update:page="changePage"
      />

      <section class="device-map-main">
        <EquipmentMapSurface
          :position="selectedPosition"
          :location-text="selectedLocation?.raw"
          :device-label="selectedDeviceLabel"
          :marker-style="markerStyle"
        />
        <EquipmentMapTelemetryPanel
          :device="selectedDevice"
          :telemetry="selectedTelemetry"
          :loading="telemetryLoading"
          :requested="telemetryRequested"
          @load="reloadSelectedTelemetry"
          @open-details="openDeviceDetails"
        />
      </section>
    </div>
  </div>
</template>

<style scoped>
.device-map-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-height: 100%;
  padding: 16px;
}

.device-map-header,
.device-map-toolbar,
.device-map-metrics,
.device-map-layout {
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  background: var(--n-color);
}

.device-map-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
}

.device-map-header h2 {
  margin: 0;
  font-size: 18px;
}

.device-map-header p {
  margin: 4px 0 0;
  color: var(--n-text-color-3);
}

.device-map-toolbar {
  display: grid;
  grid-template-columns: minmax(220px, 420px) auto;
  gap: 12px;
  padding: 12px;
}

.device-map-metrics {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
}

.device-map-metrics div {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 14px 16px;
}

.device-map-metrics span {
  color: var(--n-text-color-3);
  font-size: 13px;
}

.device-map-metrics strong {
  font-size: 22px;
}

.device-map-layout {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  min-height: 560px;
  overflow: hidden;
}

.device-map-main {
  display: grid;
  grid-template-rows: minmax(300px, 1fr) auto;
}

@media (max-width: 900px) {
  .device-map-header {
    align-items: stretch;
    flex-direction: column;
  }

  .device-map-toolbar,
  .device-map-metrics,
  .device-map-layout {
    grid-template-columns: 1fr;
  }
}
</style>
