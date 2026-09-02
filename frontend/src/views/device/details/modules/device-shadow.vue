<!--
设备影子消息面板（ROADMAP A3：离线命令缓存）。
展示当前设备的影子消息队列，支持按状态筛选、新建（在线直发/离线缓存由后端分流）和取消待投递消息。
数据边界：
1. 列表与操作都锚定当前详情页设备 ID，权限由后端设备访问守卫校验。
2. payload 必须是合法 JSON；TTL 范围 60~604800 秒，缺省 24h。
3. direct=true 表示设备在线已直接下发，不产生队列记录。
-->
<script setup lang="ts">
import { h, onMounted, reactive, ref } from 'vue'
import type { Ref } from 'vue'
import { NButton, NEmpty, NPopconfirm, NTag } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import dayjs from 'dayjs'
import { deviceShadowCancel, deviceShadowList, deviceShadowSet } from '@/service/api'
import { $t } from '@/locales'

const props = defineProps<{
  id: string
}>()

const tableData: Ref<any[]> = ref([])
const counts = ref<Record<string, number>>({})
const loading = ref(false)

const statusOptions = [
  { label: () => $t('custom.device_details.shadowStatusPending'), value: 'pending' },
  { label: () => $t('custom.device_details.shadowStatusDelivered'), value: 'delivered' },
  { label: () => $t('custom.device_details.shadowStatusExpired'), value: 'expired' },
  { label: () => $t('custom.device_details.shadowStatusCanceled'), value: 'canceled' },
  { label: () => $t('custom.device_details.shadowStatusAll'), value: '' }
]

function statusLabelOf(status: string) {
  const found = statusOptions.find(opt => opt.value === status)
  return found ? found.label() : status
}

function statusTagType(status: string): 'warning' | 'success' | 'default' | 'error' {
  switch (status) {
    case 'pending':
      return 'warning'
    case 'delivered':
      return 'success'
    case 'expired':
      return 'default'
    default:
      return 'error'
  }
}

const query = reactive({ status: 'pending' })

async function getTableData() {
  loading.value = true
  try {
    const { data, error } = await deviceShadowList(props.id, { status: query.status })
    if (!error) {
      tableData.value = data?.list || []
      counts.value = data?.counts || {}
    }
  } finally {
    loading.value = false
  }
}

function formatTime(value?: string) {
  return value ? dayjs(value).format('YYYY-MM-DD HH:mm:ss') : '--'
}

async function handleCancel(msgId: string) {
  const { error } = await deviceShadowCancel(props.id, msgId)
  if (!error) {
    window.$message?.success($t('common.operationSuccess'))
    getTableData()
  }
}

const columns: DataTableColumns<any> = [
  {
    title: () => $t('custom.device_details.messageId'),
    key: 'id',
    width: 290,
    ellipsis: { tooltip: true }
  },
  {
    title: () => $t('custom.device_details.shadowMessageType'),
    key: 'message_type',
    width: 100
  },
  {
    title: () => $t('custom.device_details.sendContent'),
    key: 'payload',
    ellipsis: { tooltip: true },
    render: row => row.payload || '{}'
  },
  {
    title: () => $t('custom.device_details.twinStatus'),
    key: 'status',
    width: 100,
    render: row =>
      h(NTag, { type: statusTagType(row.status), size: 'small' }, { default: () => statusLabelOf(row.status) })
  },
  {
    title: () => $t('custom.device_details.shadowCreatedAt'),
    key: 'created_at',
    width: 170,
    render: row => formatTime(row.created_at)
  },
  {
    title: () => $t('custom.device_details.shadowExpiresAt'),
    key: 'expires_at',
    width: 170,
    render: row => formatTime(row.expires_at)
  },
  {
    title: () => $t('common.actions'),
    key: 'actions',
    width: 100,
    render: row => {
      if (row.status !== 'pending') return null
      return h(
        NPopconfirm,
        { onPositiveClick: () => handleCancel(row.id) },
        {
          trigger: () =>
            h(NButton, { size: 'small', quaternary: true, type: 'error' }, { default: () => $t('common.cancel') }),
          default: () => $t('custom.device_details.shadowCancelConfirm')
        }
      )
    }
  }
]

