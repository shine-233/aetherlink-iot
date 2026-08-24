import { buildHttpCommands, buildMaskedCredentialMqttCommand, buildMqttCommands } from './device-access-guide-command-test-code'
import {
  findConnectInfoValue,
  inferProtocol,
  isHttpEndpoint,
  isJsonLike,
  splitMqttEndpoint
} from './device-access-guide-endpoint-state'

export { splitMqttEndpoint } from './device-access-guide-endpoint-state'

// 凭证哈希 Phase 2a（references/backend-hardening-plan.md 车道1）：设备详情响应自本批次起
// 只返回 MaskVoucher 掩码（前 10 字符 + …）。前端以「voucher_masked 标记」或「值以 … 结尾」
// 判定脱敏态，凭证解析进入显式 unavailable 分支而不是抛错/静默空对象。
export const MASKED_VOUCHER_SUFFIX = '…'

export const isMaskedVoucherText = (value: unknown): boolean =>
  typeof value === 'string' && value.trimEnd().endsWith(MASKED_VOUCHER_SUFFIX)

export type DeviceCredentialAvailability =
  | { status: 'ok'; credentials: Record<string, unknown> }
  | { status: 'unavailable'; reason: 'masked' }

// parseDeviceVoucherPayload 解析详情响应里的 voucher 字符串：
//   - 掩码形态（以 … 结尾）→ 显式 unavailable/masked，调用方进入"凭证已脱敏"降级 UI；
//   - 合法/非法 JSON → 沿用既有宽松语义返回 credentials（解析失败为空对象，不抛错）。
export const parseDeviceVoucherPayload = (raw: string | undefined | null): DeviceCredentialAvailability => {
  const text = String(raw || '')
  if (isMaskedVoucherText(text)) {
    return { status: 'unavailable', reason: 'masked' }
  }
  try {
    const parsed = JSON.parse(text || '{}') as Record<string, unknown>
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed)
      ? { status: 'ok', credentials: parsed }
      : { status: 'ok', credentials: {} }
  } catch {
    return { status: 'ok', credentials: {} }
  }
}

export type DeviceAccessGuideCommand = {
  titleKey: string
  language: string
  code: string
}

export type DeviceAccessGuideCheck = {
  titleKey: string
  descriptionKey: string
}

export type DeviceAccessGuideQuickstartStep = {
  titleKey: string
  descriptionKey: string
  copyText?: string
  copyLabelKey?: string
  actionKey?: string
}

export type DeviceAccessGuideDiagnosticItem = {
  labelKey: string
  value?: string
  valueKey?: string
  tone: 'success' | 'warning' | 'danger' | 'neutral'
}

export type ReadyCheckEvidenceMetric = {
  key: string
  labelKey: string
  value: string
  tone: 'success' | 'warning' | 'danger' | 'neutral'
}

export type ReadyCheckEvidenceCard = {
  key: 'twin' | 'command'
  titleKey: string
  descriptionKey: string
  boundaryKey: string
  status: 'ready' | 'attention' | 'next'
  statusKey: string
  summary: string
  summaryKey?: string
  metrics: ReadyCheckEvidenceMetric[]
  nextActions: string[]
  nextActionKeys?: string[]
}

export type DeviceAccessGuideDiagnosticsSummary = {
  isOnline?: boolean
  ready?: boolean
  readyLevel?: 'ok' | 'warning' | 'error' | 'unknown' | string
  readyCode?: string
  readySummary?: string
  readyNextActions?: string[]
  latestTelemetryKey?: string
  latestTelemetryAt?: string
  latestTelemetryValue?: unknown
  telemetryCurrentCount?: number
  debugEnabled?: boolean
  recentLogCount?: number
  conclusionLevel?: 'ok' | 'warning' | 'error' | 'unknown' | string
  conclusionCode?: string
  conclusionSummary?: string
  nextActions?: string[]
  latestIssue?: string
  partialWarnings?: string[]
}

export type DeviceAccessGuideState = {
  protocol: string
  authMode: string
  endpoint: string
  endpointKind: 'mqtt' | 'http'
  clientId: string
  username: string
  password: string
  reportTopic: string
  controlTopic: string
  payload: string
  lastError: string
  // credentialsUnavailable=true 表示凭证在服务端已脱敏（Phase 2a）：
  // state 不携带任何可用明文，测试命令为占位提示命令。
  credentialsUnavailable: boolean
  diagnostics: DeviceAccessGuideDiagnosticItem[]
  tlsHintKey: string
  quickstartSteps: DeviceAccessGuideQuickstartStep[]
  commands: DeviceAccessGuideCommand[]
  checks: DeviceAccessGuideCheck[]
  sdkBundle: string
}

