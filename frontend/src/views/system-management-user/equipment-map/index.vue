<!--
文件用途: 承载设备地图相关的系统管理用户侧页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useScriptTag } from '@vueuse/core'
import { deviceList, deviceMapTelemetry } from '@/service/api/device'
import { $t } from '@/locales'
import { AMAP_SDK_URL, ensureAmapSecurityConfig } from '@/constants/map-sdk'

interface DeviceRecord {
  id: string
  device_number?: string
  name?: string
  pid_number?: string
  location?: string
  is_online?: number
  warn_status?: string
}

interface TelemetryItem {
  key: string
  label?: string | null
  value?: string | number | boolean | null
  unit?: string | null
}

interface MapTelemetry {
  device_id?: string
  device_name?: string
  is_online?: number
  last_push_time?: string | null
  telemetry_data?: TelemetryItem[]
}

interface ParsedLocation {
  raw: string
  lat?: number
  lng?: number
}

const router = useRouter()
const { load: loadAmap } = useScriptTag(AMAP_SDK_URL, undefined, { manual: true })
const loading = ref(false)
const telemetryLoading = ref(false)
const mapLoading = ref(false)
const mapReady = ref(false)
const mapError = ref(false)
const mapDomRef = ref<HTMLDivElement | null>(null)
const devices = ref<DeviceRecord[]>([])
const selectedDevice = ref<DeviceRecord | null>(null)
const selectedTelemetry = ref<MapTelemetry | null>(null)
const telemetryRequested = ref(false)
const query = reactive({
  page: 1,
  page_size: 12,
  search: ''
})
const total = ref(0)

const pageCount = computed(() => Math.max(1, Math.ceil(total.value / query.page_size)))
const onlineCount = computed(() => devices.value.filter(item => item.is_online === 1).length)
const alarmCount = computed(() => devices.value.filter(item => item.warn_status === 'Y').length)
const selectedLocation = computed(() => parseLocation(selectedDevice.value?.location))
const hasSelectedCoordinates = computed(() => {
  return selectedLocation.value?.lat !== undefined && selectedLocation.value.lng !== undefined
})
const markerStyle = computed(() => {
  if (!selectedLocation.value || selectedLocation.value.lat === undefined || selectedLocation.value.lng === undefined) {
    return { left: '50%', top: '50%' }
  }
  const left = ((selectedLocation.value.lng + 180) / 360) * 100
  const top = ((90 - selectedLocation.value.lat) / 180) * 100
  return {
    left: `${Math.min(96, Math.max(4, left))}%`,
    top: `${Math.min(92, Math.max(8, top))}%`
  }
})

let amapInstance: any = null
let amapMarker: any = null
let telemetryRequestSeq = 0

function normalizeList(data: any): DeviceRecord[] {
  if (Array.isArray(data?.list)) return data.list
  if (Array.isArray(data?.data?.list)) return data.data.list
  if (Array.isArray(data)) return data
  return []
}

function normalizeTotal(data: any, fallback: number) {
  return Number(data?.total ?? data?.data?.total ?? fallback)
}

function parseLocation(value?: string): ParsedLocation | null {
  const raw = String(value || '').trim()
  if (!raw) return null

  try {
    const obj = JSON.parse(raw)
    const lat = Number(obj.lat ?? obj.latitude)
    const lng = Number(obj.lng ?? obj.lon ?? obj.longitude)
    if (Number.isFinite(lat) && Number.isFinite(lng)) return { raw, lat, lng }
  } catch {
    // Plain text locations are allowed.
  }

  const match = raw.match(/(-?\d+(?:\.\d+)?)\s*[, ]\s*(-?\d+(?:\.\d+)?)/)
  if (match) {
    const lat = Number(match[1])
    const lng = Number(match[2])
    if (Number.isFinite(lat) && Number.isFinite(lng)) return { raw, lat, lng }
  }

  return { raw }
}

function formatTime(value?: string | null) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

function telemetryLabel(item: TelemetryItem) {
  return item.label || item.key
}

function telemetryValue(item: TelemetryItem) {
  const value = item.value === undefined || item.value === null || item.value === '' ? '-' : String(item.value)
  return item.unit ? `${value} ${item.unit}` : value
}

async function fetchDevices() {
  loading.value = true
  try {
    const params = {
      page: query.page,
      page_size: query.page_size,
      search: query.search.trim() || undefined
    }
    const { data, error } = await deviceList(params)
    if (error) {
      devices.value = []
      total.value = 0
      return
    }
    devices.value = normalizeList(data)
    total.value = normalizeTotal(data, devices.value.length)
    if (devices.value.length === 0) {
      clearSelectedDevice()
      return
    }
    const selectedStillVisible = devices.value.some(device => device.id === selectedDevice.value?.id)
    if (!selectedDevice.value || !selectedStillVisible) {
      previewDevice(devices.value[0])
    }
  } finally {
    loading.value = false
  }
}

