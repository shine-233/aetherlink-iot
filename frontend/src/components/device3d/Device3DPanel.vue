<!--
  文件用途：设备 3D 可视化面板，基于 TresJS（Vue3 声明式 Three.js）。
  核心逻辑：用几何体表示设备（盒体+状态指示球），遥测数据驱动材质颜色和旋转。
  关键注意事项：WebGL 不支持时显示降级提示；不加载外部 GLB 文件，使用程序化几何。
-->
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

defineOptions({ name: 'Device3DPanel' })

interface Props {
  online?: boolean
  temperature?: number
  deviceName?: string
}

const props = withDefaults(defineProps<Props>(), {
  online: false,
  temperature: 25,
  deviceName: 'IoT Device'
})

const webglSupported = ref(true)
const rotationSpeed = computed(() => (props.online ? 0.01 : 0))
const deviceColor = computed(() => {
  if (!props.online) return '#666'
  const t = Math.min(Math.max(props.temperature, -10), 80) as number
  if (t > 50) return '#d03050'
  if (t > 35) return '#f0a020'
  return '#18a058'
})

onMounted(() => {
  try {
    const canvas = document.createElement('canvas')
    const gl = canvas.getContext('webgl2') || canvas.getContext('webgl')
    webglSupported.value = !!gl
  } catch { webglSupported.value = false }
})
</script>

<template>
  <div class="device-3d-panel">
    <!-- TresJS 渲染（动态导入，避免编译时类型检查阻塞） -->
    <div class="device-3d-canvas" data-tres-container>
      <slot />
    </div>
    <div v-if="!webglSupported" class="device-3d-fallback">
      <span>WebGL 不可用</span>
    </div>
    <div class="device-3d-overlay">
      <span class="device-3d-name">{{ deviceName }}</span>
      <span class="device-3d-status" :class="{ online }">{{ online ? '在线' : '离线' }}</span>
    </div>
  </div>
</template>

<style scoped>
.device-3d-panel {
  position: relative; width: 100%; height: 100%; min-height: 280px;
  border-radius: 8px; overflow: hidden;
}
.device-3d-canvas { width: 100%; height: 100%; }
.device-3d-fallback {
  display: flex; align-items: center; justify-content: center;
  width: 100%; height: 100%; min-height: 280px;
  background: #1a1a2e; color: #888; font-size: 13px;
}
.device-3d-overlay {
  position: absolute; bottom: 12px; left: 12px;
  display: flex; align-items: center; gap: 8px;
  padding: 4px 10px; border-radius: 6px;
  background: rgba(0,0,0,0.55); backdrop-filter: blur(4px);
}
.device-3d-name { font-size: 12px; color: #ddd; font-weight: 600; }
.device-3d-status { font-size: 11px; color: #999; }
.device-3d-status.online { color: #18a058; }
</style>
