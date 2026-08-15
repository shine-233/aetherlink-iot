<!--
  文件用途：提供明暗主题方案切换入口。
  核心逻辑：读取当前主题模式并触发主题状态更新，让全局样式和 Naive UI 主题同步切换。
  关键注意事项：主题状态会影响 CSS 变量、暗色容器和持久化配置，调整时需验证刷新后的恢复行为。
  重构建议：可将主题选项、图标和持久化逻辑收敛为统一主题服务。
-->
<script setup lang="ts">
import { computed } from 'vue'
import type { PopoverPlacement } from 'naive-ui'
import { $t } from '@/locales'

defineOptions({ name: 'ThemeSchemaSwitch' })

interface Props {
  /** Theme schema */
  themeSchema: UnionKey.ThemeScheme
  /** Show tooltip */
  showTooltip?: boolean
  /** Tooltip placement */
  tooltipPlacement?: PopoverPlacement
}

const props = withDefaults(defineProps<Props>(), {
  showTooltip: true,
  tooltipPlacement: 'bottom'
})

interface Emits {
  (e: 'switch'): void
}

const emit = defineEmits<Emits>()

function handleSwitch() {
  emit('switch')
}

const icons: Record<UnionKey.ThemeScheme, string> = {
  light: 'material-symbols:sunny',
  dark: 'material-symbols:nightlight-rounded',
  auto: 'material-symbols:hdr-auto'
}

const icon = computed(() => icons[props.themeSchema])

const tooltipContent = computed(() => {
  if (!props.showTooltip) return ''

  return $t('icon.themeSchema')
})
</script>

<template>
  <ButtonIcon
    :icon="icon"
    :tooltip-content="tooltipContent"
    :tooltip-placement="tooltipPlacement"
    @click="handleSwitch"
  />
</template>

<style scoped></style>
