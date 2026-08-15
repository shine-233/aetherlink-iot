<!--
文件用途：承载 告警消息管理 的页面级视图。
核心逻辑：组合表格、表单、弹窗、接口请求和国际化文案，完成页面初始化、查询与交互反馈。
关键注意事项：页面通常依赖权限、分页、远端接口和路由状态，改动时需同步检查测试与接口契约。
重构建议：后续可继续拆分数据编排、列配置和弹窗流程，降低页面级组件复杂度。
-->
<script setup lang="tsx">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { $t } from '@/locales'
import { parseFleetRolloutContext } from '../../device/modules/fleet-rollout-context'
import AlarmConfiguration from './components/alarm-configuration.vue'
import NewInformation from './components/new-information.vue'

const route = useRoute()
const routeDeviceId = computed(() => {
  const rawDeviceId = route.query.device_id
  return Array.isArray(rawDeviceId) ? rawDeviceId[0] || '' : rawDeviceId || ''
})
const fleetAlarmContext = computed(() => parseFleetRolloutContext(route.query as Record<string, any>))
</script>

<template>
  <div class="table-box">
    <NCard :title="$t('generate.alarm-center')">
      <n-tabs type="line" size="large">
        <n-tab-pane :name="$t('generate.alarmInfo')" :tag="$t('generate.alarmInfo')">
          <AlarmConfiguration :initial-device-id="routeDeviceId" :fleet-context="fleetAlarmContext" />
        </n-tab-pane>
        <n-tab-pane :name="$t('generate.alarmConfig')" :tag="$t('generate.alarmConfig')">
          <NewInformation />
        </n-tab-pane>
      </n-tabs>
    </NCard>
  </div>
</template>

<style scoped></style>
