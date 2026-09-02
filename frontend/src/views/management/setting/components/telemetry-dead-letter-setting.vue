<script setup lang="tsx">
import { computed, reactive, ref } from 'vue'
import { NButton, NEmpty, NPopconfirm, NSpace, NTag, NText } from 'naive-ui'
import type { DataTableColumns, PaginationProps } from 'naive-ui'
import dayjs from 'dayjs'
import {
  drainTelemetryDeadLetters,
  getTelemetryDeadLetters,
  updateTelemetryDeadLetterStatus
} from '@/service/api/telemetry-dead-letter'
import type {
  TelemetryDeadLetterAction,
  TelemetryDeadLetterRow,
  TelemetryDeadLetterStatus
} from '@/service/api/telemetry-dead-letter'
import { $t } from '@/locales'
import { useLoading } from '~/packages/hooks'

const { loading, startLoading, endLoading } = useLoading(false)
const actingId = ref('')
const draining = ref(false)
const tableData = ref<TelemetryDeadLetterRow[]>([])

const statusOptions = computed(() => [
  { label: $t('custom.management.telemetryDeadLetter.allStatuses'), value: '' },
  { label: $t('custom.management.telemetryDeadLetter.status.pending'), value: 'pending' },
  { label: $t('custom.management.telemetryDeadLetter.status.processing'), value: 'processing' },
  { label: $t('custom.management.telemetryDeadLetter.status.retrying'), value: 'retrying' },
  { label: $t('custom.management.telemetryDeadLetter.status.resolved'), value: 'resolved' },
  { label: $t('custom.management.telemetryDeadLetter.status.dead'), value: 'dead' }
])

const queryParams = reactive<{
  tenant_id: string
  device_id: string
  key: string
  status: TelemetryDeadLetterStatus | ''
}>({
  tenant_id: '',
  device_id: '',
  key: '',
  status: ''
})

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

function normalizePagedResult(result: any) {
  const data = result?.data || result || {}
  return {
    list: Array.isArray(data.list) ? data.list : [],
    total: Number(data.total || 0)
  }
}

function statusTagType(status: string): NaiveUI.ThemeColor {
  if (status === 'resolved') return 'success'
  if (status === 'dead') return 'error'
  if (status === 'processing') return 'primary'
  if (status === 'retrying') return 'warning'
  return 'info'
}

function statusLabel(status: string) {
  switch (status) {
    case 'pending':
      return $t('custom.management.telemetryDeadLetter.status.pending')
    case 'processing':
      return $t('custom.management.telemetryDeadLetter.status.processing')
    case 'retrying':
      return $t('custom.management.telemetryDeadLetter.status.retrying')
    case 'resolved':
      return $t('custom.management.telemetryDeadLetter.status.resolved')
    case 'dead':
      return $t('custom.management.telemetryDeadLetter.status.dead')
    default:
      return status || '-'
  }
}

function formatTime(value?: string | number | null) {
  if (!value) return '-'
  const time = typeof value === 'number' ? dayjs(value) : dayjs(value)
  return time.isValid() ? time.format('YYYY-MM-DD HH:mm:ss') : String(value)
}

function formatValue(row: TelemetryDeadLetterRow) {
  if (row.bool_v !== undefined) return String(row.bool_v)
  if (row.number_v !== undefined) return String(row.number_v)
  if (row.string_v !== undefined) return row.string_v
  return '-'
}

async function getTableData() {
  startLoading()
  try {
    const result = await getTelemetryDeadLetters({
      page: pagination.page,
      page_size: pagination.pageSize,
      ...queryParams
    })
    const data = normalizePagedResult(result)
    tableData.value = data.list
    pagination.itemCount = data.total
  } catch {
    tableData.value = []
    pagination.itemCount = 0
  } finally {
    endLoading()
  }
}

async function handleSearch() {
  pagination.page = 1
  await getTableData()
}

async function handleAction(id: string, action: TelemetryDeadLetterAction) {
  actingId.value = id
  try {
    await updateTelemetryDeadLetterStatus(id, action)
    window.$message?.success($t('custom.management.telemetryDeadLetter.operationSubmitted'))
    await getTableData()
  } finally {
    actingId.value = ''
  }
}

async function handleDrain() {
  draining.value = true
  try {
    const result = await drainTelemetryDeadLetters({
      tenant_id: queryParams.tenant_id,
      device_id: queryParams.device_id,
      key: queryParams.key,
      limit: pagination.pageSize
    })
    const data: any = (result as any)?.data || result || {}
    window.$message?.success(
      $t('custom.management.telemetryDeadLetter.drainComplete', {
        attempted: data.attempted || 0,
        replayed: data.replayed || 0,
        failed: data.failed || 0
      })
    )
    await getTableData()
  } finally {
    draining.value = false
  }
}

function actionButton(
  row: TelemetryDeadLetterRow,
  action: TelemetryDeadLetterAction,
  label: string,
  type: NaiveUI.ThemeColor
) {
  return (
    <NPopconfirm
      negative-text={$t('custom.management.telemetryDeadLetter.cancel')}
      positive-text={$t('custom.management.telemetryDeadLetter.confirm')}
      onPositiveClick={() => handleAction(row.id, action)}
    >
      {{
        default: () => $t('custom.management.telemetryDeadLetter.confirmAction', { action: label }),
        trigger: () => (
          <NButton
            size="small"
            type={type}
            loading={actingId.value === row.id}
            disabled={Boolean(actingId.value) || row.status === 'processing'}
          >
            {label}
          </NButton>
        )
      }}
    </NPopconfirm>
  )
}

