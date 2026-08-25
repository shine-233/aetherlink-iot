<!--
  组件用途: motion-v 动画卡片容器。
  核心逻辑: 基于 motion-v 的 AnimatePresence + motion.div 包裹 n-card 内容，支持 delay 等入场配置。
-->
<script setup lang="ts">
import { motion } from 'motion-v'

defineOptions({ name: 'MotionCard' })

interface Props {
  /** 交错入场延迟（毫秒） */
  delay?: number
  /** 是否启用悬浮上移 */
  hover?: boolean
}

withDefaults(defineProps<Props>(), {
  delay: 0,
  hover: true
})
</script>

<template>
  <motion.div
    :initial="{ opacity: 0, y: 16, scale: 0.98 }"
    :animate="{ opacity: 1, y: 0, scale: 1 }"
    :exit="{ opacity: 0, y: -8, scale: 0.98 }"
    :transition="{
      type: 'spring',
      stiffness: 260,
      damping: 26,
      delay: delay / 1000
    }"
    class="motion-card"
    :class="{ 'motion-card--hover': hover }"
  >
    <slot />
  </motion.div>
</template>

<style scoped>
.motion-card {
  will-change: transform, opacity;
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
  }
}
</style>
