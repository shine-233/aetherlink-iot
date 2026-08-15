<!--
  文件用途：渲染系统 Logo 和产品标识。
  核心逻辑：根据配置展示图标、标题或折叠态样式，服务应用导航和登录布局。
  关键注意事项：产品命名和 Logo 属于公开品牌面，修改时需同步 README、截图和部署配置。
  重构建议：可将品牌配置来源统一到应用配置层，减少硬编码展示。
-->
<script lang="ts" setup>
import { useSysSettingStore } from '@/store/modules/sys-setting'
defineOptions({ name: 'SystemLogo' })
const sysSettingStore = useSysSettingStore()
const defaultRdiLogo = '/rdi/logo.png'
export interface Props {
  width: string
}

withDefaults(defineProps<Props>(), {
  width: '40'
})
</script>

<template>
  <!-- <IconLocalLogo /> -->
  <div :style="{ height: width + 'rpx' }">
    <!--  v-if="sysSettingStore.logo_background"  -->
    <!-- {{sysSettingStore  }} -->
    <n-image
      :width="width"
      :src="sysSettingStore.logo_background === '' ? defaultRdiLogo : sysSettingStore.logo_background"
    />
    <!-- <v-else icon-local-logo /> -->
  </div>
</template>

<style scoped></style>
