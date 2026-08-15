import type {
  DeviceAccessGuideDiagnosticsSummary,
  DeviceAccessGuideState
} from '@/views/device/details/modules/device-access-guide-state'
import { firstDeviceCredentialState as credentialState } from './homeFirstDeviceSuccessProof'
import type {
  FirstDeviceBrowserTestState,
  FirstDeviceChartState,
  FirstDeviceDeploymentHealthRow,
  FirstDeviceOnboardingGuard,
  FirstDeviceReadyProof,
  FirstDeviceSummary,
  FirstDeviceSupportTestCommand,
  SimulationInitState
} from './homeFirstDeviceWorkbench'

export type FirstDeviceSupportSummaryOptions = {
  generatedAt?: Date
  device: FirstDeviceSummary | null
  accessGuide: DeviceAccessGuideState | null
  diagnostics?: DeviceAccessGuideDiagnosticsSummary | null
  simulation: SimulationInitState | null
  readyProof: FirstDeviceReadyProof
  latestProofText: string
  browserTest?: FirstDeviceBrowserTestState
  testResult: string
  chart: FirstDeviceChartState
  activeTestCommand?: FirstDeviceSupportTestCommand | null
  onboardingGuard: FirstDeviceOnboardingGuard
  deploymentHealthRows: FirstDeviceDeploymentHealthRow[]
  delivery?: {
    firstDeviceUrl?: string
    proofUrl?: string
    proofFileHint?: string
  }
}

const formatSupportValue = (value: unknown, fallback = '未取得') => {
  if (value === undefined || value === null || value === '') return fallback
  return String(value)
}

export const buildFirstDeviceSupportSummary = (options: FirstDeviceSupportSummaryOptions): string => {
  const device = options.device
  const accessGuide = options.accessGuide
  const diagnostics = options.diagnostics || {}
  const blocker = options.readyProof.items.find((item) => !item.ok)
  const failedHealth = options.deploymentHealthRows.find((row) => !row.ok)
  const proofRows = options.readyProof.items
    .map((item) => `- ${item.label}: ${item.ok ? '通过' : '待处理'}；${item.detail}`)
    .join('\n')
  const healthRows = options.deploymentHealthRows
    .map((row) =>
      [
        `- ${row.label}: ${row.ok ? `正常 ${row.latency || '--'}ms` : row.error || '异常'}`,
        !row.ok && row.nextAction ? `  下一步: ${row.nextAction}` : ''
      ]
        .filter(Boolean)
        .join('\n')
    )
    .join('\n')

  return [
    'AetherLink IoT 首台设备接入支持摘要',
    `生成时间: ${(options.generatedAt || new Date()).toLocaleString()}`,
    '',
    '[当前结论]',
    `状态: ${options.readyProof.ready ? '设备已准备好' : '仍需处理'}`,
    `当前阻塞: ${blocker ? `${blocker.label} - ${blocker.detail}` : '无'}`,
    `最新证据: ${options.latestProofText}`,
    `下一步: ${options.onboardingGuard.nextAction}`,
    `首次接入入口: ${formatSupportValue(options.delivery?.firstDeviceUrl, '/first-device')}`,
    `证明区入口: ${formatSupportValue(options.delivery?.proofUrl, '/home?onboarding=first-device&focus=proof')}`,
    `建议证明文件: ${formatSupportValue(options.delivery?.proofFileHint, 'aetherlink-first-device-proof-<device>.json')}`,
    '',
    '[设备]',
    `名称: ${formatSupportValue(device?.name)}`,
    `编号: ${formatSupportValue(device?.number)}`,
    `ID: ${formatSupportValue(device?.id)}`,
    `在线: ${device?.online ? '在线' : '未在线/未知'}`,
    `物模型: ${formatSupportValue(device?.configName, '未绑定')}`,
    '',
    '[接入参数]',
    `协议: ${formatSupportValue(accessGuide?.protocol, 'MQTT')}`,
    `端点类型: ${formatSupportValue(accessGuide?.endpointKind)}`,
    `认证方式: ${formatSupportValue(accessGuide?.authMode)}`,
    `端点: ${formatSupportValue(accessGuide?.endpoint || (options.simulation ? `${options.simulation.server}:${options.simulation.port}` : ''))}`,
    `上报入口: ${formatSupportValue(accessGuide?.endpointKind === 'http' ? accessGuide?.endpoint : accessGuide?.reportTopic || options.simulation?.topic)}`,
    `控制入口: ${formatSupportValue(accessGuide?.controlTopic, '请打开 Ready Check 查看')}`,
    `TLS 提示: ${formatSupportValue(accessGuide?.tlsHintKey)}`,
    `用户名状态: ${credentialState((accessGuide as any)?.username)}`,
    `密码/Token 状态: ${credentialState((accessGuide as any)?.password || (accessGuide as any)?.token)}`,
    `测试命令类型: ${formatSupportValue(options.activeTestCommand?.label, '未加载测试命令')}`,
    `命令占位符: ${options.onboardingGuard.commandHasPlaceholders ? 'yes' : 'no'}`,
    `可复制命令: ${options.onboardingGuard.canCopyCommand ? 'yes' : 'no'}`,
    `可浏览器测试: ${options.onboardingGuard.canRunBrowserTest ? 'yes' : 'no'}`,
    '',
    '[Ready Check / 诊断]',
    `结论: ${formatSupportValue(diagnostics.conclusionSummary || diagnostics.readySummary)}`,
    `结论代码: ${formatSupportValue(diagnostics.conclusionCode || diagnostics.readyCode)}`,
    `最新问题: ${formatSupportValue(diagnostics.latestIssue)}`,
    `下一步建议: ${Array.isArray(diagnostics.readyNextActions) && diagnostics.readyNextActions.length ? diagnostics.readyNextActions.join(' | ') : '未取得'}`,
    `最新遥测字段: ${formatSupportValue(diagnostics.latestTelemetryKey)}`,
    `最新遥测时间: ${formatSupportValue(diagnostics.latestTelemetryAt)}`,
    `部分结果警告: ${Array.isArray(diagnostics.partialWarnings) && diagnostics.partialWarnings.length ? diagnostics.partialWarnings.join(' | ') : '无'}`,
    `调试开关: ${diagnostics.debugEnabled === undefined ? '未知' : diagnostics.debugEnabled ? 'enabled' : 'disabled'}`,
    `最近调试日志数: ${formatSupportValue(diagnostics.recentLogCount)}`,
    '',
    '[浏览器测试]',
    `状态: ${formatSupportValue(options.browserTest?.status)}`,
    `结果: ${formatSupportValue(options.testResult || options.browserTest?.message, '未运行')}`,
    `发送时间: ${formatSupportValue(options.browserTest?.sentAt)}`,
    '',
    '[首张图表]',
    `状态: ${options.chart.ready ? '已生成' : '未生成'}`,
    `来源: ${options.chart.generatedFrom}`,
    `主值: ${formatSupportValue(options.chart.primaryKey ? `${options.chart.primaryKey} = ${options.chart.primaryValue}` : '')}`,
    `摘要: ${formatSupportValue(options.chart.summary)}`,
    '',
    '[交付证明]',
    proofRows || '未取得证明行',
    '',
    '[部署健康]',
    `优先失败项: ${
      failedHealth
        ? `${failedHealth.label} - ${failedHealth.error || failedHealth.description || '健康检查未通过'}${
            failedHealth.nextAction ? `；下一步: ${failedHealth.nextAction}` : ''
          }`
        : '无'
    }`,
    healthRows || '未取得部署健康行'
  ].join('\n')
}
