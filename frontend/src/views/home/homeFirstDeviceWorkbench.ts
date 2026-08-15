import type { DeviceAccessGuideState } from '@/views/device/details/modules/device-access-guide-state'
import { summarizeDeviceConnectionDiagnostics } from '@/views/device/details/modules/device-connection-diagnostics-state'

export {
  buildFirstDeviceSuccessProofPacket,
  type FirstDeviceSuccessProofPacket
} from './homeFirstDeviceSuccessProof'
export {
  buildFirstDeviceSupportSummary,
  type FirstDeviceSupportSummaryOptions
} from './homeFirstDeviceSupportSummary'
export { buildFirstDeviceOnlineTesterState } from './homeFirstDeviceOnlineTesterState'

export type FirstDeviceSummary = {
  id: string
  name: string
  number: string
  online: boolean
  configId: string
  configName: string
}

export type FirstTelemetryPoint = {
  key: string
  value: string
  ts?: string
}

export type FirstDeviceBrowserTestStatus = 'idle' | 'sending' | 'sent' | 'confirmed' | 'failed'

export type FirstDeviceBrowserTestState = {
  status: FirstDeviceBrowserTestStatus
  message: string
  sentAt?: string
  telemetryKey?: string
  telemetryValue?: string
}

export type FirstDeviceChartSource = 'latest_telemetry' | 'browser_test' | 'none'

export type FirstDeviceChartPoint = FirstTelemetryPoint & {
  barPercent: number
}

export type FirstDeviceChartState = {
  ready: boolean
  title: string
  summary: string
  primaryKey: string
  primaryValue: string
  generatedFrom: FirstDeviceChartSource
  points: FirstDeviceChartPoint[]
}

export type SimulationInitState = {
  server: string
  port: number
  topic: string
  payload: string
}

export const DEFAULT_FIRST_DEVICE_PAYLOAD = '{"temperature":25.5,"humidity":60}'

export const createIdleFirstDeviceBrowserTestState = (): FirstDeviceBrowserTestState => ({
  status: 'idle',
  message: '浏览器在线测试还没有运行。'
})

const unwrapData = (response: any) => response?.data?.data ?? response?.data ?? response ?? {}

const asArray = (value: unknown): any[] => (Array.isArray(value) ? value : [])

export const normalizeFirstDevice = (response: any): FirstDeviceSummary | null => {
  const data = unwrapData(response)
  const list = asArray(data?.list ?? data?.data?.list ?? data?.records)
  const first = list[0]
  if (!first) return null

  const id = String(first.id ?? first.device_id ?? '')
  if (!id) return null

  return {
    id,
    name: String(first.name ?? first.device_name ?? first.device_number ?? '第一台设备'),
    number: String(first.device_number ?? first.number ?? first.name ?? '--'),
    online: Number(first.online ?? first.status ?? first.device_status ?? 0) === 1,
    configId: String(first.device_config_id ?? first.config_id ?? first.device_config?.id ?? ''),
    configName: String(first.device_config_name ?? first.config_name ?? first.device_config?.name ?? '')
  }
}

export const normalizeTelemetryPoints = (response: any): FirstTelemetryPoint[] => {
  const data = unwrapData(response)
  const source = data?.list ?? data?.data ?? data
  const rows = Array.isArray(source)
    ? source
    : Object.entries(source || {}).map(([key, value]) => ({
        key,
        value
      }))

  return rows
    .map((item: any) => {
      const key = String(item.key ?? item.identify ?? item.name ?? '')
      if (!key) return null
      const rawValue = item.value ?? item.val ?? item.data ?? item.y ?? ''
      return {
        key,
        value: typeof rawValue === 'object' ? JSON.stringify(rawValue) : String(rawValue),
        ts: item.ts ?? item.time ?? item.created_at ?? item.updated_at
      }
    })
    .filter(Boolean)
    .slice(0, 5) as FirstTelemetryPoint[]
}

