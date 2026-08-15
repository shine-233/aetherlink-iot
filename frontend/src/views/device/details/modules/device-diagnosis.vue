<!--
  文件用途：设备详情页中的“诊断与调试”面板。
  核心链路：
  1. 调用 `deviceDiagnostics` 拉取设备上下行/存储成功率与最近失败记录；
  2. 调用 `getDeviceDebugStatus`、`setDeviceDebugStatus` 管理设备调试日志开关；
  3. 通过 `getDeviceDebugLogs` 周期轮询调试日志，在页面内提供现场排障视图。
  使用注意：
  1. 诊断数据为空不等于设备健康，只表示当前接口没有返回可解释的统计结果；
  2. 调试模式会持续记录设备通信报文，存在额外存储、隐私与性能成本，不应长期默认开启；
  3. 当前日志轮询周期为 3 秒，设备很多或日志量很大时要留意前后端压力。
  静态审查建议：
  1. 诊断统计、失败记录与调试日志目前共处一个组件，后续适合拆成“诊断概览”和“调试控制台”两个子块；
  2. `fetchDiagnostics` 与日志查询都只在本地吞错，缺少更明确的空态/异常态提示，适合补可观测反馈；
  3. 调试日志与诊断状态的轮询/刷新时序较隐式，后续可收敛为 composable，减少页面脚本体积。
-->
<script setup lang="ts">
import { computed, h, onMounted, onUnmounted, nextTick, ref } from 'vue'
import dayjs from 'dayjs'

import { $t } from '@/locales'
import { Refresh, HelpCircleOutline } from '@vicons/ionicons5'
import type { DataTableColumns } from 'naive-ui'
import { deviceDiagnostics, getDeviceDebugStatus, setDeviceDebugStatus, getDeviceDebugLogs } from '@/service/api'
import { writeClipboardText } from '@/utils/clipboard'

// 诊断统计卡片使用统一的数据结构，避免模板层区分接口返回中的可选字段。
interface StatisticsItem {
  success: number
  total: number
  rate: number
}

// 统计数据类型
interface Statistics {
  uplink: StatisticsItem
  downlink: StatisticsItem
  storage: StatisticsItem
}

// 失败记录类型
interface FailureRecord {
  timestamp: string
  direction: 'uplink' | 'downlink'
  stage: string
  error: string
}

interface DebugLogEntry {
  ts?: string | number
  event?: string
  stage?: string
  diagnostic_code?: string
  error?: string
  message?: string
  meta?: Record<string, unknown>
  [key: string]: unknown
}

interface DiagnosticTimelineItem {
  time: string
  title: string
  detail: string
  nextAction: string
  type: 'error' | 'info'
}

// 诊断统计数据类型
interface DiagnosticsStatsItem {
  success?: number
  total?: number
  success_rate?: number
}

// 诊断统计数据类型
interface DiagnosticsStats {
  uplink?: DiagnosticsStatsItem
  downlink?: DiagnosticsStatsItem
  storage?: DiagnosticsStatsItem
}

// 诊断数据类型
interface DiagnosticsData {
  stats?: DiagnosticsStats
  recent_failures?: Array<{
    timestamp?: string | number
    direction?: 'uplink' | 'downlink'
    stage?: string
    error?: string
  }>
}

// 诊断响应类型
interface DiagnosticsResponse {
  data?: DiagnosticsData
}

type DiagnosticsApiResponse = DiagnosticsResponse | DiagnosticsData

type DiagnosticsFailure = NonNullable<DiagnosticsData['recent_failures']>[number]

type DiagnosticsFetchState =
  | {
      status: 'ready'
      data: DiagnosticsData
    }
  | {
      status: 'empty'
    }
  | {
      status: 'error'
      error: unknown
    }

// `id` 是当前设备 ID，也是诊断查询与调试日志读写的统一主键。
const props = defineProps<{
  id: string
}>()

// 统计卡片默认展示为 0，等待接口返回后再整体替换。
const statistics = ref<Statistics>({
  uplink: {
    success: 0,
    total: 0,
    rate: 0
  },
  downlink: {
    success: 0,
    total: 0,
    rate: 0
  },
  storage: {
    success: 0,
    total: 0,
    rate: 0
  }
})

