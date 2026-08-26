<!--
设备在线状态历史弹窗，负责按时间范围、在线状态和分页筛选查看设备上下线记录。
核心链路：弹窗打开或 deviceId 变化时拉取状态历史 -> 用户可按时间范围和在线/离线状态筛选 -> 用远程分页表格展示上下线时间线。
静态维护重点：
1. 历史记录来自后端状态历史接口，与详情页头部的实时 WebSocket 在线态可能存在短暂延迟，不应把当前弹窗内容当成绝对实时状态。
2. 当前序号列按分页内 index 计算，不是全局序号；后续若前端需要跨页连续编号，要结合 page/page_size 重新计算。
3. 错误分支当前静默吞掉异常，后续若要增强排障体验，建议补最小化错误提示和空态说明。
4. `visible` 与 `deviceId` 两个 watch 都会触发查询，弹窗开关和设备切换边界上要留意重复请求。
-->
<script setup lang="ts">
import { computed, h, nextTick, reactive, ref, watch } from 'vue'
import { useLoading } from '@aetherlink/hooks'
import { deviceStatusHistory } from '@/service/api/device'
import { $t } from '@/locales'
import dayjs from 'dayjs'
import type { DataTableColumns, PaginationProps } from 'naive-ui'

/**
 * 设备状态历史记录类型定义
 * @interface StatusHistoryItem
 * @property {number} status - 状态 0: 离线 1: 在线
 * @property {string | number} change_time - 状态改变时间
 */
interface StatusHistoryItem {
  status: 0 | 1
  change_time?: string | number
}

// 请求参数类型定义
interface StatusHistoryParams {
  device_id: string
  page: number
  page_size: number
  start_time?: number
  end_time?: number
  status?: number
}

// 响应数据类型定义
interface StatusHistoryListResponse {
  list?: StatusHistoryItem[]
  total?: number
}

// 响应数据类型定义
interface StatusHistoryResponse {
  data?: StatusHistoryListResponse
  error?: unknown
}

const props = defineProps<{
  deviceId: string
  visible: boolean
}>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

const { loading, startLoading, endLoading } = useLoading()
const tableData = ref<StatusHistoryItem[]>([])
const total = ref(0)
let statusHistoryRequestSeq = 0
let inFlightStatusHistoryKey = ''

// 查询参数直接绑定筛选表单与分页，弹窗关闭后会保留最近一次用户输入，直到用户手动重置。
const queryParams = reactive({
  device_id: '',
  page: 1,
  page_size: 20,
  start_time: undefined as number | undefined,
  end_time: undefined as number | undefined,
  status: null as number | null | undefined
})

const dateRangeValue = ref<[number, number] | null>(null)

// 当前只区分在线/离线两类筛选值，与后端 status 枚举保持一致。
const statusOptions = [
  { label: $t('custom.device_details.online'), value: 1 },
  { label: $t('custom.device_details.offline'), value: 0 }
]

// 历史列表走远程分页，翻页或切换每页条数都会重新请求服务端历史接口。
// 历史列表走远程分页，翻页或切换每页条数都会重新请求服务端历史接口。
// 变量注解 : PaginationProps = reactive({...}) 与 management/user 同型：
// 上下文类型先行校验字面量，规避 naive-ui 升级后泛型 reactive 的签名失配。
const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 20,
  showSizePicker: true,
  pageSizes: [10, 20, 30, 50, 100],
  itemCount: 0,
  onChange: (page: number) => {
    queryParams.page = page
    pagination.page = page
    fetchData()
  },
  onUpdatePageSize: (pageSize: number) => {
    queryParams.page_size = pageSize
    queryParams.page = 1
    pagination.pageSize = pageSize
    pagination.page = 1
    fetchData()
  }
})

// 历史列表使用简表展示，重点是状态与时间，不在这里展开更多设备快照字段。
const columns: DataTableColumns<StatusHistoryItem> = [
  {
    title: $t('common.index'),
    key: 'index',
    width: 100,
    render: (_row: StatusHistoryItem, index: number) => {
      return index + 1
    }
  },
  {
    title: $t('common.status'),
    key: 'status',
    width: 120,
    render: (row: StatusHistoryItem) => {
      const isOnline = row.status === 1
      const text = isOnline ? $t('custom.device_details.online') : $t('custom.device_details.offline')
      return h('span', text)
    }
  },
  {
    title: $t('common.time'),
    key: 'change_time',
    width: 200,
    render: (row: StatusHistoryItem) => {
      if (row.change_time) {
        return dayjs(row.change_time).format('YYYY-MM-DD HH:mm:ss')
      }
      return '--'
    }
  }
]

