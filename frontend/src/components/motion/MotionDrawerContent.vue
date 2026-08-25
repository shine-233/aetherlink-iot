<!--
  文件用途：带弹簧滑入动画的抽屉内容包装器。
  核心逻辑：CSS 弹簧缓动从右侧滑入 + 淡出退场。
-->
<script setup lang="ts">
defineOptions({ name: 'MotionDrawerContent' })
</script>

<template>
  <div class="motion-drawer-content">
    <slot />
  </div>
</template>

<style scoped>
.motion-drawer-content {
  animation: drawer-spring-in 0.36s cubic-bezier(0.22, 1, 0.36, 1);
}

@keyframes drawer-spring-in {
  0% {
    opacity: 0;
    transform: translateX(48px);
  }
  70% {
    transform: translateX(-4px);
  }
  100% {
    opacity: 1;
    transform: translateX(0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .motion-drawer-content {
    animation: none !important;
  }
}
</style>
