<!--
文件用途: 承载系统日志相关的系统管理用户侧页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="tsx">
import { computed, getCurrentInstance, ref } from 'vue'
import type { Ref } from 'vue'
import { NButton, NEmpty, NSelect } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { useRoute } from 'vue-router'
import { getSystemLogList } from '@/service/api/system-management-user'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'
import DetailModal from './components/detail-modal.vue'
import {
  AUDIT_ACTIONS,
  REQUEST_METHODS,
  defaultSystemLogQuery,
  normalizeLogRange,
  serializeSystemLogQuery,
  systemLogQueryFromRoute
} from './query'
import type { LogTimeRange, SystemLogQuery } from './query'

const route = useRoute()

const allOption = { label: $t('custom.management.all'), value: '' }
const requestMethodOptions = [allOption, ...REQUEST_METHODS.map((value) => ({ label: value, value }))]
// TB-10 实体级动作筛选（127.sql）：create/update/delete/read/other 映射自 HTTP 方法
const auditActionOptions = [allOption, ...AUDIT_ACTIONS.map((value) => ({ label: value, value }))]

// 筛选/分页/加载态/过期请求丢弃统一交给 useListPage：慢的旧页响应不会覆盖新页，
// 搜索回到第 1 页，每页条数切换后自动回拉。时间范围是唯一真源，start_time/end_time 在序列化时派生。
// 首屏筛选来自深链（就绪检查跳转），重置则回到纯默认值（最近一个月、无其它筛选）。
const {
  query: queryParams,
  rows: tableData,
  total,
  loading,
  pagination,
  load: getTableData,
  search: handleQuery,
  reset: handleReset
} = useListPage<Api.SystemManage.SystemLogList, SystemLogQuery>({
  initialQuery: () => systemLogQueryFromRoute(route.query),
  resetValues: defaultSystemLogQuery,
  pageSizes: [10, 15, 20, 25, 30],
  serialize: serializeSystemLogQuery,
  fetcher: async (params) =>
    fromFlatResponse<Api.SystemManage.SystemLogList>(
      await getSystemLogList(params as unknown as Api.SystemManage.SystemLogSearchParams)
    )
})

const normalizeRouteQueryValue = (value: unknown) => {
  if (Array.isArray(value)) return String(value[0] || '')
  return value ? String(value) : ''
}
const routeSource = computed(() => normalizeRouteQueryValue(route.query.source))
const isReadyCheckAuditSearch = computed(() => routeSource.value === 'ready-check' && Boolean(queryParams.path))

/** 选择器变更：只选日期时把结束时间推到当天末尾，输入框与查询参数保持一致。 */
function pickerChange(value: LogTimeRange) {
  queryParams.range = normalizeLogRange(value)
}

