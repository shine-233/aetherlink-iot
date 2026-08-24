import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  routeQuery: {} as Record<string, string>,
  routerPush: vi.fn(),
  getDeviceConnectionDiagnostics: vi.fn(),
  getDeviceConnectionGuide: vi.fn(),
  commandDataById: vi.fn(),
  writeClipboardText: vi.fn()
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: hoisted.routeQuery }),
  useRouter: () => ({ push: hoisted.routerPush })
}))

vi.mock('@/service/api/device', () => ({
  getDeviceConnectionDiagnostics: hoisted.getDeviceConnectionDiagnostics,
  getDeviceConnectionGuide: hoisted.getDeviceConnectionGuide,
  commandDataById: hoisted.commandDataById
}))

vi.mock('@/utils/clipboard', () => ({
  writeClipboardText: hoisted.writeClipboardText
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import OnboardingReadyCheck from '../onboarding-ready-check.vue'

interface ReadyCheckEvidenceCard {
  key: string
}

interface ReadyCheckDeepLink {
  key: string
}

// setupState 的受控视图：只声明测试实际触达的成员，其余成员走 unknown 兜底。
interface ReadyCheckSetupState {
  primaryReadyAction: { titleKey?: string }
  evidenceCards: ReadyCheckEvidenceCard[]
  evidenceDeepLinks: ReadyCheckDeepLink[]
  openEvidenceDeepLink: (link: ReadyCheckDeepLink) => void
  runEvidenceCardAction: (card: ReadyCheckEvidenceCard) => void
  readyCheckDiagnosticSummary: string
  openCommandCenter: () => void
  copyAllEvidenceDeepLinks: () => Promise<void>
  [key: string]: unknown
}

const mountComponent = () =>
  shallowMount(OnboardingReadyCheck, {
    props: {
      id: 'device-1',
      online: 1,
      deviceData: {
        name: 'Pump 1',
        device_number: 'PUMP-1',
        device_config_id: 'profile-1'
      }
    },
    global: {
      stubs: {
        NAlert: defineComponent({ setup(_, { slots }) { return () => h('section', slots.default?.()) } }),
        NButton: defineComponent({
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default?.())
          }
        }),
        NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default?.()) } })
      }
    }
  })

const readyDiagnostics = {
  data: {
    ready_check: {
      ready: true,
      level: 'ok',
      code: 'ready',
      summary: 'Ready',
      telemetry: {
        latest_key: 'temperature',
        latest_at: '2026-07-06T15:20:00Z',
        latest_value: { value: 26 },
        current_count: 1
      }
    },
    conclusion: {
      level: 'ok',
      summary: 'Command path ready'
    }
  }
}

