<!--
文件用途：P2.2 基础异常检测页（遥测分桶序列上的 bounds / deviation 判定）。

核心逻辑：表单校验与查询构造在 anomaly-model.ts（纯函数，已单测）；本页只负责
交互与呈现，不重写规则。

关键注意事项：
1. 呈现必须区分四种态：本次失败 / 窗口内无数据 / 命中异常 / 无异常。
   "无数据"与"无异常"合并会把"没采到数据"显示成"设备正常"——这是本项目
   最忌讳的假安全。四态判定同样在 model 层（toAnomalyRows），本页不自行推断。
2. 提交前先跑 model 的校验，失败时用 $t() 渲染可读文案；不把校验文案写死在逻辑层。
3. 后端入参校验独立存在，前端校验只为提示，不作为安全边界。
-->
<script setup lang="tsx">
import { computed, ref } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NDatePicker,
  NForm,
  NFormItem,
  NInput,
  NInputNumber,
  NRadioButton,
  NRadioGroup,
  NSelect,
  NSpace,
  NTag,
  type DataTableColumns
} from 'naive-ui'
import { useLoading } from '@aetherlink/hooks'

import { $t } from '@/locales'
import { detectTelemetryAnomalies, type TelemetryAnomalyAggregate, type TelemetryAnomalyResult } from '@/service/api'

import {
  ANOMALY_DEFAULT_K,
  ANOMALY_MAX_DEVICES,
  DEFAULT_ANOMALY_FORM,
  buildAnomalyQuery,
  summarizeAnomalyRows,
  toAnomalyRows,
  type AnomalyFormState,
  type AnomalyRow,
  type AnomalyRowStatus
} from './anomaly-model'

const { loading, startLoading, endLoading } = useLoading(false)

const form = ref<AnomalyFormState>({ ...DEFAULT_ANOMALY_FORM })
const range = ref<[number, number] | null>(null)
const formError = ref('')
const requestError = ref('')
const result = ref<TelemetryAnomalyResult | null>(null)

const aggregateOptions: Array<{ label: string; value: TelemetryAnomalyAggregate }> = [
  { label: 'avg', value: 'avg' },
  { label: 'sum', value: 'sum' },
  { label: 'min', value: 'min' },
  { label: 'max', value: 'max' },
  { label: 'count', value: 'count' },
  { label: 'last', value: 'last' }
]

const rows = computed(() => toAnomalyRows(result.value))
const summary = computed(() => summarizeAnomalyRows(rows.value))

const STATUS_TAG: Record<AnomalyRowStatus, { type: 'error' | 'warning' | 'success' | 'default'; key: string }> = {
  error: { type: 'error', key: 'page.anomaly.statusError' },
  'no-data': { type: 'warning', key: 'page.anomaly.statusNoData' },
  anomaly: { type: 'error', key: 'page.anomaly.statusAnomaly' },
  clean: { type: 'success', key: 'page.anomaly.statusClean' }
}

const columns = computed<DataTableColumns<AnomalyRow>>(() => [
  { title: $t('page.anomaly.colDevice'), key: 'deviceId', minWidth: 160 },
  {
    title: $t('page.anomaly.colStatus'),
    key: 'status',
    width: 130,
    render: row => {
      const meta = STATUS_TAG[row.status]
      return <NTag type={meta.type} size="small">{() => $t(meta.key)}</NTag>
    }
  },
  { title: $t('page.anomaly.colTotal'), key: 'total', width: 110 },
  {
    title: $t('page.anomaly.colRate'),
    key: 'rate',
    width: 110,
    // 后端 rate 是 0–1 的占比，展示成百分比；不做四舍五入到整数，
    // 否则 0.4% 会被显示成 0% 而看起来"没异常"。
    render: row => `${(row.rate * 100).toFixed(2)}%`
  },
  { title: $t('page.anomaly.colHits'), key: 'hits', width: 100, render: row => row.hits.length }
])

function resetForm() {
  form.value = { ...DEFAULT_ANOMALY_FORM }
  range.value = null
  formError.value = ''
  requestError.value = ''
  result.value = null
}

async function runDetection() {
  formError.value = ''
  requestError.value = ''
  const built = buildAnomalyQuery({ form: form.value, range: range.value })
  if (!built.ok) {
    formError.value = $t(built.errorKey)
    return
  }
  startLoading()
  try {
    const { data, error } = await detectTelemetryAnomalies(built.query)
    if (error) {
      requestError.value = String((error as { message?: string }).message ?? error)
      result.value = null
      return
    }
    result.value = data ?? null
  } finally {
    endLoading()
  }
}
</script>

