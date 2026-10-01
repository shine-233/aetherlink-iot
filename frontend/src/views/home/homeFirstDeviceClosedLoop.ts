/**
 * 文件用途：构造首台设备工作台顶部「闭环 6 步」进度条的步骤数据（纯函数）。
 * 核心逻辑：每步状态 = 显式完成条件 || 流程节点 ok → done；节点 active → active；否则 todo。
 */
import type { FirstDeviceFlowNode } from './homeFirstDeviceWorkbench'

export type FirstDeviceClosedLoopState = 'done' | 'active' | 'todo'

export interface FirstDeviceClosedLoopStep {
  key: string
  section: string
  order: string
  title: string
  detail: string
  actionLabel: string
  disabled: boolean
  loading: boolean
  state: FirstDeviceClosedLoopState
  stateLabel: string
  stateType: 'success' | 'warning' | 'default'
}

export interface FirstDeviceClosedLoopInput {
  flowNodes: Pick<FirstDeviceFlowNode, 'key' | 'ok' | 'state'>[]
  device: { name?: string; number?: string; online?: boolean } | null
  deploymentHealthOk: boolean
  deploymentHealthLoading: boolean
  firstRunCreateTenantRequired: boolean
  firstRunCreateLoading: boolean
  firstDeviceLoading: boolean
  firstDeviceActionLoading: boolean
  canCopyCommand: boolean
  canRunBrowserTest: boolean
  activeTestCommand: { label?: string; code?: string } | null
  browserTest: { status?: string; message?: string } | null
  chartReady: boolean
  ready: boolean
  latestProofText: string
}

const STATE_LABEL: Record<FirstDeviceClosedLoopState, string> = { done: '已完成', active: '进行中', todo: '等待' }
const STATE_TYPE: Record<FirstDeviceClosedLoopState, FirstDeviceClosedLoopStep['stateType']> = {
  done: 'success',
  active: 'warning',
  todo: 'default'
}

export function resolveClosedLoopState(
  flowNodes: FirstDeviceClosedLoopInput['flowNodes'],
  key: string,
  done: boolean
): Pick<FirstDeviceClosedLoopStep, 'state' | 'stateLabel' | 'stateType'> {
  const node = flowNodes.find((item) => item.key === key)
  let state: FirstDeviceClosedLoopState = 'todo'
  if (done || node?.ok) state = 'done'
  else if (node?.state === 'active') state = 'active'
  return { state, stateLabel: STATE_LABEL[state], stateType: STATE_TYPE[state] }
}

export function buildFirstDeviceClosedLoopSteps(input: FirstDeviceClosedLoopInput): FirstDeviceClosedLoopStep[] {
  const state = (key: string, done: boolean) => resolveClosedLoopState(input.flowNodes, key, done)
  const hasDevice = Boolean(input.device)
  return [
    {
      key: 'deployment',
      section: 'deployment',
      order: '01',
      title: '部署健康',
      detail: input.deploymentHealthOk ? '前端、API、Broker、Redis、MQTT 可用' : '先确认部署组件都正常',
      actionLabel: input.deploymentHealthOk ? '查看详情' : '去诊断',
      disabled: false,
      loading: input.deploymentHealthLoading,
      ...state('deployment', input.deploymentHealthOk)
    },
    {
      key: 'identity',
      section: 'device',
      order: '02',
      title: '创建产品/设备',
      detail: input.device
        ? input.device.name || input.device.number || '第一台设备已生成'
        : '请先完成上方初始化步骤，系统会自动生成首台设备',
      actionLabel: hasDevice ? '定位设备信息' : '一键生成',
      disabled: input.firstRunCreateTenantRequired || !input.deploymentHealthOk,
      loading: input.firstRunCreateLoading,
      ...state('identity', hasDevice)
    },
    {
      key: 'connection',
      section: 'connection',
      order: '03',
      title: '复制 MQTT/HTTP 参数',
      detail: input.activeTestCommand?.label || '还没有可复制的测试命令，请先生成设备',
      actionLabel: input.canCopyCommand ? '复制测试命令' : '暂无可复制的命令',
      disabled: !hasDevice,
      loading: false,
      ...state('connection', Boolean(input.canCopyCommand && input.activeTestCommand?.code))
    },
    {
      key: 'browser_test',
      section: 'test',
      order: '04',
      title: '浏览器发测试数据',
      detail: input.browserTest?.message || '尚未收到测试数据，发送后这里会显示结果',
      actionLabel: input.canRunBrowserTest ? '发送测试数据' : '打开 Ready Check',
      disabled: !hasDevice,
      loading: input.firstDeviceActionLoading,
      ...state('browser_test', input.browserTest?.status === 'confirmed')
    },
    {
      key: 'telemetry',
      section: 'chart',
      order: '05',
      title: '确认在线/遥测/首图',
      detail: input.latestProofText,
      actionLabel: input.chartReady ? '查看首图' : '刷新确认',
      disabled: !hasDevice,
      loading: input.firstDeviceLoading,
      ...state('telemetry', Boolean(input.device?.online && input.chartReady))
    },
    {
      key: 'proof',
      section: 'proof',
      order: '06',
      title: '下载成功证明',
      detail: input.ready ? '成功证明条件已满足，可下载存档' : '还差关键证据，按步骤继续操作',
      actionLabel: input.ready ? '下载证明' : '查看缺口',
      disabled: !hasDevice,
      loading: false,
      ...state('chart', input.ready)
    }
  ]
}

/** 设备接入摘要（复制给现场/支持人员）。 */
export function buildFirstDeviceConnectionSummary(input: {
  device: { name?: string; number?: string } | null
  accessGuide: {
    endpoint?: string
    endpointKind?: string
    reportTopic?: string
    controlTopic?: string
    protocol?: string
  } | null
  simulation: { server?: string; port?: string | number; topic?: string } | null
  command: string
}) {
  const { accessGuide, simulation } = input
  const endpoint =
    accessGuide?.endpoint || [simulation?.server, simulation?.port].filter(Boolean).join(':') || 'not-ready'
  const reportEntry =
    accessGuide?.endpointKind === 'http'
      ? accessGuide.endpoint || endpoint
      : accessGuide?.reportTopic || simulation?.topic || 'devices/telemetry'
  return [
    'AetherLink first-device connection summary',
    `Device: ${input.device?.name || input.device?.number || 'first device'}`,
    `Protocol: ${accessGuide?.protocol || 'MQTT'}`,
    `Endpoint: ${endpoint}`,
    `Report entry: ${reportEntry}`,
    `Control entry: ${accessGuide?.controlTopic || 'open Ready Check'}`,
    input.command ? `Device connection command:\n${input.command}` : 'Device connection command: not-ready'
  ].join('\n')
}
