<!--
  文件用途：提供固定或取消固定面板的切换按钮。
  核心逻辑：根据传入状态切换 pin 图标和事件输出，供布局或标签栏控制固定行为。
  关键注意事项：固定状态通常会被持久化或影响布局宽度，修改事件名时需同步调用方。
  重构建议：可与其他 toggler 组件统一抽象为状态切换按钮。
-->
<script lang="ts" setup>
import { computed } from 'vue'
import { $t } from '@/locales'

defineOptions({ name: 'PinToggler' })

interface Props {
  pin?: boolean
}

const props = defineProps<Props>()

const icon = computed(() => (props.pin ? 'mdi-pin-off' : 'mdi-pin'))
</script>

<template>
  <ButtonIcon
    :tooltip-content="pin ? $t('icon.pin') : $t('icon.unpin')"
    tooltip-placement="bottom-start"
    trigger-parent
  >
    <SvgIcon :icon="icon" />
  </ButtonIcon>
</template>

<style scoped></style>