export const normalizeSimulationInit = (response: any): SimulationInitState => {
  const data = unwrapData(response)
  return {
    server: String(data.server || 'localhost'),
    port: Number(data.port || 1883),
    topic: String(data.topic || 'devices/telemetry'),
    payload: String(data.default_data || data.payload || DEFAULT_FIRST_DEVICE_PAYLOAD)
  }
}

export const buildPublishCommand = (simulation: SimulationInitState) => {
  const payload = simulation.payload.replaceAll("'", "'\"'\"'")
  return `mosquitto_pub -h ${simulation.server} -p ${simulation.port} -t "${simulation.topic}" -m '${payload}'`
}

export const isUsableHttpEndpoint = (value: string) => /^https?:\/\//i.test(value.trim())

export const buildHttpTelemetryRequest = (options: { endpoint: string; token?: string; payload: string }) => {
  const headers: Record<string, string> = {
    'content-type': 'application/json'
  }
  const token = options.token?.trim()
  if (token && !token.startsWith('<')) {
    headers.authorization = `Bearer ${token}`
  }

  return {
    url: options.endpoint,
    init: {
      method: 'POST',
      headers,
      body: options.payload
    } satisfies RequestInit
  }
}

export const isFirstDeviceReady = (device: FirstDeviceSummary | null, telemetry: FirstTelemetryPoint[]) => {
  return Boolean(device?.online && telemetry.length > 0)
}

export const buildFirstDeviceBrowserTestState = (options: {
  status: FirstDeviceBrowserTestStatus
  message?: string
  telemetry?: FirstTelemetryPoint | null
  sentAt?: string
}): FirstDeviceBrowserTestState => ({
  status: options.status,
  message:
    options.message ||
    (options.status === 'confirmed'
      ? '浏览器在线测试已被最新遥测确认。'
      : options.status === 'sent'
        ? '浏览器在线测试已发送，正在等待最新遥测。'
        : options.status === 'sending'
          ? '正在发送浏览器测试遥测。'
          : options.status === 'failed'
            ? '浏览器在线测试失败。'
            : '浏览器在线测试还没有运行。'),
  sentAt: options.sentAt,
  telemetryKey: options.telemetry?.key,
  telemetryValue: options.telemetry?.value
})

const toFiniteTelemetryNumber = (value: string) => {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? Math.abs(parsed) : null
}

export const buildFirstDeviceChartState = (
  telemetry: FirstTelemetryPoint[],
  browserTest?: FirstDeviceBrowserTestState
): FirstDeviceChartState => {
  const sourcePoints = telemetry.slice(0, 5)
  const numericValues = sourcePoints
    .map((point) => toFiniteTelemetryNumber(point.value))
    .filter((value): value is number => value !== null)
  const maxValue = Math.max(...numericValues, 0)
  const points = sourcePoints.map((point) => {
    const numericValue = toFiniteTelemetryNumber(point.value)
    const barPercent =
      numericValue !== null && maxValue > 0 ? Math.max(8, Math.round((numericValue / maxValue) * 100)) : 100

    return {
      ...point,
      barPercent
    }
  })
  const confirmedBrowserPoint =
    browserTest?.status === 'confirmed' && browserTest.telemetryKey
      ? {
          key: browserTest.telemetryKey,
          value: browserTest.telemetryValue || '--'
        }
      : null
  const primaryPoint =
    (confirmedBrowserPoint &&
      points.find((point) => point.key === confirmedBrowserPoint.key && point.value === confirmedBrowserPoint.value)) ||
    (confirmedBrowserPoint ? { ...confirmedBrowserPoint, barPercent: 100 } : null) ||
    points[0] ||
    null
  const ready = Boolean(primaryPoint && points.length > 0)
  const generatedFrom: FirstDeviceChartSource = ready
    ? browserTest?.status === 'confirmed'
      ? 'browser_test'
      : 'latest_telemetry'
    : 'none'

  return {
    ready,
    title: ready ? '第一张遥测图表已生成' : '等待生成第一张遥测图表',
    summary: ready
      ? `${primaryPoint?.key || 'telemetry'} = ${primaryPoint?.value || '--'}；图表已使用 ${points.length} 条最新遥测。`
      : '还没有可见的最新遥测；请发送一次浏览器测试或从设备端上报。',
    primaryKey: primaryPoint?.key || '',
    primaryValue: primaryPoint?.value || '',
    generatedFrom,
    points
  }
}

