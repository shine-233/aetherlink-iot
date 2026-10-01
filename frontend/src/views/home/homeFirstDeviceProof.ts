/**
 * 首设备工作台 · 跑通证明层。
 *
 * 把部署 / 设备 / 连接 / 遥测 / 首图事实汇总为证明项，并在此之上推导闭环画布节点、
 * 闭环进度、测试后指引、主操作按钮、首图证明文本和跑通后的交接动作。
 * 依赖方向：chart ← onboarding ← proof；本文件不得被前两层反向依赖。
 */
import {
  describeProgressState,
  readinessTone,
  resolveProgressState,
  type ProgressState,
  type ProgressTone
} from '@/utils/common/status-tone'
import type { DeviceAccessGuideState } from '@/views/device/details/modules/device-access-guide-state'
import {
  buildFirstDeviceChartState,
  type FirstDeviceBrowserTestState,
  type FirstDeviceChartState,
  type FirstTelemetryPoint
} from './homeFirstDeviceChart'
import {
  hasConnectionPlaceholder,
  type FirstDeviceQuickstartStep,
  type FirstDeviceSummary
} from './homeFirstDeviceOnboarding'

export type FirstDeviceReadyProofKey =
  'deployment' | 'identity' | 'connection' | 'browser_test' | 'online' | 'telemetry' | 'chart'

