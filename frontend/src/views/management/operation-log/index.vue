<!--
文件用途: 操作审计日志面板，对标 ThingsBoard Audit Logs 的操作轨迹查询能力。
核心链路: 筛选区（IP/操作人/方法/路径/时间范围）组装分页参数调用 /operation_logs，
  表格远程分页展示，行展开通过 MessageBlock 查看 request/response 载荷。
静态维护重点:
1. 本页默认以“系统设置 -> 操作审计日志”tab 形式暴露（菜单由后端 sys_ui_elements 驱动，
   新增独立路由必须补 DB 种子；在迁移放开前保持 tab 接线）。
2. 后端契约里 name 列当前存的是 HTTP 方法名，“接口名称”与“方法”两列同源展示，
   后端若改为存真实接口名无需改本页结构。
3. 表格为 remote 分页；耗时列排序是当前页内的本地排序（服务端暂不支持排序参数），
   若后端补齐排序参数需把 handleSorter 改为重新拉取。
4. 列渲染使用 h() 而非 JSX：vitest 环境未注册 vue-jsx 插件，JSX 会被误编译成 React.createElement。
-->
<script setup lang="ts">
import { computed, h, reactive, ref } from 'vue'
import { NButton, NTag } from 'naive-ui'
import type { DataTableColumns, PaginationProps } from 'naive-ui'
import dayjs from 'dayjs'
import { fetchOperationLogs } from '@/service/api/operation-log'
import type { OperationLogRow } from '@/service/api/operation-log'
import { $t } from '@/locales'
import { useLoading } from '~/packages/hooks'
import MessageBlock from './modules/message-block.vue'

const { loading, startLoading, endLoading } = useLoading(false)

const tableData = ref<OperationLogRow[]>([])
/** 当前页原始顺序数据，用于恢复“耗时”列取消排序后的展示 */
const rawRows = ref<OperationLogRow[]>([])
type LatencyOrder = 'ascend' | 'descend' | false
const latencyOrder = ref<LatencyOrder>(false)

const methodOptions = computed(() => [
  { label: $t('custom.management.operationLog.filter.methodAll'), value: '' },
  { label: 'GET', value: 'GET' },
  { label: 'POST', value: 'POST' },
  { label: 'PUT', value: 'PUT' },
  { label: 'DELETE', value: 'DELETE' }
])

const queryParams = reactive({
  ip: '',
  username: '',
  method: '',
  path: '',
  start_time: '',
  end_time: ''
})

/** 时间范围选择器的受控值（毫秒时间戳区间） */
const range = ref<[number, number] | null>(null)

const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 10,
  itemCount: 0,
  showSizePicker: true,
  pageSizes: [10, 20, 50],
  onChange: page => {
    pagination.page = page
    getTableData()
  },
  onUpdatePageSize: pageSize => {
    pagination.pageSize = pageSize
    pagination.page = 1
    getTableData()
  }
})

interface PagedPayload {
  list?: OperationLogRow[]
  total?: number
}

function normalizePagedResult(result: { data?: PagedPayload | null } | null): { list: OperationLogRow[]; total: number } {
  const payload = result?.data ?? null
  return {
    list: Array.isArray(payload?.list) ? (payload?.list as OperationLogRow[]) : [],
    total: Number(payload?.total ?? 0)
  }
}

/** 方法名映射 tag 颜色；name 列非方法值时回退 default */
function methodTagType(method?: string | null): NaiveUI.ThemeColor {
  switch ((method || '').toUpperCase()) {
    case 'GET':
      return 'info'
    case 'POST':
      return 'primary'
    case 'PUT':
      return 'warning'
    case 'DELETE':
      return 'error'
    case 'PATCH':
      return 'success'
    default:
      return 'default'
  }
}

function formatTime(value: string | null) {
  if (!value) return '-'
  const time = dayjs(value)
  return time.isValid() ? time.format('YYYY-MM-DD HH:mm:ss') : String(value)
}

