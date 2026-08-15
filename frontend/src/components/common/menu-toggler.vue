<!--
  文件用途：提供侧边菜单折叠切换按钮。
  核心逻辑：根据当前菜单折叠状态切换图标，并向布局状态发出展开或收起动作。
  关键注意事项：按钮状态需与布局 store 保持同步，避免移动端和桌面端菜单状态错位。
  重构建议：可把布局状态读写统一封装，减少多个切换按钮重复逻辑。
-->
<script lang="ts" setup>
import { computed } from 'vue'
import { $t } from '@/locales'

defineOptions({ name: 'MenuToggler' })

interface Props {
  /** Show collapsed icon */
  collapsed?: boolean
  /** Arrow style icon */
  arrowIcon?: boolean
}

const props = defineProps<Props>()

type NumberBool = 0 | 1

const icon = computed(() => {
  const icons: Record<NumberBool, Record<NumberBool, string>> = {
    0: {
      0: 'line-md:menu-fold-left',
      1: 'line-md:menu-fold-right'
    },
    1: {
      0: 'ph-caret-double-left-bold',
      1: 'ph-caret-double-right-bold'
    }
  }

  const arrowIcon = Number(props.arrowIcon || false) as NumberBool

  const collapsed = Number(props.collapsed || false) as NumberBool

  return icons[arrowIcon][collapsed]
})
</script>

<template>
  <ButtonIcon :tooltip-content="collapsed ? $t('icon.expand') : $t('icon.collapse')" tooltip-placement="bottom-start">
    <SvgIcon :icon="icon" />
  </ButtonIcon>
</template>

<style scoped></style>
