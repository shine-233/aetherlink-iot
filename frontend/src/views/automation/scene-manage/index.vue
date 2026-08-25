<!--
文件用途: 承载场景管理相关的自动化页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="tsx">
import { computed, ref } from 'vue'
import type { Ref } from 'vue'
import { NButton, NCard, NFlex, NPagination, NPopconfirm, NSpace, useDialog, useMessage } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import { SearchOutline } from '@vicons/ionicons5'
import dayjs from 'dayjs'
import { sceneActive, sceneDel, sceneGet, sceneLog } from '@/service/api/automation'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
import { formatDateTime } from '@/utils/common/datetime'

const dialog = useDialog()
const message = useMessage()
const { routerPushByKey } = useRouterPush()
const showLog = ref(false)
const logDataTotal = ref(0)
const logData = ref([])
// 新建场景
const sceneAdd = () => {
  routerPushByKey('automation_scene-edit')
}

// 编辑场景
const sceneEdit = (item: any) => {
  routerPushByKey('automation_scene-edit', { query: { id: item.id } })
}

// 激活场景
const sceneActivation = async (item: any) => {
  const res = await sceneActive(item.id)
  if (!res.error) {
    // eslint-disable-next-line @typescript-eslint/no-use-before-define
    await getData()
  }
}
const tableData = ref([])
const queryData = ref({
  name: '',
  page: 1,
  page_size: 10
})
const dataTotal = ref(0)

const getData = async () => {
  const { data, error } = await sceneGet(queryData.value)
  if (error || !data) {
    tableData.value = []
    dataTotal.value = 0
    return
  }
  tableData.value = data.list || []
  dataTotal.value = data.total || 0
}
const handleQuery = async () => {
  queryData.value.page = 1
  await getData()
}
const hasSceneFilters = computed(() => queryData.value.name.trim().length > 0)
const resetSceneQuery = async () => {
  queryData.value.name = ''
  queryData.value.page = 1
  await getData()
}
const logQuery = ref({
  id: '',
  page: 1,
  page_size: 10,
  time: ref<[number, number]>([dayjs().subtract(7, 'day').valueOf(), dayjs().valueOf()]),
  execution_start_time: '',
  execution_end_time: '',
  execution_result: ''
})
const getLogList = async () => {
  if (logQuery.value.time) {
    logQuery.value.execution_start_time = dayjs(logQuery.value.time[0]).format()
    logQuery.value.execution_end_time = dayjs(logQuery.value.time[1]).format()
  }
  const res = await sceneLog(logQuery.value)
  if (res.error || !res.data) {
    logData.value = []
    logDataTotal.value = 0
    return
  }
  logData.value = res.data.list || []
  logDataTotal.value = res.data.total || 0
}
// 查看日志
const openLog = (item: any) => {
  logQuery.value.id = item.id
  getLogList()
  showLog.value = true
}
// 删除场景
const deleteScene = async (item: any) => {
  dialog.warning({
    title: $t('common.deletePrompt'),
    content: $t('common.deleteSceneInfo'),
    positiveText: $t('device_template.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: async () => {
      const res = await sceneDel(item.id)
      if (!res.error) {
        await getData()
        message.success($t('custom.grouping_details.operationSuccess'))
      }
    }
  })
}
const bodyStyle = ref({
  width: '1000px'
})

const columns: Ref<any> = ref([
  {
    key: 'name',
    title: $t('generate.scene-name'),
    minWidth: '100px',
    align: 'left'
  },
  {
    key: 'description',
    title: $t('generate.scene-description'),
    minWidth: '100px',
    align: 'left'
  },
  {
    key: 'created_at',
    title: $t('common.creationTime'),
    align: 'left',
    minWidth: '100px',
    render: (row: any) => {
      return formatDateTime(row.created_at)
    }
  },
  {
    key: 'updated_at',
    title: $t('generate.modification-time'),
    align: 'left',
    minWidth: '100px',
    render: (row: any) => {
      return formatDateTime(row.updated_at)
    }
  },
  {
    key: 'actions',
    title: $t('common.actions'),
    align: 'left',
    width: '300px',
    render: row => {
      return (
        <NSpace justify={'start'}>
          <NButton size={'small'} type="success" onClick={() => sceneActivation(row)}>
            {$t('generate.activate')}
          </NButton>
          <NButton size={'small'} type="primary" onClick={() => sceneEdit(row)}>
            {$t('common.edit')}
          </NButton>
          <NButton size={'small'} type="tertiary" onClick={() => openLog(row)}>
            {$t('generate.log')}
          </NButton>
          <NPopconfirm onPositiveClick={() => deleteScene(row)}>
            {{
              default: () => $t('common.confirmDelete'),
              trigger: () => (
                <NButton type="error" size={'small'}>
                  {$t('common.delete')}
                </NButton>
              )
            }}
          </NPopconfirm>
        </NSpace>
      )
    }
  }
]) as Ref<DataTableColumns<DataService.Data>>

const execution_result_options = ref([
  {
    label: $t('custom.device_details.whole'),
    value: ''
  },
  {
    label: $t('generate.execution-successful'),
    value: 'S'
  },
  {
    label: $t('generate.execution-failed'),
    value: 'F'
  }
])

