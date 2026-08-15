<!--
文件用途: 承载系统日志相关的系统管理用户侧页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="tsx">
import { computed, getCurrentInstance, reactive, ref } from 'vue'
import type { Ref } from 'vue'
import { NButton, NSelect } from 'naive-ui'
import type { DataTableColumns, PaginationProps } from 'naive-ui'
import dayjs from 'dayjs'
import { useRoute } from 'vue-router'
import { getSystemLogList } from '@/service/api/system-management-user'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'
import DetailModal from './components/detail-modal.vue'
import { useLoading } from '~/packages/hooks'

const { loading, startLoading, endLoading } = useLoading(false)
const route = useRoute()

const range = ref<[number, number]>([dayjs().subtract(1, 'month').valueOf(), dayjs().valueOf()])
// POST PUT DELETE
const requestMethodOptions = reactive([
  {
    label: $t('custom.management.all'),
    value: ''
  },
  {
    label: 'POST',
    value: 'POST'
  },
  {
    label: 'PUT',
    value: 'PUT'
  },
  {
    label: 'DELETE',
    value: 'DELETE'
  }
])
const queryParams = reactive({
  username: '',
  selected_time: null,
  start_time: '',
  end_time: '',
  method: '',
  path: '',
  ip: ''
})
const total = ref(0)

const normalizeRouteQueryValue = (value: unknown) => {
  if (Array.isArray(value)) return String(value[0] || '')
  return value ? String(value) : ''
}
const routeSource = computed(() => normalizeRouteQueryValue(route.query.source))
const isReadyCheckAuditSearch = computed(() => routeSource.value === 'ready-check' && Boolean(queryParams.path))

const applyRouteQueryDefaults = () => {
  const method = normalizeRouteQueryValue(route.query.method).toUpperCase()
  if (requestMethodOptions.some((option) => option.value === method)) queryParams.method = method
  queryParams.path = normalizeRouteQueryValue(route.query.path)

  const startTime = normalizeRouteQueryValue(route.query.start_time)
  const endTime = normalizeRouteQueryValue(route.query.end_time)
  const start = startTime ? dayjs(startTime) : null
  const end = endTime ? dayjs(endTime) : null
  if (start?.isValid() && end?.isValid()) {
    queryParams.start_time = start.format('YYYY-MM-DDTHH:mm:ssZ')
    queryParams.end_time = end.format('YYYY-MM-DDTHH:mm:ssZ')
    range.value = [start.valueOf(), end.valueOf()]
  }
}

const tableData = ref<Api.SystemManage.SystemLogList[]>([])

function setTableData(data: Api.SystemManage.SystemLogList[] | []) {
  tableData.value = data || []
}

const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [10, 15, 20, 25, 30],
  onChange: (page: number) => {
    pagination.page = page
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
  }
})

const getTableData = async () => {
  startLoading()
  const prams = {
    page: pagination.page || 1,
    page_size: pagination.pageSize || 10,
    ...queryParams
  }
  const res = await getSystemLogList(prams)
  if (res?.data) {
    setTableData(res?.data.list || [])
    total.value = res.data.total || 0
  }
  endLoading()
}
const detailModalRef = ref<any>(null)
const handleDetail = item => {
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
    key: 'latency',
    title: $t('common.requestTime'),
    minWidth: '140px',
    align: 'left',
    render: row => `${row.latency}ms`
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
    render: row => {
      return (
        <NButton type="primary" size={'small'} onClick={() => handleDetail(row)}>
          {$t('generate.details')}
        </NButton>
      )
    }
  }
]) as Ref<DataTableColumns<DataService.Data>>

function handleQuery() {
  getTableData()
}
function handleReset() {
  queryParams.start_time = ''
  queryParams.end_time = ''
  queryParams.ip = ''
  queryParams.method = ''
  queryParams.path = ''
  queryParams.username = ''
  queryParams.selected_time = null
  range.value = [dayjs().subtract(1, 'month').valueOf(), dayjs().valueOf()]
  pagination.page = 1
  handleQuery()
}
function pickerChange(value: [number, number] | null) {
  if (value && value.length === 2) {
    const startDate = dayjs(value[0])
    const endDateMoment = dayjs(value[1])
    if (process.env.NODE_ENV === 'development') {
      /* intentionally empty */
    }

    // 检查用户是否可能只选了日期（时间部分为 00:00:00）
    // 如果是，则将结束时间调整到 23:59:59.999
    // 如果用户明确选择了时间，则尊重用户的选择
    let adjustedEndDateMoment
    if (
      endDateMoment.hour() === 0 &&
      endDateMoment.minute() === 0 &&
      endDateMoment.second() === 0 &&
      endDateMoment.millisecond() === 0
    ) {
      adjustedEndDateMoment = endDateMoment.endOf('day')
      if (process.env.NODE_ENV === 'development') {
        /* intentionally empty */
      }
    } else {
      adjustedEndDateMoment = endDateMoment // 用户选择了具体时间，保持不变
      if (process.env.NODE_ENV === 'development') {
        /* intentionally empty */
      }
    }

    queryParams.start_time = startDate.format('YYYY-MM-DDTHH:mm:ssZ')
    queryParams.end_time = adjustedEndDateMoment.format('YYYY-MM-DDTHH:mm:ssZ')
    if (process.env.NODE_ENV === 'development') {
      /* intentionally empty */
    }

    // 同步更新 range 的结束时间，让输入框显示与实际查询参数保持一致。
    range.value[1] = adjustedEndDateMoment.valueOf()
  } else {
    queryParams.start_time = ''
    queryParams.end_time = ''
    if (process.env.NODE_ENV === 'development') {
      /* intentionally empty */
    }
  }
}
const getPlatform = computed(() => {
  const { proxy }: any = getCurrentInstance()
  return proxy.getPlatform()
})
applyRouteQueryDefaults()
getTableData()
</script>

<template>
  <div>
    <NCard :title="$t('generate.system-log')">
      <NAlert v-if="isReadyCheckAuditSearch" type="info" :show-icon="false" class="mb-12px">
        {{
          $t('custom.device_details.readyCheckAuditSearchHint')
            .replace('{path}', queryParams.path || '--')
        }}
      </NAlert>
      <NForm class="mb-20px align-end" :inline="!getPlatform" label-placement="left" :model="queryParams">
        <view class="flex flex-wrap">
          <NFormItem class="w-200px" :label="$t('generate.username')" path="name">
            <NInput v-model:value="queryParams.username" />
          </NFormItem>
          <NFormItem path="selected_time">
            <NDatePicker
              v-model:value="range"
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
          <NFormItem :label="$t('generate.ipAddress')" path="ip">
            <NInput v-model:value="queryParams.ip" />
          </NFormItem>

          <NButton class="w-72px" type="primary" @click="handleQuery">{{ $t('generate.search') }}</NButton>
          <NButton class="ml-15px w-72px" type="primary" @click="handleReset">{{ $t('generate.reset') }}</NButton>
        </view>
      </NForm>
      <NDataTable :columns="columns" :data="tableData" :loading="loading" class="flex-1-hidden" />
      <div class="pagination-box">
        <NPagination v-model:page="pagination.page" :item-count="total" @update:page="getTableData" />
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
