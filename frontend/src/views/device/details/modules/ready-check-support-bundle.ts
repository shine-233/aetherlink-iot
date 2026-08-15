import { formatReadyCheckDeepLink, type ReadyCheckDeepLink } from './ready-check-deep-links'
import type { ReadyCheckEvidenceCard } from './device-access-guide-state'

export type ReadyCheckSupportEvidenceCenterItem = {
  key: string
  labelKey: string
  value: string
  detail: string
}

export type ReadyCheckSupportBackendStep = {
  key: string
  title: string
  description: string
  status: string
}

export type ReadyCheckSupportCollectionFailure = {
  key: string
  labelKey: string
}

type ReadyCheckSupportDevice = {
  id: string
  name: string
  number: string
  online: boolean
  hasConnectionIdentity: boolean
  hasTemplate: boolean
}

type ReadyCheckSupportSource = {
  sourceKey: 'ota_failed_rollout' | 'home_first_device_onboarding' | 'command_job_diagnosis' | 'device_details'
  label: string
  detail: string
  otaTaskId: string
  otaDetailId: string
  commandJobId: string
  firstDeviceOnboarding: boolean
}

type ReadyCheckSupportReadiness = {
  ready: boolean | null
  level: string
  code: string
  summary: string
  evaluatedAt: string
  connectionGuideSummary: string
}

type ReadyCheckSupportTelemetry = {
  latest: string
  latestValue: string
  currentCount: number | null
}

type ReadyCheckSupportDiagnostics = {
  nextActions: string[]
  lastConnectionIssue: string
  partialResults: string
  conclusion: unknown
  debug: {
    enabled?: boolean
    recentLogs?: Array<any>
  }
  recentFailures: Array<any>
  partialWarnings: Array<any>
}

export type ReadyCheckSupportBundleInput = {
  t: (key: string) => string
  generatedAt?: string
  device: ReadyCheckSupportDevice
  source: ReadyCheckSupportSource
  readiness: ReadyCheckSupportReadiness
  telemetry: ReadyCheckSupportTelemetry
  diagnostics: ReadyCheckSupportDiagnostics
  evidenceCenterItems: ReadyCheckSupportEvidenceCenterItem[]
  evidenceCards: ReadyCheckEvidenceCard[]
  backendNextSteps: ReadyCheckSupportBackendStep[]
  deepLinks: ReadyCheckDeepLink[]
  collectionFailures: ReadyCheckSupportCollectionFailure[]
  boundaryText: string
}

const stringifyMeta = (meta: unknown) => {
  if (!meta) return ''
  try {
    return JSON.stringify(meta)
  } catch {
    return String(meta)
  }
}

const cardSummaryText = (card: ReadyCheckEvidenceCard, t: (key: string) => string) => {
  return card.summaryKey ? t(card.summaryKey) : card.summary
}

const cardNextActionTexts = (card: ReadyCheckEvidenceCard, t: (key: string) => string) => {
  return [...(card.nextActionKeys || []).map((key) => t(key)), ...card.nextActions]
}

const debugLogLines = (logs: Array<any>) =>
  logs.slice(0, 3).map((log, index) => {
    const meta = stringifyMeta(log.meta)
    return `${index + 1}. ${[log.direction, log.action, log.outcome, log.error, meta].filter(Boolean).join(' / ')}`
  })

const failureLines = (failures: Array<any>) =>
  failures.slice(0, 3).map((failure, index) => {
    return `${index + 1}. ${[failure.timestamp, failure.direction, failure.stage, failure.error]
      .filter(Boolean)
      .join(' / ')}`
  })

const warningLines = (warnings: Array<any>) =>
  warnings.slice(0, 3).map((warning, index) => {
    return `${index + 1}. ${warning.component || 'diagnostics'}: ${warning.reason || 'partial_result'}`
  })

const collectionFailureLines = (failures: ReadyCheckSupportCollectionFailure[], t: (key: string) => string) =>
  failures.map((failure, index) => `${index + 1}. ${failure.key}: ${t(failure.labelKey)}`)