function clearSelectedDevice() {
  telemetryRequestSeq++
  selectedDevice.value = null
  selectedTelemetry.value = null
  telemetryRequested.value = false
  telemetryLoading.value = false
}

function previewDevice(device: DeviceRecord) {
  telemetryRequestSeq++
  selectedDevice.value = device
  selectedTelemetry.value = null
  telemetryRequested.value = false
  telemetryLoading.value = false
}

async function loadSelectedTelemetry() {
  if (!selectedDevice.value?.id) return
  const requestDevice = selectedDevice.value
  const requestSeq = ++telemetryRequestSeq
  telemetryRequested.value = true
  telemetryLoading.value = true
  try {
    const { data, error } = await deviceMapTelemetry(requestDevice.id)
    if (requestSeq !== telemetryRequestSeq || selectedDevice.value?.id !== requestDevice.id) return
    selectedTelemetry.value = error ? null : data
  } finally {
    if (requestSeq === telemetryRequestSeq) {
      telemetryLoading.value = false
    }
  }
}

async function selectDevice(device: DeviceRecord) {
  previewDevice(device)
  await loadSelectedTelemetry()
}

function searchDevices() {
  query.page = 1
  fetchDevices()
}

function changePage(page: number) {
  query.page = page
  fetchDevices()
}

function openDeviceDetails() {
  if (!selectedDevice.value?.id) return
  router.push({ name: 'device_details', query: { d_id: selectedDevice.value.id } })
}

function selectedLngLat(): [number, number] | null {
  const location = selectedLocation.value
  if (!location || location.lat === undefined || location.lng === undefined) return null
  return [location.lng, location.lat]
}

async function initMap() {
  if (mapReady.value || mapLoading.value || !mapDomRef.value) return
  if (!AMAP_SDK_URL) {
    mapError.value = true
    return
  }

  mapLoading.value = true
  mapError.value = false
  try {
    ensureAmapSecurityConfig()
    await loadAmap(true)
    if (!mapDomRef.value || typeof AMap === 'undefined') return
    const center = selectedLngLat() || [114.05834626586915, 22.546789983033168]
    const mapOptions = {
      zoom: selectedLngLat() ? 13 : 5,
      center,
      viewMode: '3D',
      resizeEnable: true
    } as AMap.MapOptions & { resizeEnable: boolean }
    amapInstance = new AMap.Map(mapDomRef.value, mapOptions)
    mapReady.value = true
    updateMapMarker()
  } catch {
    mapError.value = true
  } finally {
    mapLoading.value = false
  }
}

function updateMapMarker() {
  if (!mapReady.value || !amapInstance || typeof AMap === 'undefined') return

  const position = selectedLngLat()
  if (!position) {
    if (amapMarker) {
      amapInstance.remove(amapMarker)
      amapMarker = null
    }
    return
  }

  if (!amapMarker) {
    amapMarker = new AMap.Marker({
      position,
      anchor: 'bottom-center'
    })
    amapInstance.add(amapMarker)
  } else {
    amapMarker.setPosition(position)
  }
  amapInstance.setZoomAndCenter(13, position)
}

onMounted(async () => {
  await nextTick()
  initMap()
  fetchDevices()
})

onBeforeUnmount(() => {
  if (amapInstance) {
    amapInstance.destroy()
    amapInstance = null
    amapMarker = null
  }
})

