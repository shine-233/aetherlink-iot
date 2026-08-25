<!--
  文件用途：CSS 骨架加载动画集合（dotLottie 插画包的纯 CSS 替代方案）。
  核心逻辑：提供设备扫描、数据流、信号波三种场景化 CSS 加载动画。
  关键注意事项：不引入外部 .lottie 文件，零依赖纯 CSS 实现。
-->
<script setup lang="ts">
defineOptions({ name: 'DeviceScanLoader' })

interface Props {
  /** 动画类型 */
  type?: 'scan' | 'dataflow' | 'signal'
  /** 提示文字 */
  label?: string
}

withDefaults(defineProps<Props>(), {
  type: 'scan',
  label: ''
})
</script>

<template>
  <div class="loader-wrapper" :class="`loader-${type}`">
    <!-- 扫描线 -->
    <div v-if="type === 'scan'" class="loader-scan">
      <div class="scan-line" />
      <span class="loader-label">{{ label }}</span>
    </div>

    <!-- 数据流粒子 -->
    <div v-else-if="type === 'dataflow'" class="loader-dataflow">
      <span v-for="i in 6" :key="i" class="flow-dot" :style="{ animationDelay: `${(i - 1) * 0.15}s` }" />
      <span class="loader-label">{{ label }}</span>
    </div>

    <!-- 信号波纹 -->
    <div v-else class="loader-signal">
      <span v-for="i in 3" :key="i" class="signal-ring" :style="{ animationDelay: `${(i - 1) * 0.4}s` }" />
      <span class="loader-label">{{ label }}</span>
    </div>
  </div>
</template>

<style scoped>
.loader-wrapper {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 32px 16px;
}
.loader-label { font-size: 12px; color: #999; }

/* ---- 扫描线 ---- */
.loader-scan { position: relative; width: 120px; height: 3px; background: #e0e0e0; border-radius: 2px; overflow: hidden; }
.scan-line {
  position: absolute; left: 0; top: 0; width: 40%; height: 100%;
  background: linear-gradient(90deg, transparent, #2080f0, transparent);
  animation: scan-sweep 1.2s ease-in-out infinite;
}
@keyframes scan-sweep { 0% { left: -40%; } 100% { left: 100%; } }

/* ---- 数据流粒子 ---- */
.loader-dataflow { display: flex; gap: 8px; }
.flow-dot {
  width: 8px; height: 8px; border-radius: 50%;
  background: #2080f0; animation: dot-bounce 0.6s ease-in-out infinite alternate;
}
@keyframes dot-bounce { from { transform: translateY(0); opacity: 1; } to { transform: translateY(-10px); opacity: 0.3; } }

/* ---- 信号波纹 ---- */
.loader-signal { position: relative; width: 48px; height: 48px; display: flex; align-items: center; justify-content: center; }
.signal-ring {
  position: absolute; inset: 0; border-radius: 50%;
  border: 2px solid #2080f0; opacity: 0;
  animation: signal-expand 1.2s ease-out infinite;
}
@keyframes signal-expand { 0% { transform: scale(0.3); opacity: 0.8; } 100% { transform: scale(1.5); opacity: 0; } }

@media (prefers-reduced-motion: reduce) {
  .scan-line, .flow-dot, .signal-ring { animation: none !important; }
}
</style>
