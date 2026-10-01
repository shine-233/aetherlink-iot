/**
 * 面向操作员的执行计划与新手引导卡片。
 * 只消费 NormalizedDryRun；原始响应在入口处经 normalizeDryRunResponse 统一转换。
 */
import type { SceneAutomationDryRunConditionGroup, SceneAutomationDryRunNode } from '@/service/api/automation'
import { buildCountLines, firstLineText, formatPreviewValue } from './automationDryRunFormat'
import { normalizeDryRunResponse } from './automationDryRunNormalize'
import type { DryRunResponseInput, NormalizedDryRun } from './automationDryRunNormalize'
import type {
  AutomationDryRunBeginnerGuideCard,
  AutomationDryRunCustomerView,
  AutomationDryRunLine,
  AutomationDryRunOperatorPlan,
  BackendDryRunStatus
} from './automationDryRunTypes'

/** 联动规则预演载荷（字段宽松） */
type DryRunPayloadLike = {
  name?: unknown
  enabled?: unknown
  trigger_condition_groups?: SceneAutomationDryRunConditionGroup[] | null
  actions?: SceneAutomationDryRunNode[] | null
  [key: string]: unknown
}

const describeBackendState = (status: BackendDryRunStatus, backendError: string) => {
  if (status === 'available') return '当前载荷已有后端预演结果。'
  if (status === 'unavailable') return `后端预演暂不可用${backendError ? `：${backendError}` : '。'}`

  return '当前载荷尚未运行后端预演。'
}

const describeSaveReadiness = (canSave: boolean | null) => {
  if (canSave === false) return '后端判断这条规则暂不适合保存。'
  if (canSave === true) return '后端判断这条规则可以保存。'

  return '后端预演返回 can_save 前，保存状态仍未知。'
}

export const buildAutomationOperatorPlan = (
  payload: DryRunPayloadLike | null,
  status: BackendDryRunStatus,
  response: DryRunResponseInput,
  backendError = ''
): AutomationDryRunOperatorPlan => {
  const dryRun = normalizeDryRunResponse(response)
  const conditionGroups = Array.isArray(payload?.trigger_condition_groups) ? payload.trigger_condition_groups : []
  const actions = Array.isArray(payload?.actions) ? payload.actions : []
  const conditionCount = conditionGroups.reduce((count, group) => count + group.length, 0)

  return {
    source: [
      { key: 'rule-name', text: `规则：${formatPreviewValue(payload?.name)}` },
      { key: 'rule-enabled', text: `保存后启用：${payload?.enabled === false ? '否' : '是'}` },
      { key: 'dry-run-state', text: describeBackendState(status, backendError) }
    ],
    conditions: [
      {
        key: 'condition-shape',
        text: `载荷包含 ${conditionGroups.length} 个条件组和 ${conditionCount} 条条件。`
      },
      ...buildCountLines('operator-condition-type', dryRun.stats.conditionTypes)
    ],
    actions: [
      {
        key: 'action-shape',
        text: `如果规则触发，保存后的定义会包含 ${actions.length} 条已配置动作。`
      },
      ...buildCountLines('operator-action-type', dryRun.stats.actionTypes),
      ...buildCountLines('operator-reference', dryRun.referenceCounts)
    ],
    limits: [
      { key: 'save-readiness', text: describeSaveReadiness(dryRun.canSave) },
      {
        key: 'no-side-effect',
        text: '预演不会保存规则、发布命令、触发报警，也不能证明实时设备遥测已经命中。'
      },
      {
        key: 'runtime-evidence',
        text: '保存后，请用自动化日志或设备/报警证据证明规则确实触发。'
      }
    ]
  }
}

export interface AutomationDryRunBeginnerGuideOptions {
  status: BackendDryRunStatus
  response: DryRunResponseInput
  backendError: string
  customerView: AutomationDryRunCustomerView
  localBlockingErrors: AutomationDryRunLine[]
  conditionGroupCount: number
  conditionCount: number
  actionCount: number
}

