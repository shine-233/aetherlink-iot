/**
 * 文件用途：首台设备工作台的「动作意图」解析（纯函数）。
 * 核心逻辑：流程节点按钮、闭环步骤、校验主/次按钮、当前聚焦步骤、首要动作原本各自写了一套 if/emit 分支；
 *   这里统一解析为 FirstDeviceIntent，由视图里唯一的 dispatch 执行，便于单测且避免多处分支漂移。
 * 关键注意事项：只返回意图，不触发副作用；文案保持与原视图一致（面向客户的中文按钮）。
 */
import type { FirstDeviceFlowNode, FirstDeviceVerificationAction } from './homeFirstDeviceWorkbench'

export type FirstDeviceWorkbenchEmitEvent =
  | 'refreshDeploymentHealth'
  | 'openFirstDeviceAccessGuide'
  | 'createFirstRunFirstDevice'
  | 'openFirstDeviceFullGuide'
  | 'simulateFirstDeviceTelemetry'
  | 'refreshFirstDeviceWorkbench'

export type FirstDeviceIntent =
  | { kind: 'emit'; event: FirstDeviceWorkbenchEmitEvent }
  | { kind: 'openNextGuideStep' }
  | { kind: 'quickstart'; action: string }
  | { kind: 'focus'; section: string }
  | { kind: 'copyTestCommand' }
  | { kind: 'copyChartProof' }
  | { kind: 'downloadProof' }
  | { kind: 'primary' }
  | { kind: 'none' }

/** 解析动作所需的只读快照；由视图从 props 与派生状态组装。 */
export interface FirstDeviceActionContext {
  ready: boolean
  hasDevice: boolean
  deploymentHealthOk: boolean
  deploymentHealthLoading: boolean
  firstRunCreateTenantRequired: boolean
  firstRunCreateLoading: boolean
  firstDeviceActionLoading: boolean
  canCopyCommand: boolean
  canRunBrowserTest: boolean
  hasActiveTestCommand: boolean
  chartReady: boolean
  hasNextGuideStep: boolean
  postReadyAction?: string | null
  primaryQuickstartAction: string
  activeQuickstartAction?: string | null
}

export interface FirstDeviceNodeAction {
  label: string
  disabled: boolean
  loading: boolean
  intent: FirstDeviceIntent
}

const emitIntent = (event: FirstDeviceWorkbenchEmitEvent): FirstDeviceIntent => ({ kind: 'emit', event })
const focusIntent = (section: string): FirstDeviceIntent => ({ kind: 'focus', section })
const nodeAction = (label: string, intent: FirstDeviceIntent, loading = false, disabled = false) => ({
  label,
  disabled,
  loading,
  intent
})

const canCopyActiveCommand = (ctx: FirstDeviceActionContext) => ctx.canCopyCommand && ctx.hasActiveTestCommand

/** 流程图节点按钮。 */
export function resolveFlowNodeAction(
  node: Pick<FirstDeviceFlowNode, 'key' | 'ok'>,
  ctx: FirstDeviceActionContext
): FirstDeviceNodeAction {
  switch (node.key) {
    case 'deployment':
      return nodeAction(
        node.ok ? '部署正常' : '去诊断',
        emitIntent('refreshDeploymentHealth'),
        ctx.deploymentHealthLoading
      )
    case 'identity':
      return ctx.hasDevice
        ? nodeAction('打开 Ready Check', emitIntent('openFirstDeviceAccessGuide'))
        : nodeAction(
            '生成首台设备',
            emitIntent('createFirstRunFirstDevice'),
            ctx.firstRunCreateLoading,
            ctx.firstRunCreateTenantRequired || !ctx.deploymentHealthOk
          )
    case 'connection':
      return canCopyActiveCommand(ctx)
        ? nodeAction('复制测试命令', { kind: 'copyTestCommand' })
        : nodeAction('打开接入指南', emitIntent('openFirstDeviceFullGuide'))
    case 'browser_test':
      return ctx.canRunBrowserTest
        ? nodeAction(
            node.ok ? '再次测试' : '开始测试',
            emitIntent('simulateFirstDeviceTelemetry'),
            ctx.firstDeviceActionLoading
          )
        : nodeAction('打开 Ready Check', emitIntent('openFirstDeviceAccessGuide'))
    case 'online':
    case 'telemetry':
      return nodeAction('查看 Ready Check', emitIntent('openFirstDeviceAccessGuide'))
    default:
      return node.ok
        ? nodeAction('查看接入指南', emitIntent('openFirstDeviceFullGuide'))
        : nodeAction('继续处理', { kind: 'primary' })
  }
}

