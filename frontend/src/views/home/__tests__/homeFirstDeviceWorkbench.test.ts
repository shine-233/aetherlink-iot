import { describe, expect, it } from 'vitest'
import {
  buildFirstDeviceBrowserTestState,
  buildFirstDeviceChartState,
  buildFirstDeviceFlowNodes,
  buildFirstDeviceOnboardingGuard,
  buildFirstDevicePostReadyHandoff,
  buildFirstDevicePostTestGuidance,
  buildFirstDeviceReadyProof,
  buildFirstDeviceVerificationAction,
  buildHttpTelemetryRequest,
  hasConnectionPlaceholder,
  isUsableHttpEndpoint,
  resolveFirstDeviceFocusedSectionKey,
  summarizeFirstDeviceConnectionDiagnostics
} from '../homeFirstDeviceWorkbench'
import {
  buildFirstDeviceProofDelivery,
  buildFirstDeviceProofFilename,
  buildFirstDeviceSuccessProofDeliveryPacket
} from '../homeFirstDeviceProofDelivery'
import { buildFirstDeviceSupportSummary } from '../homeFirstDeviceSupportSummary'

describe('homeFirstDeviceWorkbench', () => {
  it('builds first-device proof delivery URLs, filename, and packet handoff fields', () => {
    const state = {
      device: {
        id: 'device-1',
        name: 'Pump 1',
        number: 'Pump 1/#A',
        online: true,
        configId: 'config-1',
        configName: 'MQTT'
      },
      accessGuide: {
        protocol: 'MQTT',
        endpointKind: 'mqtt',
        endpoint: 'mqtt://broker.example.com',
        reportTopic: 'devices/telemetry',
        controlTopic: 'devices/telemetry/control/PUMP-1',
        authMode: 'token',
        tlsHintKey: 'tls_optional',
        username: 'device-1',
        password: 'token-1',
        commands: [{ code: 'mosquitto_pub -t devices/telemetry -m {}' }]
      },
      simulation: null,
      readyProof: {
        ready: true,
        summary: 'Ready',
        items: [{ key: 'telemetry', label: 'Telemetry', ok: true, detail: 'received' }]
      },
      onboardingGuard: { nextAction: 'continue' },
      chart: {
        ready: true,
        source: 'latest_telemetry',
        primaryKey: 'temperature',
        primaryValue: '26',
        summary: 'Chart ready',
        points: [{ key: 'temperature', value: '26', ts: '2026-07-07T08:00:00Z' }]
      },
      browserTest: { status: 'success', message: 'sent', sentAt: '2026-07-07T08:00:00Z' },
      deploymentHealthRows: [{ key: 'api', label: 'API', ok: true }]
    }

    expect(buildFirstDeviceProofFilename(state.device)).toBe('aetherlink-first-device-proof-Pump-1-A.json')
    expect(buildFirstDeviceProofDelivery(state, 'https://iot.example.com')).toEqual({
      firstDeviceUrl: 'https://iot.example.com/first-device',
      proofUrl: 'https://iot.example.com/home?onboarding=first-device&focus=proof',
      proofFileHint: 'aetherlink-first-device-proof-Pump-1-A.json'
    })

    const packet = buildFirstDeviceSuccessProofDeliveryPacket(state, 'https://iot.example.com')
    expect(packet.delivery).toMatchObject({
      first_device_url: 'https://iot.example.com/first-device',
      proof_url: 'https://iot.example.com/home?onboarding=first-device&focus=proof',
      generated_from_page: '/first-device',
      proof_file_hint: 'aetherlink-first-device-proof-Pump-1-A.json'
    })
    expect(packet.handoff_summary.first_device_url).toBe('https://iot.example.com/first-device')

    const delivery = buildFirstDeviceProofDelivery(state, 'https://iot.example.com')
    const summary = buildFirstDeviceSupportSummary({
      generatedAt: new Date('2026-07-07T08:00:00Z'),
      device: state.device as any,
      accessGuide: state.accessGuide as any,
      diagnostics: null,
      simulation: state.simulation as any,
      readyProof: state.readyProof as any,
      latestProofText: 'Telemetry proof ready',
      browserTest: state.browserTest as any,
      testResult: 'sent',
      chart: state.chart as any,
      activeTestCommand: { label: 'MQTT test command' },
      onboardingGuard: {
        commandHasPlaceholders: false,
        canCopyCommand: true,
        canRunBrowserTest: true,
        summary: 'ready',
        nextAction: 'continue',
        activeStep: null,
        steps: []
      } as any,
      deploymentHealthRows: state.deploymentHealthRows as any,
      delivery
    })
    expect(summary).toContain(delivery.firstDeviceUrl)
    expect(summary).toContain(delivery.proofUrl)
    expect(summary).toContain(delivery.proofFileHint)
  })

  it('builds HTTP telemetry test request with bearer token and JSON payload', () => {
    const request = buildHttpTelemetryRequest({
      endpoint: 'https://aetherlink.local/api/v1/device/report',
      token: 'token-1',
      payload: '{"temperature":26}'
    })

    expect(request.url).toBe('https://aetherlink.local/api/v1/device/report')
    expect(request.init).toEqual({
      method: 'POST',
      headers: {
        'content-type': 'application/json',
        authorization: 'Bearer token-1'
      },
      body: '{"temperature":26}'
    })
  })

  it('does not send placeholder token as authorization header', () => {
    const request = buildHttpTelemetryRequest({
      endpoint: 'https://aetherlink.local/api/v1/device/report',
      token: '<access-token>',
      payload: '{}'
    })

    expect(request.init.headers).toEqual({
      'content-type': 'application/json'
    })
  })

  it('recognizes only absolute HTTP endpoints as usable browser targets', () => {
    expect(isUsableHttpEndpoint('https://aetherlink.local/api/v1/report')).toBe(true)
    expect(isUsableHttpEndpoint('http://127.0.0.1/report')).toBe(true)
    expect(isUsableHttpEndpoint('<http-endpoint>')).toBe(false)
    expect(isUsableHttpEndpoint('mqtt.example.com:1883')).toBe(false)
  })

  it('blocks placeholder commands from the first-device quickstart copy path', () => {
    const guard = buildFirstDeviceOnboardingGuard({
      device: {
        id: 'device-1',
        name: 'Pump 1',
        number: 'PUMP-1',
        online: false,
        configName: 'MQTT'
      },
      telemetry: [],
      accessGuide: {
        protocol: 'MQTT',
        authMode: 'ACCESS TOKEN / USERNAME',
        endpoint: '<mqtt-host>:1883',
        endpointKind: 'mqtt',
        clientId: '<mqtt-username>',
        username: '<mqtt-username>',
        password: '',
        reportTopic: 'devices/telemetry',
        controlTopic: 'devices/telemetry/control/PUMP-1',
        payload: '{}',
        lastError: '',
        diagnostics: [],
        tlsHintKey: '',
        quickstartSteps: [],
        sdkBundle: '',
        commands: [
          {
            titleKey: 'mosquitto',
            language: 'bash',
            code: 'mosquitto_pub -h <mqtt-host> -t "devices/telemetry" -m "{}"'
          }
        ],
        checks: []
      },
      publishCommand: 'mosquitto_pub -h <mqtt-host> -t "devices/telemetry" -m "{}"'
    })

    expect(hasConnectionPlaceholder('<mqtt-host>')).toBe(true)
    expect(guard.commandHasPlaceholders).toBe(true)
    expect(guard.canCopyCommand).toBe(false)
    expect(guard.activeStep).toMatchObject({
      key: 'connect',
      action: 'ready-check'
    })
    expect(guard.steps.find((step) => step.key === 'connect')).toMatchObject({
      status: 'active',
      action: 'ready-check'
    })
  })

  it('builds a safe quickstart path when a real command and telemetry exist', () => {
    const guard = buildFirstDeviceOnboardingGuard({
      device: {
        id: 'device-1',
        name: 'Pump 1',
        number: 'PUMP-1',
        online: true,
        configName: 'MQTT'
      },
      telemetry: [{ key: 'temperature', value: '25' }],
      accessGuide: {
        protocol: 'MQTT',
        authMode: 'ACCESS TOKEN / USERNAME',
        endpoint: 'mqtt.example.com:1883',
        endpointKind: 'mqtt',
        clientId: 'PUMP-1',
        username: 'token-1',
        password: '',
        reportTopic: 'devices/telemetry',
        controlTopic: 'devices/telemetry/control/PUMP-1',
        payload: '{}',
        lastError: '',
        diagnostics: [],
        tlsHintKey: '',
        quickstartSteps: [],
        sdkBundle: '',
        commands: [],
        checks: []
      },
      publishCommand: 'mosquitto_pub -h mqtt.example.com -t "devices/telemetry" -m "{}"'
    })

    expect(guard.canCopyCommand).toBe(true)
    expect(guard.canRunBrowserTest).toBe(true)
    expect(guard.activeStep).toBeNull()
    expect(guard.steps.map((step) => step.status)).toEqual(['done', 'done', 'done', 'done', 'done'])
  })

  it('puts deployment health first before first-device actions', () => {
    const guard = buildFirstDeviceOnboardingGuard({
      device: null,
      telemetry: [],
      accessGuide: null,
      publishCommand: '',
      deploymentHealthy: false
    })

    expect(guard.canRunBrowserTest).toBe(false)
    expect(guard.activeStep).toMatchObject({
      key: 'health',
      action: 'health',
      disabled: false
    })
    expect(guard.steps.map((step) => [step.key, step.status, step.disabled])).toEqual([
      ['health', 'active', false],
      ['create', 'todo', true],
      ['connect', 'todo', true],
      ['publish', 'todo', true],
      ['verify', 'todo', true]
    ])
  })

  it('summarizes diagnostics for the homepage access guide', () => {
    const summary = summarizeFirstDeviceConnectionDiagnostics({
      data: {
        online: { is_online: false },
        ready_check: {
          ready: false,
          level: 'warning',
          code: 'no_current_telemetry',
          summary: 'No current telemetry',
          next_actions: ['Publish a test telemetry event'],
          telemetry: {
            current_count: 0
          }
        },
        debug: {
          enabled: true,
          recent_logs: [{ action: 'publish', direction: 'inbound', error: 'bad token' }]
        }
      }
    })

    expect(summary.ready).toBe(false)
    expect(summary.readyCode).toBe('no_current_telemetry')
    expect(summary.latestIssue).toBe('[publish] inbound: bad token')
    expect(summary.readyNextActions).toEqual(['Publish a test telemetry event'])
  })

  it('builds first-device ready proof with clear missing items', () => {
    const proof = buildFirstDeviceReadyProof({
      device: {
        id: 'device-1',
        name: 'Pump 1',
        number: 'PUMP-1',
        online: false,
        configName: 'MQTT'
      },
      telemetry: [],
      accessGuide: {
        protocol: 'MQTT',
        authMode: 'ACCESS TOKEN / USERNAME',
        endpoint: '<mqtt-host>:1883',
        endpointKind: 'mqtt',
        clientId: '<mqtt-username>',
        username: '<mqtt-username>',
        password: '',
        reportTopic: 'devices/telemetry',
        controlTopic: 'devices/telemetry/control/PUMP-1',
        payload: '{}',
        lastError: '',
        diagnostics: [],
        tlsHintKey: '',
        quickstartSteps: [],
        sdkBundle: '',
        commands: [],
        checks: []
      },
      publishCommand: 'mosquitto_pub -h <mqtt-host> -t "devices/telemetry" -m "{}"',
      deploymentHealthy: true
    })

    expect(proof.ready).toBe(false)
    expect(proof.title).toBe('设备还没完全准备好')
    expect(proof.items.map((item) => [item.key, item.ok])).toEqual([
      ['deployment', true],
      ['identity', true],
      ['connection', false],
      ['browser_test', false],
      ['online', false],
      ['telemetry', false],
      ['chart', false]
    ])
  })

  it('marks first-device ready proof complete when every handoff signal is green', () => {
    const proof = buildFirstDeviceReadyProof({
      device: {
        id: 'device-1',
        name: 'Pump 1',
        number: 'PUMP-1',
        online: true,
        configName: 'MQTT'
      },
      telemetry: [{ key: 'temperature', value: '25' }],
      accessGuide: {
        protocol: 'MQTT',
        authMode: 'ACCESS TOKEN / USERNAME',
        endpoint: 'mqtt.example.com:1883',
        endpointKind: 'mqtt',
        clientId: 'PUMP-1',
        username: 'token-1',
        password: '',
        reportTopic: 'devices/telemetry',
        controlTopic: 'devices/telemetry/control/PUMP-1',
        payload: '{}',
        lastError: '',
        diagnostics: [],
        tlsHintKey: '',
        quickstartSteps: [],
        sdkBundle: '',
        commands: [],
        checks: []
      },
      publishCommand: 'mosquitto_pub -h mqtt.example.com -t "devices/telemetry" -m "{}"',
      deploymentHealthy: true,
      browserTest: buildFirstDeviceBrowserTestState({
        status: 'confirmed',
        telemetry: { key: 'temperature', value: '25' }
      })
    })

    expect(proof.ready).toBe(true)
    expect(proof.title).toBe('设备已准备好')
    expect(proof.items.every((item) => item.ok)).toBe(true)
  })

  it('builds a first-device chart from confirmed latest telemetry', () => {
    const chart = buildFirstDeviceChartState(
      [
        { key: 'temperature', value: '25' },
        { key: 'humidity', value: '50' }
      ],
      buildFirstDeviceBrowserTestState({
        status: 'confirmed',
        telemetry: { key: 'temperature', value: '25' }
      })
    )

    expect(chart.ready).toBe(true)
    expect(chart.generatedFrom).toBe('browser_test')
    expect(chart.primaryKey).toBe('temperature')
    expect(chart.points.map((point) => [point.key, point.barPercent])).toEqual([
      ['temperature', 50],
      ['humidity', 100]
    ])
  })

  it('hands a ready first device to the first automation step before dashboard work', () => {
    const handoff = buildFirstDevicePostReadyHandoff({
      ready: true,
      nextStep: {
        id: 'automation',
        title: '配置自动化',
        description: '用首台设备的最新遥测创建第一条联动规则',
        action: '新建联动规则'
      }
    })

    expect(handoff).toMatchObject({
      action: 'next-guide',
      section: 'proof',
      title: '下一步：配置第一条自动化',
      primaryLabel: '新建联动规则',
      secondaryLabel: '查看完整指南'
    })
    expect(handoff?.description).toContain('首台设备已跑通')
    expect(handoff?.completionSignal).toContain('最新遥测')
  })

  it('keeps the ready handoff on proof and full guide when no post-ready step remains', () => {
    expect(buildFirstDevicePostReadyHandoff({ ready: false })).toBeNull()

    expect(buildFirstDevicePostReadyHandoff({ ready: true })).toMatchObject({
      action: 'guide',
      section: 'proof',
      title: '首台设备已准备好',
      primaryLabel: '查看完整接入指南',
      secondaryLabel: '定位成功证明'
    })
  })
  it('builds first-device flow nodes with one active blocker', () => {
    const nodes = buildFirstDeviceFlowNodes([
      { key: 'deployment', label: 'Deployment', ok: true, detail: 'ok' },
      { key: 'connection', label: 'Connection', ok: false, detail: 'missing endpoint' },
      { key: 'telemetry', label: 'Telemetry', ok: false, detail: 'missing test telemetry' }
    ])

    expect(nodes.map((node) => [node.key, node.state, node.stateType])).toEqual([
      ['deployment', 'done', 'success'],
      ['connection', 'active', 'warning'],
      ['telemetry', 'todo', 'default']
    ])
    expect(nodes[1]).toMatchObject({
      title: '连接参数',
      short: '端点 / Topic',
      stateLabel: '当前卡点'
    })
  })

  it('resolves the focused first-device section from the active quickstart step', () => {
    expect(
      resolveFirstDeviceFocusedSectionKey({ activeStep: { key: 'health' }, ready: false, chartReady: false })
    ).toBe('deployment')
    expect(
      resolveFirstDeviceFocusedSectionKey({ activeStep: { key: 'publish' }, ready: false, chartReady: false })
    ).toBe('test')
    expect(
      resolveFirstDeviceFocusedSectionKey({
        activeStep: { key: 'verify' },
        ready: false,
        chartReady: false,
        readyProofItems: [{ key: 'chart', label: 'Chart', ok: false, detail: 'missing' }]
      })
    ).toBe('chart')
    expect(resolveFirstDeviceFocusedSectionKey({ ready: true, chartReady: true })).toBe('proof')
  })

  it('builds post-test guidance from visible proof state', () => {
    expect(
      buildFirstDevicePostTestGuidance({ testResult: '', ready: false, readyDescription: 'ready', chartReady: false })
    ).toBeNull()
    expect(
      buildFirstDevicePostTestGuidance({
        testResult: 'sent',
        ready: true,
        readyDescription: 'Go to automation',
        chartReady: true
      })
    ).toMatchObject({ type: 'success', detail: 'Go to automation' })
    expect(
      buildFirstDevicePostTestGuidance({
        testResult: 'sent',
        ready: false,
        readyDescription: 'ready',
        chartReady: false,
        currentBlocker: { label: 'Telemetry' }
      })
    ).toMatchObject({ type: 'warning', title: '测试已发送，等待可见证据' })
    expect(
      buildFirstDevicePostTestGuidance({
        testResult: 'sent',
        ready: false,
        readyDescription: 'ready',
        chartReady: true,
        currentBlocker: { label: '在线状态' }
      })
    ).toMatchObject({
      type: 'warning',
      title: '测试遥测已产生数据，继续看最终证明',
      detail: '当前还差：在线状态'
    })
  })

  it('builds the first-device verification action without changing navigation semantics', () => {
    expect(
      buildFirstDeviceVerificationAction({
        hasDevice: false,
        ready: false,
        readyDescription: 'ready',
        chartReady: false,
        canRunBrowserTest: false,
        testResult: '',
        actionLoading: false
      })
    ).toBeNull()

    expect(
      buildFirstDeviceVerificationAction({
        hasDevice: true,
        ready: false,
        readyDescription: 'ready',
        chartReady: false,
        canRunBrowserTest: true,
        testResult: '',
        actionLoading: false
      })
    ).toMatchObject({ action: 'simulate', section: 'test', label: '发送测试遥测并确认' })

    expect(
      buildFirstDeviceVerificationAction({
        hasDevice: true,
        ready: false,
        readyDescription: 'ready',
        chartReady: false,
        canRunBrowserTest: false,
        testResult: '',
        actionLoading: false,
        currentBlocker: { label: '连接参数', detail: 'missing endpoint' }
      })
    ).toMatchObject({
      type: 'warning',
      action: 'ready-check',
      section: 'connection',
      label: '打开 Ready Check'
    })

    expect(
      buildFirstDeviceVerificationAction({
        hasDevice: true,
        ready: false,
        readyDescription: 'ready',
        chartReady: true,
        canRunBrowserTest: false,
        testResult: 'confirmed',
        actionLoading: false,
        currentBlocker: { label: '在线状态', detail: 'offline' }
      })
    ).toMatchObject({
      type: 'warning',
      action: 'proof',
      section: 'proof',
      label: '查看证明项',
      secondaryLabel: '复制首图证明'
    })

    expect(
      buildFirstDeviceVerificationAction({
        hasDevice: true,
        ready: true,
        postReadyHandoff: buildFirstDevicePostReadyHandoff({
          ready: true,
          nextStep: { id: 'automation', title: 'Automation', description: 'Create first rule', action: 'Create rule' }
        }),
        readyDescription: 'ready',
        chartReady: true,
        canRunBrowserTest: false,
        testResult: 'confirmed',
        actionLoading: false
      })
    ).toMatchObject({ type: 'success', action: 'next-guide', section: 'proof', label: 'Create rule' })
  })
})
