<!--
  文件用途：场景联动执行日志弹窗（从 dataList.vue 拆出，独立持有日志列表状态）。
  核心逻辑：按 scene_automation_id 分页拉取执行日志（时间范围 + 执行结果过滤），
  列表状态收口在 useListPage；关闭时清空查询与结果，重开时按新场景 id 重新加载。
-->
<script setup lang="ts">
import { computed, getCurrentInstance, ref, watch } from 'vue'
import dayjs from 'dayjs'
import { NButton, NDatePicker, NEmpty, NFlex, NPagination, NSelect, NTable } from 'naive-ui'
import type { SelectOption } from 'naive-ui'
import { sceneAutomationsLog } from '@/service/api/automation'
import { $t } from '@/locales'
import { fromFlatResponse, useListPage } from '@/components/data-table-page/useListPage'

defineOptions({ name: 'SceneLogModal' })

const props = defineProps<{
  show: boolean
  sceneAutomationId: string
}>()

const emit = defineEmits<{
  'update:show': [value: boolean]
}>()

const bodyStyle = ref({ width: '1000px' })

type QueryFormModel = {
  execution_result: string
  queryTime: [number, number]
}

const execution_result_options: SelectOption[] = [
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
]

const createDefaultTimeRange = (): [number, number] => [dayjs().subtract(7, 'day').valueOf(), dayjs().valueOf()]

const { query: logQuery, rows: logData, total: logDataTotal, pagination, load: getLogList, search: queryLog, setPage } =
  useListPage<Record<string, any>, QueryFormModel>({
    initialQuery: () => ({ execution_result: '', queryTime: createDefaultTimeRange() }),
    fetcher: async (params) => {
      // 时间范围选择器持有 [start, end] 时间戳；请求前转换为后端的 ISO 字符串字段。
      const [start, end] = params.queryTime
      const response = await sceneAutomationsLog({
        page: params.page,
        page_size: params.page_size,
        scene_automation_id: props.sceneAutomationId,
        execution_result: params.execution_result,
        execution_start_time: dayjs(start).format(),
        execution_end_time: dayjs(end).format()
      })
      return fromFlatResponse<Record<string, any>>(response)
    }
  })

// 打开弹窗（或切换目标场景）即按当前过滤条件加载第 1 页。
watch(
  () => [props.show, props.sceneAutomationId] as const,
  ([show]) => {
    if (!show) return
    void queryLog()
  },
  { immediate: true }
)

// 关闭时清空列表、过滤条件并回到第 1 页，与旧 closeLog 语义一致。
watch(
  () => props.show,
  (show) => {
    if (show) return
    logQuery.execution_result = ''
    logQuery.queryTime = createDefaultTimeRange()
    logData.value = []
    logDataTotal.value = 0
    pagination.page = 1
  }
)

const close = () => {
  emit('update:show', false)
}

const getPlatform = computed(() => {
  const proxy = getCurrentInstance()?.proxy as any
  return proxy?.getPlatform?.() || false
})
</script>

<template>
  <n-modal
    :show="props.show"
    aria-label="dialog"
    :style="bodyStyle"
    preset="card"
    :title="$t('generate.log')"
    size="huge"
    :bordered="false"
    :class="getPlatform ? 'max-w-90%' : 'w-600px'"
    @update:show="emit('update:show', $event)"
    @close="close"
  >
    <NFlex class="mb-6">
      <n-date-picker v-model:value="logQuery.queryTime" type="datetimerange" @update:value="() => queryLog()" />
      <n-select
        v-model:value="logQuery.execution_result"
        :options="execution_result_options"
        class="max-w-40"
        :placeholder="$t('generate.select-execution-status')"
        @update:value="() => queryLog()"
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
            <th class="min-w-180px">{{ $t('generate.execution-time') }}</th>
            <th>{{ $t('generate.execution-description') }}</th>
            <th class="min-w-120px">{{ $t('generate.execution-status') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="(sceneItem, index) in logData" :key="index">
            <td class="min-w-100px">{{ index + 1 }}</td>
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
          :page="pagination.page"
          :page-size="pagination.pageSize"
          :item-count="logDataTotal"
          @update:page="setPage"
        />
      </NFlex>
    </template>
  </n-modal>
</template>

<style scoped></style>
