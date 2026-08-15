<!--
设备命令下发记录面板，负责展示当前设备的命令下发历史，并把“新增命令下发/期望消息”入口透传给公共表格组件。
核心链路：父层传入设备 ID，这里只维护命令状态映射、表格列定义和接口句柄；真正的命令弹窗、提交请求、历史查询与期望消息创建都委托给公共 `DistributionAndTable` 组件执行。
使用注意：
1. 命令下发会直接作用到真实设备，这里虽然是详情页子模块，但暴露的是具备真实副作用的发送入口。
2. 当前通过 `expect=true` 和 `expect-api` 同时接入期望消息能力，命令下发与期望消息实际上共用同一套公共交互壳层；若两条链路后续分叉，公共组件契约需要同步拆分。
3. 列表展示的是平台记录到的下发过程，不等于设备业务执行结果的强一致审计；排障时仍要结合设备状态、回执与日志一起判断。
静态审查建议：
1. 状态映射目前硬编码在页面内，后续可抽成设备详情公共枚举，避免命令、期望消息、调试页各自维护。
2. `data`、`error_message` 仍按纯文本展示，复杂命令 payload 或长错误栈的可读性一般，适合补详情抽屉、格式化查看和复制入口。
3. 业务职责大量下沉到公共组件后，本文件本身很轻，但边界较隐式；后续适合在公共组件层补更明确的 props 契约说明。
-->
<script setup lang="ts">
import { computed, h, onMounted, ref } from 'vue'
import dayjs from 'dayjs'
import { NButton } from 'naive-ui'
import DistributionAndTable from '@/views/device/details/modules/public/distribution-and-table.vue'
import type { CommandSubmitTracking } from '@/views/device/details/modules/public/useDistributionSubmitFlow'
import {
  commandDataPub,
  expectMessageAdd,
  getCommandDataSetLogs,
  getCommandDeliveryDiagnostics,
  invokeDirectMethod,
  type CommandDeliveryDiagnostics,
  type DirectMethodResult
} from '@/service/api'
import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'

const props = defineProps<{
  id: string
}>()

type CommandDeliveryRecord = {
  identify?: string
  identify_name?: string
  message_id?: string
  created_at?: string
  status?: string | number
  data?: unknown
  rsp_data?: unknown
  error_message?: string
}

const detailVisible = ref(false)
const detailRecord = ref<CommandDeliveryRecord | null>(null)
const diagnostics = ref<CommandDeliveryDiagnostics | null>(null)
const diagnosticsLoading = ref(false)
const diagnosticsError = ref('')
const directMethodResult = ref<DirectMethodResult | null>(null)

const emptyValue = '--'

const safeText = (value: unknown) => {
  if (value === undefined || value === null || value === '') return emptyValue
  if (typeof value === 'string') return value
  return JSON.stringify(value)
}

const formatMaybeJson = (value: unknown) => {
  const text = safeText(value)
  if (text === emptyValue) return text

  try {
    return JSON.stringify(JSON.parse(text), null, 2)
  } catch {
    return text
  }
}

// 命令日志接口返回的是后端状态码，这里先在详情页层翻译为可本地化展示的文案，
// 避免表格渲染直接感知 service 的原始枚举细节。
// 如果下游新增“发送中/超时/取消”等状态，优先先补这里，再评估是否抽公共常量。
const formatStatus = (status: string | number) => {
  const normalizedStatus = String(status ?? '')
  const statusjson: Record<string, string> = {
    '0': $t('custom.device_details.commandStatusPending'),
    '1': $t('custom.device_details.commandStatusSent'),
    '2': $t('custom.device_details.commandStatusSendFailed'),
    '3': $t('custom.device_details.commandStatusDeviceSuccess'),
    '4': $t('custom.device_details.commandStatusDeviceFailed')
  }

  return (
    statusjson[normalizedStatus] || `${$t('custom.device_details.commandStatusUnknown')}: ${normalizedStatus || '--'}`
  )
}

const statusKeyByBackendLabel: Record<string, string> = {
  pending: 'custom.device_details.commandStatusPending',
  sent: 'custom.device_details.commandStatusSent',
  send_failed: 'custom.device_details.commandStatusSendFailed',
  device_ack_success: 'custom.device_details.commandStatusDeviceSuccess',
  device_ack_failed: 'custom.device_details.commandStatusDeviceFailed'
}

const diagnosticsType = computed(() => {
  const level = diagnostics.value?.conclusion?.level
  if (level === 'ok') return 'success'
  if (level === 'error') return 'error'
  if (level === 'warning') return 'warning'
  return 'info'
})

