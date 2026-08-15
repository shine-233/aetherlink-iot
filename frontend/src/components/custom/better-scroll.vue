<!-- Native scrolling compatibility boundary; keeps the historical BetterScroll component API without an external runtime. -->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

defineOptions({ name: 'BetterScroll' })

export interface BetterScrollOptions {
  scrollX?: boolean
  scrollY?: boolean
  [key: string]: unknown
}

export interface BetterScrollInstance {
  refresh: () => void
  destroy: () => void
  scrollTo: (x: number, y: number, behavior?: ScrollBehavior) => void
}

const props = defineProps<{
  /** Direction flags remain compatible; unsupported legacy options are safely ignored. */
  options: BetterScrollOptions
}>()

const scrollContainer = ref<HTMLElement>()
const instance = ref<BetterScrollInstance>()
const isScrollX = computed(() => Boolean(props.options.scrollX))
const isScrollY = computed(() => Boolean(props.options.scrollY))
const overflowX = computed(() => (isScrollX.value ? 'auto' : 'hidden'))
const overflowY = computed(() => (isScrollY.value ? 'auto' : 'hidden'))

function createNativeScrollInstance(element: HTMLElement): BetterScrollInstance {
  let destroyed = false
  return {
    // Native overflow layout updates automatically; reading dimensions forces a
    // synchronous layout refresh for callers that relied on this method.
    refresh() {
      if (!destroyed) void element.scrollHeight
    },
    destroy() {
      destroyed = true
    },
    scrollTo(x, y, behavior = 'auto') {
      if (!destroyed) element.scrollTo({ left: x, top: y, behavior })
    }
  }
}

onMounted(() => {
  if (scrollContainer.value) instance.value = createNativeScrollInstance(scrollContainer.value)
})

onBeforeUnmount(() => {
  instance.value?.destroy()
  instance.value = undefined
})

defineExpose({ instance })
</script>

<template>
  <div
    ref="scrollContainer"
    class="h-full text-left"
    data-test="native-scroll-container"
    :style="{ overflowX, overflowY }"
  >
    <div class="inline-block" :class="{ 'h-full': !isScrollY, 'min-w-full': !isScrollX }">
      <slot></slot>
    </div>
  </div>
</template>