/** 卡片 builder 共享的已推导上下文 */
interface GuideContext {
  options: AutomationDryRunBeginnerGuideOptions
  dryRun: NormalizedDryRun
  firstBlocker: string
  firstWarning: string
  shapeDetail: string
}

const guideCard = (
  key: AutomationDryRunBeginnerGuideCard['key'],
  type: AutomationDryRunBeginnerGuideCard['type'],
  titleSuffix: string,
  textSuffix: string,
  detail: string
): AutomationDryRunBeginnerGuideCard => ({
  key,
  type,
  titleKey: `generate.automationDryRunBeginner${titleSuffix}`,
  textKey: `generate.automationDryRunBeginner${textSuffix}`,
  detail
})

const buildSaveCard = ({ options, firstBlocker, firstWarning, shapeDetail }: GuideContext) => {
  const { status, customerView, backendError } = options

  if (status === 'pending') return guideCard('save', 'info', 'SaveTitle', 'SaveRunning', shapeDetail)
  if (firstBlocker || customerView.canSave === false) {
    return guideCard('save', 'error', 'SaveTitle', 'SaveBlocked', firstBlocker || shapeDetail)
  }
  if (customerView.canSave === true) {
    const type = customerView.status === 'passed' ? 'success' : 'warning'
    return guideCard('save', type, 'SaveTitle', 'SaveReady', firstWarning || shapeDetail)
  }
  if (status === 'unavailable') {
    return guideCard('save', 'warning', 'SaveTitle', 'SaveBackendUnavailable', backendError || shapeDetail)
  }

  return guideCard('save', 'info', 'SaveTitle', 'SaveRunFirst', shapeDetail)
}

const buildMatchCard = ({ options, dryRun, shapeDetail }: GuideContext) => {
  const matched = dryRun.matchedDevices
  if (matched !== null) {
    return guideCard('match', matched > 0 ? 'success' : 'warning', 'MatchTitle', 'MatchKnown', String(matched))
  }
  if (options.status === 'available')
    return guideCard('match', 'warning', 'MatchTitle', 'MatchNotEvaluated', shapeDetail)

  return guideCard('match', 'info', 'MatchTitle', 'MatchRunFirst', shapeDetail)
}

const buildSkippedCard = ({ options, dryRun, firstWarning, shapeDetail }: GuideContext) => {
  const firstSkipped = dryRun.skippedConditions[0]
  if (firstSkipped || firstWarning) {
    return guideCard('skipped', 'warning', 'SkippedTitle', 'SkippedKnown', firstSkipped || firstWarning)
  }
  if (options.status === 'available') return guideCard('skipped', 'success', 'SkippedTitle', 'SkippedNone', shapeDetail)

  return guideCard('skipped', 'info', 'SkippedTitle', 'SkippedRunFirst', shapeDetail)
}

const buildActionCard = ({ options, dryRun, shapeDetail }: GuideContext) => {
  if (options.actionCount === 0) return guideCard('actions', 'error', 'ActionTitle', 'ActionMissing', shapeDetail)
  const firstUnavailable = dryRun.unavailableActions[0]
  if (firstUnavailable) {
    return guideCard('actions', 'warning', 'ActionTitle', 'ActionUnavailable', firstUnavailable)
  }
  if (options.status === 'available') return guideCard('actions', 'success', 'ActionTitle', 'ActionReady', shapeDetail)

  return guideCard('actions', 'info', 'ActionTitle', 'ActionRunFirst', shapeDetail)
}

export const buildAutomationDryRunBeginnerGuide = (
  options: AutomationDryRunBeginnerGuideOptions
): AutomationDryRunBeginnerGuideCard[] => {
  const context: GuideContext = {
    options,
    dryRun: normalizeDryRunResponse(options.response),
    firstBlocker: firstLineText(options.localBlockingErrors, options.customerView.blockingErrors),
    firstWarning: firstLineText(options.customerView.warnings),
    shapeDetail: `${options.conditionGroupCount} group(s) / ${options.conditionCount} condition row(s) / ${options.actionCount} action row(s)`
  }

  return [buildSaveCard(context), buildMatchCard(context), buildSkippedCard(context), buildActionCard(context)]
}
