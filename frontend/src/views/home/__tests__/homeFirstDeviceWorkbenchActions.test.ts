import { describe, expect, it } from 'vitest'
import {
  deferredSectionFor,
  normalizeFirstDeviceSectionKey,
  resolveClosedLoopStepIntent,
  resolveFlowNodeAction,
  resolveFocusedQuickstartIntent,
  resolvePrimaryIntent,
  resolveVerificationIntent,
  resolveVerificationSecondaryIntent,
  type FirstDeviceActionContext
} from '../homeFirstDeviceWorkbenchActions'
import {
  buildFirstDeviceClosedLoopSteps,
  buildFirstDeviceConnectionSummary,
  resolveClosedLoopState
} from '../homeFirstDeviceClosedLoop'

const ctx = (overrides: Partial<FirstDeviceActionContext> = {}): FirstDeviceActionContext => ({
  ready: false,
  hasDevice: true,
  deploymentHealthOk: true,
  deploymentHealthLoading: false,
  firstRunCreateTenantRequired: false,
  firstRunCreateLoading: false,
  firstDeviceActionLoading: false,
  canCopyCommand: true,
  canRunBrowserTest: true,
  hasActiveTestCommand: true,
  chartReady: false,
  hasNextGuideStep: false,
  postReadyAction: null,
  primaryQuickstartAction: 'health',
  activeQuickstartAction: 'copy',
  ...overrides
})

describe('resolveFlowNodeAction', () => {
  it('maps deployment to a health refresh carrying the loading flag', () => {
    expect(resolveFlowNodeAction({ key: 'deployment', ok: false }, ctx({ deploymentHealthLoading: true }))).toEqual({
      label: '去诊断',
      disabled: false,
      loading: true,
      intent: { kind: 'emit', event: 'refreshDeploymentHealth' }
    })
  })

  it('identity creates the first device when none exists and gates on tenant/health', () => {
    const action = resolveFlowNodeAction({ key: 'identity', ok: false }, ctx({ hasDevice: false, deploymentHealthOk: false }))
    expect(action.intent).toEqual({ kind: 'emit', event: 'createFirstRunFirstDevice' })
    expect(action.disabled).toBe(true)
  })

  it('connection copies only when a command is copyable, otherwise opens the guide', () => {
    expect(resolveFlowNodeAction({ key: 'connection', ok: false }, ctx()).intent).toEqual({ kind: 'copyTestCommand' })
    expect(resolveFlowNodeAction({ key: 'connection', ok: false }, ctx({ hasActiveTestCommand: false })).intent).toEqual({
      kind: 'emit',
      event: 'openFirstDeviceFullGuide'
    })
  })

  it('unknown pending nodes fall through to the primary action', () => {
    expect(resolveFlowNodeAction({ key: 'other', ok: false }, ctx()).intent).toEqual({ kind: 'primary' })
    expect(resolveFlowNodeAction({ key: 'other', ok: true }, ctx()).label).toBe('查看接入指南')
  })
})

describe('closed loop step intents', () => {
  it('disabled steps only focus their section', () => {
    expect(resolveClosedLoopStepIntent({ key: 'proof', section: 'proof', disabled: true }, ctx({ ready: true }))).toEqual({
      kind: 'focus',
      section: 'proof'
    })
  })

  it('proof downloads when ready and telemetry focuses the chart once it is ready', () => {
    expect(resolveClosedLoopStepIntent({ key: 'proof', section: 'proof' }, ctx({ ready: true }))).toEqual({
      kind: 'downloadProof'
    })
    expect(resolveClosedLoopStepIntent({ key: 'telemetry', section: 'chart' }, ctx({ chartReady: true }))).toEqual({
      kind: 'focus',
      section: 'chart'
    })
    expect(resolveClosedLoopStepIntent({ key: 'telemetry', section: 'chart' }, ctx())).toEqual({
      kind: 'emit',
      event: 'refreshFirstDeviceWorkbench'
    })
  })
})

