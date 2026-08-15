<!--
设备事件上报面板，负责展示当前设备的事件历史记录。
核心链路：上层传入设备 ID，这里只维护事件历史表格列定义与查询接口，再交给通用的 DistributionAndTable 组件统一处理筛选、分页、拉取和渲染。
使用注意：
1. 本文件本身很薄，但列定义直接决定事件历史的可见字段；后端事件数据结构变化时，这里必须同步更新。
2. `data`、`error_message` 等字段直接来自事件历史接口；如果后续要把事件 payload 做结构化展示，建议先在这里收口格式化逻辑。
3. 当前空态、错误态、查询节流和时间范围交互都委托给公共表格组件；问题排查时不要只盯当前文件。
静态审查建议：
1. 事件内容目前按纯文本直出，较长 JSON 或嵌套对象的可读性一般，后续适合补 payload 展开器或格式化预览。
2. `row.ts` 默认直接格式化，若接口未来返回秒级时间戳、空值或异常字符串，这里需要补统一兜底。
-->
<script setup lang="ts">
import dayjs from 'dayjs'
import DistributionAndTable from '@/views/device/details/modules/public/distribution-and-table.vue'
import { getEventDataSet } from '@/service/api'
import { $t } from '@/locales'

defineProps<{
  id: string
}>()

// 事件历史表格聚焦“是什么事件、何时发生、上报了什么、是否报错”这几类核心信息。
// 这里仅定义字段映射，不直接处理查询条件、分页状态和请求生命周期。
const columns = [
  { title: $t('device_template.table_header.eventIdentifier'), minWidth: '140px', key: 'identify' },
  { title: $t('device_template.table_header.eventName'), minWidth: '140px', key: 'data_name' },
  {
    title: $t('device_template.table_header.eventReportingTime'),
    minWidth: '140px',
    key: 'ts',
    // 事件时间统一格式化，方便和命令、属性、遥测记录做人工对时排查。
    render: row => dayjs(row.ts).format('YYYY-MM-DD HH:mm:ss')
  },
  { title: $t('device_template.table_header.eventContent'), minWidth: '140px', key: 'data' },
  { title: $t('generate.errorMessage'), minWidth: '140px', key: 'error_message' }
]
</script>

<template>
  <div>
    <DistributionAndTable :id="id as string" :table-columns="columns" :fetch-data-api="getEventDataSet" />
  </div>
</template>

<style scoped></style>
