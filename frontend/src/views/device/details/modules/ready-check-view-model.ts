/**
 * 文件用途: 设备 Ready Check 页面的纯函数视图模型。
 * 核心逻辑: 把 onboarding-ready-check.vue 原先内联在 computed 中的派生规则（证据中心条目、分步检查、
 * 主推荐动作选择、后端下一步裁剪、最新遥测文本）抽成无副作用函数，便于单测与复用。
 * 关键注意事项: 所有文案 key 与原组件保持一致；动作函数由调用方（useOnboardingReadyCheck）注入。
 */
import type { DeviceConnectionGuideStateInput } from './device-access-guide-state'

export type ReadyCheckTelemetry = {
  current_count?: number
  latest_key?: string
  latest_at?: string
  latest_value?: unknown
}

export type ReadyCheckEvidence = {
  ready?: boolean
  level?: string
  code?: string
  summary?: string
  next_actions?: string[]
  telemetry?: ReadyCheckTelemetry
}

export type ReadyCheckActionStatus = 'ready' | 'attention' | 'next'

export type ReadyCheckStepItem = {
  key: string
  status: ReadyCheckActionStatus
  titleKey: string
  descKey: string
  actionKey: string
  action: () => void
}

export type ReadyCheckPrimaryActionItem = {
  key: string
  status: ReadyCheckActionStatus
  titleKey: string
  descKey?: string
  actionKey: string
  action: () => void
  summary?: string
}

type Translate = (key: string) => string

const DETAILS = 'custom.device_details'

export function formatLatestTelemetryText(telemetry: ReadyCheckTelemetry | undefined, t: Translate) {
  if (!telemetry?.latest_key) return t(`${DETAILS}.accessGuideLatestTelemetryEmpty`)
  return [telemetry.latest_key, telemetry.latest_at].filter(Boolean).join(' @ ')
}

export function formatPartialResultText(guide: DeviceConnectionGuideStateInput | null, t: Translate) {
  const partialResults = Array.isArray(guide?.partial_results) ? guide.partial_results : []
  if (!partialResults.length) return t(`${DETAILS}.readyCheckEvidenceComplete`)
  return partialResults.map((warning) => `${warning.component || 'guide'}: ${warning.reason || 'partial'}`).join('; ')
}

export function buildBackendNextSteps(guide: DeviceConnectionGuideStateInput | null, t: Translate, limit = 4) {
  return (guide?.next_steps || [])
    .map((step, index) => ({
      key: step.key || `step-${index}`,
      title: step.title || t(`${DETAILS}.readyCheckEvidenceNextStepUntitled`),
      description: step.description || '',
      status: step.status || 'todo'
    }))
    .slice(0, limit)
}

export type EvidenceCenterInput = {
  sourceLabel: string
  sourceDetail: string
  evaluatedAt: string
  readinessSummary: string
  guide: DeviceConnectionGuideStateInput | null
  readyCheck: ReadyCheckEvidence
  latestTelemetryText: string
  latestTelemetryValueText: string
  lastConnectionIssue: string
  partialResultText: string
}

export function buildEvidenceCenterItems(input: EvidenceCenterInput, t: Translate) {
  const { guide, readyCheck } = input
  return [
    {
      key: 'source',
      labelKey: `${DETAILS}.readyCheckEvidenceSource`,
      value: input.sourceLabel,
      detail: input.sourceDetail
    },
    {
      key: 'evaluated-at',
      labelKey: `${DETAILS}.readyCheckEvidenceEvaluatedAt`,
      value: input.evaluatedAt,
      detail: t(`${DETAILS}.readyCheckEvidenceEvaluatedAtDesc`)
    },
    {
      key: 'readiness',
      labelKey: `${DETAILS}.readyCheckEvidenceReadiness`,
      value: input.readinessSummary,
      detail: [
        `ready=${guide?.readiness?.ready ?? readyCheck.ready ?? '--'}`,
        `level=${guide?.readiness?.level || readyCheck.level || '--'}`,
        `code=${guide?.readiness?.code || readyCheck.code || '--'}`
      ].join(' / ')
    },
    {
      key: 'telemetry',
      labelKey: `${DETAILS}.accessGuideLatestTelemetry`,
      value: input.latestTelemetryText,
      detail: [
        `${t(`${DETAILS}.readyCheckEvidenceTelemetryCount`)}: ${readyCheck.telemetry?.current_count ?? '--'}`,
        `${t(`${DETAILS}.readyCheckEvidenceTelemetryValue`)}: ${input.latestTelemetryValueText}`
      ].join(' / ')
    },
    {
      key: 'last-issue',
      labelKey: `${DETAILS}.readyCheckEvidenceLastIssue`,
      value: input.lastConnectionIssue,
      detail: guide?.last_connection_error?.code || t(`${DETAILS}.readyCheckEvidenceNoLastIssueCode`)
    },
    {
      key: 'completeness',
      labelKey: `${DETAILS}.readyCheckEvidenceCompleteness`,
      value: input.partialResultText,
      detail: t(`${DETAILS}.readyCheckEvidenceCompletenessDesc`)
    },
    {
      key: 'boundary',
      labelKey: `${DETAILS}.readyCheckEvidenceBoundary`,
      value: t(`${DETAILS}.readyCheckEvidenceBoundaryValue`),
      detail: t(`${DETAILS}.readyCheckEvidenceBoundaryDesc`)
    }
  ]
}