<template>
  <div class="min-h-500px flex flex-col gap-4">
    <NCard :title="$t('page.anomaly.title')">
      <template #header-extra>
        <NSpace>
          <NButton size="small" :disabled="loading" @click="resetForm">{{ $t('page.anomaly.reset') }}</NButton>
          <NButton size="small" type="primary" :loading="loading" @click="runDetection">
            {{ $t('page.anomaly.run') }}
          </NButton>
        </NSpace>
      </template>

      <NAlert v-if="formError" type="warning" class="mb-3" :show-icon="true">{{ formError }}</NAlert>
      <NAlert v-if="requestError" type="error" class="mb-3" :show-icon="true">{{ requestError }}</NAlert>

      <NForm label-placement="left" label-width="130" size="small">
        <NFormItem :label="$t('page.anomaly.devices')">
          <NInput
            v-model:value="form.deviceIdsText"
            type="textarea"
            :rows="4"
            :placeholder="$t('page.anomaly.devicesPlaceholder', { max: ANOMALY_MAX_DEVICES })"
          />
        </NFormItem>
        <NFormItem :label="$t('page.anomaly.key')">
          <NInput v-model:value="form.key" :placeholder="$t('page.anomaly.keyPlaceholder')" />
        </NFormItem>
        <NFormItem :label="$t('page.anomaly.timeRange')">
          <NDatePicker v-model:value="range" type="datetimerange" clearable class="w-full" />
        </NFormItem>
        <NFormItem :label="$t('page.anomaly.window')">
          <NInputNumber v-model:value="form.windowMinutes" :min="1" class="w-40" />
          <span class="ml-2 text-gray-500">{{ $t('page.anomaly.windowUnit') }}</span>
        </NFormItem>
        <NFormItem :label="$t('page.anomaly.aggregate')">
          <NSelect v-model:value="form.aggregate" :options="aggregateOptions" class="w-40" />
        </NFormItem>
        <NFormItem :label="$t('page.anomaly.rule')">
          <NRadioGroup v-model:value="form.ruleType">
            <NRadioButton value="bounds">{{ $t('page.anomaly.ruleBounds') }}</NRadioButton>
            <NRadioButton value="deviation">{{ $t('page.anomaly.ruleDeviation') }}</NRadioButton>
          </NRadioGroup>
        </NFormItem>
        <NFormItem v-if="form.ruleType === 'bounds'" :label="$t('page.anomaly.bounds')">
          <NSpace align="center">
            <span class="text-gray-500">{{ $t('page.anomaly.min') }}</span>
            <NInputNumber v-model:value="form.boundsMin" class="w-32" clearable />
            <span class="text-gray-500">{{ $t('page.anomaly.max') }}</span>
            <NInputNumber v-model:value="form.boundsMax" class="w-32" clearable />
          </NSpace>
        </NFormItem>
        <NFormItem v-else :label="$t('page.anomaly.deviationK')">
          <NInputNumber v-model:value="form.deviationK" :min="0" :placeholder="String(ANOMALY_DEFAULT_K)" class="w-32" clearable />
          <span class="ml-2 text-gray-500">{{ $t('page.anomaly.deviationHint') }}</span>
        </NFormItem>
      </NForm>
    </NCard>

    <NCard v-if="result" :title="$t('page.anomaly.results')">
      <NSpace class="mb-3">
        <NTag size="small">{{ $t('page.anomaly.summaryDevices', { count: summary.devices }) }}</NTag>
        <NTag size="small" type="error">
          {{ $t('page.anomaly.summaryAnomaly', { count: summary.anomaly, hits: summary.totalHits }) }}
        </NTag>
        <NTag size="small" type="success">{{ $t('page.anomaly.summaryClean', { count: summary.clean }) }}</NTag>
        <NTag size="small" type="warning">{{ $t('page.anomaly.summaryNoData', { count: summary.noData }) }}</NTag>
        <NTag v-if="summary.failed" size="small" type="error">
          {{ $t('page.anomaly.summaryFailed', { count: summary.failed }) }}
        </NTag>
      </NSpace>

      <NDataTable
        :columns="columns"
        :data="rows"
        :row-key="row => row.deviceId"
        size="small"
        :bordered="false"
      />
    </NCard>
  </div>
</template>
