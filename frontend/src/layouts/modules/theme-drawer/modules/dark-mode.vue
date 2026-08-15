<!--
文件用途：提供深浅色和侧栏反色设置模块。
核心逻辑：根据主题状态切换暗色模式，并控制侧栏/头部反色选项可见性。
关键注意事项：暗色状态会影响全局样式变量和菜单配色。
重构建议：可把显示条件和 store 更新逻辑拆成小函数。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { themeSchemaRecord } from '@/constants/app'
import { useThemeStore } from '@/store/modules/theme'
import { $t } from '@/locales'
import SettingItem from '../components/setting-item.vue'

defineOptions({
  name: 'DarkMode'
})

const themeStore = useThemeStore()

const icons: Record<UnionKey.ThemeScheme, string> = {
  light: 'material-symbols:sunny',
  dark: 'material-symbols:nightlight-rounded',
  auto: 'material-symbols:hdr-auto'
}

function handleSegmentChange(value: string | number) {
  themeStore.setThemeScheme(value as UnionKey.ThemeScheme)
}

const showSiderInverted = computed(() => !themeStore.darkMode && themeStore.layout.mode.includes('vertical'))
</script>

<template>
  <NDivider>{{ $t('theme.themeSchema.title') }}</NDivider>
  <div class="flex-vertical-stretch gap-16px">
    <div class="i-flex-center">
      <NTabs
        :key="themeStore.themeScheme"
        type="segment"
        size="small"
        class="relative w-214px"
        :value="themeStore.themeScheme"
        @update:value="handleSegmentChange"
      >
        <NTab v-for="(_, key) in themeSchemaRecord" :key="key" :name="key">
          <SvgIcon :icon="icons[key]" class="h-23px text-icon-small" />
        </NTab>
      </NTabs>
    </div>
    <Transition name="sider-inverted">
      <SettingItem v-if="showSiderInverted" :label="$t('theme.sider.inverted')">
        <NSwitch v-model:value="themeStore.sider.inverted" />
      </SettingItem>
    </Transition>
  </div>
</template>

<style scoped>
.sider-inverted-enter-active {
  height: 22px;
  transition: all 0.3s ease-in-out;
}

.sider-inverted-leave-active {
  height: 22px;
  transition: all 0.3s ease-in-out;
}

.sider-inverted-enter-from,
.sider-inverted-leave-to {
  transform: translateX(20px);
  opacity: 0;
  height: 0;
}
</style>
