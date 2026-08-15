<!--
文件用途：提供主题色配置模块。
核心逻辑：更新 theme store 中的主色、成功、警告、错误等颜色值。
关键注意事项：颜色变更会影响全局 CSS 变量和组件主题。
重构建议：可补充颜色合法性校验并统一默认色来源。
-->
<script setup lang="ts">
import { useThemeStore } from '@/store/modules/theme'
import { $t } from '@/locales'
import SettingItem from '../components/setting-item.vue'

defineOptions({
  name: 'ThemeColor'
})

const themeStore = useThemeStore()

function handleUpdateColor(color: string, key: App.Theme.ThemeColorKey) {
  themeStore.updateThemeColors(key, color)
}

const swatches: string[] = [
  '#3b82f6',
  '#6366f1',
  '#8b5cf6',
  '#a855f7',
  '#0ea5e9',
  '#06b6d4',
  '#f43f5e',
  '#ef4444',
  '#ec4899',
  '#d946ef',
  '#f97316',
  '#f59e0b',
  '#eab308',
  '#84cc16',
  '#22c55e',
  '#10b981'
]
</script>

<template>
  <NDivider>{{ $t('theme.themeColor.title' as any) }}</NDivider>
  <div class="flex-vertical-stretch gap-12px">
    <SettingItem v-for="(_, key) in themeStore.themeColors" :key="key" :label="$t(`theme.themeColor.${key}` as any)">
      <template v-if="key === 'info'" #suffix>
        <NCheckbox v-model:checked="themeStore.isInfoFollowPrimary">
          {{ $t('theme.themeColor.followPrimary' as any) }}
        </NCheckbox>
      </template>
      <NColorPicker
        class="w-90px"
        :value="themeStore.themeColors[key]"
        :disabled="key === 'info' && themeStore.isInfoFollowPrimary"
        :show-alpha="false"
        :swatches="swatches"
        @update:value="handleUpdateColor($event, key)"
      />
    </SettingItem>
  </div>
</template>

<style scoped></style>
