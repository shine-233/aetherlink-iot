<!--
  文件用途：实现设备数据表格页的高德地图展示模块。
  核心逻辑：仅在部署配置了 SDK 密钥且脚本成功加载时初始化地图。
  关键注意事项：高德地图是可选外部能力；缺少密钥或加载失败时必须明确降级，不能阻断页面。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useScriptTag } from '@vueuse/core'
import { AMAP_SDK_URL } from '@/constants/map-sdk'
import { $t } from '@/locales'

defineOptions({ name: 'GaodeMap' })

const scriptLoader = AMAP_SDK_URL ? useScriptTag(AMAP_SDK_URL) : null
const domRef = ref<HTMLDivElement>()
const mapUnavailable = ref(!AMAP_SDK_URL)

async function renderMap() {
  if (!scriptLoader) return

  try {
    await scriptLoader.load(true)
    if (!domRef.value) return

    const map = new AMap.Map(domRef.value, {
      zoom: 11,
      center: [114.05834626586915, 22.546789983033168],
      viewMode: '3D'
    })
    map.getCenter()
  } catch {
    mapUnavailable.value = true
  }
}

onMounted(() => {
  void renderMap()
})
</script>

<template>
  <div v-if="mapUnavailable" class="h-full w-full flex-center text-14px text-gray-500">
    {{ $t('rdi.map.mapUnavailable') }}
  </div>
  <div v-else ref="domRef" class="h-full w-full"></div>
</template>

<style scoped></style>
