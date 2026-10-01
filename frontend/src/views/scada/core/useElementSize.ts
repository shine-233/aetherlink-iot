/**
 * Reactive client size of an element.
 *
 * Uses ResizeObserver where available and falls back to window `resize` (jsdom has no
 * ResizeObserver). Every observer, listener and timer registered here is released by
 * `stop()`, which is also wired to the owning component's unmount so a view that is
 * torn down mid-layout cannot leak a live observer.
 */
import { getCurrentInstance, onBeforeUnmount, ref, type Ref } from 'vue'

export interface ElementSize {
  width: number
  height: number
}

export function useElementSize(target: Ref<HTMLElement | null>) {
  const size = ref<ElementSize>({ width: 0, height: 0 })
  let observer: ResizeObserver | null = null
  let fallbackTimer: ReturnType<typeof setTimeout> | null = null
  let listening = false

  function measure() {
    const element = target.value
    if (!element) return
    size.value = { width: element.clientWidth, height: element.clientHeight }
  }

  function stop() {
    observer?.disconnect()
    observer = null
    if (fallbackTimer !== null) {
      clearTimeout(fallbackTimer)
      fallbackTimer = null
    }
    if (listening) {
      window.removeEventListener('resize', measure)
      listening = false
    }
  }

  function start() {
    stop()
    measure()
    if (typeof ResizeObserver !== 'undefined') {
      observer = new ResizeObserver(() => measure())
      if (target.value) observer.observe(target.value)
      return
    }
    if (typeof window !== 'undefined') {
      window.addEventListener('resize', measure)
      listening = true
      fallbackTimer = setTimeout(measure, 200)
    }
  }

  if (getCurrentInstance()) onBeforeUnmount(stop)

  return { size, start, stop, measure }
}
