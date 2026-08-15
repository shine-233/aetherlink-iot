<!--
  文件用途：实现设备数据表格页及其地图展示模块。
  核心逻辑：组合表格、筛选、分页、设备卡片和地图组件，呈现设备列表与地理位置。
  关键注意事项：接口字段、地图坐标和设备在线状态需要与后端数据保持一致。
  重构建议：可把数据加载、筛选状态和地图适配拆分，减少页面组件职责。
-->
<script setup lang="tsx">
import { createApp, onMounted, ref, watch, watchEffect } from 'vue'
import { NCard } from 'naive-ui'
import { useScriptTag } from '@vueuse/core'
import dayjs from 'dayjs'
import { TENCENT_MAP_SDK_URL } from '@/constants/map-sdk'
import { $t } from '@/locales'
import { telemetryLatestApi } from '@/service/api/system-data'
import { createLogger } from '@/utils/logger'
import { isValidCoordinate } from '@/utils/common/map-validator'

const logger = createLogger('GaodeMap')
defineOptions({ name: 'TencentMap' })

const props = defineProps<{ devices: any[] }>()

const { load } = useScriptTag(TENCENT_MAP_SDK_URL)

const domRef = ref<HTMLDivElement | null>(null)
const mapUnavailable = ref(false)
let map: any = null
let multiMarker: any = null
let infoWindow: any = null
let ignoreMapClick = false

const DEFAULT_CENTER = { lat: 39.98412, lng: 116.307484 }
const DEFAULT_ZOOM = 11
const VIEWPORT_PADDING = 100

type DeviceMarker = {
  position: any
  id: any
  data: any
  telemetryItems?: any[]
}

const renderInfoWindow = (evt: any, _res: any) => {
  const statusText = {
    1: $t('custom.devicePage.online'),
    0: $t('custom.devicePage.offline')
  } as const
  const telemetryItems = Array.isArray(evt.geometry.telemetryItems) ? evt.geometry.telemetryItems : []

  return (
    <NCard
      header-style="padding:10px"
      title={`${$t('custom.devicePage.deviceName')}：${evt.geometry.data.name}`}
      class="min-h-130px min-w-200px"
    >
      <div>
        {$t('custom.devicePage.lastPushTime')}：
        {evt.geometry.data.ts ? dayjs(evt.geometry.data.ts).format('YYYY-MM-DD HH:mm:ss') : '-'}
      </div>
      <div>
        {telemetryItems.map((item: any) => {
          const label = item.label ? `${item.label}(${item.key})` : item.key

          return (
            <div class="item_label" key={`${item.key}-${item.label}`}>
              {label}：<span class="card_val">{item.value}</span> {item.unit}
            </div>
          )
        })}
      </div>
      <div>
        {$t('generate.status')}：{statusText[evt.geometry.data.is_online]}
      </div>
    </NCard>
  )
}

const createLatLng = ({ lat, lng }: { lat: number; lng: number }) => new TMap.LatLng(lat, lng)

const resetToDefaultViewport = (bounds: any) => {
  const defaultCenter = createLatLng(DEFAULT_CENTER)
  bounds.extend(defaultCenter)
  map.setCenter(defaultCenter)
  map.setZoom(DEFAULT_ZOOM)
}

const isValidMarker = (marker: DeviceMarker) => {
  const position = marker?.position
  return (
    position &&
    typeof position.lat === 'number' &&
    typeof position.lng === 'number' &&
    isValidCoordinate(position.lat, position.lng)
  )
}

const updateViewport = (markers: DeviceMarker[]) => {
  const bounds = new TMap.LatLngBounds()
  const validMarkers = markers.filter(isValidMarker)

  if (validMarkers.length === 0) {
    resetToDefaultViewport(bounds)
    return
  }

  validMarkers.forEach(marker => {
    if (bounds.isEmpty() || !bounds.contains(marker.position)) {
      bounds.extend(marker.position)
    }
  })

  map.fitBounds(bounds, {
    padding: VIEWPORT_PADDING
  })
}

const handleMapClick = () => {
  if (ignoreMapClick) {
    ignoreMapClick = false
    return
  }

  if (infoWindow) {
    infoWindow.close()
  }
}

const initializeMap = () => {
  if (map || !domRef.value) return

  map = new TMap.Map(domRef.value, {
    center: createLatLng(DEFAULT_CENTER),
    zoom: DEFAULT_ZOOM,
    maxZoom: 13,
    minZoom: 3,
    viewMode: '3D'
  })
  map.on('click', handleMapClick)
}

