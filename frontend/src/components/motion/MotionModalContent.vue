<!--
  文件用途：带弹簧进出动画的模态框包装器，替代原生 n-modal 的默认过渡。
  核心逻辑：使用 CSS 弹簧缓动曲线实现 scale+translateY 进出场效果。
  关键注意事项：包裹在 <n-modal> 的内容区域外层使用；不修改 n-modal 自身行为。
-->
<script setup lang="ts">
defineOptions({ name: 'MotionModalContent' })
</script>

<template>
  <div class="motion-modal-content">
    <slot />
  </div>
</template>

<style scoped>
.motion-modal-content {
  animation: modal-spring-in 0.32s cubic-bezier(0.34, 1.56, 0.64, 1);
}

@keyframes modal-spring-in {
  0% {
    opacity: 0;
    transform: scale(0.92) translateY(16px);
  }
  60% {
    transform: scale(1.01) translateY(-2px);
  }
  100% {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .motion-modal-content {
    animation: none !important;
  }
}
</style>
