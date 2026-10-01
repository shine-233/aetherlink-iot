/**
 * 首设备工作台 · 接入引导层。
 *
 * 负责设备/连接参数的归一化，以及「部署 → 创建 → 连接 → 上报 → 验证」五步快速开始的守卫。
 * 只依赖遥测层（homeFirstDeviceChart），不依赖证明层。
 */
import {
  resolveProgressState,
  resolveProgressTone,
  type ProgressState,
  type ProgressTone
} from '@/utils/common/status-tone'
import type { DeviceAccessGuideState } from '@/views/device/details/modules/device-access-guide-state'
import { unwrapFirstDeviceResponse, type FirstTelemetryPoint } from './homeFirstDeviceChart'

export type FirstDeviceSummary = {
  id: string
  name: string
  number: string
  online: boolean
  configId: string
  configName: string
}

export type SimulationInitState = {
  server: string
  port: number
  topic: string
  payload: string
}

export type FirstDeviceQuickstartAction = 'health' | 'create' | 'copy' | 'test' | 'ready-check'

export type FirstDeviceQuickstartStepKey = 'health' | 'create' | 'connect' | 'publish' | 'verify'

export type FirstDeviceQuickstartStep = {
  key: FirstDeviceQuickstartStepKey
  title: string
  description: string
  status: ProgressState
  statusLabel: string
  statusType: ProgressTone
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

export const DEFAULT_FIRST_DEVICE_PAYLOAD = '{"temperature":25.5,"humidity":60}'

const PLACEHOLDER_PATTERN = /<[^>]+>|\bundefined\b|\bnull\b|连接参数加载中|loading/i

const asArray = (value: unknown): any[] => (Array.isArray(value) ? value : [])

export const normalizeFirstDevice = (response: any): FirstDeviceSummary | null => {
  const data = unwrapFirstDeviceResponse(response)
  const first = asArray(data?.list ?? data?.data?.list ?? data?.records)[0]
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

export const normalizeSimulationInit = (response: any): SimulationInitState => {
  const data = unwrapFirstDeviceResponse(response)
  return {
    server: String(data.server || 'localhost'),
    port: Number(data.port || 1883),
    topic: String(data.topic || 'devices/telemetry'),
    payload: String(data.default_data || data.payload || DEFAULT_FIRST_DEVICE_PAYLOAD)
  }
}

/** 生成可直接粘贴到 POSIX shell 的 mosquitto_pub 命令（payload 内单引号做 '"'"' 转义）。 */
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
  // `<token>` 之类的占位符不能当成真实凭证发出去。
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

export const hasConnectionPlaceholder = (value: string) => PLACEHOLDER_PATTERN.test(value.trim())

const QUICKSTART_STATUS_LABELS: Record<ProgressState, string> = {
  done: '已完成',
  active: '现在做',
  todo: '待处理'
}

type QuickstartStepInput = Omit<FirstDeviceQuickstartStep, 'status' | 'statusLabel' | 'statusType'> & {
  done: boolean
}

/**
 * 步骤状态统一由「是否完成 + 是否为当前激活步骤」推导：
 * 完成优先，其次只有 activeKey 命中的那一步是 active。
 */
const toQuickstartStep = (
  { done, ...step }: QuickstartStepInput,
  activeKey: FirstDeviceQuickstartStepKey | null
): FirstDeviceQuickstartStep => {
  const status = resolveProgressState(done, step.key === activeKey)
  return {
    ...step,
    status,
    statusLabel: QUICKSTART_STATUS_LABELS[status],
    statusType: resolveProgressTone(status)
  }
}

type OnboardingFacts = {
  deploymentHealthy: boolean
  hasDevice: boolean
  canCopyCommand: boolean
  hasTelemetry: boolean
  ready: boolean
}

/** 按顺序的门槛：第一个未满足的门槛就是当前该做的步骤。 */
const QUICKSTART_GATES: ReadonlyArray<[FirstDeviceQuickstartStepKey, (facts: OnboardingFacts) => boolean]> = [
  ['health', (facts) => facts.deploymentHealthy],
  ['create', (facts) => facts.hasDevice],
  ['connect', (facts) => facts.canCopyCommand],
  ['publish', (facts) => facts.hasTelemetry],
  ['verify', (facts) => facts.ready]
]

const resolveActiveQuickstartKey = (facts: OnboardingFacts): FirstDeviceQuickstartStepKey | null =>
  QUICKSTART_GATES.find(([, passed]) => !passed(facts))?.[0] ?? null

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
  const activeKey = resolveActiveQuickstartKey({ deploymentHealthy, hasDevice, canCopyCommand, hasTelemetry, ready })

  const steps = (
    [
      {
        key: 'health',
        title: '0. 检查部署健康',
        description: deploymentHealthy
          ? '前端、API、数据库、Redis 和 MQTT Broker 已经可以支撑首台设备接入。'
          : '先确认前端、API、数据库、Redis 和 MQTT Broker 都正常，再创建或测试第一台设备。',
        done: deploymentHealthy,
        action: 'health',
        actionLabel: '检查部署健康',
        disabled: false
      },
      {
        key: 'create',
        title: '1. 生成第一台设备',
        description: hasDevice
          ? `已找到 ${options.device?.name || options.device?.number || '第一台设备'}`
          : '一键创建产品、物模型、MQTT/HTTP 配置和第一台设备，也可以手动添加。',
        done: hasDevice,
        action: 'create',
        actionLabel: hasDevice ? '定位设备信息' : '一键生成',
        disabled: !deploymentHealthy
      },
      {
        key: 'connect',
        title: '2. 拿到可用连接参数',
        description: canCopyCommand
          ? '命令里已经没有占位符，可以复制到设备端或终端测试。'
          : '连接参数还不完整，请打开 Ready Check 查看缺少的端点、凭证或 topic。',
        done: canCopyCommand,
        action: canCopyCommand ? 'copy' : 'ready-check',
        actionLabel: canCopyCommand ? '复制命令' : '打开 Ready Check',
        disabled: !deploymentHealthy || !hasDevice
      },
      {
        key: 'publish',
        title: '3. 发一条遥测',
        description: hasTelemetry
          ? `已收到 ${options.telemetry[0]?.key || 'telemetry'} 的最新数据。`
          : '复制命令到真实设备，或直接点浏览器在线测试发送一条测试遥测。',
        done: hasTelemetry,
        action: 'test',
        actionLabel: '浏览器在线测试',
        disabled: !canRunBrowserTest
      },
      {
        key: 'verify',
        title: '4. 确认设备真的可交付',
        description: ready
          ? '设备在线且已有最新遥测，可以继续做告警、自动化和看板。'
          : '进入 Ready Check 看在线、最新遥测、命令回包和下一步修复建议。',
        done: ready,
        action: 'ready-check',
        actionLabel: '打开 Ready Check',
        disabled: !deploymentHealthy || !hasDevice
      }
    ] satisfies QuickstartStepInput[]
  ).map((step) => toQuickstartStep(step, activeKey))

  const activeStep = steps.find((step) => step.status === 'active') ?? null
  return {
    commandHasPlaceholders,
    canCopyCommand,
    canRunBrowserTest,
    summary: ready
      ? '第一台设备已跑通，可以复制这套流程给更多设备。'
      : activeStep?.description || '按步骤完成第一台设备接入。',
    nextAction: activeStep?.actionLabel || '继续下一步',
    activeStep,
    steps
  }
}