export const summarizeFirstDeviceConnectionDiagnostics = summarizeDeviceConnectionDiagnostics

export type FirstDeviceQuickstartAction = 'health' | 'create' | 'copy' | 'test' | 'ready-check'

export type FirstDeviceQuickstartStep = {
  key: 'health' | 'create' | 'connect' | 'publish' | 'verify'
  title: string
  description: string
  status: 'done' | 'active' | 'todo'
  statusLabel: string
  statusType: 'success' | 'warning' | 'default'
  action: FirstDeviceQuickstartAction
  actionLabel: string
  disabled: boolean
}

export type FirstDeviceOnboardingGuard = {
  commandHasPlaceholders: boolean
  canCopyCommand: boolean
  canRunBrowserTest: boolean
  summary: string
  nextAction: string
  activeStep: FirstDeviceQuickstartStep | null
  steps: FirstDeviceQuickstartStep[]
}

export type FirstDeviceReadyProofItem = {
  key: 'deployment' | 'identity' | 'connection' | 'browser_test' | 'online' | 'telemetry' | 'chart'
  label: string
  ok: boolean
  detail: string
}

export type FirstDeviceReadyProof = {
  ready: boolean
  title: string
  summary: string
  items: FirstDeviceReadyProofItem[]
}

export type FirstDeviceDeploymentHealthRow = {
  key?: string
  label: string
  ok: boolean
  description?: string
  nextAction?: string
  error?: string
  latency?: number | string
}

export type FirstDeviceSupportTestCommand = {
  label: string
}

export type FirstDevicePostReadyStep = {
  id?: string
  title?: string
  description?: string
  action?: string
}

export type FirstDevicePostReadyHandoff = {
  title: string
  description: string
  primaryLabel: string
  secondaryLabel: string
  completionSignal: string
  action: 'next-guide' | 'guide'
  section: 'proof'
}

const PLACEHOLDER_PATTERN = /<[^>]+>|\bundefined\b|\bnull\b|连接参数加载中|loading/i

export type FirstDeviceFlowNodeState = 'done' | 'active' | 'todo'

export type FirstDeviceFlowNode = FirstDeviceReadyProofItem & {
  title: string
  short: string
  state: FirstDeviceFlowNodeState
  stateLabel: string
  stateType: 'success' | 'warning' | 'default'
}

export type FirstDeviceClosureSummary = {
  ready: boolean
  doneCount: number
  totalCount: number
  remainingCount: number
  percent: number
  statusLabel: string
  nextTitle: string
  nextDetail: string
  completionSignal: string
}

export type FirstDeviceOnlineTesterState = {
  type: 'success' | 'warning' | 'info' | 'error'
  statusLabel: string
  title: string
  description: string
  actionLabel: string
  disabledReason: string
  lastSignal: string
  echoRows: Array<{ label: string; value: string }>
}

export type FirstDevicePostTestGuidance = {
  type: 'success' | 'warning'
  title: string
  detail: string
}

export type FirstDeviceVerificationAction = {
  type: 'success' | 'warning'
  title: string
  detail: string
  label: string
  secondaryLabel: string
  action: 'simulate' | 'ready-check' | 'guide' | 'next-guide' | 'proof'
  section: 'test' | 'connection' | 'proof'
  loading: boolean
  disabled: boolean
}

const FIRST_DEVICE_FLOW_NODE_META: Record<FirstDeviceReadyProofItem['key'], { title: string; short: string }> = {
  deployment: { title: '部署可用', short: 'API / DB / MQTT' },
  identity: { title: '设备身份', short: '已创建设备' },
  connection: { title: '连接参数', short: '端点 / Topic' },
  browser_test: { title: '发送测试遥测', short: '浏览器测试' },
  online: { title: '在线状态', short: '设备在线' },
  telemetry: { title: '最新遥测', short: '收到数据' },
  chart: { title: '首张图表', short: '可视化证明' }
}

