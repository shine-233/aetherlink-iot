<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useMessage } from 'naive-ui'
import dayjs from 'dayjs'
import { addMonths } from 'date-fns'
import { telemetryHistoryData } from '@/service/api'
import { $t } from '@/locales'
import { getBaseServerUrl } from '@/utils/common/tool'
import { buildTelemetryHistoryDownloadUrl } from './history-data-state'
import { useLoading } from '~/packages/hooks'

interface Created {
  deviceId: string
  theKey: string
}

interface Params {
  device_id: string
  end_time: number
  start_time: number
  export_excel: boolean
  key: string
  page: number
  page_size: number
}

interface HistoryData {
  key: string
  ts: string
  value: number
}

const props = defineProps<Created>()
const baseURL = getBaseServerUrl()
const message = useMessage()
const { loading, startLoading, endLoading } = useLoading()

const end_time = dayjs().endOf('day').valueOf()
const start_time = dayjs().subtract(1, 'day').startOf('day').valueOf()

const params = reactive<Params>({
  device_id: props.deviceId,
  end_time,
  start_time,
  export_excel: false,
  key: props.theKey,
  page: 1,
  page_size: 5
})

const tableData = ref<HistoryData[]>([])
const dateRange = ref<[number, number] | null>([params.start_time, params.end_time])

const pagination = reactive({
  page: 1,
  pageSize: 5,
  showSizePicker: true,
  pageSizes: [5, 10, 15, 20],
  itemCount: 0,
  onChange: (page: number) => {
    pagination.page = page
    params.page = page
    void getTelemetryHistoryData()
  },
  onUpdatePageSize: (pageSize: number) => {
    pagination.pageSize = pageSize
    pagination.page = 1
    params.page_size = pageSize
    params.page = 1
    void getTelemetryHistoryData()
  }
})

const columns = [
  {
    title: $t('common.time'),
    key: 'time',
    render: (row: HistoryData) => dayjs(row.ts).format('YYYY-MM-DD HH:mm:ss')
  },
  {
    title: $t('device_template.table_header.dataIdentifier'),
    key: 'key'
  },
  {
    title: $t('generate.fieldValue'),
    key: 'value',
    render: (row: HistoryData) => row.value.toString()
  }
]

const getTelemetryHistoryData = async () => {
  if (!props.deviceId || !props.theKey) {
    tableData.value = []
    return
  }

  startLoading()
  const requestParams = { ...params }
  const isExportRequest = requestParams.export_excel
  try {
    const { data, error } = await telemetryHistoryData(requestParams)

    if (isExportRequest) {
      params.export_excel = false
      if (error) {
        message.error($t('custom.device_details.telemetryExportFailed'))
        return
      }

      const downloadUrl = buildTelemetryHistoryDownloadUrl(baseURL, data?.filePath)
      if (downloadUrl) {
        window.open(downloadUrl)
      } else {
        message.error($t('custom.device_details.telemetryExportFailed'))
      }
      return
    }

    if (error) {
      message.error($t('custom.device_details.telemetryHistoryLoadFailed'))
      return
    }

    tableData.value = data?.list || []
    pagination.itemCount = data?.total || 0
  } finally {
    endLoading()
  }
}

const checkDateRange = (value: [number, number] | null) => {
  if (!value) return

  const [start, end] = value

  if (start && end && addMonths(start, 1).getTime() < end) {
    dateRange.value = null
    message.error($t('common.withinOneMonth'))
    return
  }

  params.start_time = start
  params.end_time = end
  params.export_excel = false
  void getTelemetryHistoryData()
}

const refresh = () => {
  params.export_excel = false
  pagination.page = 1
  params.page = 1
  void getTelemetryHistoryData()
}

const exportHistoryData = () => {
  if (!dateRange.value) return
  params.export_excel = true
  void getTelemetryHistoryData()
}

onMounted(getTelemetryHistoryData)
</script>

<template>
  <n-card>
    <n-flex justify="space-between" align="center">
      <n-flex justify="space-between" align="center">
        <n-date-picker
          v-model:value="dateRange"
          type="datetimerange"
          format="yyyy-MM-dd HH:mm:ss"
          :default-time="['00:00:00', '23:59:59']"
          :time-picker-props="[{ defaultValue: 0 }, { defaultValue: 86399 }]"
          @update:value="checkDateRange"
        />
        <n-button class="ml-2" @click="refresh">{{ $t('generate.refresh') }}</n-button>
      </n-flex>

      <n-button type="primary" :disabled="!dateRange || loading" @click="exportHistoryData">
        {{ $t('generate.export') }}
      </n-button>
    </n-flex>
    <div class="mt-4">
      <n-text v-if="!dateRange" depth="3">{{ $t('generate.hour-24') }}</n-text>
      <n-alert v-else-if="!loading && tableData.length === 0" type="info" class="mb-3" :show-icon="true">
        {{ $t('custom.device_details.telemetryNoData') }}
      </n-alert>
      <n-data-table :loading="loading" :columns="columns" :data="tableData" />
      <div class="mt-4 flex justify-end">
        <n-pagination
          v-model:page="pagination.page"
          v-model:page-size="pagination.pageSize"
          :item-count="pagination.itemCount"
          :page-sizes="pagination.pageSizes"
          :show-size-picker="pagination.showSizePicker"
          @update:page="pagination.onChange"
          @update:page-size="pagination.onUpdatePageSize"
        />
      </div>
    </div>
  </n-card>
</template>

<style scoped>
.n-card {
  width: 100%;
}
</style>