// 最近失败记录只做只读展示，不在前端维护编辑态。
const failureRecords = ref<FailureRecord[]>([])
const diagnosticsFetchState = ref<DiagnosticsFetchState>({ status: 'empty' })

// 表格聚焦“何时失败、哪一段失败、为什么失败”这三类排障最关键的信息。
const columns: DataTableColumns<FailureRecord> = [
  {
    title: $t('custom.device_details.time'),
    key: 'timestamp',
    width: 200,
    render: (row: FailureRecord) => {
      if (row.timestamp) {
        return dayjs(row.timestamp).format('YYYY-MM-DD HH:mm:ss')
      }
      return '--'
    }
  },
  {
    title: $t('custom.device_details.direction'),
    key: 'direction',
    width: 150,
    render: (row: FailureRecord) => {
      const direction =
        row.direction === 'uplink' ? $t('custom.device_details.uplink') : $t('custom.device_details.downlink')
      return h('span', {}, { default: () => direction })
    }
  },
  {
    title: $t('custom.device_details.phase'),
    key: 'stage',
    width: 200
  },
  {
    title: $t('custom.device_details.errorDescription'),
    key: 'error',
    ellipsis: {
      tooltip: true
    }
  }
]

const requestDiagnostics = async (deviceId: string): Promise<DiagnosticsApiResponse> => {
  return (await deviceDiagnostics(deviceId)) as DiagnosticsApiResponse
}

// 诊断接口历史上可能返回 `{ data }` 包装，也可能直接返回主体对象，这里先在页面层归一化。
const normalizeDiagnosticsResponse = (response: DiagnosticsApiResponse | null | undefined): DiagnosticsData | null => {
  const data = (response as DiagnosticsResponse | undefined)?.data || (response as DiagnosticsData | null | undefined)

  if (!data?.stats) {
    return null
  }

  return data
}

const mapDiagnosticsProgress = (statsItem?: DiagnosticsStatsItem): StatisticsItem => ({
  success: statsItem?.success ?? 0,
  total: statsItem?.total ?? 0,
  rate: statsItem?.success_rate ?? 0
})

const mapDiagnosticsStats = (stats: DiagnosticsStats): Statistics => ({
  uplink: mapDiagnosticsProgress(stats.uplink),
  downlink: mapDiagnosticsProgress(stats.downlink),
  storage: mapDiagnosticsProgress(stats.storage)
})

const mapDiagnosticsFailureRecord = (failure: DiagnosticsFailure): FailureRecord => ({
  timestamp: String(failure.timestamp ?? ''),
  direction: failure.direction ?? 'uplink',
  stage: failure.stage ?? '',
  error: failure.error ?? ''
})

const mapDiagnosticsFailureRecords = (failures?: DiagnosticsData['recent_failures']): FailureRecord[] => {
  if (!Array.isArray(failures)) {
    return []
  }

  return failures.map(mapDiagnosticsFailureRecord)
}

const mapDiagnosticsReadyState = (data: DiagnosticsData | null): DiagnosticsFetchState => {
  if (!data) {
    return { status: 'empty' }
  }

  return {
    status: 'ready',
    data
  }
}

const mapDiagnosticsErrorState = (error: unknown): DiagnosticsFetchState => ({
  status: 'error',
  error
})

// 目前只有 ready 态会真正改 UI；empty/error 仍保持旧值，避免接口抖动时整块面板闪空。
const applyDiagnosticsUiState = (state: DiagnosticsFetchState) => {
  diagnosticsFetchState.value = state

  if (state.status !== 'ready') {
    return
  }

  statistics.value = mapDiagnosticsStats(state.data.stats ?? {})
  failureRecords.value = mapDiagnosticsFailureRecords(state.data.recent_failures)
}