export const buildFirstDeviceFlowNodes = (items: FirstDeviceReadyProofItem[] = []): FirstDeviceFlowNode[] => {
  const firstBlockedIndex = items.findIndex((item) => !item.ok)

  return items.map((item, index) => {
    const meta = FIRST_DEVICE_FLOW_NODE_META[item.key] || { title: item.label, short: item.label }
    const state: FirstDeviceFlowNodeState = item.ok ? 'done' : index === firstBlockedIndex ? 'active' : 'todo'

    return {
      ...item,
      title: meta.title,
      short: meta.short,
      state,
      stateLabel: state === 'done' ? '已通过' : state === 'active' ? '当前卡点' : '待处理',
      stateType: state === 'done' ? 'success' : state === 'active' ? 'warning' : 'default'
    }
  })
}

export const buildFirstDeviceClosureSummary = (nodes: FirstDeviceFlowNode[] = []): FirstDeviceClosureSummary => {
  const totalCount = nodes.length
  const doneCount = nodes.filter((node) => node.ok).length
  const remainingCount = Math.max(totalCount - doneCount, 0)
  const nextNode = nodes.find((node) => node.state === 'active') || nodes.find((node) => !node.ok) || null
  const ready = totalCount > 0 && remainingCount === 0

  return {
    ready,
    doneCount,
    totalCount,
    remainingCount,
    percent: totalCount > 0 ? Math.round((doneCount / totalCount) * 100) : 0,
    statusLabel: ready ? '设备已准备好' : remainingCount > 0 ? `还差 ${remainingCount} 项` : '等待首设备证据',
    nextTitle: ready ? '可以交付这台设备' : nextNode?.title || '等待首页确认下一步',
    nextDetail: ready
      ? '在线状态、最新遥测和首张图表都已经可见，可以继续做自动化、看板或批量设备。'
      : nextNode?.detail || '刷新首页后继续按橙色卡点处理，直到所有证明项变绿。',
    completionSignal: ready
      ? '完成标准：右侧闭环画布全部为绿色，并且首图证明可以复制给客户或支持。'
      : '完成标准：部署、设备、连接、上报、在线、遥测、图表全部通过。'
  }
}

export const resolveFirstDeviceFocusedSectionKey = (options: {
  activeStep?: Pick<FirstDeviceQuickstartStep, 'key'> | null
  ready: boolean
  readyProofItems?: FirstDeviceReadyProofItem[]
  chartReady: boolean
}) => {
  const key = options.activeStep?.key
  if (key === 'health') return 'deployment'
  if (key === 'create') return 'device'
  if (key === 'connect') return 'connection'
  if (key === 'publish') return 'test'
  if (key === 'verify') {
    return options.chartReady
      ? 'proof'
      : options.readyProofItems?.find((item) => !item.ok)?.key === 'chart'
        ? 'chart'
        : 'proof'
  }
  return options.ready ? 'proof' : 'quickstart'
}

export const buildFirstDevicePostTestGuidance = (options: {
  testResult: string
  ready: boolean
  readyDescription: string
  chartReady: boolean
  currentBlocker?: Pick<FirstDeviceReadyProofItem, 'label'> | null
}): FirstDevicePostTestGuidance | null => {
  if (!options.testResult) return null
  if (options.ready) {
    return {
      type: 'success',
      title: '闭环已确认',
      detail: options.readyDescription
    }
  }
  if (options.chartReady) {
    return {
      type: 'warning',
      title: '测试遥测已产生数据，继续看最终证明',
      detail: options.currentBlocker
        ? `当前还差：${options.currentBlocker.label}`
        : '图表已经出现，继续检查右侧证明项是否全部变绿。'
    }
  }
  return {
    type: 'warning',
    title: '测试已发送，等待可见证据',
    detail: options.currentBlocker
      ? `当前卡点：${options.currentBlocker.label}。如果长时间不变，请打开 Ready Check 或复制支持摘要。`
      : '首页正在等待最新遥测；如果没有更新，请查看 Ready Check 的连接诊断。'
  }
}

