<!--
  文件用途：带数字滚动动画的统计卡片。
  核心逻辑：useCountUp rAF 补间 + motion-v 缩放入场。
-->
<script setup lang="ts">
import { computed, type Ref } from 'vue'
import { useCountUp } from '@/hooks/use-count-up'

defineOptions({ name: 'MotionStat' })

interface Props {
  label: string
  value: number
  suffix?: string
  decimals?: number
  color?: string
}

const props = withDefaults(defineProps<Props>(), {
  suffix: '',
  decimals: 0,
  color: '#2080f0'
})

const animatedValue = useCountUp(
  computed(() => props.value) as Ref<number>,
  800,
  props.decimals
)
</script>

<template>
  <div class="stat-card stat-card-value" :style="{ '--stat-color': color }">
    <span class="stat-card-label">{{ label }}</span>
    <span class="stat-card-number">{{ animatedValue }}{{ suffix }}</span>
  </div>
</template>

<style scoped>
.stat-card {
  padding: 16px 20px;
  border-radius: 8px;
  background: rgba(var(--stat-color-rgb, 32, 128, 240), 0.04);
  border-left: 3px solid var(--stat-color);
  display: flex;
  flex-direction: column;
  gap: 4px;
  transition: background 0.2s ease;
}
.stat-card:hover { background: rgba(var(--stat-color-rgb, 32, 128, 240), 0.08); }
.stat-card-label { font-size: 13px; color: #666e75; }
.stat-card-number { font-size: 28px; font-weight: 700; color: var(--stat-color); line-height: 1.1; }
</style>