// 诊断数据刷新入口。
// 这里不抛出错误给上层，而是把失败收敛为本地状态，保持详情页其他 tab 不受影响。
const fetchDiagnostics = async () => {
  try {
    const response = await requestDiagnostics(props.id)
    const data = normalizeDiagnosticsResponse(response)
    applyDiagnosticsUiState(mapDiagnosticsReadyState(data))
  } catch (error) {
    applyDiagnosticsUiState(mapDiagnosticsErrorState(error))
  }
}

// 手动刷新同时刷新诊断统计和调试开关状态，但不会强制重建轮询器。
const refresh = () => {
  fetchDiagnostics()
  getLogStatus()
}

// 调试日志相关状态。
const DEBUG_LOG_FETCH_LIMIT = 100
const DEBUG_LOG_POLL_INTERVAL_MS = 3000
const DIAGNOSIS_SUMMARY_LOG_LIMIT = 8
const sensitiveDiagnosticKeyPattern = /(password|passwd|secret|token|credential|private|key|cert|authorization|cookie)/i
const logEnabled = ref(false)
const debugLogs = ref<string[]>([])
const debugLogEntries = ref<DebugLogEntry[]>([])
let logTimer: NodeJS.Timeout | null = null
const logContainerRef = ref<HTMLElement | null>(null)

const formatSupportStat = (label: string, item: StatisticsItem) => {
  return `${label}: ${item.success}/${item.total} (${item.rate.toFixed(1)}%)`
}

const sanitizeDiagnosticValue = (value: unknown): unknown => {
  if (Array.isArray(value)) {
    return value.map((item) => sanitizeDiagnosticValue(item))
  }

  if (!value || typeof value !== 'object') {
    return value
  }

  return Object.fromEntries(
    Object.entries(value as Record<string, unknown>).map(([key, item]) => [
      key,
      sensitiveDiagnosticKeyPattern.test(key) ? '***' : sanitizeDiagnosticValue(item)
    ])
  )
}

const maskSensitiveDiagnosticText = (value: unknown) => {
  return String(value ?? '')
    .replace(/("(?:password|passwd|pwd|token|secret|voucher|authorization|cookie|key|cert)"\s*:\s*)"[^"]*"/gi, '$1"***"')
    .replace(/\b(password|passwd|pwd|token|secret|voucher|authorization|cookie|key|cert)=([^\s,;]+)/gi, '$1=***')
}

const formatTimelineTime = (time: string | number | undefined) => {
  if (!time) return '--'
  const parsed = dayjs(time)
  return parsed.isValid() ? parsed.format('YYYY-MM-DD HH:mm:ss') : String(time)
}

const getFailureNextAction = (item: FailureRecord) => {
  if (item.direction === 'uplink') {
    return $t('custom.device_details.nextActionUplink')
  }

  return $t('custom.device_details.nextActionDownlink')
}

const getDebugLogTitle = (item: DebugLogEntry) => {
  return String(item.diagnostic_code || item.event || item.stage || 'debug_log')
}

const getDebugLogDetail = (item: DebugLogEntry) => {
  return maskSensitiveDiagnosticText(item.error || item.message || item.stage || JSON.stringify(sanitizeDiagnosticValue(item)))
}

const getDebugLogNextAction = (item: DebugLogEntry) => {
  const code = String(item.diagnostic_code || item.event || item.stage || '').toLowerCase()
  if (code.includes('auth')) return $t('custom.device_details.nextActionAuth')
  if (code.includes('topic')) return $t('custom.device_details.nextActionTopic')
  if (code.includes('disconnect')) return $t('custom.device_details.nextActionDisconnect')
  if (code.includes('payload') || code.includes('parse')) return $t('custom.device_details.nextActionPayload')
  return $t('custom.device_details.nextActionDefault')
}