export type DeviceConnectionGuideStateInput = {
  evaluated_at?: string
  access?: {
    protocol?: string
    credential_mode?: string
    connection_info?: unknown
    connection_profile?: {
      protocol?: string
      endpoint?: string
      host?: string
      port?: string
      tls_enabled?: boolean
      credential_mode?: string
      device_type?: string
      device_number?: string
      client_id?: string
      username?: string
      telemetry_topic?: string
      command_topic?: string
      test_payload?: string
      sample_payload?: string
      http_address?: string
      sub_topic_prefix?: string
    }
    tls?: {
      enabled?: boolean
    }
  }
  readiness?: {
    level?: string
    code?: string
    summary?: string
    online?: boolean
    ready?: boolean
    latest_telemetry_at?: string
    next_actions?: string[]
    evidence?: string[]
  }
  last_connection_error?: {
    code?: string
    summary?: string
    evidence?: string[]
  } | null
  command_summary?: {
    level?: string
    code?: string
    summary?: string
    latest_status?: string
    latest_message_id?: string
    next_actions?: string[]
  } | null
  twin_summary?: {
    desiredCount?: number
    reportedCount?: number
    matchedCount?: number
    deltaCount?: number
    unavailableCount?: number
  } | null
  partial_results?: Array<{
    component?: string
    reason?: string
  }>
  next_steps?: Array<{
    key?: string
    title?: string
    description?: string
    status?: string
  }>
}

const normalizeDiagnosticsInput = (
  diagnostics: DeviceAccessGuideDiagnosticsSummary | string
): DeviceAccessGuideDiagnosticsSummary =>
  typeof diagnostics === 'string'
    ? {
        latestIssue: diagnostics
      }
    : diagnostics

const conclusionValueKeyByCode: Record<string, string> = {
  no_evidence: 'custom.device_details.accessGuideDiagnosticConclusionNoEvidence',
  recent_debug_error: 'custom.device_details.accessGuideDiagnosticConclusionDebugError',
  recent_failure: 'custom.device_details.accessGuideDiagnosticConclusionRecentFailure',
  offline: 'custom.device_details.accessGuideDiagnosticConclusionOffline',
  partial_evidence: 'custom.device_details.accessGuideDiagnosticConclusionPartial',
  online: 'custom.device_details.accessGuideDiagnosticConclusionOnline'
}

const nextActionValueKeyByCode: Record<string, string> = {
  no_evidence: 'custom.device_details.accessGuideDiagnosticNextActionNoEvidence',
  recent_debug_error: 'custom.device_details.accessGuideDiagnosticNextActionDebugError',
  recent_failure: 'custom.device_details.accessGuideDiagnosticNextActionRecentFailure',
  offline: 'custom.device_details.accessGuideDiagnosticNextActionOffline',
  partial_evidence: 'custom.device_details.accessGuideDiagnosticNextActionPartial',
  online: 'custom.device_details.accessGuideDiagnosticNextActionOnline'
}

const readyValueKeyByCode: Record<string, string> = {
  ready: 'custom.device_details.accessGuideReadyCheckReady',
  no_current_telemetry: 'custom.device_details.accessGuideReadyCheckNoTelemetry',
  offline: 'custom.device_details.accessGuideReadyCheckOffline',
  no_diagnostics: 'custom.device_details.accessGuideReadyCheckUnknown'
}

const evidenceStatusFromLevel = (level?: string): ReadyCheckEvidenceCard['status'] => {
  if (level === 'ok') return 'ready'
  if (level === 'error' || level === 'warning') return 'attention'
  return 'next'
}

const numberText = (value: unknown) => (Number.isFinite(Number(value)) ? String(Number(value)) : '0')

const metricTone = (value: unknown, goodWhenZero = false): ReadyCheckEvidenceMetric['tone'] => {
  const count = Number(value || 0)
  if (goodWhenZero) return count > 0 ? 'warning' : 'success'
  return count > 0 ? 'success' : 'neutral'
}

