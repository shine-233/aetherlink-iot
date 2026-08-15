<!--
  文件用途: 设备配置详情页的自动化关联展示面板。
  核心逻辑: 复用自动化列表组件，以设备配置 ID 为过滤条件展示当前配置关联的场景联动规则。
  主要链路: 父页面传入 configId -> 本组件透传给 sceneLinkage -> sceneLinkage 负责查询、展示和回退到配置详情上下文。
  关键注意事项:
  1. 这里承载的是配置级自动化，不是单设备临时规则；一条规则可能影响同配置下的多台设备。
  2. 当前组件本身不做空态、加载态和错误态判断，完全依赖下游 sceneLinkage 的实现质量。
  3. `back-type="config"` 是回退上下文约定，若自动化模块调整路由契约，这里需要同步更新。
  静态审查建议:
  1. 当前只是薄封装，后续可把 config 场景的查询参数、空态文案和权限提示集中在这里收口。
  2. 注释中的“配置级影响面”应同步反映到交互提示，避免用户误以为只影响当前详情页。
  3. 建议补无规则、规则已失效、配置已解绑等静态验证场景，减少委托组件升级带来的回归。
-->
<script setup lang="ts">
import sceneLinkage from '@/views/automation/scene-linkage/modules/dataList.vue'

defineProps<{
  // 设备配置 ID 是自动化列表的过滤主键，sceneLinkage 会据此只展示当前配置相关规则。
  configId: string
}>()
</script>

<template>
  <div class="h-500px flex-col">
    <!--      <n-empty :description="$t('common.noData')"></n-empty>-->
    <!-- 自动化查询和展示逻辑全部委托给通用组件，这里只负责提供配置上下文。 -->
    <sceneLinkage :device-config-id="configId" back-type="config" />
  </div>
</template>

<style scoped></style>
