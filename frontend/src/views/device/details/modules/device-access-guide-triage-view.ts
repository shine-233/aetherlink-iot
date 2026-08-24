import type { DeviceDebugLogEntry, DeviceDebugStatus } from '@/service/api/device'
import type { DeviceAccessGuideState } from './device-access-guide-state'

type Translate = (key: any) => string

export type DeviceAccessGuideTriageView = {
  tone: 'success' | 'warning' | 'danger' | 'neutral'
  summary: string
  issue: string
  nextAction: string
  ready: string
  telemetry: string
  completeness: string
  primaryTestCommand: string
  debugEnabled: boolean
  latestDebugEvidence: string
}

export type DeviceAccessGuideAccessPacket = {
  schema: 'aetherlink.device.access-packet.v2'
  generatedAt: string
  credentialPolicy: {
    secretsRedacted: true
    shareRawCredentialsSeparately: true
    redactedFields: string[]
  }
  connection: {
    protocol: string
    authMode: string
    endpoint: string
    endpointKind: string
    clientId: string
    username: string
    password: string
    reportTopic: string
    controlTopic: string
    tlsHintKey: string
    payload: string
    lastConnectionError: string
  }
  diagnosis: {
    ready: string
    latestTelemetry: string
    conclusion: string
    currentIssue: string
    nextAction: string
    completeness: string
  }
  debug: {
    enabled: boolean
    expires: string
    remainingSeconds: number
    logs: Array<{
      title: string
      message: string
    }>
  }
  testCommands: Array<{
    order: number
    title: string
    language: string
    code: string
  }>
  firstRunnableTestCommand: string
  markdownPacket: string
  supportSummary: string
  verificationBoundary: {
    platformEvidenceOnly: true
    fieldConnectionNotProven: true
    message: string
  }
}

const diagnosticText = (accessGuide: DeviceAccessGuideState, labelKey: string, t: Translate) => {
  const item = accessGuide.diagnostics.find((diagnostic) => diagnostic.labelKey === labelKey)
  if (!item) return t('custom.device_details.accessGuideDiagnosticUnknown')

  return item.valueKey ? t(item.valueKey) : item.value || t('custom.device_details.accessGuideDiagnosticUnknown')
}

export const formatAccessGuideDebugTime = (value: unknown) => {
  if (!value) return '--'
  if (typeof value === 'number') return new Date(value * 1000).toLocaleString()
  return String(value)
}

export const formatAccessGuideDebugLogTitle = (log: DeviceDebugLogEntry) => {
  return [log.ts, log.direction, log.action || log.event || log.stage, log.outcome].filter(Boolean).join(' / ') || '--'
}

const debugMetaString = (log: DeviceDebugLogEntry, key: string) => {
  const value = log.meta?.[key]
  if (value === undefined || value === null || value === '') return ''
  return String(value)
}

const debugDiagnosticCodeMessageKey: Record<string, string> = {
  disconnect_error: 'custom.device_details.accessGuideDebugDisconnectError',
  disconnect_normal: 'custom.device_details.accessGuideDebugDisconnectNormal'
}

const formatDebugDiagnosticCode = (log: DeviceDebugLogEntry, t?: Translate) => {
  const code = debugMetaString(log, 'diagnostic_code')
  if (!code) return ''
  const messageKey = debugDiagnosticCodeMessageKey[code]
  if (messageKey && t) return `${t(messageKey)} (${code})`
  return `code=${code}`
}

export const formatAccessGuideDebugLogMessage = (log: DeviceDebugLogEntry, t?: Translate) => {
  const parts = [
    log.error,
    formatDebugDiagnosticCode(log, t),
    debugMetaString(log, 'topic') && `topic=${debugMetaString(log, 'topic')}`,
    debugMetaString(log, 'payload_size') && `payload=${debugMetaString(log, 'payload_size')}B`,
    debugMetaString(log, 'recommended_action') && `next=${debugMetaString(log, 'recommended_action')}`,
    log.stage,
    log.protocol
  ].filter(Boolean)
  return parts.join(' / ') || '--'
}

