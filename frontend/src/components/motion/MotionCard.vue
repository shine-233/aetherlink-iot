<!--
  文件用途：带入场动画和悬浮效果的卡片包装器（纯 CSS 实现，无外部依赖）。
  核心逻辑：CSS keyframes 入场动画 + hover 悬浮上移效果。
  关键注意事项：delay 属性控制交错入场延迟；prefers-reduced-motion 时禁用。
-->
<script setup lang="ts">
defineOptions({ name: 'MotionCard' })

interface Props {
  delay?: number
  hover?: boolean
}

withDefaults(defineProps<Props>(), {
  delay: 0,
  hover: true
})
</script>

<template>
  <div
    class="motion-card"
    :class="{ 'motion-card--hover': hover }"
    :style="{ animationDelay: `${delay}ms` }"
  >
    <slot />
  </div>
</template>

<style scoped>
.motion-card {
  opacity: 0;
  animation: motion-card-enter 0.35s cubic-bezier(0.34, 1.56, 0.64, 1) forwards;
}
@keyframes motion-card-enter {
  from { opacity: 0; transform: scale(0.98) translateY(16px); }
  to   { opacity: 1; transform: scale(1) translateY(0); }
}
.motion-card--hover {
  transition: transform 0.25s cubic-bezier(0.4, 0, 0.2, 1), box-shadow 0.25s cubic-bezier(0.4, 0, 0.2, 1);
}
.motion-card--hover:hover {
  transform: translateY(-3px);
  box-shadow: 0 8px 24px rgba(32, 128, 240, 0.10), 0 2px 8px rgba(0, 0, 0, 0.06);
}

@media (prefers-reduced-motion: reduce) {
  .motion-card,
  .motion-card--hover {
    animation: none !important;
    transition: none !important;
    opacity: 1 !important;
  }
}
</style>