const diagnosticTimeline = computed<DiagnosticTimelineItem[]>(() => {
  const failures = failureRecords.value.slice(0, DIAGNOSIS_SUMMARY_LOG_LIMIT).map(item => ({
    time: formatTimelineTime(item.timestamp),
    title: `${item.direction || 'unknown'} / ${item.stage || 'unknown stage'}`,
    detail: maskSensitiveDiagnosticText(item.error || 'No error detail returned'),
    nextAction: getFailureNextAction(item),
    type: 'error' as const
  }))

  const logs = debugLogEntries.value.slice(0, DIAGNOSIS_SUMMARY_LOG_LIMIT).map(item => ({
    time: formatTimelineTime(item.ts),
    title: getDebugLogTitle(item),
    detail: getDebugLogDetail(item),
    nextAction: getDebugLogNextAction(item),
    type: 'info' as const
  }))

  return [...failures, ...logs]
})

const diagnosticNextSteps = computed(() => {
  if (diagnosticsFetchState.value.status === 'error') {
    return [
      $t('custom.device_details.nextStepErrorRefresh'),
      $t('custom.device_details.nextStepErrorReadyCheck')
    ]
  }
  if (!logEnabled.value) {
    return [
      $t('custom.device_details.nextStepEnableDebug'),
      $t('custom.device_details.nextStepDisableDebugAfter')
    ]
  }
  if (debugLogs.value.length === 0) {
    return [
      $t('custom.device_details.nextStepDebugNoLogs'),
      $t('custom.device_details.nextStepCheckEndpoint')
    ]
  }
  return [$t('custom.device_details.nextStepReviewLatest'), $t('custom.device_details.nextStepCopySummary')]
})

const diagnosticSupportSummary = computed(() => {
  const lines = [
    $t('custom.device_details.summaryTitle'),
    `${$t('custom.device_details.summaryGeneratedAt')}: ${dayjs().format('YYYY-MM-DD HH:mm:ss')}`,
    `${$t('custom.device_details.summaryDeviceId')}: ${props.id || '--'}`,
    `${$t('custom.device_details.summaryDebugMode')}: ${logEnabled.value ? 'enabled' : 'disabled'}`,
    $t('custom.device_details.summarySensitiveNote'),
    '',
    $t('custom.device_details.summaryStatsSection'),
    formatSupportStat('uplink', statistics.value.uplink),
    formatSupportStat('downlink', statistics.value.downlink),
    formatSupportStat('storage', statistics.value.storage),
    '',
    $t('custom.device_details.summaryFailuresSection'),
    `${$t('custom.device_details.summaryCount')}: ${failureRecords.value.length}`
  ]

  if (failureRecords.value.length === 0) {
    lines.push(`- ${$t('custom.device_details.summaryNoFailures')}`)
  } else {
    failureRecords.value.slice(0, DIAGNOSIS_SUMMARY_LOG_LIMIT).forEach(item => {
      lines.push(
        `- ${formatTimelineTime(item.timestamp)} ${item.direction || 'unknown'} ${item.stage || 'unknown'} ${maskSensitiveDiagnosticText(item.error)}`
      )
    })
  }

  lines.push('', $t('custom.device_details.summaryTimelineSection'))
  if (diagnosticTimeline.value.length === 0) {
    lines.push(`- ${$t('custom.device_details.diagnosticEvidenceEmpty')}`)
  } else {
    diagnosticTimeline.value.forEach(item => {
      lines.push(
        `- ${item.time} [${item.type}] ${item.title}; ${item.detail}; ${$t('custom.device_details.nextStepLabel')}: ${item.nextAction}`
      )
    })
  }

  lines.push('', $t('custom.device_details.summaryNextStepsSection'))
  diagnosticNextSteps.value.forEach(step => {
    lines.push(`- ${step}`)
  })

  return lines.join('\n')
})

const copyDiagnosticSupportSummary = async () => {
  const copied = await writeClipboardText(diagnosticSupportSummary.value)
  if (copied) {
    window.$message?.success($t('custom.device_details.diagnosticSummaryCopied'))
  } else {
    window.$message?.warning($t('common.copyFailed'))
  }
}

// 调试日志目前直接把单条日志对象 JSON 化后输出到类终端窗口中，便于原样排障。
const formatDebugLogEntry = (item: DebugLogEntry) => {
  const time = item.ts ? dayjs(item.ts).format('YYYY-MM-DD HH:mm:ss.SSS') : ''
  return `[${time}] ${maskSensitiveDiagnosticText(JSON.stringify(sanitizeDiagnosticValue(item)))}`
}