export const buildDeviceAccessGuideTriageView = (input: {
  accessGuide: DeviceAccessGuideState
  debugStatus?: DeviceDebugStatus
  debugLogs?: DeviceDebugLogEntry[]
  t: Translate
}): DeviceAccessGuideTriageView => {
  const diagnosticConclusion = input.accessGuide.diagnostics.find(
    (diagnostic) => diagnostic.labelKey === 'custom.device_details.accessGuideDiagnosticConclusion'
  )
  const latestDebugLog = input.debugLogs?.[0]

  return {
    tone: diagnosticConclusion?.tone || 'neutral',
    summary: diagnosticText(input.accessGuide, 'custom.device_details.accessGuideDiagnosticConclusion', input.t),
    issue: diagnosticText(input.accessGuide, 'custom.device_details.accessGuideDiagnosticCurrentIssue', input.t),
    nextAction: diagnosticText(input.accessGuide, 'custom.device_details.accessGuideDiagnosticNextActions', input.t),
    ready: diagnosticText(input.accessGuide, 'custom.device_details.accessGuideReadyCheck', input.t),
    telemetry: diagnosticText(input.accessGuide, 'custom.device_details.accessGuideLatestTelemetry', input.t),
    completeness: diagnosticText(input.accessGuide, 'custom.device_details.accessGuideDiagnosticPartial', input.t),
    primaryTestCommand: input.accessGuide.commands[0]?.code || '',
    debugEnabled: Boolean(input.debugStatus?.enabled),
    latestDebugEvidence: latestDebugLog ? formatAccessGuideDebugLogMessage(latestDebugLog, input.t) : '--'
  }
}

export const buildDeviceAccessGuideSupportSummary = (input: {
  accessGuide: DeviceAccessGuideState
  triageView: DeviceAccessGuideTriageView
  debugStatus?: DeviceDebugStatus
  debugLogs?: DeviceDebugLogEntry[]
  t?: Translate
}) => {
  const { accessGuide, triageView } = input
  const usernameIsSecret = accessGuide.endpointKind === 'http' || /token/i.test(accessGuide.authMode)
  const secretTokens = [accessGuide.password, usernameIsSecret ? accessGuide.username : ''].filter(Boolean)
  const testCommandLines = accessGuide.commands.map((command, index) => {
    const title = input.t ? input.t(command.titleKey) : command.titleKey
    return `${index + 1}. ${title} (${command.language || 'text'})`
  })
  const debugLogLines = (input.debugLogs || []).slice(0, 5).map((log, index) => {
    const title = formatAccessGuideDebugLogTitle(log)
    const message = formatAccessGuideDebugLogMessage(log, input.t)
    return `${index + 1}. ${title} | ${message}`
  })

  const summary = [
    '# AetherLink device access support summary',
    '',
    '## Connection',
    `protocol=${accessGuide.protocol}`,
    `authMode=${accessGuide.authMode}`,
    `endpoint=${accessGuide.endpoint}`,
    `clientId=${accessGuide.clientId}`,
    `username=${accessGuide.username}`,
    // 凭证哈希 Phase 2a：详情凭证已脱敏时明确标记来源掩码，避免被误读为"设备未配置密码"。
    accessGuide.credentialsUnavailable
      ? 'password=<masked-after-creation>'
      : accessGuide.password
        ? 'password=<provided>'
        : 'password=<empty>',
    accessGuide.endpointKind === 'mqtt' ? `reportTopic=${accessGuide.reportTopic}` : '',
    accessGuide.endpointKind === 'mqtt' ? `controlTopic=${accessGuide.controlTopic}` : '',
    `tlsHint=${accessGuide.tlsHintKey}`,
    '',
    '## Diagnosis',
    `ready=${triageView.ready}`,
    `latestTelemetry=${triageView.telemetry}`,
    `conclusion=${triageView.summary}`,
    `currentIssue=${triageView.issue}`,
    `nextAction=${triageView.nextAction}`,
    `diagnosticCompleteness=${triageView.completeness}`,
    '',
    '## Debug Evidence',
    `debugEnabled=${Boolean(input.debugStatus?.enabled)}`,
    input.debugStatus?.enabled ? `debugExpires=${formatAccessGuideDebugTime(input.debugStatus.expire_at)}` : '',
    input.debugStatus?.enabled ? `debugRemainingSeconds=${input.debugStatus.remaining_seconds || 0}` : '',
    debugLogLines.length ? debugLogLines.join('\n') : 'No recent broker debug logs.',
    '',
    '## Test Commands',
    `testCommandCount=${accessGuide.commands.length}`,
    `recommendedTestCommand=${testCommandLines[0] || '<none>'}`,
    testCommandLines.length ? testCommandLines.join('\n') : '<none>',
    '',
    '## First Runnable Test Command',
    triageView.primaryTestCommand || '<none>',
    '',
    '## Operator Checklist',
    '- Confirm broker host, port, credentials, topics, and TLS mode.',
    '- Publish one test telemetry message from the device network.',
    '- Refresh Ready Check and debug evidence after the device retries.',
    '- If online state or telemetry is still missing, attach this summary to the support ticket.'
  ]
    .filter((line) => line !== '')
    .join('\n')
  return redactText(summary, secretTokens)
}

