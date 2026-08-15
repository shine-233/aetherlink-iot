<!--
文件用途：渲染全局页脚区域。
核心逻辑：提供主布局底部展示容器。
关键注意事项：保持展示职责，避免耦合权限或业务状态。
重构建议：后续可将页脚文案和链接配置化。
-->
<script setup lang="ts">
import { $t } from '@/locales'
import useVersionInfo from '@/hooks/business/use-version-info'
import { useSysSettingStore } from '@/store/modules/sys-setting'

defineOptions({
  name: 'GlobalFooter'
})

const sysSettingStore = useSysSettingStore()
const { currentVersion, latestVersion } = useVersionInfo()
</script>

<template>
  <DarkModeContainer class="h-full flex-center">
    <div class="flex flex-wrap items-center justify-center gap-x-2 text-center text-xs text-[var(--n-text-color)]">
      <a
        href="https://github.com/shine-233/aetherlink-iot/blob/main/LICENSE"
        target="_blank"
        rel="noopener noreferrer"
      >
        Copyright &copy; 2024 {{ sysSettingStore.system_name }}
      </a>
      <span>|</span>
      <span>{{ $t('card.versionInfo.currentVersion') }} {{ currentVersion }}</span>
      <span>|</span>
      <span>{{ $t('card.versionInfo.latestVersion') }}{{ latestVersion }}</span>
    </div>
  </DarkModeContainer>
</template>

<style scoped></style>
