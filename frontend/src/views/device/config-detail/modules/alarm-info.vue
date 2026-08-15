<!--
  文件用途: 设备配置告警信息面板。
  核心逻辑: 作为设备配置维度的告警入口，承接“新增规则”和现有规则列表展示。
  查询链路: 当前文件自身不直连接口，告警列表查询由 alarmDataList 基于 configId 负责。
  保存链路: 点击新增按钮后带着 device_config_id 跳转到告警编辑页，保存动作在目标页面完成。
  关键注意事项: 当前配置 ID 是列表过滤和新增告警回填的唯一上下文，丢失后会影响返回链路。
-->
<script setup lang="ts">
import { NButton, NFlex } from 'naive-ui'
import { useRouterPush } from '@/hooks/common/router'
import alarmDataList from '@/views/automation/scene-linkage/modules/dataList.vue'
import { $t } from '@/locales'

const { routerPushByKey } = useRouterPush()

const props = defineProps<{
  configId: string
}>()
// 跳转链路: 将当前配置 ID 作为来源上下文带到告警规则编辑页，供后续保存和返回使用。
const alarmAdd = () => {
  routerPushByKey('automation_linkage-edit', {
    query: { device_config_id: props.configId, backType: 'config' }
  })
}
</script>

<template>
  <div class="alarm-list">
    <NFlex justify="flex-end" class="mb-4">
      <NButton type="primary" @click="alarmAdd()">{{ $t('generate.addAlarmRule') }}</NButton>
    </NFlex>
    <!-- 查询链路由子组件负责，这里只透传配置上下文与告警模式。 -->
    <alarmDataList :is-alarm="true" :device-config-id="props.configId" back-type="config"></alarmDataList>
  </div>
</template>

<style scoped lang="scss">
.alarm-box {
  display: flex;
  flex-flow: row;
  justify-content: flex-start;
  align-items: center;
  flex-wrap: wrap;
  padding: 10px 40px;

  .alarm-item {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
    //margin: 0 10px;
    padding: 18px;
    flex: 0 0 23%;
    margin-right: calc(30% / 3);
    margin-bottom: 30px;

    .item-name {
      display: flex;
      flex-flow: row;
      align-items: center;
      justify-content: space-between;
    }

    .item-desc {
      margin: 15px 0;
    }

    .item-operate {
      display: flex;
      flex-flow: row;
      justify-content: space-between;
      align-items: center;
    }
  }
}
</style>
