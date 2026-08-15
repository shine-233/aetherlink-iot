<!--
  文件用途：承载 frontend/src/views/_builtin/login/modules/login-bg.vue 对应的页面或局部组件视图。
  核心逻辑：组合模板、响应式状态、路由或局部组件，向用户呈现当前页面所需的主要内容和交互入口。
  关键注意事项：修改可见文案、路由依赖或交互分支时，要同步维护相邻测试和 README 职责说明。
  重构建议：当模板或脚本继续变长时，优先抽出局部组件或组合式函数，再用 focused tests 锁定行为一致性。
-->
<script lang="ts" setup>
import { computed } from 'vue'
type SysSetting = Omit<Api.GeneralSetting.ThemeSetting, 'id'>
interface Props {
  /** 主题颜色 */
  themeColor: string
  sysSetting: SysSetting
}
const props = defineProps<Props>()

const bgColor = computed(() => {
  return props.sysSetting?.home_background || ''
})
</script>

<template>
  <div class="absolute-lt z-1 wh-full overflow-hidden">
    <NImage
      v-if="bgColor != ''"
      object-fit="cover"
      style="min-width: 100%; min-height: 100%"
      preview-disabled
      :src="bgColor"
      :img-props="{
        style: {
          minWidth: '100%'
        }
      }"
    />
    <SvgIcon v-else local-icon="Wave" class="size-full" />
  </div>
</template>

<style scoped></style>
