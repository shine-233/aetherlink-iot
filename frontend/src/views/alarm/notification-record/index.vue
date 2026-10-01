<!--
文件用途：承载 告警通知记录 的页面级视图。
核心逻辑：组合表格、表单、弹窗、接口请求和国际化文案，完成页面初始化、查询与交互反馈。
关键注意事项：页面通常依赖权限、分页、远端接口和路由状态，改动时需同步检查测试与接口契约。
重构建议：后续可继续拆分数据编排、列配置和弹窗流程，降低页面级组件复杂度。
-->
<script setup lang="tsx">
import { computed, getCurrentInstance, ref } from 'vue'
import type { Ref } from 'vue'
import { NButton, NEmpty } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { getNotificationHistoryList } from '@/service/api/notification'
import { notificationOptions } from '@/constants/business'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'
import { defaultNotificationRecordQuery, serializeNotificationRecordQuery } from './query'
import type { NotificationRecordQuery, SendTimeRange } from './query'

// 分页/加载态/过期请求丢弃由 useListPage 统一处理；时间范围是唯一真源，
// 序列化时映射为后端契约字段 send_time_start / send_time_stop。重置会重新取"最近一个月"。
const {
  query: queryParams,
  rows: tableData,
  loading,
  pagination,
  load: getTableData,
  search: handleQuery,
  reset: handleReset
} = useListPage<Api.Alarm.NotificationHistoryList, NotificationRecordQuery>({
  initialQuery: () => defaultNotificationRecordQuery(),
  pageSizes: [10, 15, 20, 25, 30],
  serialize: serializeNotificationRecordQuery,
  fetcher: async (params) =>
    fromFlatResponse<Api.Alarm.NotificationHistoryList>(
      await getNotificationHistoryList(params as unknown as Api.Alarm.NotificationHistoryParams)
    )
})

function pickerChange(value: SendTimeRange) {
  queryParams.range = value && value.length === 2 ? value : null
}

const columns: Ref<DataTableColumns<DataService.Data>> = ref([
  {
    key: 'send_time',
    title: $t('custom.device_details.sendTime'),
    align: 'left',
    minWidth: '180px',
    render: (row: any) => {
      return formatDateTime(row.send_time)
    }
  },
  {
    key: 'send_content',
    minWidth: '180px',
    title: $t('custom.device_details.titleOrContent'),
    align: 'left'
  },
  {
    key: 'send_target',
    minWidth: '100px',
    title: $t('generate.recipient'),
    align: 'left',
    width: '200'
  },
  {
    key: 'send_result',
    title: $t('custom.device_details.sendResults'),
    minWidth: '140px',
    align: 'left'
  },
  {
    key: 'notification_type',
    title: $t('generate.notification-type'),
    minWidth: '140px',
    align: 'left'
  }
]) as Ref<DataTableColumns<DataService.Data>>

const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
getTableData()
</script>

<template>
  <div>
    <NCard :title="$t('generate.notification-record')">
      <div class="h-full flex-col">
        <NForm label-placement="left" :inline="!getPlatform" :model="queryParams">
          <NFormItem path="name" :label="$t('generate.notification-type')">
            <n-select
              v-model:value="queryParams.notification_type"
              :options="notificationOptions"
              :placeholder="$t('generate.notification-type')"
              class="input-style min-w-160px"
              clearable
            />
          </NFormItem>
          <NFormItem path="selected_time">
            <NDatePicker
              :value="queryParams.range"
              type="datetimerange"
              clearable
              separator="-"
              @update:value="pickerChange"
            />
          </NFormItem>
          <NFormItem path="send_target">
            <NInput v-model:value="queryParams.send_target" clearable :placeholder="$t('generate.recipient')" />
          </NFormItem>
          <NFormItem>
            <NButton type="primary" @click="handleQuery">{{ $t('common.search') }}</NButton>
            <NButton class="ml-12px" @click="handleReset">{{ $t('common.reset') }}</NButton>
          </NFormItem>
        </NForm>
        <NDataTable
          :columns="columns"
          :data="tableData"
          :loading="loading"
          :pagination="pagination"
          :remote="true"
          class="flex-1-hidden mt-4"
        >
          <template #empty>
            <NEmpty :description="$t('common.noData')" class="py-24px" />
          </template>
        </NDataTable>
      </div>
    </NCard>
  </div>
</template>

<style scoped>
.pagination-box {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}
</style>