const diagnosticsStatusText = computed(() => {
  const latestLog = diagnostics.value?.latest_log
  if (!latestLog) return $t('custom.device_details.commandDiagnosticsNoLog')
  const statusKey = statusKeyByBackendLabel[latestLog.status_label || '']
  return statusKey ? $t(statusKey) : formatStatus(latestLog.status)
})

const diagnosticsSummary = computed(() => {
  if (diagnosticsError.value) return diagnosticsError.value
  return diagnostics.value?.conclusion?.summary || $t('custom.device_details.commandDiagnosticsLoading')
})

const diagnosticsActions = computed(() => diagnostics.value?.conclusion?.next_actions?.filter(Boolean) || [])

const directMethodResultType = computed(() => {
  if (directMethodResult.value?.outcome === 'device_succeeded') return 'success'
  if (directMethodResult.value?.outcome === 'timeout') return 'warning'
  return 'error'
})

const directMethodOutcomeText = computed(() => {
  const outcome = directMethodResult.value?.outcome
  const keys: Partial<Record<DirectMethodResult['outcome'], string>> = {
    awaiting_response: 'custom.device_details.directMethodOutcomeAwaiting',
    device_succeeded: 'custom.device_details.directMethodOutcomeSucceeded',
    device_failed: 'custom.device_details.directMethodOutcomeDeviceFailed',
    delivery_failed: 'custom.device_details.directMethodOutcomeDeliveryFailed',
    timeout: 'custom.device_details.directMethodOutcomeTimeout'
  }
  return outcome && keys[outcome] ? $t(keys[outcome] as string) : emptyValue
})

const commandDiagnosticsSupportBundle = computed(() => {
  const currentDiagnostics = diagnostics.value
  const latestLog = currentDiagnostics?.latest_log
  const recentLogs = currentDiagnostics?.recent_logs || []
  const channels = currentDiagnostics?.confirmation_channels || []
  const evidence = currentDiagnostics?.conclusion?.evidence || []
  const latestDirectMethod = directMethodResult.value

  return [
    '# AetherLink 命令下发支持包',
    '',
    '## 设备',
    `deviceId=${props.id || '<empty>'}`,
    `generatedAt=${dayjs().format('YYYY-MM-DD HH:mm:ss')}`,
    `evaluatedAt=${currentDiagnostics?.evaluated_at || '<unknown>'}`,
    `online=${currentDiagnostics?.is_online ?? '<unknown>'}`,
    `deviceStatus=${currentDiagnostics?.device_status ?? '<unknown>'}`,
    '',
    '## 最新命令',
    `messageId=${latestLog?.message_id || '<none>'}`,
    `identifier=${latestLog?.identify || '<none>'}`,
    `status=${latestLog ? diagnosticsStatusText.value : '<none>'}`,
    `statusLabel=${latestLog?.status_label || '<none>'}`,
    `createdAt=${latestLog?.created_at || '<none>'}`,
    `requestPayload=${safeText(latestLog?.data)}`,
    `responsePayload=${safeText(latestLog?.response_data)}`,
    `error=${safeText(latestLog?.error_message)}`,
    '',
    '## 最近一次即时在线命令',
    `messageId=${latestDirectMethod?.message_id || '<none>'}`,
    `outcome=${latestDirectMethod?.outcome || '<none>'}`,
    `status=${latestDirectMethod?.status || '<none>'}`,
    `published=${latestDirectMethod?.published ?? '<unknown>'}`,
    `deviceResponded=${latestDirectMethod?.device_responded ?? '<unknown>'}`,
    `timedOut=${latestDirectMethod?.timed_out ?? '<unknown>'}`,
    `timeoutSeconds=${latestDirectMethod?.timeout_seconds ?? '<unknown>'}`,
    `elapsedMs=${latestDirectMethod?.elapsed_ms ?? '<unknown>'}`,
    `responsePayload=${safeText(latestDirectMethod?.response_payload)}`,
    `error=${safeText(latestDirectMethod?.error_message)}`,
    '',
    '## 诊断',
    `level=${currentDiagnostics?.conclusion?.level || '<unknown>'}`,
    `code=${currentDiagnostics?.conclusion?.code || '<unknown>'}`,
    `summary=${diagnosticsSummary.value}`,
    `nextActions=${diagnosticsActions.value.length ? diagnosticsActions.value.join(' | ') : '<none>'}`,
    '',
    '## 证据',
    evidence.length ? evidence.map((item, index) => `${index + 1}. ${item}`).join('\n') : '<none>',
    '',
    '## 确认通道',
    channels.length
      ? channels
          .map((channel, index) => `${index + 1}. ${channel.code} / ${channel.label}: ${channel.description}`)
          .join('\n')
      : '<none>',
    '',
    '## 近期命令日志',
    recentLogs.length
      ? recentLogs
          .slice(0, 5)
          .map((log, index) =>
            `${index + 1}. ${[log.created_at, log.message_id, log.identify, log.status_label, log.error_message]
              .filter(Boolean)
              .join(' / ')}`
          )
          .join('\n')
      : '<none>',
    '',
    '## 证据边界',
    '本支持包汇总 AetherLink 返回的平台命令日志、设备响应证据和诊断结果。除非设备明确上报执行结果，否则它不能单独证明设备侧业务动作已经完成。'
  ].join('\n')
})

