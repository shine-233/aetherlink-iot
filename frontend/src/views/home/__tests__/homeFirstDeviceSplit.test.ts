import { describe, expect, it } from 'vitest'
import * as barrel from '../homeFirstDeviceWorkbench'
import * as chart from '../homeFirstDeviceChart'
import * as onboarding from '../homeFirstDeviceOnboarding'
import * as proof from '../homeFirstDeviceProof'

const device = { id: 'd1', name: 'Pump', number: 'P-1', online: true, configId: 'c1', configName: 'cfg' }
const mqttGuide = { endpointKind: 'mqtt', endpoint: 'mqtt://broker:1883', protocol: 'MQTT' } as any
const command = 'mosquitto_pub -h broker -p 1883 -t "t" -m \'{}\''

describe('homeFirstDeviceWorkbench split', () => {
  it('barrel re-exports every submodule binding by identity', () => {
    for (const mod of [chart, onboarding, proof]) {
      for (const [name, value] of Object.entries(mod)) {
        expect((barrel as Record<string, unknown>)[name], name).toBe(value)
      }
    }
    // 拆分前的入口级导出仍然存在
    for (const name of [
      'buildFirstDeviceSuccessProofPacket',
      'buildFirstDeviceSupportSummary',
      'buildFirstDeviceOnlineTesterState',
      'summarizeFirstDeviceConnectionDiagnostics',
      'DEFAULT_FIRST_DEVICE_PAYLOAD',
      'createIdleFirstDeviceBrowserTestState',
      'normalizeFirstDevice',
      'normalizeTelemetryPoints',
      'normalizeSimulationInit',
      'buildPublishCommand'
    ]) {
      expect(typeof (barrel as Record<string, unknown>)[name], name).not.toBe('undefined')
    }
  })

  it('advances the quickstart active step gate by gate', () => {
    const guard = (overrides: Partial<Parameters<typeof onboarding.buildFirstDeviceOnboardingGuard>[0]>) =>
      onboarding.buildFirstDeviceOnboardingGuard({
        device,
        telemetry: [{ key: 'temperature', value: '25' }],
        accessGuide: mqttGuide,
        publishCommand: command,
        deploymentHealthy: true,
        ...overrides
      })
    const statuses = (g: ReturnType<typeof guard>) =>
      g.steps.map((step) => `${step.key}:${step.status}:${step.statusType}`)

    expect(statuses(guard({ deploymentHealthy: false }))).toEqual([
      'health:active:warning',
      'create:done:success',
      'connect:done:success',
      'publish:done:success',
      'verify:todo:default'
    ])
    expect(guard({ device: null, telemetry: [] }).activeStep?.key).toBe('create')
    expect(guard({ publishCommand: 'mosquitto_pub -h <host>' }).activeStep?.key).toBe('connect')
    expect(guard({ telemetry: [] }).activeStep?.key).toBe('publish')
    expect(guard({ device: { ...device, online: false } }).activeStep?.key).toBe('verify')

    const ready = guard({})
    expect(ready.activeStep).toBeNull()
    expect(ready.steps.every((step) => step.status === 'done' && step.statusLabel === '已完成')).toBe(true)
    expect(ready.nextAction).toBe('继续下一步')
  })

  it('marks only the first blocked proof item as the active flow node', () => {
    const nodes = proof.buildFirstDeviceFlowNodes([
      { key: 'deployment', label: 'a', ok: true, detail: '' },
      { key: 'identity', label: 'b', ok: false, detail: 'missing' },
      { key: 'chart', label: 'c', ok: false, detail: '' }
    ])
    expect(nodes.map((node) => [node.state, node.stateLabel, node.stateType])).toEqual([
      ['done', '已通过', 'success'],
      ['active', '当前卡点', 'warning'],
      ['todo', '待处理', 'default']
    ])
    const summary = proof.buildFirstDeviceClosureSummary(nodes)
    expect(summary).toMatchObject({
      doneCount: 1,
      remainingCount: 2,
      percent: 33,
      nextTitle: '设备身份',
      nextDetail: 'missing'
    })
  })

  it('resolves the focused section for every quickstart step', () => {
    const resolve = (key: any, extra: Partial<Parameters<typeof proof.resolveFirstDeviceFocusedSectionKey>[0]> = {}) =>
      proof.resolveFirstDeviceFocusedSectionKey({
        activeStep: key ? { key } : null,
        ready: false,
        chartReady: false,
        ...extra
      })
    expect(resolve('health')).toBe('deployment')
    expect(resolve('create')).toBe('device')
    expect(resolve('connect')).toBe('connection')
    expect(resolve('publish')).toBe('test')
    expect(resolve('verify')).toBe('proof')
    expect(resolve('verify', { readyProofItems: [{ key: 'chart', label: '', ok: false, detail: '' }] })).toBe('chart')
    expect(
      resolve('verify', { chartReady: true, readyProofItems: [{ key: 'chart', label: '', ok: false, detail: '' }] })
    ).toBe('proof')
    expect(resolve(null)).toBe('quickstart')
    expect(resolve(null, { ready: true })).toBe('proof')
  })

  it('prefers the confirmed browser-test point as the chart primary', () => {
    const telemetry = [
      { key: 'humidity', value: '60' },
      { key: 'temperature', value: '3' }
    ]
    const confirmed = chart.buildFirstDeviceBrowserTestState({
      status: 'confirmed',
      telemetry: { key: 'temperature', value: '3' }
    })
    const state = chart.buildFirstDeviceChartState(telemetry, confirmed)
    expect(state).toMatchObject({
      ready: true,
      primaryKey: 'temperature',
      primaryValue: '3',
      generatedFrom: 'browser_test'
    })
    expect(state.points.map((point) => point.barPercent)).toEqual([100, 8])

    const pending = chart.buildFirstDeviceChartState([], confirmed)
    expect(pending).toMatchObject({ ready: false, generatedFrom: 'none', primaryKey: 'temperature' })
    expect(chart.buildFirstDeviceBrowserTestState({ status: 'sending' }).message).toBe('正在发送浏览器测试遥测。')
  })

  it('keeps post-ready handoff copy for known, unknown and prototype-like step ids', () => {
    expect(proof.buildFirstDevicePostReadyHandoff({ ready: true, nextStep: { id: 'automation' } })).toMatchObject({
      title: '下一步：配置第一条自动化',
      primaryLabel: '新建首条联动规则'
    })
    expect(
      proof.buildFirstDevicePostReadyHandoff({ ready: true, nextStep: { id: 'dashboard', action: '去建' } })
    ).toMatchObject({
      title: '下一步：创建第一个客户看板',
      primaryLabel: '去建'
    })
    expect(
      proof.buildFirstDevicePostReadyHandoff({ ready: true, nextStep: { id: 'constructor', title: '报警' } })
    ).toMatchObject({ title: '下一步：报警', primaryLabel: '继续下一步' })
    expect(proof.buildFirstDevicePostReadyHandoff({ ready: false })).toBeNull()
  })
})
