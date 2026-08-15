<!-- Lightweight telemetry number animation; replaces the single-purpose external package. -->
<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    mNum: number | string
    quantileShow?: boolean
  }>(),
  { quantileShow: true }
)

const ANIMATION_DURATION_MS = 300
const MAX_DECIMAL_PLACES = 12

const decimalPlaces = (value: number | string) => {
  if (!props.quantileShow) return 0
  const match = String(value).match(/\.(\d+)/)
  return Math.min(match?.[1].length ?? 0, MAX_DECIMAL_PLACES)
}

const formatValue = (value: number, source: number | string) => value.toFixed(decimalPlaces(source))
const parseFiniteNumber = (value: number | string) => {
  const parsed = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(parsed) ? parsed : null
}

const initialNumber = parseFiniteNumber(props.mNum)
const displayValue = ref(initialNumber === null ? String(props.mNum) : formatValue(initialNumber, props.mNum))
let displayedNumber = initialNumber
let animationFrame: number | null = null

const cancelAnimation = () => {
  if (animationFrame !== null && typeof cancelAnimationFrame === 'function') cancelAnimationFrame(animationFrame)
  animationFrame = null
}

const shouldSkipAnimation = () =>
  typeof requestAnimationFrame !== 'function' ||
  (typeof matchMedia === 'function' && matchMedia('(prefers-reduced-motion: reduce)').matches)

watch(
  () => props.mNum,
  nextValue => {
    cancelAnimation()
    const target = parseFiniteNumber(nextValue)
    if (target === null) {
      displayedNumber = null
      displayValue.value = String(nextValue)
      return
    }

    if (displayedNumber === null || shouldSkipAnimation()) {
      displayedNumber = target
      displayValue.value = formatValue(target, nextValue)
      return
    }

    const start = displayedNumber
    const startedAt = performance.now()
    const animate = (timestamp: number) => {
      const progress = Math.min((timestamp - startedAt) / ANIMATION_DURATION_MS, 1)
      const easedProgress = 1 - (1 - progress) ** 3
      const current = start + (target - start) * easedProgress
      displayedNumber = current
      displayValue.value = formatValue(current, nextValue)
      if (progress < 1) animationFrame = requestAnimationFrame(animate)
      else animationFrame = null
    }
    animationFrame = requestAnimationFrame(animate)
  }
)

onBeforeUnmount(cancelAnimation)
</script>

<template>
  <span>{{ displayValue }}</span>
</template>