const copyCommandDiagnosticsSupportBundle = async () => {
  if (!diagnostics.value && !diagnosticsError.value) {
    window.$message?.warning($t('custom.device_details.commandDiagnosticsBundleRefreshFirst'))
    return
  }
  const copied = await writeClipboardText(commandDiagnosticsSupportBundle.value)
  window.$message?.[copied ? 'success' : 'error'](
    copied ? $t('custom.device_details.commandDiagnosticsBundleCopied') : $t('common.copyFailed')
  )
}

const fetchCommandDiagnostics = async () => {
  diagnosticsLoading.value = true
  diagnosticsError.value = ''
  try {
    const response = await getCommandDeliveryDiagnostics(props.id, { limit: 5 })
    diagnostics.value = response?.data || (response as any) || null
  } catch {
    diagnostics.value = null
    diagnosticsError.value = $t('custom.device_details.commandDiagnosticsLoadFailed')
  } finally {
    diagnosticsLoading.value = false
  }
}

const handleSubmittedTracking = async (_tracking: CommandSubmitTracking) => {
  await fetchCommandDiagnostics()
}

const handleDirectMethodResult = (result: DirectMethodResult) => {
  directMethodResult.value = result
}

const copyDirectMethodResult = async () => {
  if (!directMethodResult.value) return
  const copied = await writeClipboardText(JSON.stringify(directMethodResult.value, null, 2))
  window.$message?.[copied ? 'success' : 'error'](
    copied ? $t('custom.device_details.commandDetailCopySuccess') : $t('custom.device_details.commandDetailCopyFailed')
  )
}

const openDetail = (row: CommandDeliveryRecord) => {
  detailRecord.value = row
  detailVisible.value = true
}

const copyDetailText = async (value: unknown) => {
  const text = formatMaybeJson(value)
  if (text === emptyValue) {
    window.$message?.warning($t('custom.device_details.commandDetailCopyEmpty'))
    return
  }
  const copied = await writeClipboardText(text)
  window.$message?.[copied ? 'success' : 'error'](
    copied ? $t('custom.device_details.commandDetailCopySuccess') : $t('custom.device_details.commandDetailCopyFailed')
  )
}

const detailRows = () => {
  const row = detailRecord.value
  if (!row) return []

  return [
    {
      label: $t('device_template.table_header.commandIdentifier'),
      value: safeText(row.identify)
    },
    {
      label: $t('device_template.table_header.commandName'),
      value: safeText(row.identify_name)
    },
    {
      label: 'Message ID',
      value: safeText(row.message_id)
    },
    {
      label: $t('generate.commandIssuanceTime'),
      value: row.created_at ? dayjs(row.created_at).format('YYYY-MM-DD HH:mm:ss') : emptyValue
    },
    {
      label: $t('generate.status'),
      value: formatStatus(row.status ?? '')
    }
  ]
}

// 表格聚焦“发了什么命令、什么时候发、当前状态如何、是否有错误”这几类运维最常看的字段。
// 真正的查询条件、分页和下发弹窗都交给公共组件维护，这里只保留设备详情域自己的列语义。
const columns = [
  {
    title: $t('device_template.table_header.commandIdentifier'),
    minWidth: '140px',
    key: 'identify'
  },
  {
    title: $t('device_template.table_header.commandName'),
    minWidth: '140px',
    key: 'identify_name',
    render: (row) => row.identify_name || '--'
  },
  {
    title: 'Message ID',
    minWidth: '180px',
    key: 'message_id',
    render: (row) => row.message_id || '--'
  },
  {
    title: $t('generate.commandIssuanceTime'),
    minWidth: '140px',
    key: 'created_at',
    // 下发时间与设备事件、属性历史统一使用相同格式，便于人工比对前后因果。
    render: (row) => dayjs(row.created_at).format('YYYY-MM-DD HH:mm:ss')
  },
  {
    title: $t('generate.status'),
    minWidth: '140px',
    key: 'status',
    render: (row) => formatStatus(row.status)
  },
  { title: $t('generate.commandConetnt'), minWidth: '140px', key: 'data' },
  {
    title: $t('custom.device_details.commandResponsePayload'),
    minWidth: '180px',
    key: 'rsp_data',
    render: (row) => row.rsp_data || '--'
  },
  {
    title: $t('generate.errorMessage'),
    minWidth: '140px',
    render: (row) => row.error_message || '--'
  },
  {
    title: $t('common.actions'),
    minWidth: '120px',
    key: 'actions',
    render: (row: CommandDeliveryRecord) =>
      h(
        NButton,
        {
          size: 'small',
          secondary: true,
          onClick: () => openDetail(row)
        },
        { default: () => $t('custom.device_details.commandDetailAction') }
      )
  }
]