const redactText = (value: string, secrets: string[]) =>
  secrets.reduce((text, secret) => {
    if (!secret) return text
    return text.split(secret).join('<redacted>')
  }, value)

const redactedCredential = (value: string, shouldRedact: boolean) => {
  if (!value) return ''
  return shouldRedact ? '<redacted>' : value
}

export const buildDeviceAccessGuideAccessPacket = (input: {
  accessGuide: DeviceAccessGuideState
  triageView: DeviceAccessGuideTriageView
  debugStatus?: DeviceDebugStatus
  debugLogs?: DeviceDebugLogEntry[]
  t: Translate
  generatedAt?: string
}): DeviceAccessGuideAccessPacket => {
  const { accessGuide, triageView } = input
  const usernameIsSecret = accessGuide.endpointKind === 'http' || /token/i.test(accessGuide.authMode)
  const redactedFields = [
    accessGuide.password ? 'connection.password' : '',
    usernameIsSecret && accessGuide.username ? 'connection.username' : '',
    accessGuide.password ? 'testCommands.code.password' : '',
    usernameIsSecret && accessGuide.username ? 'testCommands.code.username' : ''
  ].filter(Boolean)
  const secretTokens = [accessGuide.password, usernameIsSecret ? accessGuide.username : ''].filter(Boolean)
  const supportSummary = redactText(buildDeviceAccessGuideSupportSummary(input), secretTokens)

  return {
    schema: 'aetherlink.device.access-packet.v2',
    generatedAt: input.generatedAt || new Date().toISOString(),
    credentialPolicy: {
      secretsRedacted: true,
      shareRawCredentialsSeparately: true,
      // 脱敏态下 password 字段在源头即为掩码，仍列入 redactedFields 让消费方知道无明文可分享。
      redactedFields: accessGuide.credentialsUnavailable
        ? [...new Set([...redactedFields, 'connection.password'])]
        : redactedFields
    },
    connection: {
      protocol: accessGuide.protocol,
      authMode: accessGuide.authMode,
      endpoint: accessGuide.endpoint,
      endpointKind: accessGuide.endpointKind,
      clientId: accessGuide.clientId,
      username: redactedCredential(accessGuide.username, usernameIsSecret),
      password: accessGuide.credentialsUnavailable
        ? '<masked-after-creation>'
        : redactedCredential(accessGuide.password, true),
      reportTopic: accessGuide.endpointKind === 'mqtt' ? accessGuide.reportTopic : '',
      controlTopic: accessGuide.endpointKind === 'mqtt' ? accessGuide.controlTopic : '',
      tlsHintKey: accessGuide.tlsHintKey,
      payload: accessGuide.payload,
      lastConnectionError: accessGuide.lastError || ''
    },
    diagnosis: {
      ready: triageView.ready,
      latestTelemetry: triageView.telemetry,
      conclusion: triageView.summary,
      currentIssue: triageView.issue,
      nextAction: triageView.nextAction,
      completeness: triageView.completeness
    },
    debug: {
      enabled: Boolean(input.debugStatus?.enabled),
      expires: input.debugStatus?.enabled ? formatAccessGuideDebugTime(input.debugStatus.expire_at) : '',
      remainingSeconds: input.debugStatus?.enabled ? input.debugStatus.remaining_seconds || 0 : 0,
      logs: (input.debugLogs || []).slice(0, 5).map((log) => ({
        title: formatAccessGuideDebugLogTitle(log),
        message: formatAccessGuideDebugLogMessage(log, input.t)
      }))
    },
    testCommands: accessGuide.commands.map((command, index) => ({
      order: index + 1,
      title: input.t(command.titleKey),
      language: command.language,
      code: redactText(command.code, secretTokens)
    })),
    firstRunnableTestCommand: redactText(triageView.primaryTestCommand, secretTokens),
    markdownPacket: redactText(accessGuide.sdkBundle, secretTokens),
    supportSummary,
    verificationBoundary: {
      platformEvidenceOnly: true,
      fieldConnectionNotProven: true,
      message:
        'This access packet only summarizes platform-side guidance and diagnostic evidence. It does not prove that the physical field device is connected. Share raw credentials through a separate secure channel.'
    }
  }
}