export const buildReadyCheckDiagnosticMarkdown = (input: ReadyCheckSupportBundleInput) => {
  const t = input.t
  const debugLines = debugLogLines(input.diagnostics.debug.recentLogs || [])
  const recentFailureLines = failureLines(input.diagnostics.recentFailures || [])
  const partialWarningLines = warningLines(input.diagnostics.partialWarnings || [])
  const collectorFailureLines = collectionFailureLines(input.collectionFailures || [], t)

  return [
    '# AetherLink Ready Check 诊断摘要',
    '',
    '## 设备',
    `id=${input.device.id || '<empty>'}`,
    `name=${input.device.name}`,
    `number=${input.device.number}`,
    `online=${input.device.online}`,
    `hasConnectionIdentity=${input.device.hasConnectionIdentity}`,
    `hasTemplate=${input.device.hasTemplate}`,
    `source=${input.source.sourceKey}`,
    `otaTaskId=${input.source.otaTaskId || '<none>'}`,
    `otaDetailId=${input.source.otaDetailId || '<none>'}`,
    `commandJobId=${input.source.commandJobId || '<none>'}`,
    `sourceLabel=${input.source.label}`,
    `sourceDetail=${input.source.detail}`,
    `connectionGuideEvaluatedAt=${input.readiness.evaluatedAt || '<unknown>'}`,
    '',
    '## Ready Check',
    `ready=${input.readiness.ready ?? '<unknown>'}`,
    `level=${input.readiness.level || '<unknown>'}`,
    `code=${input.readiness.code || '<unknown>'}`,
    `summary=${input.readiness.summary}`,
    `latestTelemetry=${input.telemetry.latest}`,
    `latestTelemetryValue=${input.telemetry.latestValue}`,
    `telemetryCurrentCount=${input.telemetry.currentCount ?? '<unknown>'}`,
    `readinessSummary=${input.readiness.connectionGuideSummary}`,
    `lastConnectionIssue=${input.diagnostics.lastConnectionIssue}`,
    `partialResults=${input.diagnostics.partialResults}`,
    '',
    '## 设备影子证据',
    ...input.evidenceCards
      .filter((card) => card.key === 'twin')
      .flatMap((card) => [
        `status=${card.status}`,
        `summary=${cardSummaryText(card, t)}`,
        `boundary=${t(card.boundaryKey)}`,
        ...card.metrics.map((metric) => `${metric.key}=${metric.value}`),
        `nextActions=${cardNextActionTexts(card, t).length ? cardNextActionTexts(card, t).join(' | ') : '<none>'}`
      ]),
    '',
    '## 命令证据',
    ...input.evidenceCards
      .filter((card) => card.key === 'command')
      .flatMap((card) => [
        `status=${card.status}`,
        `summary=${cardSummaryText(card, t)}`,
        `boundary=${t(card.boundaryKey)}`,
        ...card.metrics.map((metric) => `${metric.key}=${metric.value}`),
        `nextActions=${cardNextActionTexts(card, t).length ? cardNextActionTexts(card, t).join(' | ') : '<none>'}`
      ]),
    '',
    '## 下一步动作',
    input.diagnostics.nextActions.length
      ? input.diagnostics.nextActions.map((action, index) => `${index + 1}. ${action}`).join('\n')
      : '<none>',
    '',
    '## 诊断结论',
    `level=${(input.diagnostics.conclusion as { level?: string } | null)?.level || '<unknown>'}`,
    `code=${(input.diagnostics.conclusion as { code?: string } | null)?.code || '<unknown>'}`,
    `summary=${(input.diagnostics.conclusion as { summary?: string } | null)?.summary || '<unknown>'}`,
    '',
    '## 调试证据',
    `debugEnabled=${input.diagnostics.debug.enabled ?? '<unknown>'}`,
    debugLines.length ? debugLines.join('\n') : '暂无近期调试日志。',
    '',
    '## 近期失败',
    recentFailureLines.length ? recentFailureLines.join('\n') : '暂无近期失败。',
    '',
    '## 部分诊断',
    partialWarningLines.length ? partialWarningLines.join('\n') : '暂无部分诊断告警。',
    '',
    '## 前端采集失败',
    collectorFailureLines.length
      ? collectorFailureLines.join('\n')
      : '本次刷新未发现前端采集器失败。',
    '',
    '## 后端建议步骤',
    input.backendNextSteps.length
      ? input.backendNextSteps
          .map((step, index) => `${index + 1}. [${step.status}] ${step.title} - ${step.description}`)
          .join('\n')
      : '后端未返回建议步骤。',
    '',
    '## 证据入口',
    input.deepLinks
      .map((link, index) => `${index + 1}. ${t(link.labelKey)}: ${formatReadyCheckDeepLink(link)} / ${t(link.boundaryKey)}`)
      .join('\n'),
    '',
    '## 证据边界',
    input.boundaryText
  ].join('\n')
}

export const buildReadyCheckSupportBundle = (input: ReadyCheckSupportBundleInput) => {
  const t = input.t
  const markdownSummary = buildReadyCheckDiagnosticMarkdown(input)

  return {
    schema: 'aetherlink.ready-check.diagnostics.v1',
    generated_at: input.generatedAt || new Date().toISOString(),
    device: input.device,
    source: input.source,
    readiness: input.readiness,
    telemetry: input.telemetry,
    diagnostics: input.diagnostics,
    evidenceCenter: input.evidenceCenterItems.map((item) => ({
      key: item.key,
      label: t(item.labelKey),
      value: item.value,
      detail: item.detail
    })),
    evidenceCards: input.evidenceCards.map((card) => ({
      key: card.key,
      status: card.status,
      title: t(card.titleKey),
      summary: cardSummaryText(card, t),
      boundary: t(card.boundaryKey),
      metrics: card.metrics.map((metric) => ({
        key: metric.key,
        label: t(metric.labelKey),
        value: metric.value,
        tone: metric.tone
      })),
      nextActions: cardNextActionTexts(card, t)
    })),
    backendNextSteps: input.backendNextSteps,
    deepLinks: input.deepLinks.map((link) => ({
      key: link.key,
      label: t(link.labelKey),
      url: formatReadyCheckDeepLink(link),
      boundary: t(link.boundaryKey)
    })),
    collectionFailures: (input.collectionFailures || []).map((failure) => ({
      key: failure.key,
      label: t(failure.labelKey)
    })),
    markdownSummary,
    evidenceBoundary: input.boundaryText
  }
}

export const readyCheckSupportFileName = (rawName: string) => {
  const safeName = String(rawName || 'device')
    .replace(/[^a-zA-Z0-9._-]/g, '_')
    .replace(/^_+|_+$/g, '')
  return `aetherlink-ready-check-${safeName || 'device'}-diagnostics.json`
}