onMounted(() => {
  fetchCommandDiagnostics()
})
</script>

<template>
  <div>
    <div class="command-diagnostics-section">
      <div class="command-diagnostics-header">
        <div>
          <div class="command-diagnostics-title">
            {{ $t('custom.device_details.commandDiagnosticsTitle') }}
          </div>
          <div class="command-diagnostics-subtitle">
            {{ $t('custom.device_details.commandDiagnosticsSubtitle') }}
          </div>
        </div>
        <NSpace size="small">
          <NButton size="small" secondary :loading="diagnosticsLoading" @click="fetchCommandDiagnostics">
            {{ $t('generate.refresh') }}
          </NButton>
          <NButton size="small" secondary type="primary" @click="copyCommandDiagnosticsSupportBundle">
            {{ $t('custom.device_details.commandDiagnosticsCopyBundle') }}
          </NButton>
        </NSpace>
      </div>

      <NAlert :type="diagnosticsType" :show-icon="false">
        {{ diagnosticsSummary }}
      </NAlert>

      <div class="command-diagnostics-grid">
        <div class="command-diagnostics-item">
          <span>{{ $t('custom.device_details.commandDiagnosticsOnline') }}</span>
          <strong>
            {{
              diagnostics
                ? diagnostics.is_online
                  ? $t('custom.device_details.commandDiagnosticsOnlineYes')
                  : $t('custom.device_details.commandDiagnosticsOnlineNo')
                : emptyValue
            }}
          </strong>
        </div>
        <div class="command-diagnostics-item">
          <span>Message ID</span>
          <strong>{{ diagnostics?.latest_log?.message_id || emptyValue }}</strong>
        </div>
        <div class="command-diagnostics-item">
          <span>{{ $t('generate.status') }}</span>
          <strong>{{ diagnosticsStatusText }}</strong>
        </div>
      </div>

      <ul v-if="diagnosticsActions.length" class="command-diagnostics-actions">
        <li v-for="action in diagnosticsActions" :key="action">{{ action }}</li>
      </ul>
    </div>

    <NAlert v-if="directMethodResult" :type="directMethodResultType" :show-icon="false" class="direct-method-result">
      <div class="direct-method-result__header">
        <strong>{{ $t('custom.device_details.directMethodResultTitle') }} · {{ directMethodOutcomeText }}</strong>
        <NButton size="tiny" secondary @click="copyDirectMethodResult">
          {{ $t('custom.device_details.commandDetailCopy') }}
        </NButton>
      </div>
      <div class="command-diagnostics-grid">
        <div class="command-diagnostics-item">
          <span>Message ID</span>
          <strong>{{ directMethodResult.message_id }}</strong>
        </div>
        <div class="command-diagnostics-item">
          <span>{{ $t('generate.status') }}</span>
          <strong>{{ formatStatus(directMethodResult.status) }}</strong>
        </div>
        <div class="command-diagnostics-item">
          <span>{{ $t('custom.device_details.directMethodElapsed') }}</span>
          <strong>{{ directMethodResult.elapsed_ms }} ms / {{ directMethodResult.timeout_seconds }} s</strong>
        </div>
      </div>
      <div v-if="directMethodResult.response_payload" class="direct-method-result__payload">
        <strong>{{ $t('custom.device_details.commandResponsePayload') }}</strong>
        <pre class="command-detail-code">{{ formatMaybeJson(directMethodResult.response_payload) }}</pre>
      </div>
      <div v-if="directMethodResult.error_message" class="direct-method-result__payload">
        <strong>{{ $t('generate.errorMessage') }}</strong>
        <pre class="command-detail-code command-detail-code--error">{{
          formatMaybeJson(directMethodResult.error_message)
        }}</pre>
      </div>
    </NAlert>

    <DistributionAndTable
      :id="props.id as string"
      :button-name="$t('generate.issueCommand')"
      :is-command="true"
      :table-columns="columns"
      :fetch-data-api="getCommandDataSetLogs"
      :submit-api="commandDataPub"
      :direct-method-api="invokeDirectMethod"
      :direct-method-online="diagnostics?.is_online"
      :expect="true"
      :expect-api="expectMessageAdd"
      :on-direct-method-result="handleDirectMethodResult"
      :on-submitted-tracking="handleSubmittedTracking"
    />
    <NDrawer v-model:show="detailVisible" width="min(720px, calc(100vw - 32px))" placement="right">
      <NDrawerContent :title="$t('custom.device_details.commandDetailTitle')" closable>
        <NAlert type="info" class="mb-4" :show-icon="false">
          {{ $t('custom.device_details.commandDetailAuditHint') }}
        </NAlert>

        <NDescriptions bordered :column="1" size="small" class="mb-4">
          <NDescriptionsItem v-for="item in detailRows()" :key="item.label" :label="item.label">
            {{ item.value }}
          </NDescriptionsItem>
        </NDescriptions>

        <NSpace vertical size="medium">
          <NCard size="small" :title="$t('custom.device_details.commandDetailRequestPayload')">
            <template #header-extra>
              <NButton size="tiny" secondary @click="copyDetailText(detailRecord?.data)">
                {{ $t('custom.device_details.commandDetailCopy') }}
              </NButton>
            </template>
            <pre class="command-detail-code">{{ formatMaybeJson(detailRecord?.data) }}</pre>
          </NCard>

          <NCard size="small" :title="$t('custom.device_details.commandDetailResponsePayload')">
            <template #header-extra>
              <NButton size="tiny" secondary @click="copyDetailText(detailRecord?.rsp_data)">
                {{ $t('custom.device_details.commandDetailCopy') }}
              </NButton>
            </template>
            <pre class="command-detail-code">{{ formatMaybeJson(detailRecord?.rsp_data) }}</pre>
          </NCard>

          <NCard size="small" :title="$t('generate.errorMessage')">
            <template #header-extra>
              <NButton size="tiny" secondary @click="copyDetailText(detailRecord?.error_message)">
                {{ $t('custom.device_details.commandDetailCopy') }}
              </NButton>
            </template>
            <pre class="command-detail-code command-detail-code--error">{{
              formatMaybeJson(detailRecord?.error_message)
            }}</pre>
          </NCard>
        </NSpace>
      </NDrawerContent>
    </NDrawer>
  </div>
