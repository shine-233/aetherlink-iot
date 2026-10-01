<!--
  文件用途：设备详情页中的“诊断与调试”面板（薄壳）。
  结构：
  1. `useDeviceDiagnosticsStats`：拉取上下行/存储成功率与最近失败记录，显式 loading/ready/empty/error 状态；
  2. `useDeviceDebugConsole`：调试开关读写 + 日志轮询（setTimeout 链、开启 3s/关闭 15s、失败指数退避、
     页面隐藏暂停、卸载释放）；
  3. `DeviceDiagnosticsOverview` / `DeviceDebugConsole`：两个纯展示子块；
  4. 本组件只负责组合两份状态，派生“诊断时间线 / 下一步建议 / 支持摘要”，并处理刷新与复制。
  使用注意：
  1. 诊断数据为空不等于设备健康，只表示当前接口没有返回可解释的统计结果；
  2. 调试模式会持续记录设备通信报文，存在额外存储、隐私与性能成本，不应长期默认开启。
-->
<script setup lang="ts">
import { computed } from 'vue'

import { $t } from '@/locales'
import { writeClipboardText } from '@/utils/clipboard'
import DeviceDebugConsole from './DeviceDebugConsole.vue'
import DeviceDiagnosticsOverview from './DeviceDiagnosticsOverview.vue'
import { useDeviceDebugConsole } from './useDeviceDebugConsole'
import {
  buildDiagnosticSupportSummary,
  buildDiagnosticTimeline,
  resolveDiagnosticNextSteps,
  useDeviceDiagnosticsStats
} from './useDeviceDiagnosticsStats'

// `id` 是当前设备 ID，也是诊断查询与调试日志读写的统一主键；切换设备时两个 composable 会自动重置。
const props = defineProps<{
  id: string
}>()

const deviceId = () => props.id

const {
  statistics,
  failureRecords,
  status: diagnosticsStatus,
  error: diagnosticsError,
  refresh: fetchDiagnostics
} = useDeviceDiagnosticsStats(deviceId)

const {
  logEnabled,
  logSwitching,
  statusError,
  debugLogEntries,
  debugLogs,
  logsError,
  fetchLogs,
  refreshStatus: getLogStatus,
  handleLogSwitch
} = useDeviceDebugConsole(deviceId)

const diagnosticTimeline = computed(() => buildDiagnosticTimeline(failureRecords.value, debugLogEntries.value))

const diagnosticNextSteps = computed(() =>
  resolveDiagnosticNextSteps({
    diagnosticsStatus: diagnosticsStatus.value,
    logEnabled: logEnabled.value,
    logCount: debugLogs.value.length
  })
)

// 摘要只在复制时生成，时间戳即复制时刻；不做成 computed，避免缓存住过期的“生成时间”。
const buildSupportSummary = () =>
  buildDiagnosticSupportSummary({
    deviceId: props.id,
    logEnabled: logEnabled.value,
    statistics: statistics.value,
    failureRecords: failureRecords.value,
    timeline: diagnosticTimeline.value,
    nextSteps: diagnosticNextSteps.value
  })

const copyDiagnosticSupportSummary = async () => {
  const copied = await writeClipboardText(buildSupportSummary())
  if (copied) {
    window.$message?.success($t('custom.device_details.diagnosticSummaryCopied'))
  } else {
    window.$message?.warning($t('common.copyFailed'))
  }
}

// 手动刷新：诊断统计、调试开关状态、调试日志三者各拉一次；不重建轮询器。
const refresh = () => {
  void fetchDiagnostics()
  void getLogStatus()
  void fetchLogs()
}
</script>

<template>
  <div>
    <DeviceDiagnosticsOverview
      :statistics="statistics"
      :failure-records="failureRecords"
      :status="diagnosticsStatus"
      :error="diagnosticsError"
      :next-steps="diagnosticNextSteps"
      @refresh="refresh"
    />
    <DeviceDebugConsole
      :log-enabled="logEnabled"
      :log-switching="logSwitching"
      :debug-logs="debugLogs"
      :timeline="diagnosticTimeline"
      :next-steps="diagnosticNextSteps"
      :logs-error="logsError"
      :status-error="statusError"
      @update:log-enabled="handleLogSwitch"
      @copy-summary="copyDiagnosticSupportSummary"
    />
  </div>
</template>