const clearMarkerLayer = () => {
  if (!multiMarker) return

  multiMarker.setMap(null)
  multiMarker = null
}

const createDeviceMarker = (device: any): DeviceMarker | null => {
  if (!device?.location) return null

  const locations = String(device.location).split(',')
  const latitude = Number(locations[1] || 0)
  const longitude = Number(locations[0] || 0)

  if (!isValidCoordinate(latitude, longitude)) return null

  return {
    position: new TMap.LatLng(latitude, longitude),
    id: device.id,
    data: device
  }
}

const createDeviceMarkers = (devices: any[] = []) =>
  devices.reduce<DeviceMarker[]>((markers, device) => {
    const marker = createDeviceMarker(device)
    if (marker) {
      markers.push(marker)
    }

    return markers
  }, [])

const normalizeTelemetryItems = (items: any[]) =>
  items
    .filter((item: any) => item.label || item.key)
    .map((item: any) => ({
      label: item?.label == null ? '' : String(item.label),
      key: item?.key == null ? '' : String(item.key),
      value: item?.value == null ? '' : String(item.value),
      unit: item?.unit == null ? '' : String(item.unit)
    }))

const ignoreNextMapClick = () => {
  ignoreMapClick = true
  setTimeout(() => {
    ignoreMapClick = false
  }, 10)
}

const ensureInfoWindow = () => {
  if (!infoWindow) {
    infoWindow = new TMap.InfoWindow({
      map,
      position: new TMap.LatLng(39.984104, 116.307503),
      offset: { x: 0, y: -32 },
      enableCustom: true
    })
  }

  return infoWindow
}

const renderInfoWindowHtml = (evt: any, res: any) => {
  const app = createApp({
    setup() {
      return () => renderInfoWindow(evt, res)
    }
  })

  return app.mount(document.createElement('div')).$el.outerHTML
}

const openMarkerInfoWindow = (evt: any, res: any) => {
  const markerInfoWindow = ensureInfoWindow()
  const html = renderInfoWindowHtml(evt, res)

  markerInfoWindow.open()
  markerInfoWindow.setPosition(evt.geometry.position)
  markerInfoWindow.setContent(html)
  evt.originalEvent.stopPropagation()
}

const handleMarkerClick = (evt: any) => {
  if (!evt?.geometry?.data?.id) return

  telemetryLatestApi(evt.geometry.data.id).then((res: any) => {
    if (!Array.isArray(res?.data)) return

    evt.geometry.telemetryItems = normalizeTelemetryItems(res.data)
    ignoreNextMapClick()
    openMarkerInfoWindow(evt, res)
  })
}

const createMarkerLayer = (markers: DeviceMarker[]) => {
  if (markers.length === 0) return

  multiMarker = new TMap.MultiMarker({
    map,
    styles: {
      marker: new TMap.MarkerStyle({
        width: 20,
        height: 30,
        anchor: { x: 10, y: 30 },
        color: '#333'
      })
    },
    geometries: markers
  })

  multiMarker.on('click', handleMarkerClick)
}

async function renderMap() {
  if (!TENCENT_MAP_SDK_URL) {
    mapUnavailable.value = true
    return
  }

  try {
    await load(true)
  } catch {
    mapUnavailable.value = true
    return
  }
  if (!domRef.value || typeof TMap === 'undefined') {
    mapUnavailable.value = true
    return
  }
  mapUnavailable.value = false

  initializeMap()
  clearMarkerLayer()

  const markers = createDeviceMarkers(props.devices)
  createMarkerLayer(markers)
  updateViewport(markers)
}

onMounted(() => {
  renderMap()
})

watch(
  () => props.devices,
  async newValue => {
    logger.info(newValue)
    await renderMap()
    if (infoWindow) {
      infoWindow.close()
    }
  },
  { deep: true }
)

watchEffect(async () => {
  await renderMap()
})
</script>

<template>
  <div class="relative h-full w-full">
    <div ref="domRef" class="h-full w-full"></div>
    <div v-if="mapUnavailable" class="absolute inset-0 flex-center bg-[var(--n-color)] text-sm">
      {{ $t('rdi.map.mapUnavailable') }}
    </div>
  </div>
</template>

<style lang="scss" scoped>
:deep(.n-card) {
  display: flex;
  align-items: flex-start;
  border: 0;
  padding: 0 15px;
}

:deep(.n-card__content) {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  padding-left: 10px;
}

.card_map {
  padding: 10px !important;
}

:deep(.card_val) {
  color: rgb(var(--nprogress-color)) !important;
}

:deep(.item_label) {
  display: flex;
}
</style>