</template>

<style scoped>
.command-detail-code {
  overflow: auto;
  max-height: 260px;
  margin: 0;
  padding: 12px;
  border-radius: 8px;
  background: #0f172a;
  color: #e2e8f0;
  font-size: 12px;
  line-height: 1.6;
  white-space: pre-wrap;
  word-break: break-word;
}

.command-detail-code--error {
  background: #2a1215;
  color: #ffd6d6;
}

.direct-method-result {
  margin-bottom: 16px;
}

.direct-method-result__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.direct-method-result__payload {
  margin-top: 12px;
}

.direct-method-result__payload strong {
  display: block;
  margin-bottom: 6px;
}

.command-diagnostics-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 16px;
  padding: 14px;
  border: 1px solid #d9e3f0;
  border-radius: 8px;
  background: #f8fafc;
}

.command-diagnostics-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.command-diagnostics-title {
  color: #0f172a;
  font-size: 15px;
  font-weight: 600;
}

.command-diagnostics-subtitle {
  margin-top: 4px;
  color: #64748b;
  font-size: 12px;
  line-height: 1.5;
}

.command-diagnostics-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
}

.command-diagnostics-item {
  min-width: 0;
  padding: 10px 12px;
  border: 1px solid #e2e8f0;
  border-radius: 6px;
  background: #fff;
}

.command-diagnostics-item span {
  display: block;
  color: #64748b;
  font-size: 12px;
}

.command-diagnostics-item strong {
  display: block;
  overflow-wrap: anywhere;
  margin-top: 4px;
  color: #111827;
  font-size: 13px;
  font-weight: 600;
}

.command-diagnostics-actions {
  margin: 0;
  padding-left: 18px;
  color: #334155;
  font-size: 12px;
  line-height: 1.6;
}

@media (max-width: 720px) {
  .command-diagnostics-header {
    flex-direction: column;
  }

  .command-diagnostics-grid {
    grid-template-columns: 1fr;
  }
}
</style>
