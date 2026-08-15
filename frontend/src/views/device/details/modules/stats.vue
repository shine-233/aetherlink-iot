<!--
  文件用途: 设备详情页里的“属性下发配置 + 下发日志”双面板。
  实际职责:
  1. 上半区展示当前可下发属性集合，并允许删除单条属性配置；
  2. 下半区展示属性下发日志，并通过公共组件发起新的属性下发；
  3. 两块区域都依赖 `DistributionAndTable`，但数据源、动作能力和业务口径不同。
  阅读提示:
  1. 虽然文件名是 `stats`，当前实现并不是纯统计卡片，而是属性配置与记录管理入口；
  2. `columns0` 面向当前属性数据集，`columns` 面向历史下发日志，维护时不要混用字段；
  3. 删除属性集会主动刷新上半区列表，但不会改变下半区日志的查询口径。
  静态审查建议:
  1. 文件名与真实职责存在偏差，后续适合补目录级命名说明，避免维护者按“统计摘要”方向误判；
  2. 两组表头都以内联 render 函数声明，后续可下沉为列配置 helper，减少列表页脚本噪音；
  3. 操作类型与状态格式化仍是硬编码 switch，若后端枚举继续扩展，适合统一沉淀成字典映射。
-->
<script setup lang="tsx">
import { ref } from 'vue'
import { NButton, NPopconfirm } from 'naive-ui'
import dayjs from 'dayjs'
import { $t } from '@/locales'
import DistributionAndTable from '@/views/device/details/modules/public/distribution-and-table.vue'
import {
  attributeDataPub,
  deleteAttributeDataSet,
  expectMessageAdd,
  getAttributeDataSet,
  getAttributeDataSetLogs
} from '@/service/api'
defineProps<{
  id: string
}>()
const attributeRef = ref()

// 上半区: 当前属性数据集。
// 这里的数据更接近“可操作配置快照”，支持删除，因此和下半区的发送日志不是同一种领域对象。
const columns0 = [
  {
    title: $t('device_template.table_header.attributeIdentifier'),
    minWidth: '140px',
    key: 'key'
  },
  {
    title: $t('device_template.table_header.attributeName'),
    minWidth: '140px',
    key: 'data_name'
  },
  {
    title: $t('device_template.table_header.attributeValue'),
    minWidth: '140px',
    key: 'value',
    render: row => `${row.value}${row.unit !== null ? row.unit : ''}`
  },
  {
    title: $t('device_template.table_header.updateTime'),
    minWidth: '140px',
    key: 'ts',
    render: row => dayjs(row.ts).format('YYYY-MM-DD HH:mm:ss')
  },
  {
    title: $t('common.actions'),
    key: 'created_at',
    minWidth: '140px',
    render: row => (
      <NPopconfirm
        onPositiveClick={async () => {
          await deleteAttributeDataSet(row.id)
          attributeRef.value.refresh()
        }}
      >
        {{
          trigger: () => (
            <NButton text size="small">
              {$t('common.delete')}
            </NButton>
          ),
          default: () => $t('common.confirmDelete')
        }}
      </NPopconfirm>
    )
  }
]

// 下发日志里的“操作类型”枚举解释。
// 当前直接依赖接口返回的状态码字符串，若后端枚举变更，这里会是第一处耦合点。
const formatOperationType = status => {
  switch (status) {
    case '1':
      return $t('custom.device_details.manualOperation')
    case '2':
      return $t('custom.device_details.automaticTriggering')
    default:
      return ''
  }
}

// 下发日志里的“发送/回执状态”枚举解释。
// 这里对未知值采用空字符串静默降级，排障时可能需要补更显式的兜底文案或观测信息。
const formatStatus = status => {
  switch (status) {
    case '1':
      return $t('generate.sendingSuccess')
    case '2':
      return $t('generate.sendingFail')
    case '3':
      return $t('generate.returnSuccess')
    case '4':
      return $t('generate.returnFail')
    default:
      return ''
  }
}

// 下半区: 属性下发记录。
// 这一组列同时服务“历史查看”和“下发表单提交后的记录回显”，字段命名要与日志接口严格一致。
const columns = [
  {
    title: $t('custom.device_details.attributeDistributionTime'),
    minWidth: '140px',
    key: 'created_at',
    render: row => dayjs(row.created_at).format('YYYY-MM-DD HH:mm:ss')
  },
  {
    title: $t('custom.device_details.messageId'),
    minWidth: '140px',
    key: 'message_id'
  },
  {
    title: $t('custom.device_details.sendContent'),
    minWidth: '140px',
    key: 'data'
  },
  {
    title: $t('custom.device_details.operationType'),
    minWidth: '140px',
    key: 'operation_type',
    render: row => formatOperationType(row.status)
  },
  {
    title: $t('generate.status'),
    minWidth: '140px',
    key: 'status',
    render: row => formatStatus(row.status)
  },
  {
    title: $t('generate.errorMessage'),
    minWidth: '140px',
    key: 'error_message'
  }
]
</script>

<template>
  <div>
    <DistributionAndTable
      :id="id as string"
      ref="attributeRef"
      :no-refresh="true"
      :table-columns="columns0"
      :fetch-data-api="getAttributeDataSet"
    />
  </div>

  <div>
    <!--
      第二个 DistributionAndTable 绑定的是“属性下发日志 + 下发表单”。
      它和上面的表共用壳组件，但 fetch/submit/expect API 完全不同，后续抽象时需要保留这层差异。
    -->
    <DistributionAndTable
      :id="id as string"
      :button-name="$t('generate.issue-attribute')"
      :table-columns="columns"
      :fetch-data-api="getAttributeDataSetLogs"
      :submit-api="attributeDataPub"
      :expect="true"
      :expect-api="expectMessageAdd"
    />
  </div>
</template>

<style scoped></style>