async function getTableData() {
  startLoading()
  try {
    const result = await fetchOperationLogs({
      page: pagination.page,
      page_size: pagination.pageSize,
      ip: queryParams.ip,
      username: queryParams.username,
      method: queryParams.method,
      path: queryParams.path,
      start_time: queryParams.start_time,
      end_time: queryParams.end_time
    })
    const data = normalizePagedResult(result)
    rawRows.value = data.list.slice()
    tableData.value = data.list
    pagination.itemCount = data.total
    // 新数据到来后重置本地排序状态，避免误以为仍按旧顺序排序
    latencyOrder.value = false
  } catch {
    tableData.value = []
    rawRows.value = []
    pagination.itemCount = 0
  } finally {
    endLoading()
  }
}

function applyLatencySort() {
  if (!latencyOrder.value) {
    tableData.value = rawRows.value.slice()
    return
  }
  const rows = rawRows.value.slice()
  rows.sort((a, b) => (latencyOrder.value === 'ascend' ? a.latency - b.latency : b.latency - a.latency))
  tableData.value = rows
}

/**
 * 远程表格的排序入口：耗时列在当前页内本地排序，
 * 其余列不参与排序。兼容 columnKey/key 两种载荷字段命名。
 */
function handleSorter(entry: { columnKey?: string | null; key?: string | null; order?: LatencyOrder }) {
  const sortKey = entry.columnKey ?? entry.key ?? ''
  if (sortKey !== 'latency') return
  latencyOrder.value = entry.order === false ? false : entry.order || false
  applyLatencySort()
}

/** 时间范围变化：只选日期时结束时间自动补到当天 23:59:59.999 */
function handleRangeChange(value: [number, number] | null) {
  if (value && value.length === 2) {
    const start = dayjs(value[0])
    let end = dayjs(value[1])
    if (start.isValid() && end.isValid()) {
      if (end.hour() === 0 && end.minute() === 0 && end.second() === 0 && end.millisecond() === 0) {
        end = end.endOf('day')
      }
      // RFC3339（带本地时区偏移），与后端 time.Time 解析约定一致
      queryParams.start_time = start.format('YYYY-MM-DDTHH:mm:ssZ')
      queryParams.end_time = end.format('YYYY-MM-DDTHH:mm:ssZ')
      range.value = [start.valueOf(), end.valueOf()]
      return
    }
  }
  queryParams.start_time = ''
  queryParams.end_time = ''
  range.value = null
}

async function handleSearch() {
  pagination.page = 1
  await getTableData()
}

async function handleReset() {
  queryParams.ip = ''
  queryParams.username = ''
  queryParams.method = ''
  queryParams.path = ''
  handleRangeChange(null)
  pagination.page = 1
  await getTableData()
}

async function handleRefresh() {
  await getTableData()
}

const columns = computed<DataTableColumns<OperationLogRow>>(() => [
  {
    type: 'expand',
    renderExpand: row =>
      h('div', { class: 'operation-log-expand' }, [
        h(MessageBlock, {
          label: $t('custom.management.operationLog.detail.request'),
          message: row.request_message
        }),
        h(MessageBlock, {
          label: $t('custom.management.operationLog.detail.response'),
          message: row.response_message
        })
      ])
  },
  {
    key: 'created_at',
    title: $t('custom.management.operationLog.table.time'),
    width: 170,
    render: row => formatTime(row.created_at)
  },
  {
    key: 'ip',
    title: $t('custom.management.operationLog.table.ip'),
    width: 130,
    ellipsis: { tooltip: true }
  },
  {
    key: 'username',
    title: $t('custom.management.operationLog.table.username'),
    minWidth: 120,
    ellipsis: { tooltip: true },
    render: row => row.username || row.user_id || '-'
  },
  {
    key: 'name',
    title: $t('custom.management.operationLog.table.name'),
    minWidth: 110,
    ellipsis: { tooltip: true },
    render: row => row.name || '-'
  },
  {
    key: 'method',
    title: $t('custom.management.operationLog.table.method'),
    width: 100,
    render: row => h(NTag, { size: 'small', type: methodTagType(row.name) }, { default: () => row.name || '-' })
  },
  {
    key: 'path',
    title: $t('custom.management.operationLog.table.path'),
    minWidth: 200,
    ellipsis: { tooltip: true },
    render: row => row.path || '-'
  },
  {
    key: 'latency',
    title: $t('custom.management.operationLog.table.latency'),
    width: 110,
    sorter: (rowA, rowB) => rowA.latency - rowB.latency,
    sortOrder: latencyOrder.value,
    render: row => `${row.latency}ms`
  },
  {
    key: 'remark',
    title: $t('custom.management.operationLog.table.remark'),
    minWidth: 140,
    ellipsis: { tooltip: true },
    render: row => row.remark || '-'
  }
])