// 查询链路由 deviceId、分页、状态筛选和时间范围共同驱动；
// 当前只在成功时覆盖表格数据，失败时保留旧数据或空表，后续更适合补显式错误反馈。
const fetchData = async () => {
  const requestParams = buildStatusHistoryParams()
  if (!requestParams) {
    return
  }

  const requestKey = JSON.stringify(requestParams)
  if (loading.value && requestKey === inFlightStatusHistoryKey) {
    return
  }

  const requestSeq = ++statusHistoryRequestSeq
  inFlightStatusHistoryKey = requestKey
  startLoading()
  try {
    const response = (await deviceStatusHistory(requestParams)) as StatusHistoryResponse
    if (requestSeq !== statusHistoryRequestSeq) {
      return
    }

    const { data, error } = response

    if (!error && data) {
      tableData.value = data.list ?? []
      total.value = data.total ?? 0
      pagination.itemCount = total.value
    }
  } catch (error) {
    /* intentionally empty */
  } finally {
    if (requestSeq === statusHistoryRequestSeq) {
      inFlightStatusHistoryKey = ''
      endLoading()
    }
  }
}

const buildStatusHistoryParams = (): StatusHistoryParams | null => {
  if (!props.visible || !props.deviceId) {
    return null
  }

  const params: StatusHistoryParams = {
    device_id: props.deviceId,
    page: queryParams.page,
    page_size: queryParams.page_size
  }

  if (queryParams.start_time) {
    params.start_time = queryParams.start_time
  }
  if (queryParams.end_time) {
    params.end_time = queryParams.end_time
  }
  if (queryParams.status !== null && queryParams.status !== undefined) {
    params.status = queryParams.status
  }

  return params
}

// 搜索时统一回到第一页，保证筛选结果从头开始浏览。
const handleSearch = () => {
  queryParams.page = 1
  pagination.page = 1
  fetchData()
}

// 重置会清空时间和状态筛选，再异步重新拉取默认历史列表，确保表单值与请求参数保持一致。
const handleReset = () => {
  dateRangeValue.value = null
  queryParams.start_time = undefined
  queryParams.end_time = undefined
  queryParams.status = null
  queryParams.page = 1
  pagination.page = 1
  nextTick(() => {
    fetchData()
  })
}

// 日期选择器返回毫秒时间戳，这里统一换算成秒级参数传给后端。
// DatePicker 返回毫秒时间戳，而接口使用秒级时间戳，这里统一做一次换算。
const handleDateRangeChange = (value: [number, number] | null) => {
  dateRangeValue.value = value
  if (value && value.length === 2) {
    queryParams.start_time = Math.floor(value[0] / 1000)
    queryParams.end_time = Math.floor(value[1] / 1000)
  } else {
    queryParams.start_time = undefined
    queryParams.end_time = undefined
  }
}

const showModal = computed({
  get: () => props.visible,
  set: value => emit('update:visible', value)
})

// 弹窗首次打开时自动拉取当前设备历史，保证用户看到的总是最新分页第一页。
// 弹窗从关闭变为打开时，以当前 deviceId 为基准重新拉历史记录，避免看到上次会话残留数据。
watch(
  () => props.visible,
  newVal => {
    if (newVal && props.deviceId) {
      queryParams.device_id = props.deviceId
      queryParams.page = 1
      pagination.page = 1
      fetchData()
    }
  },
  { immediate: true }
)

// 设备切换但弹窗未关闭时，也需要重查新的设备状态历史。
// 如果弹窗保持打开但父层切换了设备，也要立即切换查询上下文，避免旧设备历史留在当前视图。
watch(
  () => props.deviceId,
  newVal => {
    if (newVal && props.visible) {
      queryParams.device_id = newVal
      queryParams.page = 1
      pagination.page = 1
      fetchData()
    }
  }
)
</script>

<template>
  <NModal
    v-model:show="showModal"
    preset="dialog"
    :showIcon="false"
    :title="$t('common.deviceActiveTime')"
    :style="{ minWidth: '600px', maxHeight: '90vh' }"
  >
    <NCard>
      <NForm :model="queryParams" :show-feedback="false" label-placement="left" label-width="100px" label-align="left">
        <NFlex :vertical="false" :gap="8" class="mb-4">
          <NFormItem :label="$t('common.timeFrame')">
            <NDatePicker
              v-model:value="dateRangeValue"
              type="datetimerange"
              clearable
              :placeholder="$t('common.selectPlaceholder')"
              @update:value="handleDateRangeChange"
            />
          </NFormItem>
          <NFormItem :label="$t('common.status')">
            <NSelect
              v-model:value="queryParams.status"
              :options="statusOptions"
              clearable
              :placeholder="$t('generate.selectStatus')"
              style="width: 200px"
            />
          </NFormItem>
          <NFlex :vertical="false" :gap="8">
            <NButton type="primary" @click="handleSearch">{{ $t('common.search') }}</NButton>
            <NButton @click="handleReset">{{ $t('common.reset') }}</NButton>
          </NFlex>
        </NFlex>
      </NForm>

      <div class="table-container">
        <NDataTable
          :columns="columns"
          :data="tableData"
          :loading="loading"
          :pagination="pagination"
          :bordered="false"
          :max-height="350"
          remote
        >
          <template #empty>
            <n-empty :description="$t('common.noData')" />
          </template>
        </NDataTable>
      </div>
    </NCard>
  </NModal>
</template>

<style scoped lang="scss">
.table-container {
  margin-top: 16px;
  width: 100%;
  overflow: hidden;
}
</style>
