const PLACEHOLDER_PATTERN = /<[^>]+>|\bundefined\b|\bnull\b|连接参数加载中|loading/i

export type FirstDeviceSuccessProofPacket = {
  schema: 'aetherlink.first-device.success-proof.v1'
  scope: 'home.first-device'
  generated_at: string
  ready: boolean
  conclusion: string
  next_action: string
  current_blocker: {
    key: string
    label: string
    detail: string
  } | null
  handoff_summary: {
    ready_to_deliver: boolean
    passed_proof_items: number
    total_proof_items: number
    remaining_proof_items: number
    first_missing_label: string
    test_command_state: string
    deployment_failed_count: number
    first_device_url: string
    proof_url: string
  }
  delivery: {
    first_device_url: string
    proof_url: string
    generated_from_page: string
    proof_file_hint: string
  }
  boundary: string
  device: {
    id: string
    name: string
    number: string
    online: boolean
    config_id: string
    config_name: string
  }
  connection: {
    protocol: string
    endpoint_kind: string
    endpoint: string
    report_entry: string
    control_topic: string
    auth_mode: string
    tls_hint_key: string
    username_state: string
    password_or_token_state: string
  }
  browser_test: {
    status: string
    message: string
    sent_at: string
    telemetry_key: string
    telemetry_value: string
  }
  latest_telemetry: {
    available: boolean
    source: string
    key: string
    value: string
    observed_at: string
    online: boolean
  }
  chart: {
    ready: boolean
    source: string
    primary_key: string
    primary_value: string
    summary: string
    points: Array<{ key: string; value: string; ts?: string }>
  }
  proof_items: Array<{
    key: string
    label: string
    ok: boolean
    detail: string
  }>
  deployment_health: Array<{
    key: string
    label: string
    ok: boolean
    description: string
    next_action: string
    error: string
    latency: string
  }>
}

export const firstDeviceCredentialState = (value: unknown) => {
  const text = String(value || '').trim()
  if (!text) return 'empty'
  return PLACEHOLDER_PATTERN.test(text) ? 'placeholder' : 'present'
}

export const buildFirstDeviceSuccessProofPacket = (options: {
  generatedAt?: Date
  device: any
  accessGuide: any
  simulation: any
  readyProof: any
  onboardingGuard: any
  chart: any
  browserTest?: any
  deploymentHealthRows: any[]
  delivery?: {
    firstDeviceUrl?: string
    proofUrl?: string
    generatedFromPage?: string
    proofFileHint?: string
  }
}): FirstDeviceSuccessProofPacket => {
  const device = options.device
  const accessGuide = options.accessGuide
  const chart = options.chart
  const failedProofItem = options.readyProof.items.find((item: any) => !item.ok) || null
  const passedProofItems = options.readyProof.items.filter((item: any) => item.ok).length
  const totalProofItems = options.readyProof.items.length
  const testCommand = accessGuide?.commands?.[0]?.code || accessGuide?.sdkBundle || ''
  const testCommandState = firstDeviceCredentialState(testCommand)
  const deploymentFailedCount = options.deploymentHealthRows.filter((row: any) => !row.ok).length
  const latestPoint = chart.points?.[0] || null
  const firstDeviceUrl = options.delivery?.firstDeviceUrl || '/first-device'
  const proofUrl = options.delivery?.proofUrl || '/home?onboarding=first-device&focus=proof'
  const proofFileHint = options.delivery?.proofFileHint || 'aetherlink-first-device-proof-<device>.json'

  return {
    schema: 'aetherlink.first-device.success-proof.v1',
    scope: 'home.first-device',
    generated_at: (options.generatedAt || new Date()).toISOString(),
    ready: options.readyProof.ready,
    conclusion: options.readyProof.summary,
    next_action: options.readyProof.ready ? 'continue_to_automation_or_dashboard' : options.onboardingGuard.nextAction,
    current_blocker: failedProofItem
      ? {
          key: failedProofItem.key,
          label: failedProofItem.label,
          detail: failedProofItem.detail
        }
      : null,
    handoff_summary: {
      ready_to_deliver: Boolean(options.readyProof.ready),
      passed_proof_items: passedProofItems,
      total_proof_items: totalProofItems,
      remaining_proof_items: Math.max(totalProofItems - passedProofItems, 0),
      first_missing_label: failedProofItem?.label || '',
      test_command_state: testCommandState,
      deployment_failed_count: deploymentFailedCount,
      first_device_url: firstDeviceUrl,
      proof_url: proofUrl
    },
    delivery: {
      first_device_url: firstDeviceUrl,
      proof_url: proofUrl,
      generated_from_page: options.delivery?.generatedFromPage || 'home.first-device',
      proof_file_hint: proofFileHint
    },
    boundary:
      'Homepage-visible first-device evidence only. Archive runtime API/E2E/Playwright evidence before treating this as release proof.',
    device: {
      id: device?.id || '',
      name: device?.name || '',
      number: device?.number || '',
      online: Boolean(device?.online),
      config_id: device?.configId || '',
      config_name: device?.configName || ''
    },
    connection: {
      protocol: accessGuide?.protocol || 'MQTT',
      endpoint_kind: accessGuide?.endpointKind || '',
      endpoint:
        accessGuide?.endpoint || (options.simulation ? `${options.simulation.server}:${options.simulation.port}` : ''),
      report_entry:
        (accessGuide?.endpointKind === 'http'
          ? accessGuide?.endpoint
          : accessGuide?.reportTopic || options.simulation?.topic) || '',
      control_topic: accessGuide?.controlTopic || '',
      auth_mode: accessGuide?.authMode || '',
      tls_hint_key: accessGuide?.tlsHintKey || '',
      username_state: firstDeviceCredentialState(accessGuide?.username),
      password_or_token_state: firstDeviceCredentialState(accessGuide?.password || accessGuide?.token)
    },
    browser_test: {
      status: options.browserTest?.status || 'unknown',
      message: options.browserTest?.message || '',
      sent_at: options.browserTest?.sentAt || '',
      telemetry_key: options.browserTest?.telemetryKey || '',
      telemetry_value: options.browserTest?.telemetryValue || ''
    },
    latest_telemetry: {
      available: Boolean(latestPoint?.key || options.browserTest?.telemetryKey),
      source: chart.generatedFrom || (options.browserTest?.status === 'confirmed' ? 'browser_test' : 'none'),
      key: latestPoint?.key || options.browserTest?.telemetryKey || '',
      value: latestPoint?.value || options.browserTest?.telemetryValue || '',
      observed_at: latestPoint?.ts || options.browserTest?.sentAt || '',
      online: Boolean(device?.online)
    },
    chart: {
      ready: chart.ready,
      source: chart.generatedFrom,
      primary_key: chart.primaryKey,
      primary_value: chart.primaryValue,
      summary: chart.summary,
      points: chart.points.map((point: any) => ({
        key: point.key,
        value: point.value,
        ts: point.ts
      }))
    },
    proof_items: options.readyProof.items.map((item: any) => ({
      key: item.key,
      label: item.label,
      ok: item.ok,
      detail: item.detail
    })),
    deployment_health: options.deploymentHealthRows.map((row: any) => ({
      key: row.key || '',
      label: row.label,
      ok: row.ok,
      description: row.description || '',
      next_action: row.nextAction || '',
      error: row.error || '',
      latency: String(row.latency || '')
    }))
  }
}
