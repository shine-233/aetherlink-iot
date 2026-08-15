<!--
设备期望消息面板，负责查询和维护当前设备的期望消息队列。
核心链路：基于设备 ID 组合筛选条件调用 `expectMessageList` 拉取记录，表格展示消息类型、标签、payload、过期时间、发送状态与处理信息；删除操作通过 `expectMessageDelete` 删除单条记录后再整表回刷。
使用注意：
1. 期望消息通常与设备在线状态、命令确认和延迟下发有关，重复删除或重复创建都可能影响真实业务链路。
2. 当前默认筛选 `pending`，更偏向待发送消息排查；如果运维关注已发送或已过期记录，需要显式切换筛选状态。
3. 删除按钮直接作用于真实期望消息记录，当前只有二次确认，没有额外权限或状态校验。
静态审查建议：
1. `getTableData` 与 `handleDeleteTable` 都没有独立 loading 和 `try/finally` 收口，网络慢或接口异常时反馈较弱。
2. 查询条件、分页状态和首屏拉取写在同一文件内，后续可抽成 composable，减少设备详情各面板的重复远程表格样板代码。
3. `payload` 仍按纯文本直出，复杂 JSON 消息适合补格式化预览或复制入口。
-->
<script setup lang="tsx">
import { reactive, ref } from 'vue'
import type { Ref } from 'vue'
import type { PaginationProps } from 'naive-ui'
import { NButton, NPopconfirm } from 'naive-ui'
import dayjs from 'dayjs'
import { expectMessageDelete, expectMessageList } from '@/service/api'
import { $t } from '@/locales'

const props = defineProps<{
  id: string
}>()

// 表格数据完全由远程接口驱动，本地不维护编辑态，只保留当前页结果。
const tableData = ref([])

// 状态选项同时服务于顶部筛选和状态列渲染，接口码值变更时需要同步维护这里。
const statusOptions = ref([
  { label: $t('page.expect.pending'), value: 'pending' },
  { label: $t('page.expect.send'), value: 'sent' },
  { label: $t('page.expect.expired'), value: 'expired' }
])

// 消息类型覆盖遥测、属性、命令三类常见期望消息场景，既用于筛选也用于结果展示。
const typeOptions = ref([
  { label: $t('custom.device_details.telemetry'), value: 'telemetry' },
  { label: $t('custom.device_details.attributes'), value: 'attribute' },
  { label: $t('page.expect.command'), value: 'command' }
])

// 查询条件保持为响应式对象，顶部筛选和分页器都会回写这里。
// 页面只维护一份请求参数快照，避免筛选栏与分页状态彼此漂移。
const query = reactive({
  status: 'pending',
  type: null,
  label: null,
  page: 1,
  page_size: 10
})

// 分页采用远程模式，页码和每页条数变化时都要同步回写 query，再重新请求服务端结果。
// 这里不做前端切片，保证筛选、总数和列表口径全部以后端返回为准。
const pagination: PaginationProps = reactive({
  page: 1,
  pageSize: 10,
  showSizePicker: true,
  pageSizes: [10, 15, 20, 25, 30],
  itemCount: 0,
  onChange: (page: number) => {
    pagination.page = page
    query.page = page
    getTableData()
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
    query.page_size = pageSize
    getTableData()
  }
})

// 期望消息列表查询统一从这里出发：
// 1. 固定注入当前设备 ID，避免详情页切设备时串到别的设备记录；
// 2. 把顶部筛选条件和分页参数透传给接口；
// 3. 回写表格数据和总条数，驱动远程分页。
async function getTableData() {
  const { data, error } = await expectMessageList({
    device_id: props.id,
    send_type: query.type,
    ...query
  })
  if (!error) {
    const list: any = data.list || []
    tableData.value = list
    pagination.itemCount = data.total || 0
  }
}

// 删除期望消息后直接整表回刷，保持前端列表与后端真实状态一致。
// 当前没有区分“待发送可删 / 已发送不可删”等更细粒度约束，真实边界完全依赖后端接口。
const handleDeleteTable = async id => {
  const { error } = await expectMessageDelete(id)
  if (!error) {
    window.$message?.success($t('common.deleteSuccess'))
    getTableData()
  }
}