export const buildFirstDeviceVerificationAction = (options: {
  hasDevice: boolean
  ready: boolean
  postReadyHandoff?: FirstDevicePostReadyHandoff | null
  readyDescription: string
  chartReady: boolean
  canRunBrowserTest: boolean
  testResult: string
  actionLoading: boolean
  currentBlocker?: Pick<FirstDeviceReadyProofItem, 'label' | 'detail'> | null
}): FirstDeviceVerificationAction | null => {
  if (!options.hasDevice) return null
  if (options.ready) {
    const handoff = options.postReadyHandoff
    return {
      type: 'success',
      title: handoff?.title || '设备已准备好',
      detail: handoff?.description || options.readyDescription,
      label: handoff?.primaryLabel || '查看完整指南',
      secondaryLabel: handoff?.secondaryLabel || '定位成功证明',
      action: handoff?.action || 'guide',
      section: handoff?.section || 'proof',
      loading: false,
      disabled: false
    }
  }
  if (!options.chartReady && options.canRunBrowserTest) {
    return {
      type: 'warning',
      title: options.testResult ? '继续确认遥测并生成首图' : '下一步：发送测试遥测并生成首图',
      detail: options.testResult
        ? '测试已经发出，仍可再次发送一条测试遥测；首页会继续刷新最新遥测，并在收到数据后生成第一张图表。'
        : '设备已经创建，现在最该做的是点一次浏览器在线测试，让平台收到第一条遥测并自动生成首图。',
      label: options.testResult ? '再次发送并确认' : '发送测试遥测并确认',
      secondaryLabel: '定位测试区',
      action: 'simulate',
      section: 'test',
      loading: options.actionLoading,
      disabled: false
    }
  }
  if (!options.chartReady) {
    return {
      type: 'warning',
      title: '下一步：补齐参数再验证',
      detail: options.currentBlocker
        ? `当前还差：${options.currentBlocker.label}。先打开 Ready Check 看诊断，或回到参数区补齐端点、凭证和 topic。`
        : '设备存在，但暂时还不能直接发送浏览器测试；先确认连接参数和 Ready Check 诊断。',
      label: '打开 Ready Check',
      secondaryLabel: '定位参数区',
      action: 'ready-check',
      section: 'connection',
      loading: false,
      disabled: false
    }
  }
  return {
    type: 'warning',
    title: '首图已生成，继续确认最终证明',
    detail: options.currentBlocker
      ? `首张图表已经出来，当前还差：${options.currentBlocker.label}。把右侧证明项处理完，首页就会显示“设备已准备好”。`
      : '首张图表已经出来，继续查看证明项是否全部变绿。',
    label: '查看证明项',
    secondaryLabel: '复制首图证明',
    action: 'proof',
    section: 'proof',
    loading: false,
    disabled: false
  }
}

export const hasConnectionPlaceholder = (value: string) => PLACEHOLDER_PATTERN.test(value.trim())

const quickstartStep = (
  options: Omit<FirstDeviceQuickstartStep, 'statusLabel' | 'statusType'> & {
    status: FirstDeviceQuickstartStep['status']
  }
): FirstDeviceQuickstartStep => ({
  ...options,
  statusLabel: options.status === 'done' ? '已完成' : options.status === 'active' ? '现在做' : '待处理',
  statusType: options.status === 'done' ? 'success' : options.status === 'active' ? 'warning' : 'default'
})