describe('onboarding-ready-check.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.routeQuery = {}
    hoisted.getDeviceConnectionDiagnostics.mockResolvedValue(readyDiagnostics)
    hoisted.getDeviceConnectionGuide.mockResolvedValue({
      data: {
        readiness: { ready: true, online: true },
        twin_summary: { desiredCount: 1, reportedCount: 1, matchedCount: 1, deltaCount: 0, unavailableCount: 0 },
        command_summary: { level: 'ok', summary: 'Latest command was acknowledged', latest_status: 'success' },
        last_connection_error: { code: 'disconnect_error', summary: 'Broker disconnected recently' },
        partial_results: [{ component: 'command_delivery', reason: 'log_query_partial' }],
        next_steps: [{ key: 'ready_check', title: 'Refresh Ready Check', description: 'Refresh after retry.', status: 'todo' }],
        evaluated_at: '2026-07-06T15:21:00Z'
      }
    })
    hoisted.commandDataById.mockResolvedValue({
      data: [
        {
          data_identifier: 'reboot',
          data_name: 'Reboot device',
          params: { force: false }
        }
      ]
    })
  })

  it('makes first-device ready check lead to the first telemetry automation rule', async () => {
    hoisted.routeQuery = { onboarding: 'first-device' }
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as ReadyCheckSetupState

    expect(state.primaryReadyAction.actionKey).toBe('custom.automation.createFirstTelemetryRule')
    state.primaryReadyAction.action()

    expect(hoisted.routerPush).toHaveBeenCalledWith({
      path: '/automation/linkage-edit',
      query: expect.objectContaining({
        onboarding: 'first-device',
        starter: 'first-telemetry-rule',
        device_id: 'device-1',
        telemetry_key: 'temperature'
      })
    })
  })

  it('promotes actionable twin evidence when base readiness is not blocked', async () => {
    hoisted.getDeviceConnectionGuide.mockResolvedValue({
      data: {
        readiness: { ready: true, online: true },
        twin_summary: { desiredCount: 2, reportedCount: 2, matchedCount: 1, deltaCount: 1, unavailableCount: 0 },
        command_summary: { level: 'ok', summary: 'Latest command was acknowledged', latest_status: 'success' }
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as ReadyCheckSetupState

    expect(state.primaryReadyAction.titleKey).toBe('custom.device_details.readyCheckTwinTitle')
    expect(state.evidenceCards.find((card: ReadyCheckEvidenceCard) => card.key === 'twin')).toEqual(
      expect.objectContaining({
        boundaryKey: 'custom.device_details.readyCheckTwinBoundary'
      })
    )
    expect(state.readyCheckDiagnosticSummary).toContain('boundary=custom.device_details.readyCheckTwinBoundary')
    state.primaryReadyAction.action()

    expect(hoisted.routerPush).toHaveBeenCalledWith({
      path: '/device/details',
      query: expect.objectContaining({
        d_id: 'device-1',
        tab: 'device-twin'
      })
    })
  })

  it('keeps command evidence cards directly actionable', async () => {
    hoisted.getDeviceConnectionGuide.mockResolvedValue({
      data: {
        readiness: { ready: true, online: true },
        twin_summary: { desiredCount: 1, reportedCount: 1, matchedCount: 1, deltaCount: 0, unavailableCount: 0 },
        command_summary: { level: 'warning', summary: 'Command response needs review', latest_status: 'timeout' }
      }
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as ReadyCheckSetupState
    const commandCard = state.evidenceCards.find((card: ReadyCheckEvidenceCard) => card.key === 'command')

    expect(commandCard).toEqual(
      expect.objectContaining({
        boundaryKey: 'custom.device_details.readyCheckCommandBoundary'
      })
    )
    expect(state.readyCheckDiagnosticSummary).toContain('boundary=custom.device_details.readyCheckCommandBoundary')
    state.runEvidenceCardAction(commandCard)

    expect(hoisted.routerPush).toHaveBeenCalledWith({
      path: '/device/details',
      query: expect.objectContaining({
        d_id: 'device-1',
        tab: 'command-delivery'
      })
    })
  })

  it('builds a visible evidence center with source, freshness, telemetry, and boundary', async () => {
    hoisted.routeQuery = {
      source: 'ota',
      ota_task_id: 'task-1',
      ota_detail_id: 'detail-1'
    }
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as ReadyCheckSetupState

    expect(state.evidenceCenterItems).toEqual(expect.arrayContaining([
      expect.objectContaining({
        key: 'source',
        value: 'custom.device_details.readyCheckSourceOta',
        detail: 'task=task-1 / detail=detail-1'
      }),
      expect.objectContaining({
        key: 'evaluated-at',
        value: '2026-07-06T15:21:00Z'
      }),
      expect.objectContaining({
        key: 'telemetry',
        value: 'temperature @ 2026-07-06T15:20:00Z',
        detail: 'custom.device_details.readyCheckEvidenceTelemetryCount: 1 / custom.device_details.readyCheckEvidenceTelemetryValue: {"value":26}'
      }),
      expect.objectContaining({
        key: 'last-issue',
        value: 'Broker disconnected recently',
        detail: 'disconnect_error'
      }),
      expect.objectContaining({
        key: 'completeness',
        value: 'command_delivery: log_query_partial'
      }),
      expect.objectContaining({
        key: 'boundary',
        value: 'custom.device_details.readyCheckEvidenceBoundaryValue'
      })
    ]))
    expect(state.backendNextSteps).toEqual([
      {
        key: 'ready_check',
        title: 'Refresh Ready Check',
        description: 'Refresh after retry.',
        status: 'todo'
      }
    ])
    expect(state.readyCheckDiagnosticSummary).toContain('connectionGuideEvaluatedAt=2026-07-06T15:21:00Z')
    expect(state.readyCheckDiagnosticSummary).toContain('sourceDetail=task=task-1 / detail=detail-1')
    expect(state.readyCheckDiagnosticSummary).toContain('lastConnectionIssue=Broker disconnected recently')
    expect(state.readyCheckDiagnosticSummary).toContain('1. [todo] Refresh Ready Check - Refresh after retry.')
  })

  it('builds evidence deep links without claiming the target evidence is refreshed', async () => {
    hoisted.routeQuery = {
      source: 'ota',
      ota_task_id: 'task-1',
      ota_detail_id: 'detail-1'
    }
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as ReadyCheckSetupState

    expect(state.evidenceDeepLinks.map((link: ReadyCheckDeepLink) => link.key)).toEqual([
      'telemetry',
      'device-twin',
      'command-delivery',
      'ota',
      'audit-log'
    ])
    expect(state.evidenceDeepLinks).toEqual(expect.arrayContaining([
      expect.objectContaining({
        key: 'telemetry',
        path: '/device/details',
        query: expect.objectContaining({
          d_id: 'device-1',
          tab: 'telemetry',
          source: 'ota',
          ota_detail_id: 'detail-1'
        }),
        boundaryKey: 'custom.device_details.readyCheckDeepLinkDeviceTabBoundary'
      }),
      expect.objectContaining({
        key: 'ota',
        path: '/product/update-ota',
        query: {
          source: 'ready-check',
          ota_task_id: 'task-1',
          ota_detail_id: 'detail-1'
        },
        boundaryKey: 'custom.device_details.readyCheckDeepLinkOtaBoundary'
      }),
      expect.objectContaining({
        key: 'audit-log',
        path: '/system-management-user/system-log',
        query: expect.objectContaining({
          path: '/device/device-1'
        }),
        boundaryKey: 'custom.device_details.readyCheckDeepLinkAuditBoundary'
      })
    ]))

    state.openEvidenceDeepLink(state.evidenceDeepLinks.find((link: ReadyCheckDeepLink) => link.key === 'ota'))

    expect(hoisted.routerPush).toHaveBeenCalledWith({
      path: '/product/update-ota',
      query: {
        source: 'ready-check',
        ota_task_id: 'task-1',
        ota_detail_id: 'detail-1'
      }
    })
    expect(state.readyCheckDiagnosticSummary).toContain('/device/details?source=ota')
    expect(state.readyCheckDiagnosticSummary).toContain('custom.device_details.readyCheckDeepLinkAuditBoundary')
  })

  it('uses the recommended command collector to prefill Command Center drafts', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as ReadyCheckSetupState

    expect(state.recommendedCommandDraft).toEqual({
      identify: 'reboot',
      label: 'Reboot device',
      value: '{"force":false}'
    })

    state.openCommandCenter()

    expect(hoisted.routerPush).toHaveBeenCalledWith({
      path: '/device/command-center',
      query: expect.objectContaining({
        command_source: 'ready_check',
        command_identify: 'reboot',
        command_value: '{"force":false}',
        timeout_seconds: 60
      })
    })
  })

  it('gates the primary recommendation when frontend evidence collection is incomplete', async () => {
    hoisted.commandDataById.mockRejectedValueOnce(new Error('command catalog unavailable'))
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as ReadyCheckSetupState

    expect(state.collectionFailures).toEqual([
      {
        key: 'commands',
        labelKey: 'custom.device_details.readyCheckCollectionCommands'
      }
    ])
    expect(state.primaryReadyAction).toEqual(
      expect.objectContaining({
        key: 'collection-failure',
        status: 'attention',
        titleKey: 'custom.device_details.readyCheckCollectionWarningTitle',
        actionKey: 'custom.device_details.accessGuideDiagnosticRefresh'
      })
    )
    expect(state.readyCheckDiagnosticSummary).toContain('## 前端采集失败')
    expect(state.readyCheckDiagnosticSummary).toContain('commands: custom.device_details.readyCheckCollectionCommands')
  })

  it('copies all evidence deep links with boundary text', async () => {
    hoisted.writeClipboardText.mockResolvedValueOnce(true)
    const wrapper = mountComponent()
    await flushPromises()
    const state = wrapper.vm.$.setupState as ReadyCheckSetupState

    await state.copyAllEvidenceDeepLinks()

    expect(hoisted.writeClipboardText).toHaveBeenCalledWith(expect.stringContaining('/device/details?d_id=device-1'))
    expect(hoisted.writeClipboardText).toHaveBeenCalledWith(
      expect.stringContaining('custom.device_details.readyCheckEvidenceBoundary')
    )
  })
})