const detailModalRef = ref<any>(null)
const handleDetail = (item) => {
  detailModalRef.value && detailModalRef.value.show && detailModalRef.value.show(item)
}
const columns: Ref<DataTableColumns<DataService.Data>> = ref([
  {
    key: 'created_at',
    title: $t('common.time'),
    minWidth: '140px',
    align: 'left',
    render: (row: any) => {
      return formatDateTime(row.created_at)
    }
  },
  {
    key: 'ip',
    minWidth: '140px',
    title: 'IP',
    align: 'left'
  },
  {
    key: 'path',
    title: $t('common.requestPath'),
    minWidth: '140px',
    align: 'left'
  },
  {
    key: 'name',
    minWidth: '140px',
    title: $t('common.requestMethod'),
    align: 'left'
  },
  {
    // TB-10（127.sql）：实体级动作（create/update/delete/read/other），旧数据为空
    key: 'action',
    minWidth: '100px',
    title: $t('page.systemLog.action'),
    align: 'left',
    render: (row: any) => row.action || '--'
  },
  {
    // TB-10（127.sql）：审计实体类型，自请求路径 /api/v1/<entity> 解析
    key: 'entity_type',
    minWidth: '120px',
    title: $t('page.systemLog.entityType'),
    align: 'left',
    render: (row: any) => row.entity_type || '--'
  },
  {
    // TB-10（127.sql）：审计实体ID，路径第二段 UUID 形态
    key: 'entity_id',
    minWidth: '200px',
    title: $t('page.systemLog.entityId'),
    align: 'left',
    render: (row: any) => row.entity_id || '--'
  },
  {
    // TB-10（127.sql）：HTTP 响应状态码
    key: 'status_code',
    minWidth: '100px',
    title: $t('page.systemLog.statusCode'),
    align: 'left',
    render: (row: any) => (row.status_code === null || row.status_code === undefined ? '--' : String(row.status_code))
  },
  {
    key: 'latency',
    title: $t('common.requestTime'),
    minWidth: '140px',
    align: 'left',
    render: (row) => `${row.latency}ms`
  },
  {
    key: 'username',
    title: $t('generate.username'),
    minWidth: '140px',
    align: 'left'
  },
  {
    key: '',
    title: $t('common.actions'),
    minWidth: '140px',
    align: 'left',
    render: (row) => {
      return (
        <NButton type="primary" size={'small'} onClick={() => handleDetail(row)}>
          {$t('generate.details')}
        </NButton>
      )
    }
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
    <NCard :title="$t('generate.system-log')">
      <NAlert v-if="isReadyCheckAuditSearch" type="info" :show-icon="false" class="mb-12px">
        {{ $t('custom.device_details.readyCheckAuditSearchHint').replace('{path}', queryParams.path || '--') }}
      </NAlert>
      <NForm class="mb-20px align-end" :inline="!getPlatform" label-placement="left" :model="queryParams">
        <view class="flex flex-wrap">
          <NFormItem class="w-200px" :label="$t('generate.username')" path="name">
            <NInput v-model:value="queryParams.username" />
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
          <NFormItem :label="$t('generate.requestMethod')" path="method">
            <NSelect v-model:value="queryParams.method" class="w-200px" :options="requestMethodOptions"></NSelect>
          </NFormItem>
          <NFormItem class="w-260px" :label="$t('common.requestPath')" path="path">
            <NInput v-model:value="queryParams.path" clearable />
          </NFormItem>
          <NFormItem :label="$t('page.systemLog.action')" path="action">
            <NSelect v-model:value="queryParams.action" class="w-160px" :options="auditActionOptions"></NSelect>
          </NFormItem>
          <NFormItem class="w-180px" :label="$t('page.systemLog.entityType')" path="entity_type">
            <NInput v-model:value="queryParams.entity_type" clearable />
          </NFormItem>
          <NFormItem class="w-280px" :label="$t('page.systemLog.entityId')" path="entity_id">
            <NInput v-model:value="queryParams.entity_id" clearable />
          </NFormItem>
          <NFormItem :label="$t('generate.ipAddress')" path="ip">
            <NInput v-model:value="queryParams.ip" />
          </NFormItem>

          <NButton class="w-72px" type="primary" @click="handleQuery">{{ $t('generate.search') }}</NButton>
          <NButton class="ml-15px w-72px" type="primary" @click="handleReset">{{ $t('generate.reset') }}</NButton>
        </view>
      </NForm>
      <NDataTable :columns="columns" :data="tableData" :loading="loading" :remote="true" class="flex-1-hidden">
        <template #empty>
          <NEmpty :description="$t('common.noData')" class="py-24px" />
        </template>
      </NDataTable>
      <div class="pagination-box">
        <NPagination
          :page="pagination.page"
          :page-size="pagination.pageSize"
          :item-count="total"
          :show-size-picker="true"
          :page-sizes="pagination.pageSizes"
          @update:page="pagination.onUpdatePage"
          @update:page-size="pagination.onUpdatePageSize"
        />
      </div>
    </NCard>
    <DetailModal ref="detailModalRef"></DetailModal>
  </div>
</template>

<style scoped>
.pagination-box {
  margin-top: 12px;
  display: flex;
  justify-content: flex-end;
}

.align-end {
  align-items: flex-end;
}
</style>