export const buildFirstDeviceOnboardingGuard = (options: {
  device: FirstDeviceSummary | null
  telemetry: FirstTelemetryPoint[]
  accessGuide: DeviceAccessGuideState | null
  publishCommand: string
  actionLoading?: boolean
  deploymentHealthy?: boolean
}): FirstDeviceOnboardingGuard => {
  const deploymentHealthy = options.deploymentHealthy !== false
  const hasDevice = Boolean(options.device)
  const hasTelemetry = options.telemetry.length > 0
  const commandHasPlaceholders = !options.publishCommand || hasConnectionPlaceholder(options.publishCommand)
  const endpointUsable =
    options.accessGuide?.endpointKind === 'http' ? isUsableHttpEndpoint(options.accessGuide.endpoint) : hasDevice
  const canCopyCommand = hasDevice && Boolean(options.publishCommand) && !commandHasPlaceholders
  const canRunBrowserTest =
    deploymentHealthy &&
    hasDevice &&
    Boolean(options.publishCommand) &&
    endpointUsable &&
    !commandHasPlaceholders &&
    !options.actionLoading
  const ready = deploymentHealthy && isFirstDeviceReady(options.device, options.telemetry)
  const activeKey = !deploymentHealthy
    ? 'health'
    : !hasDevice
      ? 'create'
      : !canCopyCommand
        ? 'connect'
        : !hasTelemetry
          ? 'publish'
          : !ready
            ? 'verify'
            : ''

  const steps = [
    quickstartStep({
      key: 'health',
      title: '0. 检查部署健康',
      description: deploymentHealthy
        ? '前端、API、数据库、Redis 和 MQTT Broker 已经可以支撑首台设备接入。'
        : '先确认前端、API、数据库、Redis 和 MQTT Broker 都正常，再创建或测试第一台设备。',
      status: deploymentHealthy ? 'done' : 'active',
      action: 'health',
      actionLabel: '检查部署健康',
      disabled: false
    }),
    quickstartStep({
      key: 'create',
      title: '1. 生成第一台设备',
      description: hasDevice
        ? `已找到 ${options.device?.name || options.device?.number || '第一台设备'}`
        : '一键创建产品、物模型、MQTT/HTTP 配置和第一台设备，也可以手动添加。',
      status: hasDevice ? 'done' : activeKey === 'create' ? 'active' : 'todo',
      action: 'create',
      actionLabel: hasDevice ? '定位设备信息' : '一键生成',
      disabled: !deploymentHealthy
    }),
    quickstartStep({
      key: 'connect',
      title: '2. 拿到可用连接参数',
      description: canCopyCommand
        ? '命令里已经没有占位符，可以复制到设备端或终端测试。'
        : '连接参数还不完整，请打开 Ready Check 查看缺少的端点、凭证或 topic。',
      status: canCopyCommand ? 'done' : activeKey === 'connect' ? 'active' : 'todo',
      action: canCopyCommand ? 'copy' : 'ready-check',
      actionLabel: canCopyCommand ? '复制命令' : '打开 Ready Check',
      disabled: !deploymentHealthy || !hasDevice
    }),
    quickstartStep({
      key: 'publish',
      title: '3. 发一条遥测',
      description: hasTelemetry
        ? `已收到 ${options.telemetry[0]?.key || 'telemetry'} 的最新数据。`
        : '复制命令到真实设备，或直接点浏览器在线测试发送一条测试遥测。',
      status: hasTelemetry ? 'done' : activeKey === 'publish' ? 'active' : 'todo',
      action: 'test',
      actionLabel: '浏览器在线测试',
      disabled: !canRunBrowserTest
    }),
    quickstartStep({
      key: 'verify',
      title: '4. 确认设备真的可交付',
      description: ready
        ? '设备在线且已有最新遥测，可以继续做告警、自动化和看板。'
        : '进入 Ready Check 看在线、最新遥测、命令回包和下一步修复建议。',
      status: ready ? 'done' : activeKey === 'verify' ? 'active' : 'todo',
      action: 'ready-check',
      actionLabel: '打开 Ready Check',
      disabled: !deploymentHealthy || !hasDevice
    })
  ]

  const activeStep = steps.find((step) => step.status === 'active')
  return {
    commandHasPlaceholders,
    canCopyCommand,
    canRunBrowserTest,
    summary: ready
      ? '第一台设备已跑通，可以复制这套流程给更多设备。'
      : activeStep?.description || '按步骤完成第一台设备接入。',
    nextAction: activeStep?.actionLabel || '继续下一步',
    activeStep: activeStep ?? null,
    steps
  }
}