const showCreateModal = ref(false)
const creating = ref(false)
const createForm = reactive({
  message_type: 'command',
  payload: '{\n  "method": "set",\n  "params": {}\n}',
  ttl_seconds: 86400
})
const messageTypeOptions = [
  { label: $t('custom.device_details.shadowTypeCommand'), value: 'command' },
  { label: $t('custom.device_details.shadowTypeProperty'), value: 'property' },
  { label: $t('custom.device_details.shadowTypeNotification'), value: 'notification' }
]

async function submitCreate() {
  let payloadValue: unknown
  try {
    payloadValue = JSON.parse(createForm.payload)
  } catch {
    window.$message?.error($t('custom.device_details.shadowPayloadInvalid'))
    return
  }
  creating.value = true
  try {
    const { data, error } = await deviceShadowSet(props.id, {
      message_type: createForm.message_type,
      payload: payloadValue,
      ttl_seconds: createForm.ttl_seconds
    })
    if (!error) {
      window.$message?.success(data?.direct ? $t('custom.device_details.shadowDirectSent') : $t('custom.device_details.shadowQueued'))
      showCreateModal.value = false
      query.status = 'pending'
      getTableData()
    }
  } finally {
    creating.value = false
  }
}

onMounted(getTableData)
</script>

<template>
  <div class="shadow-panel">
    <n-alert type="info" :show-icon="false" class="shadow-intro">
      {{ $t('custom.device_details.shadowQueueIntro') }}
    </n-alert>

    <div class="shadow-toolbar">
      <n-radio-group v-model:value="query.status" @update:value="getTableData">
        <n-radio-button
          v-for="opt in statusOptions"
          :key="opt.value"
          :label="`${opt.label()}${counts[opt.value] ? ` (${counts[opt.value]})` : ''}`"
          :value="opt.value"
        />
      </n-radio-group>
      <n-button type="primary" @click="showCreateModal = true">
        {{ $t('custom.device_details.shadowNew') }}
      </n-button>
    </div>

    <n-data-table
      :columns="columns"
      :data="tableData"
      :loading="loading"
      :bordered="false"
      size="small"
    >
      <template #empty>
        <NEmpty :description="$t('common.noData')" class="py-24px" />
      </template>
    </n-data-table>

    <n-modal
      v-model:show="showCreateModal"
      preset="dialog"
      :title="$t('custom.device_details.shadowNew')"
      :positive-text="$t('custom.device_details.shadowSubmit')"
      :negative-text="$t('common.cancel')"
      :loading="creating"
      @positive-click="submitCreate"
    >
      <n-form label-placement="left" label-width="90">
        <n-form-item :label="$t('custom.device_details.shadowMessageType')">
          <n-select v-model:value="createForm.message_type" :options="messageTypeOptions" />
        </n-form-item>
        <n-form-item :label="$t('custom.device_details.shadowPayload')">
          <n-input
            v-model:value="createForm.payload"
            type="textarea"
            :autosize="{ minRows: 5, maxRows: 12 }"
            :placeholder="$t('custom.device_details.shadowPayloadHint')"
          />
        </n-form-item>
        <n-form-item :label="$t('custom.device_details.shadowTTL')">
          <n-input-number v-model:value="createForm.ttl_seconds" :min="60" :max="604800" :step="60">
            <template #suffix>{{ $t('custom.device_details.shadowTTLUnit') }}</template>
          </n-input-number>
        </n-form-item>
      </n-form>
    </n-modal>
  </div>
</template>

<style scoped>
.shadow-panel {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.shadow-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
</style>