export const buildReadyCheckEvidenceCards = (
  guide: DeviceConnectionGuideStateInput | null | undefined
): ReadyCheckEvidenceCard[] => {
  const twin = guide?.twin_summary
  const command = guide?.command_summary
  const partialResults = Array.isArray(guide?.partial_results) ? guide.partial_results : []
  const twinPartialResults = partialResults.filter((warning) => warning.component === 'device_twin')
  const commandPartialResults = partialResults.filter((warning) => warning.component === 'command_delivery')
  const twinPartialMessages = twinPartialResults.map(
    (warning) => `${warning.component || 'device_twin'}: ${warning.reason || 'partial_result'}`
  )
  const commandPartialMessages = commandPartialResults.map(
    (warning) => `${warning.component || 'command_delivery'}: ${warning.reason || 'partial_result'}`
  )
  const twinDelta = Number(twin?.deltaCount || 0)
  const twinUnavailable = Number(twin?.unavailableCount || 0)
  const hasTwinEvidence = Boolean(twin && (twin.desiredCount || twin.reportedCount || twin.matchedCount))
  const twinStatus: ReadyCheckEvidenceCard['status'] =
    twinDelta > 0 || twinUnavailable > 0 || twinPartialResults.length ? 'attention' : hasTwinEvidence ? 'ready' : 'next'
  const commandStatus = commandPartialResults.length ? 'attention' : evidenceStatusFromLevel(command?.level)

  return [
    {
      key: 'twin',
      titleKey: 'custom.device_details.readyCheckTwinTitle',
      descriptionKey: 'custom.device_details.readyCheckTwinDesc',
      boundaryKey: 'custom.device_details.readyCheckTwinBoundary',
      status: twinStatus,
      statusKey:
        twinStatus === 'ready'
          ? 'custom.device_details.readyCheckEvidenceSynced'
          : twinStatus === 'attention'
            ? 'custom.device_details.readyCheckEvidenceNeedsReview'
            : 'custom.device_details.readyCheckEvidenceUnknown',
      summary: '',
      summaryKey: hasTwinEvidence
        ? twinDelta > 0 || twinUnavailable > 0
          ? 'custom.device_details.readyCheckTwinSummaryNeedsReview'
          : 'custom.device_details.readyCheckTwinSummarySynced'
        : 'custom.device_details.readyCheckTwinSummaryUnknown',
      metrics: [
        {
          key: 'desired',
          labelKey: 'custom.device_details.twinDesired',
          value: numberText(twin?.desiredCount),
          tone: metricTone(twin?.desiredCount)
        },
        {
          key: 'reported',
          labelKey: 'custom.device_details.twinReported',
          value: numberText(twin?.reportedCount),
          tone: metricTone(twin?.reportedCount)
        },
        {
          key: 'matched',
          labelKey: 'custom.device_details.readyCheckTwinMatched',
          value: numberText(twin?.matchedCount),
          tone: metricTone(twin?.matchedCount)
        },
        {
          key: 'delta',
          labelKey: 'custom.device_details.readyCheckTwinDelta',
          value: numberText(twin?.deltaCount),
          tone: metricTone(twin?.deltaCount, true)
        },
        {
          key: 'unavailable',
          labelKey: 'custom.device_details.readyCheckTwinUnavailable',
          value: numberText(twin?.unavailableCount),
          tone: metricTone(twin?.unavailableCount, true)
        }
      ],
      nextActions: twinPartialMessages,
      nextActionKeys:
        twinStatus === 'attention'
          ? ['custom.device_details.readyCheckTwinNextReview']
          : twinStatus === 'next'
            ? ['custom.device_details.readyCheckTwinNextCreateDesired']
            : []
    },
    {
      key: 'command',
      titleKey: 'custom.device_details.readyCheckCommandTitle',
      descriptionKey: 'custom.device_details.readyCheckCommandDesc',
      boundaryKey: 'custom.device_details.readyCheckCommandBoundary',
      status: commandStatus,
      statusKey:
        commandStatus === 'ready'
          ? 'custom.device_details.readyCheckEvidenceConfirmed'
          : commandStatus === 'attention'
            ? 'custom.device_details.readyCheckEvidenceNeedsReview'
            : 'custom.device_details.readyCheckEvidenceUnknown',
      summary: command?.summary || '',
      summaryKey: command?.summary ? undefined : 'custom.device_details.readyCheckCommandSummaryUnknown',
      metrics: [
        {
          key: 'latest-status',
          labelKey: 'custom.device_details.readyCheckCommandLatestStatus',
          value: command?.latest_status || '--',
          tone: command?.latest_status ? commandStatus === 'attention' ? 'warning' : 'success' : 'neutral'
        },
        {
          key: 'message-id',
          labelKey: 'custom.device_details.messageId',
          value: command?.latest_message_id || '--',
          tone: command?.latest_message_id ? 'success' : 'neutral'
        },
        {
          key: 'code',
          labelKey: 'custom.device_details.readyCheckCommandCode',
          value: command?.code || '--',
          tone: command?.code ? (commandStatus === 'attention' ? 'warning' : 'success') : 'neutral'
        }
      ],
      nextActions: [...(Array.isArray(command?.next_actions) ? command.next_actions : []), ...commandPartialMessages]
    }
  ]
}