const mapDebugLogsForConsole = (items: DebugLogEntry[] = []) => {
  return [...items].reverse().map(formatDebugLogEntry)
}

const scrollDebugLogsToBottom = () => {
  nextTick(() => {
    if (logContainerRef.value) {
      logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
    }
  })
}

// 日志开关状态由后端真实配置决定，避免页面本地状态和设备实际调试模式漂移。
const getLogStatus = async () => {
  try {
    const res = await getDeviceDebugStatus(props.id)
    if (res.data) {
      logEnabled.value = res.data.enabled ?? false
    }
  } catch (e) {
    console.error(e)
  }
}

// 开关切换失败时回滚本地 UI，保持视觉状态和真实服务端状态一致。
const handleLogSwitch = async (value: boolean) => {
  try {
    await setDeviceDebugStatus(props.id, { enabled: value })
    logEnabled.value = value
  } catch (e) {
    console.error(e)
    // 恢复开关状态
    logEnabled.value = !value
  }
}

// 日志轮询每次都全量取最近 N 条，再在前端反转顺序供控制台展示。
const fetchLogs = async () => {
  try {
    const res = await getDeviceDebugLogs(props.id, { limit: DEBUG_LOG_FETCH_LIMIT })
    if (res.data && res.data.list) {
      debugLogEntries.value = res.data.list as DebugLogEntry[]
      debugLogs.value = mapDebugLogsForConsole(debugLogEntries.value)
      scrollDebugLogsToBottom()
    }
  } catch (e) {
    console.error(e)
  }
}

// 页面进入即启动日志轮询；当前没有根据开关状态暂停轮询，后续可评估是否优化为按需拉取。
const startLogPolling = () => {
  if (logTimer) return
  fetchLogs() // 立即执行一次
  logTimer = setInterval(fetchLogs, DEBUG_LOG_POLL_INTERVAL_MS)
}

// 组件卸载时必须释放定时器，避免切换设备或离开页面后继续轮询旧设备日志。
const stopLogPolling = () => {
  if (logTimer) {
    clearInterval(logTimer)
    logTimer = null
  }
}

onMounted(() => {
  fetchDiagnostics()
  getLogStatus()
  startLogPolling()
})

onUnmounted(() => {
  stopLogPolling()
})
</script>