export type FirstDeviceReadyProofItem = {
  key: FirstDeviceReadyProofKey
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

export type FirstDeviceFlowNodeState = ProgressState

export type FirstDeviceFlowNode = FirstDeviceReadyProofItem & {
  title: string
  short: string
  state: FirstDeviceFlowNodeState
  stateLabel: string
  stateType: ProgressTone
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

export type FirstDeviceFocusedSectionKey =
  'deployment' | 'device' | 'connection' | 'test' | 'chart' | 'proof' | 'quickstart'

const telemetryPair = (point?: { key?: string; value?: string } | null) =>
  `${point?.key || 'telemetry'} = ${point?.value || '--'}`

const deviceDisplayName = (device: FirstDeviceSummary | null) => device?.name || device?.number || '第一台设备'

// ---------------------------------------------------------------------------
// 跑通证明
// ---------------------------------------------------------------------------

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
  // 真实设备上报（在线 + 遥测 + 首图）与浏览器测试确认等价，都算首条上报已确认。
  const firstReportConfirmed = browserTestOk || (online && hasTelemetry && chart.ready)
  const latest = options.telemetry[0]

  const firstReportDetail = browserTestOk
    ? `${options.browserTest?.telemetryKey || 'telemetry'} = ${options.browserTest?.telemetryValue || '--'}，测试上报已被最新遥测确认。`
    : firstReportConfirmed
      ? `${telemetryPair(latest)}，真实设备上报已被在线状态、最新遥测和首图确认。`
      : options.browserTest?.message || '点击浏览器在线测试，或让真实设备发送一条测试遥测。'

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
      detail: hasDevice ? `${deviceDisplayName(options.device)} 已创建。` : '还没有可验证的第一台设备。'
    },
    {
      key: 'connection',
      label: '连接参数',
      ok: connectionReady,
      detail: connectionReady
        ? `${options.accessGuide?.protocol || 'MQTT/HTTP'} 参数和发布命令已经可用。`
        : '连接端点、凭证或发布命令仍未完整。'
    },
    { key: 'browser_test', label: '首条上报确认', ok: firstReportConfirmed, detail: firstReportDetail },
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
      detail: hasTelemetry ? `已收到 ${telemetryPair(latest)}。` : '还没有收到第一条遥测。'
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
// ---------------------------------------------------------------------------
// 闭环画布与进度
// ---------------------------------------------------------------------------

const FIRST_DEVICE_FLOW_NODE_META: Record<FirstDeviceReadyProofKey, { title: string; short: string }> = {
  deployment: { title: '部署可用', short: 'API / DB / MQTT' },
  identity: { title: '设备身份', short: '已创建设备' },
  connection: { title: '连接参数', short: '端点 / Topic' },
  browser_test: { title: '发送测试遥测', short: '浏览器测试' },
  online: { title: '在线状态', short: '设备在线' },
  telemetry: { title: '最新遥测', short: '收到数据' },
  chart: { title: '首张图表', short: '可视化证明' }
}

const FLOW_NODE_STATE_LABELS: Record<ProgressState, string> = {
  done: '已通过',
  active: '当前卡点',
  todo: '待处理'
}

/** 第一个未通过的证明项是当前卡点（active），其后未通过的为 todo。 */
export const buildFirstDeviceFlowNodes = (items: FirstDeviceReadyProofItem[] = []): FirstDeviceFlowNode[] => {
  const firstBlockedIndex = items.findIndex((item) => !item.ok)

  return items.map((item, index) => {
    const meta = FIRST_DEVICE_FLOW_NODE_META[item.key] || { title: item.label, short: item.label }
    const state = resolveProgressState(item.ok, index === firstBlockedIndex)
    const { label, tone } = describeProgressState(state, FLOW_NODE_STATE_LABELS)

    return {
      ...item,
      title: meta.title,
      short: meta.short,
      state,
      stateLabel: label,
      stateType: tone
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

const QUICKSTART_STEP_SECTION: Record<
  Exclude<FirstDeviceQuickstartStep['key'], 'verify'>,
  FirstDeviceFocusedSectionKey
> = {
  health: 'deployment',
  create: 'device',
  connect: 'connection',
  publish: 'test'
}

export const resolveFirstDeviceFocusedSectionKey = (options: {
  activeStep?: Pick<FirstDeviceQuickstartStep, 'key'> | null
  ready: boolean
  readyProofItems?: FirstDeviceReadyProofItem[]
  chartReady: boolean
}): FirstDeviceFocusedSectionKey => {
  const key = options.activeStep?.key
  if (key === 'verify') {
    // 验证阶段：首图还没出且首个卡点正是图表时，聚焦图表区，否则聚焦证明区。
    const firstBlocked = options.readyProofItems?.find((item) => !item.ok)?.key
    return !options.chartReady && firstBlocked === 'chart' ? 'chart' : 'proof'
  }
  if (key && key in QUICKSTART_STEP_SECTION) return QUICKSTART_STEP_SECTION[key as keyof typeof QUICKSTART_STEP_SECTION]
  return options.ready ? 'proof' : 'quickstart'
}

// ---------------------------------------------------------------------------
// 测试后指引与主操作
// ---------------------------------------------------------------------------

export const buildFirstDevicePostTestGuidance = (options: {
  testResult: string
  ready: boolean
  readyDescription: string
  chartReady: boolean
  currentBlocker?: Pick<FirstDeviceReadyProofItem, 'label'> | null
}): FirstDevicePostTestGuidance | null => {
  if (!options.testResult) return null
  const blocker = options.currentBlocker
  if (options.ready) {
    return { type: readinessTone(true), title: '闭环已确认', detail: options.readyDescription }
  }
  if (options.chartReady) {
    return {
      type: readinessTone(false),
      title: '测试遥测已产生数据，继续看最终证明',
      detail: blocker ? `当前还差：${blocker.label}` : '图表已经出现，继续检查右侧证明项是否全部变绿。'
    }
  }
  return {
    type: readinessTone(false),
    title: '测试已发送，等待可见证据',
    detail: blocker
      ? `当前卡点：${blocker.label}。如果长时间不变，请打开 Ready Check 或复制支持摘要。`
      : '首页正在等待最新遥测；如果没有更新，请查看 Ready Check 的连接诊断。'
  }
}

type VerificationActionBody = Omit<FirstDeviceVerificationAction, 'type' | 'loading' | 'disabled'> &
  Partial<Pick<FirstDeviceVerificationAction, 'loading'>>

/** 所有未就绪分支都是 warning、默认不 loading、不 disabled，只差文案与去向。 */
const pendingVerificationAction = (body: VerificationActionBody): FirstDeviceVerificationAction => ({
  type: readinessTone(false),
  loading: false,
  disabled: false,
  ...body
})

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
  const blocker = options.currentBlocker

  if (options.ready) {
    const handoff = options.postReadyHandoff
    return {
      type: readinessTone(true),
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
    const resend = Boolean(options.testResult)
    return pendingVerificationAction({
      title: resend ? '继续确认遥测并生成首图' : '下一步：发送测试遥测并生成首图',
      detail: resend
        ? '测试已经发出，仍可再次发送一条测试遥测；首页会继续刷新最新遥测，并在收到数据后生成第一张图表。'
        : '设备已经创建，现在最该做的是点一次浏览器在线测试，让平台收到第一条遥测并自动生成首图。',
      label: resend ? '再次发送并确认' : '发送测试遥测并确认',
      secondaryLabel: '定位测试区',
      action: 'simulate',
      section: 'test',
      loading: options.actionLoading
    })
  }

  if (!options.chartReady) {
    return pendingVerificationAction({
      title: '下一步：补齐参数再验证',
      detail: blocker
        ? `当前还差：${blocker.label}。先打开 Ready Check 看诊断，或回到参数区补齐端点、凭证和 topic。`
        : '设备存在，但暂时还不能直接发送浏览器测试；先确认连接参数和 Ready Check 诊断。',
      label: '打开 Ready Check',
      secondaryLabel: '定位参数区',
      action: 'ready-check',
      section: 'connection'
    })
  }

  return pendingVerificationAction({
    title: '首图已生成，继续确认最终证明',
    detail: blocker
      ? `首张图表已经出来，当前还差：${blocker.label}。把右侧证明项处理完，首页就会显示“设备已准备好”。`
      : '首张图表已经出来，继续查看证明项是否全部变绿。',
    label: '查看证明项',
    secondaryLabel: '复制首图证明',
    action: 'proof',
    section: 'proof'
  })
}
// ---------------------------------------------------------------------------
// 首图证明文本与成功事实
// ---------------------------------------------------------------------------

const numberedLines = <T>(rows: T[], format: (row: T) => string) =>
  rows.length ? rows.map((row, index) => `${index + 1}. ${format(row)}`).join('\n') : '<暂无>'

const chartSourceLabel = (chart: Partial<FirstDeviceChartState>) =>
  chart.generatedFrom === 'browser_test' ? '浏览器在线测试' : '最新遥测'

export const buildFirstDeviceChartProofSummary = (options: {
  device: any
  chart: Partial<FirstDeviceChartState> | null | undefined
  readyProof: Partial<FirstDeviceReadyProof> | null | undefined
}): string => {
  const chart = options.chart || {}
  const chartPoints: any[] = Array.isArray(chart.points) ? chart.points : []
  const readyProof = options.readyProof || {}
  const proofItems: any[] = Array.isArray(readyProof.items) ? readyProof.items : []
  const device = options.device

  return [
    '# AetherLink 首次接入首图证明',
    '',
    '## 设备',
    `id=${device?.id || '<未知>'}`,
    `name=${device?.name || '<未知>'}`,
    `number=${device?.device_number || device?.number || '<未知>'}`,
    `online=${Boolean(device?.online)}`,
    '',
    '## 首图证明',
    `ready=${Boolean(chart.ready)}`,
    `source=${chartSourceLabel(chart)}`,
    `primary=${chart.primaryKey || 'telemetry'}=${chart.primaryValue ?? '--'}`,
    `summary=${chart.summary || '<空>'}`,
    '',
    '## 图表点位',
    numberedLines(chartPoints, (point) => `${point.key}=${point.value}`),
    '',
    '## 跑通证明',
    `ready=${Boolean(readyProof.ready)}`,
    `title=${readyProof.title || '<空>'}`,
    `summary=${readyProof.summary || '<空>'}`,
    numberedLines(proofItems, (item) => `${item.label}: ${item.ok ? '已通过' : '待处理'} - ${item.detail || ''}`),
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
  if (chart.ready) return `${chart.primaryKey || 'telemetry'} = ${chart.primaryValue || '--'}`
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
    { key: 'online', label: '在线状态', value: options.device?.online ? '已在线' : '未在线' },
    { key: 'latest-proof', label: '最新证据', value: options.latestProofText },
    {
      key: 'chart-source',
      label: '图表来源',
      value: chart.ready ? (chart.generatedFrom === 'browser_test' ? '浏览器测试确认' : '最新遥测生成') : '等待生成'
    }
  ]
}

// ---------------------------------------------------------------------------
// 跑通后的交接
// ---------------------------------------------------------------------------

/** 已知的后续引导步骤有专门的标题与默认按钮文案，其余步骤用通用模板。 */
const KNOWN_POST_READY_STEPS = new Map<string, { title: string; defaultAction: string }>([
  ['automation', { title: '下一步：配置第一条自动化', defaultAction: '新建首条联动规则' }],
  ['dashboard', { title: '下一步：创建第一个客户看板', defaultAction: '创建客户看板' }]
])

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

  const known = nextStep.id ? KNOWN_POST_READY_STEPS.get(nextStep.id) : undefined
  const title = known?.title ?? `下一步：${nextStep.title || '继续闭环'}`
  const defaultAction = known?.defaultAction ?? (nextStep.action || '继续下一步')

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