const buildDiagnosticItems = (
  diagnostics: DeviceAccessGuideDiagnosticsSummary,
  lastError: string
): DeviceAccessGuideDiagnosticItem[] => {
  const onlineTone = diagnostics.isOnline === undefined ? 'neutral' : diagnostics.isOnline ? 'success' : 'warning'
  const debugTone =
    diagnostics.debugEnabled === undefined ? 'neutral' : diagnostics.debugEnabled ? 'success' : 'warning'
  const conclusionTone =
    diagnostics.conclusionLevel === 'ok'
      ? 'success'
      : diagnostics.conclusionLevel === 'error'
        ? 'danger'
        : diagnostics.conclusionLevel === 'warning'
          ? 'warning'
          : 'neutral'
  const partialWarnings = diagnostics.partialWarnings?.filter(Boolean) || []
  const nextActions = diagnostics.nextActions?.filter(Boolean) || []
  const readyNextActions = diagnostics.readyNextActions?.filter(Boolean) || []
  const conclusionValueKey = diagnostics.conclusionCode
    ? conclusionValueKeyByCode[diagnostics.conclusionCode]
    : undefined
  const nextActionValueKey = diagnostics.conclusionCode
    ? nextActionValueKeyByCode[diagnostics.conclusionCode]
    : undefined
  const readyValueKey = diagnostics.readyCode ? readyValueKeyByCode[diagnostics.readyCode] : undefined
  const readyTone =
    diagnostics.readyLevel === 'ok'
      ? 'success'
      : diagnostics.readyLevel === 'error'
        ? 'danger'
        : diagnostics.readyLevel === 'warning'
          ? 'warning'
          : diagnostics.ready === true
            ? 'success'
            : diagnostics.ready === false
              ? 'warning'
              : 'neutral'
  const latestTelemetryValue =
    diagnostics.latestTelemetryKey && diagnostics.latestTelemetryAt
      ? `${diagnostics.latestTelemetryKey} @ ${diagnostics.latestTelemetryAt}`
      : diagnostics.latestTelemetryKey || ''

  return [
    {
      labelKey: 'custom.device_details.accessGuideReadyCheck',
      value: readyValueKey ? '' : diagnostics.readySummary || '',
      valueKey:
        readyValueKey || (diagnostics.readySummary ? undefined : 'custom.device_details.accessGuideDiagnosticUnknown'),
      tone: readyTone
    },
    {
      labelKey: 'custom.device_details.accessGuideLatestTelemetry',
      value: latestTelemetryValue,
      valueKey: latestTelemetryValue ? undefined : 'custom.device_details.accessGuideLatestTelemetryEmpty',
      tone: diagnostics.latestTelemetryKey ? 'success' : 'warning'
    },
    {
      labelKey: 'custom.device_details.accessGuideDiagnosticConclusion',
      value: conclusionValueKey ? '' : diagnostics.conclusionSummary || '',
      valueKey:
        conclusionValueKey ||
        (diagnostics.conclusionSummary ? undefined : 'custom.device_details.accessGuideDiagnosticUnknown'),
      tone: conclusionTone
    },
    {
      labelKey: 'custom.device_details.accessGuideDiagnosticOnline',
      valueKey:
        diagnostics.isOnline === undefined
          ? 'custom.device_details.accessGuideDiagnosticUnknown'
          : diagnostics.isOnline
            ? 'custom.device_details.online'
            : 'custom.device_details.offline',
      tone: onlineTone
    },
    {
      labelKey: 'custom.device_details.accessGuideDiagnosticDebug',
      valueKey:
        diagnostics.debugEnabled === undefined
          ? 'custom.device_details.accessGuideDiagnosticUnknown'
          : diagnostics.debugEnabled
            ? 'custom.device_details.accessGuideDiagnosticDebugOn'
            : 'custom.device_details.accessGuideDiagnosticDebugOff',
      tone: debugTone
    },
    {
      labelKey: 'custom.device_details.accessGuideDiagnosticRecentLogs',
      value: diagnostics.recentLogCount === undefined ? '' : String(diagnostics.recentLogCount),
      valueKey:
        diagnostics.recentLogCount === undefined ? 'custom.device_details.accessGuideDiagnosticUnknown' : undefined,
      tone: diagnostics.recentLogCount ? 'neutral' : 'warning'
    },
    {
      labelKey: 'custom.device_details.accessGuideDiagnosticCurrentIssue',
      value: lastError,
      valueKey: lastError ? undefined : 'custom.device_details.accessGuideLastErrorEmpty',
      tone: lastError ? 'danger' : 'success'
    },
    {
      labelKey: 'custom.device_details.accessGuideDiagnosticNextActions',
      value: nextActionValueKey ? '' : (readyNextActions.length ? readyNextActions : nextActions).join('; '),
      valueKey:
        nextActionValueKey ||
        (readyNextActions.length || nextActions.length
          ? undefined
          : 'custom.device_details.accessGuideDiagnosticUnknown'),
      tone: readyNextActions.length ? readyTone : nextActionValueKey || nextActions.length ? conclusionTone : 'neutral'
    },
    {
      labelKey: 'custom.device_details.accessGuideDiagnosticPartial',
      value: partialWarnings.join('; '),
      valueKey: partialWarnings.length ? undefined : 'custom.device_details.accessGuideDiagnosticComplete',
      tone: partialWarnings.length ? 'warning' : 'success'
    }
  ]
}