export type ReadyCheckStepInput = {
  hasConnectionIdentity: boolean
  isOnline: boolean
  hasRecentTelemetry: boolean
  hasTemplate: boolean
  openTab: (tab: string) => void
  openCommandCenter: () => void
}

export function buildReadyCheckSteps(input: ReadyCheckStepInput): ReadyCheckStepItem[] {
  const { isOnline, openTab } = input
  return [
    {
      key: 'connect',
      status: input.hasConnectionIdentity ? 'ready' : 'attention',
      titleKey: `${DETAILS}.readyCheckConnectTitle`,
      descKey: `${DETAILS}.readyCheckConnectDesc`,
      actionKey: `${DETAILS}.readyCheckOpenConnection`,
      action: () => openTab('join')
    },
    {
      key: 'online',
      status: isOnline ? 'ready' : 'attention',
      titleKey: `${DETAILS}.readyCheckOnlineTitle`,
      descKey: isOnline ? `${DETAILS}.readyCheckOnlineReadyDesc` : `${DETAILS}.readyCheckOnlineWaitingDesc`,
      actionKey: `${DETAILS}.readyCheckOpenConnection`,
      action: () => openTab('join')
    },
    {
      key: 'telemetry',
      status: input.hasRecentTelemetry ? 'ready' : isOnline ? 'attention' : 'next',
      titleKey: `${DETAILS}.readyCheckTelemetryTitle`,
      descKey: input.hasTemplate
        ? `${DETAILS}.readyCheckTelemetryDesc`
        : `${DETAILS}.readyCheckTelemetryNoTemplateDesc`,
      actionKey: `${DETAILS}.readyCheckOpenTelemetry`,
      action: () => openTab('telemetry')
    },
    {
      key: 'twin',
      status: 'next',
      titleKey: `${DETAILS}.readyCheckTwinTitle`,
      descKey: `${DETAILS}.readyCheckTwinDesc`,
      actionKey: `${DETAILS}.readyCheckOpenTwin`,
      action: () => openTab('device-twin')
    },
    {
      key: 'command',
      status: 'next',
      titleKey: `${DETAILS}.readyCheckCommandTitle`,
      descKey: `${DETAILS}.readyCheckCommandDesc`,
      actionKey: `${DETAILS}.readyCheckOpenCommand`,
      action: () => openTab('command-delivery')
    },
    {
      key: 'jobs',
      status: 'next',
      titleKey: `${DETAILS}.readyCheckJobsTitle`,
      descKey: `${DETAILS}.readyCheckJobsDesc`,
      actionKey: `${DETAILS}.readyCheckOpenCommandCenter`,
      action: input.openCommandCenter
    }
  ]
}

export type PrimaryActionInput = {
  collectionFailureSummary: string | null
  firstDeviceReady: boolean
  steps: ReadyCheckStepItem[]
  evidenceActions: ReadyCheckPrimaryActionItem[]
  refresh: () => void
  openFirstDeviceAutomation: () => void
  t: Translate
}

/** 主推荐动作优先级：采集失败 > 首台设备已就绪 > 首个 attention 步骤/证据 > 首个 next 步骤/证据 > 第一步。 */
export function selectPrimaryReadyAction(input: PrimaryActionInput): ReadyCheckPrimaryActionItem {
  if (input.collectionFailureSummary !== null) {
    return {
      key: 'collection-failure',
      status: 'attention',
      titleKey: `${DETAILS}.readyCheckCollectionWarningTitle`,
      actionKey: `${DETAILS}.accessGuideDiagnosticRefresh`,
      action: input.refresh,
      summary: input.collectionFailureSummary
    }
  }
  if (input.firstDeviceReady) {
    return {
      key: 'first-device-automation',
      status: 'next',
      titleKey: `${DETAILS}.readyCheckFirstDeviceNextTitle`,
      actionKey: 'custom.automation.createFirstTelemetryRule',
      action: input.openFirstDeviceAutomation,
      summary: input.t(`${DETAILS}.readyCheckFirstDeviceNextDesc`)
    }
  }
  const { steps, evidenceActions } = input
  return (
    steps.find((step) => step.status === 'attention') ??
    evidenceActions.find((card) => card.status === 'attention') ??
    steps.find((step) => step.status === 'next') ??
    evidenceActions.find((card) => card.status === 'next') ??
    steps[0]
  )
}