getTableData()
</script>

<template>
  <div class="h-full flex-col gap-16px">
    <NForm :model="queryParams" label-placement="left" :label-width="90">
      <NGrid :cols="24" :x-gap="12" :y-gap="12">
        <NFormItemGridItem :span="6" :label="$t('custom.management.operationLog.filter.ip')">
          <NInput
            v-model:value="queryParams.ip"
            clearable
            :placeholder="$t('custom.management.operationLog.filter.ipPlaceholder')"
          />
        </NFormItemGridItem>
        <NFormItemGridItem :span="6" :label="$t('custom.management.operationLog.filter.username')">
          <NInput
            v-model:value="queryParams.username"
            clearable
            :placeholder="$t('custom.management.operationLog.filter.usernamePlaceholder')"
          />
        </NFormItemGridItem>
        <NFormItemGridItem :span="6" :label="$t('custom.management.operationLog.filter.method')">
          <NSelect v-model:value="queryParams.method" :options="methodOptions" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="6" :label="$t('custom.management.operationLog.filter.path')">
          <NInput
            v-model:value="queryParams.path"
            clearable
            :placeholder="$t('custom.management.operationLog.filter.pathPlaceholder')"
          />
        </NFormItemGridItem>
        <NFormItemGridItem :span="10" :label="$t('custom.management.operationLog.filter.timeRange')">
          <NDatePicker
            v-model:value="range"
            class="w-full"
            type="datetimerange"
            clearable
            @update:value="handleRangeChange"
          />
        </NFormItemGridItem>
        <NGi :span="14" class="flex items-end justify-end">
          <NSpace>
            <NButton type="primary" :loading="loading" @click="handleSearch">
              {{ $t('custom.management.operationLog.action.search') }}
            </NButton>
            <NButton @click="handleReset">
              {{ $t('custom.management.operationLog.action.reset') }}
            </NButton>
            <NButton :loading="loading" @click="handleRefresh">
              {{ $t('custom.management.operationLog.action.refresh') }}
            </NButton>
          </NSpace>
        </NGi>
      </NGrid>
    </NForm>

    <NDataTable
      remote
      :columns="columns"
      :data="tableData"
      :loading="loading"
      :pagination="pagination"
      :scroll-x="1280"
      flex-height
      min-height="360px"
      @update:sorter="handleSorter"
    >
      <template #empty>
        <div class="operation-log-empty">
          <p class="operation-log-empty__title">{{ $t('custom.management.operationLog.empty.title') }}</p>
          <p class="operation-log-empty__hint">{{ $t('custom.management.operationLog.empty.hint') }}</p>
        </div>
      </template>
    </NDataTable>
  </div>
</template>

<style lang="scss" scoped>
.operation-log-expand {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  padding: 4px 8px;
}

.operation-log-empty {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 24px 0;

  &__title {
    margin: 0;
    font-size: 14px;
  }

  &__hint {
    margin: 0;
    color: rgba(128, 128, 128, 0.6);
    font-size: 12px;
  }
}
</style>