const buildCredentialCopyText = (guide: {
  endpointKind: 'mqtt' | 'http'
  clientId: string
  username: string
  password: string
}) => {
  if (guide.endpointKind === 'http') return `token=${guide.username}`

  return [
    `clientId=${guide.clientId}`,
    `username=${guide.username}`,
    guide.password ? `password=${guide.password}` : ''
  ]
    .filter(Boolean)
    .join('\n')
}

const buildQuickstartSteps = (guide: {
  endpointKind: 'mqtt' | 'http'
  endpoint: string
  clientId: string
  username: string
  password: string
  commands: DeviceAccessGuideCommand[]
}): DeviceAccessGuideQuickstartStep[] => [
  {
    titleKey: 'custom.device_details.accessGuideQuickstartEndpoint',
    descriptionKey:
      guide.endpointKind === 'http'
        ? 'custom.device_details.accessGuideQuickstartEndpointHttpDesc'
        : 'custom.device_details.accessGuideQuickstartEndpointMqttDesc',
    copyText: guide.endpoint,
    copyLabelKey: 'custom.device_details.accessGuideQuickstartCopyEndpoint'
  },
  {
    titleKey: 'custom.device_details.accessGuideQuickstartCredential',
    descriptionKey:
      guide.endpointKind === 'http'
        ? 'custom.device_details.accessGuideQuickstartCredentialHttpDesc'
        : 'custom.device_details.accessGuideQuickstartCredentialMqttDesc',
    copyText: buildCredentialCopyText(guide),
    copyLabelKey: 'custom.device_details.accessGuideQuickstartCopyCredential'
  },
  {
    titleKey: 'custom.device_details.accessGuideQuickstartPublish',
    descriptionKey: 'custom.device_details.accessGuideQuickstartPublishDesc',
    copyText: guide.commands[0]?.code,
    copyLabelKey: 'custom.device_details.accessGuideQuickstartCopyTestCommand'
  },
  {
    titleKey: 'custom.device_details.accessGuideQuickstartVerify',
    descriptionKey: 'custom.device_details.accessGuideQuickstartVerifyDesc',
    actionKey: 'custom.device_details.accessGuideNextStepRunReadyCheck'
  }
]