export const buildFirstDeviceReadyProof = (options: {
  device: FirstDeviceSummary | null
  telemetry: FirstTelemetryPoint[]
  accessGuide: DeviceAccessGuideState | null
  publishCommand: string
  deploymentHealthy: boolean
  browserTest?: FirstDeviceBrowserTestState
  chart?: FirstDeviceChartState
}): FirstDeviceReadyProof => {
  const hasDevice = Boolean(options.device?.id)
  const connectionReady = Boolean(options.accessGuide?.endpoint) && !hasConnectionPlaceholder(options.publishCommand)
  const browserTestOk = options.browserTest?.status === 'confirmed'
  const online = Boolean(options.device?.online)
  const hasTelemetry = options.telemetry.length > 0
  const chart = options.chart || buildFirstDeviceChartState(options.telemetry, options.browserTest)
  const firstReportConfirmed = browserTestOk || (online && hasTelemetry && chart.ready)

  const items: FirstDeviceReadyProofItem[] = [
    {
      key: 'deployment',
      label: '部署健康',
      ok: options.deploymentHealthy,
      detail: options.deploymentHealthy
        ? '前端、API、数据库、Redis 和 MQTT 已通过健康检查。'
        : '先处理部署健康里的红色依赖。'
    },
    {
      key: 'identity',
      label: '设备身份',
      ok: hasDevice,
      detail: hasDevice
        ? `${options.device?.name || options.device?.number || '第一台设备'} 已创建。`
        : '还没有可验证的第一台设备。'
    },
    {
      key: 'connection',
      label: '连接参数',
      ok: connectionReady,
      detail: connectionReady
        ? `${options.accessGuide?.protocol || 'MQTT/HTTP'} 参数和发布命令已经可用。`
        : '连接端点、凭证或发布命令仍未完整。'
    },
    {
      key: 'browser_test',
      label: '首条上报确认',
      ok: firstReportConfirmed,
      detail: browserTestOk
        ? `${options.browserTest?.telemetryKey || 'telemetry'} = ${options.browserTest?.telemetryValue || '--'}，测试上报已被最新遥测确认。`
        : firstReportConfirmed
          ? `${options.telemetry[0]?.key || 'telemetry'} = ${options.telemetry[0]?.value || '--'}，真实设备上报已被在线状态、最新遥测和首图确认。`
          : options.browserTest?.message ||
            '点击浏览器在线测试，或让真实设备发送一条测试遥测。'
    },
    {
      key: 'online',
      label: '在线状态',
      ok: online,
      detail: online ? '设备当前在线。' : '还没有看到设备在线。'
    },
    {
      key: 'telemetry',
      label: '最新遥测',
      ok: hasTelemetry,
      detail: hasTelemetry
        ? `已收到 ${options.telemetry[0]?.key || 'telemetry'} = ${options.telemetry[0]?.value || '--'}。`
        : '还没有收到第一条遥测。'
    },
    {
      key: 'chart',
      label: '首张图表',
      ok: chart.ready,
      detail: chart.ready ? chart.summary : '先从最新遥测自动生成第一张图表，再交付给客户。'
    }
  ]
  const ready = items.every((item) => item.ok)

  return {
    ready,
    title: ready ? '设备已准备好' : '设备还没完全准备好',
    summary: ready
      ? '第一台设备已经完成部署、身份、连接、在线和遥测闭环，可以继续配置告警、自动化和看板。'
      : '按下面红色或灰色项继续处理，全部变绿后就是可交付的第一台设备。',
    items
  }
}

