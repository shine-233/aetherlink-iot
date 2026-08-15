import { nextTick, onMounted, onUnmounted, ref, type Ref } from 'vue'

export interface ViewportDeferredMountOptions {
  rootMargin?: string
  fallbackDelay?: number
}

export function useViewportDeferredMount(
  targetRef: Ref<HTMLElement | null>,
  options: ViewportDeferredMountOptions = {}
) {
  const shouldMount = ref(false)
  const rootMargin = options.rootMargin || '360px 0px'
  const fallbackDelay = options.fallbackDelay ?? 600
  let observer: IntersectionObserver | null = null
  let fallbackTimer: ReturnType<typeof setTimeout> | null = null

  const clearObserver = () => {
    observer?.disconnect()
    observer = null
    if (fallbackTimer !== null) {
      clearTimeout(fallbackTimer)
      fallbackTimer = null
    }
  }

  const mountNow = () => {
    shouldMount.value = true
    clearObserver()
  }

  const reset = () => {
    shouldMount.value = false
    clearObserver()
  }

  const observe = async () => {
    clearObserver()
    if (shouldMount.value) return

    await nextTick()
    const target = targetRef.value
    if (!target) return
    if (typeof window === 'undefined') {
      mountNow()
      return
    }
    if (!('IntersectionObserver' in window)) {
      fallbackTimer = setTimeout(mountNow, fallbackDelay)
      return
    }

    observer = new IntersectionObserver(
      (entries) => {
        if (entries.some(entry => entry.isIntersecting || entry.intersectionRatio > 0)) {
          mountNow()
        }
      },
      { rootMargin, threshold: 0.01 }
    )
    observer.observe(target)
  }

  onMounted(() => {
    void observe()
  })

  onUnmounted(() => {
    clearObserver()
  })

  return {
    shouldMount,
    mountNow,
    reset,
    observe,
    clearObserver
  }
}
