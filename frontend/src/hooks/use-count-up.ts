/**
 * 文件用途：数字滚动动画 hook，用于看板统计卡的数值补间展示。
 * 核心逻辑：requestAnimationFrame 从起始值缓动到目标值，支持小数精度。
 */
import { ref, watch, type Ref } from 'vue'

export function useCountUp(target: Ref<number>, duration = 800, decimals = 0) {
  const display = ref(target.value)
  let rafId: number | null = null

  watch(target, (newVal, oldVal) => {
    if (rafId !== null) cancelAnimationFrame(rafId)
    const startVal = oldVal ?? 0
    const startTime = performance.now()

    const step = (now: number) => {
      const progress = Math.min((now - startTime) / duration, 1)
      const eased = 1 - Math.pow(1 - progress, 3)
      display.value = Number((startVal + (newVal - startVal) * eased).toFixed(decimals))
      if (progress < 1) rafId = requestAnimationFrame(step)
    }
    rafId = requestAnimationFrame(step)
  }, { immediate: true })

  return display
}