export const buildFirstDeviceChartProofSummary = (options: {
  device: any
  chart: Partial<FirstDeviceChartState> | null | undefined
  readyProof: Partial<FirstDeviceReadyProof> | null | undefined
}): string => {
  const chart = options.chart || {}
  const chartPoints = Array.isArray(chart.points) ? chart.points : []
  const readyProof = options.readyProof || {}
  const proofItems = Array.isArray(readyProof.items) ? readyProof.items : []

  return [
    '# AetherLink 首次接入首图证明',
    '',
    '## 设备',
    `id=${options.device?.id || '<未知>'}`,
    `name=${options.device?.name || '<未知>'}`,
    `number=${options.device?.device_number || options.device?.number || '<未知>'}`,
    `online=${Boolean(options.device?.online)}`,
    '',
    '## 首图证明',
    `ready=${Boolean(chart.ready)}`,
    `source=${chart.generatedFrom === 'browser_test' ? '浏览器在线测试' : '最新遥测'}`,
    `primary=${chart.primaryKey || 'telemetry'}=${chart.primaryValue ?? '--'}`,
    `summary=${chart.summary || '<空>'}`,
    '',
    '## 图表点位',
    chartPoints.length
      ? chartPoints.map((point: any, index: number) => `${index + 1}. ${point.key}=${point.value}`).join('\n')
      : '<暂无>',
    '',
    '## 跑通证明',
    `ready=${Boolean(readyProof.ready)}`,
    `title=${readyProof.title || '<空>'}`,
    `summary=${readyProof.summary || '<空>'}`,
    proofItems.length
      ? proofItems
          .map(
            (item: any, index: number) =>
              `${index + 1}. ${item.label}: ${item.ok ? '已通过' : '待处理'} - ${item.detail || ''}`
          )
          .join('\n')
      : '<暂无>',
    '',
    '## 证据边界',
    '此证明来自首页首次接入流程展示的在线状态、最新遥测和首图结果。若要作为发版门禁，请再结合运行时 API 或端到端验证。'
  ].join('\n')
}

export const buildFirstDeviceLatestProofText = (options: {
  device: any
  chart: Partial<FirstDeviceChartState> | null | undefined
  testResult: string
}) => {
  const chart = options.chart || {}
  if (chart.ready) {
    return `${chart.primaryKey || 'telemetry'} = ${chart.primaryValue || '--'}`
  }
  if (options.testResult) return options.testResult
  if (options.device?.online) return `${options.device.name} 当前在线`
  return '等待第一条可见遥测'
}

export const buildFirstDeviceSuccessFacts = (options: {
  device: any
  chart: Partial<FirstDeviceChartState> | null | undefined
  latestProofText: string
}) => {
  const chart = options.chart || {}

  return [
    {
      key: 'online',
      label: '在线状态',
      value: options.device?.online ? '已在线' : '未在线'
    },
    {
      key: 'latest-proof',
      label: '最新证据',
      value: options.latestProofText
    },
    {
      key: 'chart-source',
      label: '图表来源',
      value: chart.ready ? (chart.generatedFrom === 'browser_test' ? '浏览器测试确认' : '最新遥测生成') : '等待生成'
    }
  ]
}

export const buildFirstDevicePostReadyHandoff = (options: {
  ready: boolean
  nextStep?: FirstDevicePostReadyStep | null
}): FirstDevicePostReadyHandoff | null => {
  if (!options.ready) return null

  const nextStep = options.nextStep || null
  if (!nextStep) {
    return {
      title: '首台设备已准备好',
      description: '首页已经看到了在线状态、最新遥测和第一张图表；现在这套接入方式可以继续复制给更多设备。',
      primaryLabel: '查看完整接入指南',
      secondaryLabel: '定位成功证明',
      completionSignal: '完成标准：第 5 步显示“设备已准备好”，并且在线状态、最新遥测和首图都通过。',
      action: 'guide',
      section: 'proof'
    }
  }

  const title =
    nextStep.id === 'automation'
      ? '下一步：配置第一条自动化'
      : nextStep.id === 'dashboard'
        ? '下一步：创建第一个客户看板'
        : `下一步：${nextStep.title || '继续闭环'}`
  const defaultAction =
    nextStep.id === 'automation'
      ? '新建首条联动规则'
      : nextStep.id === 'dashboard'
        ? '创建客户看板'
        : nextStep.action || '继续下一步'

  return {
    title,
    description: `首台设备已跑通。现在去做「${nextStep.title || '下一步'}」：${nextStep.description || '继续把首台设备接入闭环做完整。'}`,
    primaryLabel: nextStep.action || defaultAction,
    secondaryLabel: '查看完整指南',
    completionSignal: `下一步完成标准：${nextStep.description || '完成当前交接动作，并留下可见证据。'}`,
    action: 'next-guide',
    section: 'proof'
  }
}