watch(selectedLocation, updateMapMarker)
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
      <section class="device-map-list">
        <NSpin :show="loading">
          <NEmpty v-if="devices.length === 0" :description="$t('common.noData')" />
          <button
            v-for="device in devices"
            :key="device.id"
            type="button"
            class="device-map-list-item"
            :class="{ active: selectedDevice?.id === device.id }"
            @click="selectDevice(device)"
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
          :page="query.page"
          :page-count="pageCount"
          @update:page="changePage"
        />
      </section>

      <section class="device-map-main">
        <div class="device-map-surface">
          <div ref="mapDomRef" class="device-map-sdk" :class="{ visible: mapReady && !mapError }"></div>
          <div v-if="!mapReady || mapError || !hasSelectedCoordinates" class="device-map-grid"></div>
          <div
            v-if="selectedDevice && (!mapReady || mapError || !hasSelectedCoordinates)"
            class="device-map-marker"
            :style="markerStyle"
          >
            <span></span>
          </div>
          <div class="device-map-status">
            <strong>{{ selectedDevice?.name || selectedDevice?.device_number || $t('rdi.map.selectDevice') }}</strong>
            <small>
              {{ mapError ? $t('rdi.map.mapUnavailable') : selectedLocation?.raw || $t('rdi.map.locationMissing') }}
            </small>
          </div>
          <NSpin v-if="mapLoading" class="device-map-loading" :show="mapLoading" />
        </div>

        <div class="device-map-detail">
          <div class="device-map-detail-head">
            <div>
              <h3>{{ selectedDevice?.name || selectedDevice?.device_number || '-' }}</h3>
              <p>{{ selectedDevice?.location || $t('rdi.map.locationMissing') }}</p>
            </div>
            <NSpace>
              <NButton :disabled="!selectedDevice" :loading="telemetryLoading" @click="loadSelectedTelemetry">
                {{ $t('rdi.map.loadTelemetry') }}
              </NButton>
              <NButton :disabled="!selectedDevice" @click="openDeviceDetails">{{ $t('rdi.map.openDetails') }}</NButton>
            </NSpace>
          </div>

          <NSpin :show="telemetryLoading">
            <div class="device-map-telemetry-summary">
              <span>{{ $t('rdi.map.lastPushTime') }}</span>
              <strong>{{ formatTime(selectedTelemetry?.last_push_time) }}</strong>
            </div>
            <div class="device-map-telemetry">
              <NEmpty
                v-if="!selectedTelemetry?.telemetry_data?.length"
                :description="telemetryRequested ? $t('rdi.map.noTelemetry') : $t('rdi.map.telemetryNotLoaded')"
              >
                <template v-if="selectedDevice && !telemetryRequested" #extra>
                  <NButton type="primary" size="small" @click="loadSelectedTelemetry">
                    {{ $t('rdi.map.loadTelemetry') }}
                  </NButton>
                </template>
              </NEmpty>
              <div v-for="item in selectedTelemetry?.telemetry_data || []" :key="item.key">
                <span>{{ telemetryLabel(item) }}</span>
                <strong>{{ telemetryValue(item) }}</strong>
              </div>
            </div>
          </NSpin>
        </div>
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
.device-map-layout,
.device-map-detail {
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

.device-map-header h2,
.device-map-detail h3 {
  margin: 0;
  font-size: 18px;
}

.device-map-header p,
.device-map-detail p {
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

.device-map-metrics span,
.device-map-telemetry-summary span,
.device-map-telemetry span {
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

.device-map-main {
  display: grid;
  grid-template-rows: minmax(300px, 1fr) auto;
}

.device-map-surface {
  position: relative;
  min-height: 320px;
  overflow: hidden;
  background: #eef3f8;
}

.device-map-sdk {
  position: absolute;
  inset: 0;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.device-map-sdk.visible {
  opacity: 1;
}

.device-map-grid {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(69, 90, 120, 0.12) 1px, transparent 1px),
    linear-gradient(90deg, rgba(69, 90, 120, 0.12) 1px, transparent 1px);
  background-size: 36px 36px;
}

.device-map-marker {
  position: absolute;
  transform: translate(-50%, -50%);
}

.device-map-marker span {
  display: block;
  width: 16px;
  height: 16px;
  border: 3px solid #ffffff;
  border-radius: 999px;
  background: #d03050;
  box-shadow: 0 2px 10px rgba(28, 35, 45, 0.35);
}

.device-map-status {
  position: absolute;
  left: 16px;
  bottom: 16px;
  display: flex;
  max-width: min(520px, calc(100% - 32px));
  flex-direction: column;
  gap: 4px;
  border: 1px solid rgba(69, 90, 120, 0.16);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.92);
  padding: 10px 12px;
}

.device-map-status small {
  color: #667085;
}

.device-map-loading {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(238, 243, 248, 0.56);
}

.device-map-detail {
  border-width: 1px 0 0;
  border-radius: 0;
  padding: 16px;
}

.device-map-detail-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.device-map-telemetry-summary {
  display: flex;
  justify-content: space-between;
  gap: 16px;
  margin-top: 16px;
  border-top: 1px solid var(--n-border-color);
  padding-top: 12px;
}

.device-map-telemetry {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 10px;
  margin-top: 12px;
}

.device-map-telemetry div {
  display: flex;
  min-width: 0;
  flex-direction: column;
  gap: 4px;
  border: 1px solid var(--n-border-color);
  border-radius: 8px;
  padding: 10px;
}

.device-map-telemetry strong {
  overflow-wrap: anywhere;
}

@media (max-width: 900px) {
  .device-map-header,
  .device-map-detail-head {
    align-items: stretch;
    flex-direction: column;
  }

  .device-map-toolbar,
  .device-map-metrics,
  .device-map-layout {
    grid-template-columns: 1fr;
  }

  .device-map-list {
    border-right: 0;
    border-bottom: 1px solid var(--n-border-color);
  }
}
</style>