const buildSdkBundle = (guide: {
  protocol: string
  authMode: string
  endpoint: string
  endpointKind: 'mqtt' | 'http'
  clientId: string
  username: string
  password: string
  reportTopic: string
  controlTopic: string
  payload: string
  tlsHintKey: string
  lastError: string
  commands: DeviceAccessGuideCommand[]
}) => {
  const sections = [
    '# AetherLink 设备接入包',
    '',
    '## 连接参数',
    `protocol=${guide.protocol}`,
    `authMode=${guide.authMode}`,
    `endpoint=${guide.endpoint}`,
    `clientId=${guide.clientId}`,
    `username=${guide.username}`,
    guide.password ? `password=${guide.password}` : 'password=<空>',
    guide.endpointKind === 'mqtt' ? `reportTopic=${guide.reportTopic}` : '',
    guide.endpointKind === 'mqtt' ? `controlTopic=${guide.controlTopic}` : '',
    `tlsHint=${guide.tlsHintKey}`,
    guide.lastError ? `lastConnectionError=${guide.lastError}` : 'lastConnectionError=<无>',
    '',
    '## 上报载荷',
    guide.payload,
    '',
    '## 可运行测试命令',
    ...guide.commands.flatMap((command) => [
      '',
      `### ${command.language}`,
      '```',
      command.code,
      '```'
    ]),
    '',
    '## 接入检查清单',
    '- 确认主机、端口、凭证、主题和 TLS 模式。',
    '- 发布一条遥测数据，然后刷新 Ready Check。',
    '- 如果仍然离线，请启用调试证据，并从设备所在网络重试。'
  ]

  return sections.filter((line) => line !== '').join('\n')
}

const normalizeRecord = (value: unknown): Record<string, unknown> =>
  value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {}

const connectInfoFromConnectionProfile = (
  profile: NonNullable<NonNullable<DeviceConnectionGuideStateInput['access']>['connection_profile']> | undefined
) => {
  if (!profile) return {}

  const endpoint = profile.http_address || profile.endpoint || ''
  const connectInfo: Record<string, unknown> = {
    protocol: profile.protocol,
    endpoint,
    host: profile.host,
    port: profile.port,
    client_id: profile.client_id || profile.username,
    username: profile.username,
    report_topic: profile.telemetry_topic,
    control_topic: profile.command_topic,
    test_payload: profile.test_payload,
    sample_payload: profile.sample_payload,
    sub_topic_prefix: profile.sub_topic_prefix,
    tls_enabled: profile.tls_enabled
  }

  return Object.fromEntries(Object.entries(connectInfo).filter(([, value]) => value !== undefined && value !== ''))
}

const summarizeConnectionGuideDiagnostics = (
  guide: DeviceConnectionGuideStateInput,
  fallbackDiagnostics: DeviceAccessGuideDiagnosticsSummary
): DeviceAccessGuideDiagnosticsSummary => {
  const partialWarnings =
    guide.partial_results?.map((warning) => `${warning.component || 'guide'}: ${warning.reason || 'partial'}`) || []
  const readinessNextActions = Array.isArray(guide.readiness?.next_actions) ? guide.readiness.next_actions : []
  const commandNextActions = Array.isArray(guide.command_summary?.next_actions) ? guide.command_summary.next_actions : []

  return {
    ...fallbackDiagnostics,
    isOnline: guide.readiness?.online ?? fallbackDiagnostics.isOnline,
    ready: guide.readiness?.ready ?? fallbackDiagnostics.ready,
    readyLevel: guide.readiness?.level || fallbackDiagnostics.readyLevel,
    readyCode: guide.readiness?.code || fallbackDiagnostics.readyCode,
    readySummary: guide.readiness?.summary || fallbackDiagnostics.readySummary,
    readyNextActions: readinessNextActions.length ? readinessNextActions : fallbackDiagnostics.readyNextActions,
    latestTelemetryAt: guide.readiness?.latest_telemetry_at || fallbackDiagnostics.latestTelemetryAt,
    conclusionLevel: guide.command_summary?.level || fallbackDiagnostics.conclusionLevel,
    conclusionCode: guide.command_summary?.code || fallbackDiagnostics.conclusionCode,
    conclusionSummary: guide.command_summary?.summary || fallbackDiagnostics.conclusionSummary,
    nextActions: commandNextActions.length ? commandNextActions : fallbackDiagnostics.nextActions,
    latestIssue: guide.last_connection_error?.summary || fallbackDiagnostics.latestIssue,
    partialWarnings: partialWarnings.length
      ? [...(fallbackDiagnostics.partialWarnings || []), ...partialWarnings]
      : fallbackDiagnostics.partialWarnings
  }
}