<template>
  <div>
    <!-- 统计概览部分 -->
    <div class="mb-4">
      <div class="flex items-center justify-between">
        <div class="text-18px">{{ $t('custom.device_details.statisticsOverview') }}</div>
        <NButton :bordered="false" @click="refresh">
          <NIcon size="18">
            <Refresh />
          </NIcon>
          {{ $t('common.refresh') }}
        </NButton>
      </div>
      <!-- <div class="text-14px text-gray-500">
        {{ $t('custom.device_details.statisticsRange') }}
      </div> -->
      <NFlex :gap="16" class="mt-4">
        <!-- 上行成功率卡片 -->
        <NCard class="flex-1" :title="$t('custom.device_details.uplinkSuccessRate')">
          <NFlex vertical :gap="8">
            <NText type="info" class="text-28px font-bold">
              <NNumberAnimation :from="0" :to="statistics.uplink.rate" :precision="1" />
              <span>%</span>
            </NText>
            <NText :depth="2" class="text-14px">{{ statistics.uplink.success }}/{{ statistics.uplink.total }}{{ $t('custom.device_details.diagnosisCountUnit') }}</NText>
          </NFlex>
        </NCard>

        <!-- 下行成功率卡片 -->
        <NCard class="flex-1" :title="$t('custom.device_details.downlinkSuccessRate')">
          <NFlex vertical :gap="8">
            <NText type="success" class="text-28px font-bold">
              <NNumberAnimation :from="0" :to="statistics.downlink.rate" :precision="1" />
              <span>%</span>
            </NText>
            <NText :depth="2" class="text-14px">
              {{ statistics.downlink.success }}/{{ statistics.downlink.total }}{{ $t('custom.device_details.diagnosisCountUnit') }}
            </NText>
          </NFlex>
        </NCard>

        <!-- 存储成功率卡片 -->
        <NCard class="flex-1" :title="$t('custom.device_details.storageSuccessRate')">
          <NFlex vertical :gap="8">
            <NText type="warning" class="text-28px font-bold">
              <NNumberAnimation :from="0" :to="statistics.storage.rate" :precision="1" />
              <span>%</span>
            </NText>
            <NText :depth="2" class="text-14px">
              {{ statistics.storage.success }}/{{ statistics.storage.total }}{{ $t('custom.device_details.diagnosisCountUnit') }}
            </NText>
          </NFlex>
        </NCard>
      </NFlex>
    </div>

    <!-- 最近失败记录部分 -->
    <div>
      <div class="flex items-center justify-between mb-4">
        <div class="text-18px">
          {{ $t('custom.device_details.recentFailureRecords') }}
        </div>
      </div>

      <NAlert v-if="failureRecords.length === 0" type="info" class="mb-3" :title="$t('custom.device_details.diagnosisNoFailureRecords')">
        <div class="text-13px leading-6">
          <div v-for="step in diagnosticNextSteps" :key="step">- {{ step }}</div>
        </div>
      </NAlert>
      <NDataTable :columns="columns" :data="failureRecords" :max-height="350" remote />
    </div>

    <!-- 设备调试日志部分 -->
    <div class="mt-4">
      <div class="flex items-center justify-between mb-4">
        <div class="text-18px">{{ $t('custom.device_details.diagnosisDebugLog') }}</div>
        <div class="flex items-center gap-2">
          <NButton size="small" secondary type="primary" @click="copyDiagnosticSupportSummary">
            {{ $t('custom.device_details.diagnosisCopySummary') }}
          </NButton>
          <NTooltip trigger="hover">
            <template #trigger>
              <div class="flex items-center gap-1 cursor-help">
                <span>{{ $t('custom.device_details.diagnosisDebugMode') }}</span>
                <NIcon size="14" class="text-gray-400">
                  <HelpCircleOutline />
                </NIcon>
              </div>
            </template>
            {{ $t('custom.device_details.diagnosisDebugModeHint') }}
          </NTooltip>
          <NSwitch :value="logEnabled" @update:value="handleLogSwitch" />
        </div>
      </div>

      <NCard class="mb-4" size="small" :title="$t('custom.device_details.diagnosisTimelineTitle')">
        <NEmpty v-if="diagnosticTimeline.length === 0" :description="$t('custom.device_details.diagnosticEvidenceEmpty')">
          <template #extra>
            <div class="text-left text-13px leading-6 text-gray-500">
              <div v-for="step in diagnosticNextSteps" :key="step">- {{ step }}</div>
            </div>
          </template>
        </NEmpty>
        <NTimeline v-else>
          <NTimelineItem
            v-for="(item, index) in diagnosticTimeline"
            :key="`${item.title}-${index}`"
            :type="item.type"
            :time="item.time"
            :title="item.title"
          >
            <div class="whitespace-pre-wrap break-all text-12px">{{ item.detail }}</div>
            <div class="mt-1 text-12px text-gray-500">{{ $t('custom.device_details.diagnosisNextStep') }}{{ item.nextAction }}</div>
          </NTimelineItem>
        </NTimeline>
      </NCard>

      <div
        ref="logContainerRef"
        class="bg-[#1e1e1e] text-[#d4d4d4] font-mono p-4 rounded h-[400px] overflow-auto whitespace-pre-wrap break-all text-xs"
      >
        <div v-if="debugLogs.length === 0" class="text-center text-gray-500 py-10">{{ $t('custom.device_details.diagnosisNoLogs') }}</div>
        <div
          v-for="(log, index) in debugLogs"
          :key="index"
          class="mb-1 border-b border-gray-700/50 pb-1 last:border-0 hover:bg-[#2a2d2e]"
        >
          {{ log }}
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped lang="scss">
:deep(.n-card-header) {
  font-size: 16px;
}
</style>