/** 顶部闭环进度条的步骤点击。 */
export function resolveClosedLoopStepIntent(
  step: { key: string; section: string; disabled?: boolean },
  ctx: FirstDeviceActionContext
): FirstDeviceIntent {
  if (step.disabled) return focusIntent(step.section)
  switch (step.key) {
    case 'deployment':
      return emitIntent('refreshDeploymentHealth')
    case 'identity':
      return ctx.hasDevice ? focusIntent('device') : emitIntent('createFirstRunFirstDevice')
    case 'connection':
      return canCopyActiveCommand(ctx) ? { kind: 'copyTestCommand' } : emitIntent('openFirstDeviceFullGuide')
    case 'browser_test':
      return ctx.canRunBrowserTest
        ? emitIntent('simulateFirstDeviceTelemetry')
        : emitIntent('openFirstDeviceAccessGuide')
    case 'telemetry':
      return ctx.chartReady ? focusIntent('chart') : emitIntent('refreshFirstDeviceWorkbench')
    case 'proof':
      return ctx.ready ? { kind: 'downloadProof' } : focusIntent('proof')
    default:
      return { kind: 'none' }
  }
}

/** 校验卡主按钮。 */
export function resolveVerificationIntent(
  action: Pick<FirstDeviceVerificationAction, 'action'> | null | undefined,
  ctx: FirstDeviceActionContext
): FirstDeviceIntent {
  switch (action?.action) {
    case 'simulate':
      return emitIntent('simulateFirstDeviceTelemetry')
    case 'ready-check':
      return emitIntent('openFirstDeviceAccessGuide')
    case 'guide':
      return emitIntent('openFirstDeviceFullGuide')
    case 'next-guide':
      return ctx.hasNextGuideStep ? { kind: 'openNextGuideStep' } : { kind: 'none' }
    case 'proof':
      return focusIntent('proof')
    default:
      return { kind: 'none' }
  }
}

/** 校验卡次按钮。 */
export function resolveVerificationSecondaryIntent(
  action: Pick<FirstDeviceVerificationAction, 'action' | 'section'> | null | undefined
): FirstDeviceIntent {
  if (!action) return { kind: 'none' }
  if (action.action === 'next-guide') return emitIntent('openFirstDeviceFullGuide')
  if (action.action === 'proof') return { kind: 'copyChartProof' }
  return focusIntent(action.section)
}

/** 当前聚焦步骤的执行按钮：就绪后引导到下一步，否则执行当前 quickstart 步骤。 */
export function resolveFocusedQuickstartIntent(ctx: FirstDeviceActionContext): FirstDeviceIntent {
  if (ctx.ready) return ctx.hasNextGuideStep ? { kind: 'openNextGuideStep' } : emitIntent('openFirstDeviceFullGuide')
  if (!ctx.activeQuickstartAction) return { kind: 'none' }
  return { kind: 'quickstart', action: ctx.activeQuickstartAction }
}

/** 首要动作（流程节点「继续处理」）。 */
export function resolvePrimaryIntent(ctx: FirstDeviceActionContext): FirstDeviceIntent {
  if (ctx.ready) {
    return ctx.postReadyAction === 'next-guide' && ctx.hasNextGuideStep
      ? { kind: 'openNextGuideStep' }
      : emitIntent('openFirstDeviceFullGuide')
  }
  return { kind: 'quickstart', action: ctx.primaryQuickstartAction }
}

/**
 * 统一的区块键：流程节点使用 identity/browser_test/online/telemetry，
 * 视图区块使用 device/test/proof/chart，这里把两套键归一到视图区块键。
 */
const SECTION_ALIASES: Record<string, string> = {
  identity: 'device',
  browser_test: 'test',
  online: 'proof',
  telemetry: 'chart'
}

export const normalizeFirstDeviceSectionKey = (key: string) => SECTION_ALIASES[key] || key

export type FirstDeviceDeferredSection = 'connectionTest' | 'successProof' | 'supportSummary'

/** 区块所在的延迟挂载分组；非延迟区块返回 null。 */
export function deferredSectionFor(key: string): FirstDeviceDeferredSection | null {
  const section = normalizeFirstDeviceSectionKey(key)
  if (section === 'connection' || section === 'test') return 'connectionTest'
  if (section === 'chart' || section === 'proof') return 'successProof'
  if (section === 'support') return 'supportSummary'
  return null
}