describe('verification, quickstart and primary intents', () => {
  it('next-guide requires a next step', () => {
    expect(resolveVerificationIntent({ action: 'next-guide' } as any, ctx())).toEqual({ kind: 'none' })
    expect(resolveVerificationIntent({ action: 'next-guide' } as any, ctx({ hasNextGuideStep: true }))).toEqual({
      kind: 'openNextGuideStep'
    })
  })

  it('secondary action copies chart proof for proof and focuses otherwise', () => {
    expect(resolveVerificationSecondaryIntent({ action: 'proof', section: 'proof' } as any)).toEqual({
      kind: 'copyChartProof'
    })
    expect(resolveVerificationSecondaryIntent({ action: 'simulate', section: 'test' } as any)).toEqual({
      kind: 'focus',
      section: 'test'
    })
    expect(resolveVerificationSecondaryIntent(null)).toEqual({ kind: 'none' })
  })

  it('focused quickstart runs the active step before readiness and hands off after', () => {
    expect(resolveFocusedQuickstartIntent(ctx())).toEqual({ kind: 'quickstart', action: 'copy' })
    expect(resolveFocusedQuickstartIntent(ctx({ activeQuickstartAction: null }))).toEqual({ kind: 'none' })
    expect(resolveFocusedQuickstartIntent(ctx({ ready: true }))).toEqual({
      kind: 'emit',
      event: 'openFirstDeviceFullGuide'
    })
  })

  it('primary uses post-ready handoff only for next-guide', () => {
    expect(resolvePrimaryIntent(ctx())).toEqual({ kind: 'quickstart', action: 'health' })
    expect(
      resolvePrimaryIntent(ctx({ ready: true, postReadyAction: 'next-guide', hasNextGuideStep: true }))
    ).toEqual({ kind: 'openNextGuideStep' })
  })
})

describe('section keys', () => {
  it('normalizes flow-node keys onto view section keys and groups deferred sections', () => {
    expect(normalizeFirstDeviceSectionKey('browser_test')).toBe('test')
    expect(normalizeFirstDeviceSectionKey('identity')).toBe('device')
    expect(deferredSectionFor('browser_test')).toBe('connectionTest')
    expect(deferredSectionFor('online')).toBe('successProof')
    expect(deferredSectionFor('support')).toBe('supportSummary')
    expect(deferredSectionFor('deployment')).toBeNull()
  })
})

describe('closed loop steps', () => {
  it('derives state from explicit completion or flow node state', () => {
    const nodes = [{ key: 'connection', ok: false, state: 'active' as const }]
    expect(resolveClosedLoopState(nodes, 'connection', false)).toEqual({
      state: 'active',
      stateLabel: '进行中',
      stateType: 'warning'
    })
    expect(resolveClosedLoopState(nodes, 'connection', true).state).toBe('done')
    expect(resolveClosedLoopState(nodes, 'missing', false).state).toBe('todo')
  })

  it('builds six ordered steps and disables device-dependent steps without a device', () => {
    const steps = buildFirstDeviceClosedLoopSteps({
      flowNodes: [],
      device: null,
      deploymentHealthOk: true,
      deploymentHealthLoading: false,
      firstRunCreateTenantRequired: false,
      firstRunCreateLoading: false,
      firstDeviceLoading: false,
      firstDeviceActionLoading: false,
      canCopyCommand: false,
      canRunBrowserTest: false,
      activeTestCommand: null,
      browserTest: null,
      chartReady: false,
      ready: false,
      latestProofText: 'none'
    })
    expect(steps.map((s) => s.order)).toEqual(['01', '02', '03', '04', '05', '06'])
    expect(steps[0].state).toBe('done')
    expect(steps.slice(2).every((s) => s.disabled)).toBe(true)
  })

  it('connection summary falls back to simulation endpoint and marks missing command', () => {
    const text = buildFirstDeviceConnectionSummary({
      device: { number: 'D-1' },
      accessGuide: null,
      simulation: { server: 'mqtt.local', port: 1883, topic: 't/1' },
      command: ''
    })
    expect(text).toContain('Device: D-1')
    expect(text).toContain('Endpoint: mqtt.local:1883')
    expect(text).toContain('Report entry: t/1')
    expect(text).toContain('Device connection command: not-ready')
  })
})
