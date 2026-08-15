<!--
文件用途：渲染主题抽屉中的布局模式卡片。
核心逻辑：展示不同布局模式预览，并通过事件通知父级更新模式。
关键注意事项：组件应保持纯展示和事件派发，不直接写 theme store。
重构建议：可把布局模式配置集中到常量，减少模板重复。
-->
<script setup lang="ts">
import type { PopoverPlacement } from 'naive-ui'
import { themeLayoutModeRecord } from '@/constants/app'
import { $t } from '@/locales'

defineOptions({
  name: 'LayoutModeCard'
})

interface Props {
  /** Layout mode */
  mode: UnionKey.ThemeLayoutMode
  /** Disabled */
  disabled?: boolean
}

const props = defineProps<Props>()

interface Emits {
  /** Layout mode change */
  (e: 'update:mode', mode: UnionKey.ThemeLayoutMode): void
}

const emit = defineEmits<Emits>()

type LayoutConfig = Record<
  UnionKey.ThemeLayoutMode,
  {
    placement: PopoverPlacement
    headerClass: string
    menuClass: string
    mainClass: string
  }
>

const layoutConfig: LayoutConfig = {
  vertical: {
    placement: 'bottom',
    headerClass: '',
    menuClass: 'w-1/3 h-full',
    mainClass: 'w-2/3 h-3/4'
  },
  'vertical-mix': {
    placement: 'bottom',
    headerClass: '',
    menuClass: 'w-1/4 h-full',
    mainClass: 'w-2/3 h-3/4'
  },
  horizontal: {
    placement: 'bottom',
    headerClass: '',
    menuClass: 'w-full h-1/4',
    mainClass: 'w-full h-3/4'
  },
  'horizontal-mix': {
    placement: 'bottom',
    headerClass: '',
    menuClass: 'w-full h-1/4',
    mainClass: 'w-2/3 h-3/4'
  }
}

function handleChangeMode(mode: UnionKey.ThemeLayoutMode) {
  if (props.disabled) return

  emit('update:mode', mode)
}
</script>

<template>
  <div class="flex-center flex-wrap gap-x-32px gap-y-16px">
    <div
      v-for="(item, key) in layoutConfig"
      :key="key"
      class="flex cursor-pointer border-2px rounded-6px hover:border-primary"
      :class="[mode === key ? 'border-primary' : 'border-transparent']"
      @click="handleChangeMode(key)"
    >
      <NTooltip :placement="item.placement">
        <template #trigger>
          <div
            class="h-64px w-96px gap-6px rd-4px p-6px shadow dark:shadow-coolGray-5"
            :class="[key.includes('vertical') ? 'flex' : 'flex-vertical']"
          >
            <slot :name="key"></slot>
          </div>
        </template>
        {{ $t(themeLayoutModeRecord[key]) }}
      </NTooltip>
    </div>
  </div>
</template>

<style scoped></style>
