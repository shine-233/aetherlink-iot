/**
 * 鏂囦欢鐢ㄩ€旓細鏁板瓧婊氬姩鍔ㄧ敾 hook锛岀敤浜庣湅鏉跨粺璁″崱鐨勬暟鍊艰ˉ闂村睍绀恒€? * 鏍稿績閫昏緫锛歳equestAnimationFrame 浠庤捣濮嬪€肩紦鍔ㄥ埌鐩爣鍊硷紝鏀寔灏忔暟绮惧害銆? */
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