const queryLog = () => {
  logQuery.value.page = 1
  // eslint-disable-next-line @typescript-eslint/no-use-before-define
  getLogList()
}

const logClose = () => {
  logQuery.value = {
    id: '',
    page: 1,
    page_size: 10,
    time: [dayjs().subtract(7, 'day').valueOf(), dayjs().valueOf()],
    execution_start_time: '',
    execution_end_time: '',
    execution_result: ''
  }
}
getData()
</script>

<template>
  <div class="h-full w-full">
    <NCard>
      <NFlex justify="space-between" class="mb-4">
        <NButton type="primary" @click="sceneAdd()">{{ $t('generate.+add-scene') }}</NButton>
        <NFlex align="center" justify="flex-end" :wrap="false">
          <NInput
            v-model:value="queryData.name"
            :placeholder="$t('generate.enterSceneName')"
            class="search-input"
            type="text"
            clearable
          >
            <template #prefix>
              <NIcon>
                <SearchOutline />
              </NIcon>
            </template>
          </NInput>
          <NButton class="w-72px" type="primary" @click="handleQuery">{{ $t('common.search') }}</NButton>
        </NFlex>
      </NFlex>
      <n-data-table :columns="columns" :data="tableData" class="mt-4">
        <template #empty>
          <div class="scene-empty-state">
            <n-empty
              :description="
                hasSceneFilters
                  ? $t('page.automation.sceneManage.emptyFilteredTitle')
                  : $t('page.automation.sceneManage.emptyTitle')
              "
            >
              <template #extra>
                <div class="scene-empty-extra">
                  <p>
                    {{
                      hasSceneFilters
                        ? $t('page.automation.sceneManage.emptyFilteredDesc')
                        : $t('page.automation.sceneManage.emptyDesc')
                    }}
                  </p>
                  <NButton v-if="hasSceneFilters" secondary @click="resetSceneQuery">
                    {{ $t('page.automation.sceneManage.emptyClearFilters') }}
                  </NButton>
                  <NButton v-else type="primary" @click="sceneAdd">
                    {{ $t('page.automation.sceneManage.emptyCreateFirst') }}
                  </NButton>
                </div>
              </template>
            </n-empty>
          </div>
        </template>
      </n-data-table>
      <NFlex justify="flex-end" class="mt-4">
        <NPagination
          v-model:page="queryData.page"
          :page-size="queryData.page_size"
          :item-count="dataTotal"
          @update:page="getData"
        />
      </NFlex>
    </NCard>
    <n-modal
      aria-label="dialog"
      v-model:show="showLog"
      :style="bodyStyle"
      preset="card"
      :title="$t('generate.log')"
      size="huge"
      :bordered="false"
      @close="logClose()"
    >
      <NFlex class="mb-6">
        <n-date-picker v-model:value="logQuery.time" type="datetimerange" @update:value="queryLog" />
        <n-select
          v-model:value="logQuery.execution_result"
          :options="execution_result_options"
          class="max-w-40"
          :placeholder="$t('generate.select-execution-status')"
          @update:value="queryLog"
        ></n-select>
        <NButton type="primary" @click="queryLog()">{{ $t('common.search') }}</NButton>
      </NFlex>
      <n-empty
        v-if="logDataTotal === 0"
        size="huge"
        :description="$t('common.noData')"
        class="min-h-60 justify-center"
      ></n-empty>
      <template v-else>
        <NTable size="small" :bordered="false" :single-line="false" class="mb-6">
          <thead>
            <tr>
              <th>{{ $t('generate.order-number') }}</th>
              <th class="w-180px">{{ $t('generate.execution-time') }}</th>
              <th>{{ $t('generate.execution-description') }}</th>
              <th class="w-120px">{{ $t('generate.execution-status') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="(sceneItem, index) in logData" :key="index">
              <td class="w-100px">{{ index + 1 }}</td>
              <td>{{ dayjs(sceneItem['executed_at']).format('YYYY-MM-DD HH:mm:ss') }}</td>
              <td>{{ sceneItem['detail'] }}</td>
              <td>
                <span v-if="sceneItem['execution_result'] === 'S'">{{ $t('generate.execution-successful') }}</span>
                <span v-if="sceneItem['execution_result'] === 'F'">{{ $t('generate.execution-failed') }}</span>
              </td>
            </tr>
          </tbody>
        </NTable>
        <NFlex justify="end">
          <NPagination
            v-model:page="logQuery.page"
            :page-size="logQuery.page_size"
            :item-count="logDataTotal"
            @update:page="getLogList"
          />
        </NFlex>
      </template>
    </n-modal>
  </div>
</template>

<style scoped>
.scene-empty-state {
  display: flex;
  min-height: 220px;
  align-items: center;
  justify-content: center;
  padding: 28px 16px;
}

.scene-empty-extra {
  display: grid;
  justify-items: center;
  gap: 12px;
}

.scene-empty-extra p {
  max-width: 440px;
  margin: 0;
  color: var(--text-color-3);
  font-size: 13px;
  line-height: 1.6;
  text-align: center;
}
</style>
