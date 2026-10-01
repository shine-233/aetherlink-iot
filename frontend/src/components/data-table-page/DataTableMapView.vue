<!--
  文件用途：<data-table-page> 的可选地图视图适配层。
  核心逻辑：把腾讯地图 SDK（异步分包加载）、加载遮罩和空态插槽封装在这里，
           通用表格组件只负责“是否渲染地图视图”这一开关，不再直接依赖具体地图厂商。
  关键注意事项：TencentMap 仅在本组件首次渲染（即用户切到地图视图）时才会拉取 chunk；
             devices 结构需满足 tencent-map.vue 的 MapDevice 约定（id/name/is_online/location/ts）。
-->
<script setup lang="ts">
import { defineAsyncComponent } from 'vue'
import { NSpin } from 'naive-ui'

defineOptions({ name: 'DataTableMapView' })

/** Structural mirror of tencent-map.vue's (unexported) MapDevice contract. */
interface DataTableMapDevice {
  id: string | number
  name: string
  ts?: string | number | null
  is_online: 0 | 1
  location?: string | null
}

const TencentMap = defineAsyncComponent(() => import('./modules/tencent-map.vue'))

withDefaults(
  defineProps<{
    devices: DataTableMapDevice[]
    loading?: boolean
    /** When true the `empty` slot replaces the map (parent decides: not loading + no rows + slot given). */
    showEmpty?: boolean
  }>(),
  { loading: false, showEmpty: false }
)

defineSlots<{ empty?: () => unknown }>()
</script>

<template>
  <NSpin :show="loading">
    <slot v-if="showEmpty" name="empty" />
    <div v-else class="map-view-container">
      <TencentMap :devices="devices" />
    </div>
  </NSpin>
</template>

<style scoped>
.map-view-container {
  height: calc(100vh - 442px);
  min-height: 360px;
}
</style>
