<!--
文件用途: 承载联动场景相关的自动化页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script lang="tsx" setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import DataList from '@/views/automation/scene-linkage/modules/dataList.vue'
import { readAutomationRouteContext } from '@/views/automation/linkage-edit/modules/automationEditorState'

const route = useRoute()
const automationRouteContext = computed(() => readAutomationRouteContext(route.query))
</script>

<template>
  <div class="w-full">
    <DataList
      :device-id="automationRouteContext.propsData.device_id"
      :device-config-id="automationRouteContext.propsData.device_config_id"
      :back-type="automationRouteContext.backType || 'automation'"
      :onboarding="automationRouteContext.onboarding"
      :starter="automationRouteContext.starter.type"
      :first-device-name="automationRouteContext.starter.deviceName"
      :first-device-number="automationRouteContext.starter.deviceNumber"
      :telemetry-key="automationRouteContext.starter.telemetryKey"
      :telemetry-value="automationRouteContext.starter.telemetryValue"
      :telemetry-at="automationRouteContext.starter.telemetryAt"
    ></DataList>
  </div>
</template>

<style scoped lang="scss"></style>
