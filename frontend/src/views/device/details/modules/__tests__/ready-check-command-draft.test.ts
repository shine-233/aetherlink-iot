import { buildCommandJobReadiness } from '@/views/device/command-center/commandCenterSubmitGate'
import { buildFleetCommandPayload, parseCommandCenterScopeContext } from '@/views/device/command-center/commandCenterState'
import { parseCommandCenterRouteDraft } from '@/views/device/command-center/commandCenterRouteDraft'
import {
  buildReadyCheckCommandCenterQuery,
  buildRecommendedCommandDraft,
  normalizeRecommendedCommandValue
} from '../ready-check-command-draft'

describe('ready-check-command-draft', () => {
  it('keeps the first usable command as a Command Center route draft', () => {
    const draft = buildRecommendedCommandDraft([
      { data_name: 'blank command' },
      {
        data_identifier: 'collect_diagnostics',
        data_name: 'Collect diagnostics',
        params: { mode: 'quick' }
      }
    ])

    expect(draft).toEqual({
      identify: 'collect_diagnostics',
      label: 'Collect diagnostics',
      value: '{"mode":"quick"}'
    })

    const query = buildReadyCheckCommandCenterQuery({
      deviceId: 'dev-1',
      draft,
      timeoutSeconds: 75
    })
    expect(query).toMatchObject({
      device_ids: 'dev-1',
      fleet_source: 'device_details',
      fleet_scope: 'single_device',
      fleet_selected_count: 1,
      first_device_id: 'dev-1',
      command_source: 'ready_check',
      command_identify: 'collect_diagnostics',
      command_value: '{"mode":"quick"}',
      timeout_seconds: 75
    })

    const routeDraft = parseCommandCenterRouteDraft(query)
    expect(routeDraft).toMatchObject({
      identify: 'collect_diagnostics',
      value: '{"mode":"quick"}',
      source: 'ready_check',
      timeoutSeconds: 75,
      hasDraft: true
    })
    expect(routeDraft.signature).toBe('ready_check|collect_diagnostics|{"mode":"quick"}|75')
  })

  it('keeps Ready Check single-device scope on the selected-device execution path', () => {
    const query = buildReadyCheckCommandCenterQuery({
      deviceId: 'dev-1',
      draft: {
        identify: 'collect_diagnostics',
        label: 'Collect diagnostics',
        value: '{"mode":"quick"}'
      }
    })
    const scopeContext = parseCommandCenterScopeContext(query)
    expect(scopeContext.scopeType).toBe('selected_devices')
    expect(scopeContext.routeScope).toBe('single_device')
    expect(scopeContext.deviceIds).toEqual(['dev-1'])
    expect(scopeContext.deviceFilter).toEqual({})

    const payload = buildFleetCommandPayload({
      deviceIds: scopeContext.deviceIds,
      scopeType: scopeContext.scopeType,
      identify: String(query.command_identify),
      value: String(query.command_value),
      timeoutSeconds: Number(query.timeout_seconds)
    })

    expect(payload).toEqual({
      device_ids: ['dev-1'],
      scope_type: 'selected_devices',
      identify: 'collect_diagnostics',
      value: '{"mode":"quick"}',
      timeout_seconds: 60
    })

    const readiness = buildCommandJobReadiness(
      {
        hasCommandJobScope: true,
        commandIdentify: String(query.command_identify),
        previewResult: null,
        previewPayloadFingerprint: 'before-preview',
        currentPayloadFingerprint: 'before-preview',
        previewCoversFullFilterScope: true
      },
      (key) => key
    )
    expect(readiness.canPreview).toBe(true)
    expect(readiness.canSubmit).toBe(false)
    expect(readiness.blockingReason).toBe('custom.commandCenter.submitBlockedPreviewMissing')
  })

  it('drops unsafe oversized command values instead of preloading a huge route draft', () => {
    expect(normalizeRecommendedCommandValue('x'.repeat(4001))).toBe('')
    expect(
      buildRecommendedCommandDraft([
        {
          identify: 'too_large',
          value: 'x'.repeat(4001)
        }
      ])
    ).toEqual({
      identify: 'too_large',
      label: 'too_large',
      value: ''
    })
  })
})