// 表格列围绕“消息何时创建、属于哪类、发了什么、何时过期、当前状态如何”组织，
// 方便排查设备为何未按预期收到消息，或为何某条命令迟迟没有被设备消费。
const columns: Ref<any> = ref([
  {
    key: 'created_at',
    minWidth: '200px',
    title: () => $t('page.expect.createTime'),
    render: row => {
      // 列表时间统一在前端格式化，便于与命令日志、事件日志直接对时比对。
      return row.created_at ? dayjs(row.created_at).format('YYYY-MM-DD hh:mm:ss') : ''
    }
  },
  {
    key: 'send_type',
    minWidth: '100px',
    title: () => $t('page.expect.commandType'),
    render: row => {
      return typeOptions.value.find(v => v.value === row.send_type)?.label
    }
  },
  {
    key: 'label',
    minWidth: '100px',
    title: () => $t('page.expect.label')
  },
  {
    key: 'payload',
    minWidth: '200px',
    title: () => $t('page.expect.commandContent')
  },
  {
    key: 'expiry_time',
    minWidth: '200px',
    title: () => $t('page.expect.expireTime'),
    render: row => {
      return row.expiry_time ? dayjs(row.expiry_time).format('YYYY-MM-DD hh:mm:ss') : ''
    }
  },
  {
    key: 'status',
    minWidth: '100px',
    title: () => $t('page.expect.status'),
    render: row => {
      return statusOptions.value.find(v => v.value === row.status)?.label
    }
  },
  {
    key: 'message',
    minWidth: '140px',
    title: () => $t('page.expect.statusInfo')
  },
  {
    key: 'send_time',
    minWidth: '200px',
    title: () => $t('page.expect.dealTime'),
    render: row => {
      return row.send_time ? dayjs(row.send_time).format('YYYY-MM-DD hh:mm:ss') : ''
    }
  },
  {
    title: $t('common.actions'),
    key: 'created_at',
    minWidth: '100px',
    render: row => {
      return (
        <NPopconfirm
          negative-text={$t('common.cancel')}
          positive-text={$t('common.confirm')}
          onPositiveClick={() => handleDeleteTable(row.id)}
        >
          {{
            default: () => $t('common.confirm'),
            trigger: () => (
              <NButton type="error" size={'small'}>
                {$t('common.delete')}
              </NButton>
            )
          }}
        </NPopconfirm>
      )
    }
  }
]) as Ref<any>

// 搜索统一从第一页重新查询，避免保留旧页码后出现“有条件但列表空白”的错觉。
const handleSearch = () => {
  pagination.page = 1
  getTableData()
}

// 首屏进入时默认按 pending 拉取一次，优先暴露最值得关注的待发送消息。
handleSearch()

// 重置时恢复默认筛选，再复用搜索入口统一触发第一页查询。
const handleReset = () => {
  query.status = 'pending'
  query.type = null
  query.label = null
  query.page = 1
  query.page_size = 10
  handleSearch()
}
</script>

<template>
  <div class="flex flex-col gap-15px rounded-lg">
    <div class="row flex items-end justify-between gap-4">
      <NForm class="flex-wrap" inline label-placement="left" label-align="right" label-width="120">
        <NFormItem>
          <NSelect
            v-model:value="query.status"
            :options="statusOptions"
            :placeholder="$t('page.expect.send')"
            class="input-style w-200px"
            clearable
          />
        </NFormItem>
        <NFormItem>
          <NSelect
            v-model:value="query.type"
            :options="typeOptions"
            :placeholder="$t('page.expect.selectCommandTypePlease')"
            class="input-style w-200px"
            clearable
          />
        </NFormItem>
        <NFormItem>
          <NInput
            v-model:value="query.label"
            :placeholder="$t('page.expect.inputLabelPlease')"
            class="input-style w-200px"
          />
        </NFormItem>
        <NFormItem>
          <NButton type="primary" @click="handleSearch">{{ $t('common.search') }}</NButton>
          <NButton class="ml-12px" @click="handleReset">{{ $t('common.reset') }}</NButton>
        </NFormItem>
      </NForm>
    </div>
  </div>
  <n-data-table :columns="columns" :data="tableData" :pagination="pagination" :remote="true" />
</template>