const columns = computed<DataTableColumns<TelemetryDeadLetterRow>>(() => [
  {
    key: 'status',
    title: $t('custom.management.telemetryDeadLetter.statusLabel'),
    width: 110,
    render: row => <NTag type={statusTagType(row.status)}>{statusLabel(row.status)}</NTag>
  },
  {
    key: 'device_id',
    title: $t('custom.management.telemetryDeadLetter.deviceId'),
    minWidth: 170,
    ellipsis: {
      tooltip: true
    }
  },
  {
    key: 'tenant_id',
    title: $t('custom.management.telemetryDeadLetter.tenantId'),
    minWidth: 150,
    ellipsis: {
      tooltip: true
    }
  },
  {
    key: 'key',
    title: $t('custom.management.telemetryDeadLetter.key'),
    minWidth: 130,
    ellipsis: {
      tooltip: true
    }
  },
  {
    key: 'value',
    title: $t('custom.management.telemetryDeadLetter.value'),
    minWidth: 120,
    render: row => <NText>{formatValue(row)}</NText>
  },
  {
    key: 'attempts',
    title: $t('custom.management.telemetryDeadLetter.attempts'),
    width: 90
  },
  {
    key: 'ts',
    title: $t('custom.management.telemetryDeadLetter.telemetryTime'),
    minWidth: 170,
    render: row => formatTime(row.ts)
  },
  {
    key: 'next_retry_at',
    title: $t('custom.management.telemetryDeadLetter.nextRetry'),
    minWidth: 170,
    render: row => formatTime(row.next_retry_at)
  },
  {
    key: 'last_error',
    title: $t('custom.management.telemetryDeadLetter.lastError'),
    minWidth: 220,
    ellipsis: {
      tooltip: true
    },
    render: row => row.last_error || '-'
  },
  {
    key: 'actions',
    title: $t('custom.management.telemetryDeadLetter.actions'),
    width: 280,
    fixed: 'right',
    render: row => (
      <NSpace size={8}>
        {actionButton(row, 'replay', $t('custom.management.telemetryDeadLetter.replay'), 'primary')}
        {actionButton(row, 'retry', $t('custom.management.telemetryDeadLetter.retry'), 'warning')}
        {actionButton(row, 'resolve', $t('custom.management.telemetryDeadLetter.resolve'), 'success')}
        {actionButton(row, 'ignore', $t('custom.management.telemetryDeadLetter.ignore'), 'error')}
      </NSpace>
    )
  }
])

getTableData()
</script>

<template>
  <div class="h-full flex-col gap-16px">
    <NForm :model="queryParams" label-placement="left" :label-width="80">
      <NGrid :cols="24" :x-gap="12" :y-gap="12">
        <NFormItemGridItem :span="6" :label="$t('custom.management.telemetryDeadLetter.statusLabel')">
          <NSelect v-model:value="queryParams.status" :options="statusOptions" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="6" :label="$t('custom.management.telemetryDeadLetter.deviceId')">
          <NInput
            v-model:value="queryParams.device_id"
            clearable
            :placeholder="$t('custom.management.telemetryDeadLetter.deviceIdPlaceholder')"
          />
        </NFormItemGridItem>
        <NFormItemGridItem :span="6" :label="$t('custom.management.telemetryDeadLetter.key')">
          <NInput
            v-model:value="queryParams.key"
            clearable
            :placeholder="$t('custom.management.telemetryDeadLetter.keyPlaceholder')"
          />
        </NFormItemGridItem>
        <NFormItemGridItem :span="6" :label="$t('custom.management.telemetryDeadLetter.tenantId')">
          <NInput
            v-model:value="queryParams.tenant_id"
            clearable
            :placeholder="$t('custom.management.telemetryDeadLetter.tenantIdPlaceholder')"
          />
        </NFormItemGridItem>
      </NGrid>
    </NForm>

    <NSpace justify="end" align="center">
      <NSpace>
        <NButton :loading="loading" @click="handleSearch">
          {{ $t('custom.management.telemetryDeadLetter.refresh') }}
        </NButton>
        <NPopconfirm
          :negative-text="$t('custom.management.telemetryDeadLetter.cancel')"
          :positive-text="$t('custom.management.telemetryDeadLetter.confirm')"
          @positive-click="handleDrain"
        >
          <template #trigger>
            <NButton type="primary" :loading="draining">
              {{ $t('custom.management.telemetryDeadLetter.drainReady') }}
            </NButton>
          </template>
          {{ $t('custom.management.telemetryDeadLetter.confirmDrain') }}
        </NPopconfirm>
      </NSpace>
    </NSpace>

    <NDataTable
      remote
      :columns="columns"
      :data="tableData"
      :loading="loading"
      :pagination="pagination"
      :scroll-x="1500"
      flex-height
      min-height="360px"
    >
      <template #empty>
        <NEmpty :description="$t('common.noData')" class="py-24px" />
      </template>
    </NDataTable>
  </div>
</template>
