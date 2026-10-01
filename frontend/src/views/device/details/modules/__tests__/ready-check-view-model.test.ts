import { describe, expect, it, vi } from 'vitest'
import {
  buildBackendNextSteps,
  buildEvidenceCenterItems,
  buildReadyCheckSteps,
  formatLatestTelemetryText,
  formatPartialResultText,
  selectPrimaryReadyAction
} from '../ready-check-view-model'

const t = (key: string) => key

describe('ready-check-view-model', () => {
  it('formats latest telemetry and partial results', () => {
    expect(formatLatestTelemetryText(undefined, t)).toBe('custom.device_details.accessGuideLatestTelemetryEmpty')
    expect(formatLatestTelemetryText({ latest_key: 'temp', latest_at: '2026-01-01' }, t)).toBe('temp @ 2026-01-01')
    expect(formatLatestTelemetryText({ latest_key: 'temp' }, t)).toBe('temp')

    expect(formatPartialResultText(null, t)).toBe('custom.device_details.readyCheckEvidenceComplete')
    expect(formatPartialResultText({ partial_results: [{ component: 'cmd', reason: 'timeout' }, {}] } as any, t)).toBe(
      'cmd: timeout; guide: partial'
    )
  })

  it('caps backend next steps and fills defaults', () => {
    const guide = {
      next_steps: [{ title: 'A' }, { key: 'b', description: 'd', status: 'done' }, {}, {}, {}]
    } as any
    const steps = buildBackendNextSteps(guide, t)
    expect(steps).toHaveLength(4)
    expect(steps[0]).toEqual({ key: 'step-0', title: 'A', description: '', status: 'todo' })
    expect(steps[1]).toEqual({
      key: 'b',
      title: 'custom.device_details.readyCheckEvidenceNextStepUntitled',
      description: 'd',
      status: 'done'
    })
  })

  it('builds evidence center items with readiness fallbacks from ready check', () => {
    const items = buildEvidenceCenterItems(
      {
        sourceLabel: 'src',
        sourceDetail: 'detail',
        evaluatedAt: 'now',
        readinessSummary: 'ok',
        guide: null,
        readyCheck: { ready: false, level: 'warn', code: 'x', telemetry: { current_count: 3 } },
        latestTelemetryText: 'lt',
        latestTelemetryValueText: 'v',
        lastConnectionIssue: 'none',
        partialResultText: 'complete'
      },
      t
    )
    expect(items.map((item) => item.key)).toEqual([
      'source',
      'evaluated-at',
      'readiness',
      'telemetry',
      'last-issue',
      'completeness',
      'boundary'
    ])
    expect(items[2].detail).toBe('ready=false / level=warn / code=x')
    expect(items[3].detail).toContain(': 3')
  })

  it('derives step statuses from device state', () => {
    const openTab = vi.fn()
    const openCommandCenter = vi.fn()
    const offline = buildReadyCheckSteps({
      hasConnectionIdentity: false,
      isOnline: false,
      hasRecentTelemetry: false,
      hasTemplate: false,
      openTab,
      openCommandCenter
    })
    expect(offline.map((step) => step.status)).toEqual(['attention', 'attention', 'next', 'next', 'next', 'next'])
    expect(offline[2].descKey).toBe('custom.device_details.readyCheckTelemetryNoTemplateDesc')

    const online = buildReadyCheckSteps({
      hasConnectionIdentity: true,
      isOnline: true,
      hasRecentTelemetry: false,
      hasTemplate: true,
      openTab,
      openCommandCenter
    })
    expect(online.map((step) => step.status).slice(0, 3)).toEqual(['ready', 'ready', 'attention'])
    online[2].action()
    expect(openTab).toHaveBeenCalledWith('telemetry')
    online[5].action()
    expect(openCommandCenter).toHaveBeenCalled()
  })

  it('selects the primary action by priority', () => {
    const refresh = vi.fn()
    const openFirstDeviceAutomation = vi.fn()
    const steps = buildReadyCheckSteps({
      hasConnectionIdentity: true,
      isOnline: true,
      hasRecentTelemetry: true,
      hasTemplate: true,
      openTab: vi.fn(),
      openCommandCenter: vi.fn()
    })
    const evidenceActions = [
      { key: 'evidence-command', status: 'attention' as const, titleKey: 'cmd', actionKey: 'a', action: vi.fn() }
    ]
    const base = { steps, evidenceActions, refresh, openFirstDeviceAutomation, t }

    expect(selectPrimaryReadyAction({ ...base, collectionFailureSummary: 'failed', firstDeviceReady: true }).key).toBe(
      'collection-failure'
    )
    expect(selectPrimaryReadyAction({ ...base, collectionFailureSummary: null, firstDeviceReady: true }).key).toBe(
      'first-device-automation'
    )
    expect(selectPrimaryReadyAction({ ...base, collectionFailureSummary: null, firstDeviceReady: false }).key).toBe(
      'evidence-command'
    )
    expect(
      selectPrimaryReadyAction({
        ...base,
        evidenceActions: [],
        collectionFailureSummary: null,
        firstDeviceReady: false
      }).key
    ).toBe('twin')
  })
})