export type DeviceAccessGuideStateOptions = {
  // credentialsUnavailable：详情响应的凭证为掩码形态（voucher_masked / … 结尾）。
  // true 时测试命令降级为占位提示命令，authMode 标记 UNAVAILABLE，不再生成可执行凭证明文。
  credentialsUnavailable?: boolean
}

const MASKED_AUTH_MODE = 'UNAVAILABLE (MASKED)'

export const buildDeviceAccessGuideState = (
  connectInfo: Record<string, unknown>,
  deviceNumber: string,
  credentials: Record<string, unknown> = {},
  diagnosticsInput: DeviceAccessGuideDiagnosticsSummary | string = '',
  options: DeviceAccessGuideStateOptions = {}
): DeviceAccessGuideState => {
  const credentialsUnavailable = options.credentialsUnavailable === true
  const diagnostics = normalizeDiagnosticsInput(diagnosticsInput)
  const endpoint = findConnectInfoValue(connectInfo, [
    (key, value) =>
      key.includes('server') ||
      key.includes('address') ||
      key.includes('endpoint') ||
      key.includes('接入') ||
      /^mqtts?:\/\//i.test(value) ||
      /^[^:/\s]+:\d{2,5}$/.test(value) ||
      isHttpEndpoint(value),
    (_key, value) => /^:?(\d{2,5})$/.test(value)
  ])
  const clientId = findConnectInfoValue(connectInfo, [
    (key, value) =>
      key.includes('clientid') || key.includes('client id') || key.includes('client_id') || value.startsWith('mqtt_')
  ])
  const username =
    findConnectInfoValue(credentials, [
      (key) => key.includes('username') || key === 'user' || key.includes('access_token') || key.includes('token')
    ]) || findConnectInfoValue(connectInfo, [(key) => key.includes('username') || key === 'user'])
  const password =
    findConnectInfoValue(credentials, [(key) => key.includes('password') || key.includes('secret')]) ||
    findConnectInfoValue(connectInfo, [(key) => key.includes('password') || key.includes('secret')])
  const reportTopic = findConnectInfoValue(connectInfo, [
    (key, value) =>
      key.includes('report') || (value.includes('telemetry') && !value.includes('control') && !isJsonLike(value))
  ])
  const controlTopic = findConnectInfoValue(connectInfo, [
    (key, value) => key.includes('control') || value.includes('/control/')
  ])
  const payload =
    findConnectInfoValue(connectInfo, [
      (key, value) => key.includes('payload') || key.includes('remark') || isJsonLike(value)
    ]) || '{"temperature": 26, "humidity": 60}'
  const lastError =
    diagnostics.latestIssue ||
    findConnectInfoValue(connectInfo, [
      (key) =>
        key.includes('last_error') || key.includes('last error') || key.includes('error_message') || key === 'error'
    ])
  const protocol = inferProtocol(connectInfo, endpoint)
  const endpointKind = protocol === 'HTTP' ? 'http' : 'mqtt'
  const mqtt = splitMqttEndpoint(endpoint)
  const safeDeviceNumber = deviceNumber || '<device-number>'
  const safeReportTopic = reportTopic || 'devices/telemetry'
  const safeControlTopic = controlTopic || `devices/telemetry/control/${safeDeviceNumber}`
  const safeUsername = username || (endpointKind === 'http' ? '<access-token>' : '<mqtt-username>')
  // 脱敏态下 state 一律不携带明文口令（credentials 里即便混入也强制丢弃）。
  const safePassword = credentialsUnavailable ? '' : password || ''
  const safeClientId = clientId || safeUsername || safeDeviceNumber
  const safeEndpoint = endpoint || (endpointKind === 'http' ? '<http-endpoint>' : `${mqtt.host}:${mqtt.port}`)
  const authMode = credentialsUnavailable ? MASKED_AUTH_MODE : safePassword ? 'BASIC' : 'ACCESS TOKEN / USERNAME'
  const commands = credentialsUnavailable
    ? [
        buildMaskedCredentialMqttCommand({
          host: mqtt.host,
          port: mqtt.port,
          clientId: safeClientId,
          username: safeUsername,
          reportTopic: safeReportTopic,
          payload
        })
      ]
    : endpointKind === 'http'
      ? buildHttpCommands(isHttpEndpoint(safeEndpoint) ? safeEndpoint : '<http-endpoint>', safeUsername, payload)
      : buildMqttCommands({
          endpoint: safeEndpoint,
          host: mqtt.host,
          port: mqtt.port,
          clientId: safeClientId,
          username: safeUsername,
          password: safePassword,
          reportTopic: safeReportTopic,
          controlTopic: safeControlTopic,
          payload
        })
  const tlsHintKey =
    safeEndpoint.startsWith('https://') || safeEndpoint.startsWith('mqtts://') || mqtt.port === '8883'
      ? 'custom.device_details.accessGuideTlsEnabled'
      : 'custom.device_details.accessGuideTlsDefault'

  return {
    protocol,
    authMode,
    endpoint: safeEndpoint,
    endpointKind,
    clientId: safeClientId,
    username: safeUsername,
    password: safePassword,
    reportTopic: safeReportTopic,
    controlTopic: safeControlTopic,
    payload,
    lastError,
    credentialsUnavailable,
    diagnostics: buildDiagnosticItems(diagnostics, lastError),
    tlsHintKey,
    quickstartSteps: buildQuickstartSteps({
      endpointKind,
      endpoint: safeEndpoint,
      clientId: safeClientId,
      username: safeUsername,
      password: safePassword,
      commands
    }).map((step) =>
      // 脱敏态不提供"复制凭证"入口：凭证不可见也不可复制，仅保留步骤说明。
      credentialsUnavailable && step.titleKey === 'custom.device_details.accessGuideQuickstartCredential'
        ? { ...step, copyText: undefined, copyLabelKey: undefined }
        : step
    ),
    commands,
    checks: [
      {
        titleKey: 'custom.device_details.accessGuideCheckCredential',
        descriptionKey: 'custom.device_details.accessGuideCheckCredentialDesc'
      },
      {
        titleKey: 'custom.device_details.accessGuideCheckTelemetry',
        descriptionKey: 'custom.device_details.accessGuideCheckTelemetryDesc'
      },
      {
        titleKey: 'custom.device_details.accessGuideCheckError',
        descriptionKey: 'custom.device_details.accessGuideCheckErrorDesc'
      }
    ],
    sdkBundle: buildSdkBundle({
      protocol,
      authMode,
      endpoint: safeEndpoint,
      endpointKind,
      clientId: safeClientId,
      username: safeUsername,
      password: safePassword,
      reportTopic: safeReportTopic,
      controlTopic: safeControlTopic,
      payload,
      tlsHintKey,
      lastError,
      commands
    })
  }
}

