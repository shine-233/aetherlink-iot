<!--
  鏂囦欢鐢ㄩ€旓細甯﹀脊绨х墿鐞嗗叆鍦哄姩鐢荤殑鍗＄墖鍖呰鍣紙鍩轰簬 motion-v锛夈€?  鏍稿績閫昏緫锛氫娇鐢?motion-v 鐨?AnimatePresence + motion.div 瀹炵幇杩涘嚭鍦哄脊绨ф晥鏋溿€?  鍏抽敭娉ㄦ剰浜嬮」锛氬寘瑁瑰湪 n-card 澶栧眰浣跨敤锛沝elay 鎺у埗浜ら敊鍏ュ満銆?-->
<script setup lang="ts">
defineOptions({ name: 'MotionCard' })

interface Props {
  /** 浜ら敊鍏ュ満寤惰繜锛堟绉掞級 */
  delay?: number
  /** 鏄惁鍚敤鎮诞涓婄Щ */
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
