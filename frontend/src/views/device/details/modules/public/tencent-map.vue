<!--
  文件用途:
  设备详情中的腾讯地图定位组件，用于展示当前经纬度，并支持用户点击地图重新选点。

  数据流:
  1. 父组件通过 longitude/latitude 传入当前位置。
  2. 组件挂载后加载腾讯地图 SDK，并以传入坐标或默认中心点初始化地图。
  3. 用户点击地图后创建/更新标记，并通过 position-selected 事件把新坐标抛回父组件。

  使用注意:
  1. 该组件依赖外部 SDK 脚本加载成功，网络异常或 key 配置问题都会影响渲染。
  2. longitude/latitude 支持字符串与数字，但内部会统一转为 Number 后再做校验。
  3. currentMarker 为单实例状态，后续若支持多点位展示，需要重新设计标记管理方式。

  静态审查建议:
  1. renderMap 同时承担 SDK 加载、地图初始化、事件绑定职责，审查时重点关注异常分支是否足够清晰。
  2. watch 只在新旧坐标均非空时更新地图，需确认这与上层“清空定位”的交互预期一致。
  3. map/currentMarker 当前未显式声明 SDK 类型，后续可补类型封装以减少运行期 API 误用。
-->
<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useScriptTag } from '@vueuse/core'
import { TENCENT_MAP_SDK_URL } from '@/constants/map-sdk'
import { $t } from '@/locales'
import { isValidCoordinate, getCoordinateValidationError } from '@/utils/common/map-validator'

const { load } = useScriptTag(TENCENT_MAP_SDK_URL)

// 点击地图选点后，将标准化后的经纬度回传给父组件。
const emit = defineEmits(['position-selected'])

// 显式命名，便于调试工具和 keep-alive 场景识别组件。
defineOptions({ name: 'TencentMap' })

// 接收父层传入的设备经纬度，允许为空字符串以兼容表单态数据。
interface Props {
  longitude?: string | number
  latitude?: string | number
}

const props = withDefaults(defineProps<Props>(), {
  longitude: '',
  latitude: ''
})
const domRef = ref<HTMLDivElement | null>(null)
const mapUnavailable = ref(false)
let map
let currentMarker

// 地图初始化入口：
// 1. 异步加载腾讯地图 SDK；
// 2. 根据 props 计算中心点与缩放级别；
// 3. 绑定点击事件并向父组件回传新坐标。
// 静态审查建议：这里没有显式 try/catch，若后续需要更稳定的失败提示，可优先从此处补充。
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

  // 优先使用外部传入坐标；为空或无效时退回默认中心点。
  const lat = Number(props.latitude) || 39.98412
  const lng = Number(props.longitude) || 116.307484
  const hasValidCoords =
    props.latitude && props.longitude && props.latitude !== '' && props.longitude !== '' && isValidCoordinate(lat, lng)

  const center = new TMap.LatLng(lat, lng)

  map = new TMap.Map(domRef.value, {
    center,
    // 有有效坐标时采用更近的缩放级别，便于直接查看设备位置。
    zoom: hasValidCoords ? 15 : 9,
    maxZoom: 18,
    minZoom: 6
  })

  // 初始化时如果已有合法坐标，则直接渲染当前位置标记。
  if (hasValidCoords) {
    addCurrentLocationMarker(lat, lng)
  }

  map.on('click', event => {
    const lat = event.latLng.getLat()
    const lng = event.latLng.getLng()

    // 地图点击返回的数据仍需校验，避免异常坐标继续污染上层状态。
    if (!isValidCoordinate(lat, lng)) {
      const error = getCoordinateValidationError(lat, lng)
      console.error('地图点击事件返回了无效经纬度', { lat, lng, error })
      return
    }

    // 仅保留最新一次选点标记，避免视觉上出现多设备点位的误解。
    if (currentMarker) {
      currentMarker.setMap(null)
    }

    addCurrentLocationMarker(lat, lng)

    emit('position-selected', { lat, lng })
  })
}

// 在地图上渲染当前位置标记。
// 使用注意：该方法会主动移除旧 marker，因此它本质上是“替换当前位置”而不是“追加标记”。
function addCurrentLocationMarker(lat: number, lng: number) {
  if (!isValidCoordinate(lat, lng)) {
    const error = getCoordinateValidationError(lat, lng)
    console.error('addCurrentLocationMarker 接收到无效经纬度参数', { lat, lng, error })
    return
  }

  if (currentMarker) {
    currentMarker.setMap(null)
  }

  const position = new TMap.LatLng(lat, lng)

  // 统一定义当前定位点样式，避免 SDK 默认图标与业务视觉不一致。
  const markerStyle = new TMap.MarkerStyle({
    width: 30,
    height: 40,
    anchor: { x: 15, y: 40 },
    src:
      'data:image/svg+xml;base64,' +
      btoa(`
      <svg xmlns="http://www.w3.org/2000/svg" width="30" height="40" viewBox="0 0 30 40">
        <path d="M15 0C6.716 0 0 6.716 0 15c0 8.284 15 25 15 25s15-16.716 15-25C30 6.716 23.284 0 15 0z" fill="#ff4444"/>
        <circle cx="15" cy="15" r="8" fill="white"/>
        <circle cx="15" cy="15" r="5" fill="#ff4444"/>
      </svg>
    `)
  })

  currentMarker = new TMap.MultiMarker({
    map,
    styles: {
      'current-location': markerStyle
    },
    geometries: [
      {
        id: 'current-position',
        styleId: 'current-location',
        position
      }
    ]
  })
}

onMounted(() => {
  renderMap()
})

// 监听外部坐标变化并同步更新地图中心与标记。
// 静态审查建议：当前只有“非空且合法”时才会进入更新分支，调用方若传空值不会主动清理旧标记。
watch(
  () => [props.latitude, props.longitude],
  ([newLat, newLng]) => {
    if (map && newLat && newLng && newLat !== '' && newLng !== '') {
      const lat = Number(newLat)
      const lng = Number(newLng)
      if (isValidCoordinate(lat, lng)) {
        const center = new TMap.LatLng(lat, lng)
        map.setCenter(center)
        addCurrentLocationMarker(lat, lng)
      } else {
        const error = getCoordinateValidationError(lat, lng)
        console.warn('监听到无效的经纬度更新', { lat, lng, error })
      }
    }
  },
  { immediate: false }
)
</script>

<template>
  <div class="relative w-full h-full">
    <div ref="domRef" class="w-full h-full"></div>
    <div v-if="mapUnavailable" class="absolute inset-0 flex-center bg-[var(--n-color)] text-sm">
      {{ $t('rdi.map.mapUnavailable') }}
    </div>
  </div>
</template>

<style scoped></style>