export const buildDeviceAccessGuideStateFromConnectionGuide = (
  guide: DeviceConnectionGuideStateInput | null | undefined,
  deviceNumber: string,
  credentials: Record<string, unknown> = {},
  fallbackConnectInfo: Record<string, unknown> = {},
  fallbackDiagnosticsInput: DeviceAccessGuideDiagnosticsSummary | string = '',
  options: DeviceAccessGuideStateOptions = {}
): DeviceAccessGuideState => {
  const profileConnectInfo = connectInfoFromConnectionProfile(guide?.access?.connection_profile)
  const guideConnectInfo = normalizeRecord(guide?.access?.connection_info)
  const fallbackDiagnostics = normalizeDiagnosticsInput(fallbackDiagnosticsInput)
  const state = buildDeviceAccessGuideState(
    Object.keys(profileConnectInfo).length
      ? profileConnectInfo
      : Object.keys(guideConnectInfo).length
        ? guideConnectInfo
        : fallbackConnectInfo,
    deviceNumber,
    credentials,
    guide ? summarizeConnectionGuideDiagnostics(guide, fallbackDiagnostics) : fallbackDiagnostics,
    options
  )
  const tlsHintKey =
    guide?.access?.tls?.enabled === true
      ? 'custom.device_details.accessGuideTlsEnabled'
      : guide?.access?.tls?.enabled === false
        ? 'custom.device_details.accessGuideTlsDefault'
        : state.tlsHintKey

  return {
    ...state,
    protocol: guide?.access?.protocol || state.protocol,
    authMode: guide?.access?.credential_mode || state.authMode,
    tlsHintKey,
    sdkBundle: buildSdkBundle({
      protocol: guide?.access?.protocol || state.protocol,
      authMode: guide?.access?.credential_mode || state.authMode,
      endpoint: state.endpoint,
      endpointKind: state.endpointKind,
      clientId: state.clientId,
      username: state.username,
      password: state.password,
      reportTopic: state.reportTopic,
      controlTopic: state.controlTopic,
      payload: state.payload,
      tlsHintKey,
      lastError: state.lastError,
      commands: state.commands
    })
  }
}
