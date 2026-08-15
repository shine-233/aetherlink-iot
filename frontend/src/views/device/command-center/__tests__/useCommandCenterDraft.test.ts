import { afterEach, describe, expect, it, vi } from 'vitest'
import { useCommandCenterDraft } from '../useCommandCenterDraft'

describe('useCommandCenterDraft', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('blocks preview and submit when payload looks like invalid JSON', () => {
    const setError = vi.fn()
    const draft = useCommandCenterDraft({
      selectedDeviceIds: () => ['dev-1'],
      scopeType: () => 'selected_devices',
      deviceFilter: () => ({}),
      requestedTotal: () => 1,
      currentPageCount: () => 1,
      source: () => 'device_manage',
      hasSelectedDevices: () => true,
      hasDeviceFilter: () => false,
      setError,
      t: (key: string) => key
    })

    draft.commandIdentify.value = 'reboot'
    draft.commandValue.value = '{"delay":'

    expect(draft.validateFleetCommandPayload()).toBe(false)
    expect(setError).toHaveBeenLastCalledWith('custom.commandCenter.commandValueInvalidJson')
  })

  it('blocks raw plain-text payloads because backend requires valid JSON', () => {
    const setError = vi.fn()
    const draft = useCommandCenterDraft({
      selectedDeviceIds: () => ['dev-1'],
      scopeType: () => 'selected_devices',
      deviceFilter: () => ({}),
      requestedTotal: () => 1,
      currentPageCount: () => 1,
      source: () => 'device_manage',
      hasSelectedDevices: () => true,
      hasDeviceFilter: () => false,
      setError,
      t: (key: string) => key
    })

    draft.commandIdentify.value = 'restart'
    draft.commandValue.value = 'restart now'

    expect(draft.validateFleetCommandPayload()).toBe(false)
    expect(setError).toHaveBeenLastCalledWith('custom.commandCenter.commandValueInvalidJson')
  })

  it('allows JSON string payloads for text-based commands', () => {
    const setError = vi.fn()
    const draft = useCommandCenterDraft({
      selectedDeviceIds: () => ['dev-1'],
      scopeType: () => 'selected_devices',
      deviceFilter: () => ({}),
      requestedTotal: () => 1,
      currentPageCount: () => 1,
      source: () => 'device_manage',
      hasSelectedDevices: () => true,
      hasDeviceFilter: () => false,
      setError,
      t: (key: string) => key
    })

    draft.commandIdentify.value = 'restart'
    draft.commandValue.value = '"restart now"'

    expect(draft.validateFleetCommandPayload()).toBe(true)
  })

  it('keeps the planned execution time in the preview and submit payload fingerprint', () => {
    const draft = useCommandCenterDraft({
      selectedDeviceIds: () => ['dev-1'],
      scopeType: () => 'selected_devices',
      deviceFilter: () => ({}),
      requestedTotal: () => 1,
      currentPageCount: () => 1,
      source: () => 'device_manage',
      hasSelectedDevices: () => true,
      hasDeviceFilter: () => false,
      setError: vi.fn(),
      t: (key: string) => key
    })

    draft.commandIdentify.value = 'reboot'
    draft.scheduledAt.value = Date.parse('2026-07-20T01:00:00.000Z')

    expect(draft.buildCurrentFleetCommandPayload().scheduled_at).toBe('2026-07-20T01:00:00.000Z')
    expect(draft.currentPayloadFingerprint.value).toContain('2026-07-20T01:00:00.000Z')
  })

  it('blocks an accidental past schedule instead of silently dispatching immediately', () => {
    vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-07-20T00:00:00.000Z'))
    const setError = vi.fn()
    const draft = useCommandCenterDraft({
      selectedDeviceIds: () => ['dev-1'],
      scopeType: () => 'selected_devices',
      deviceFilter: () => ({}),
      requestedTotal: () => 1,
      currentPageCount: () => 1,
      source: () => 'device_manage',
      hasSelectedDevices: () => true,
      hasDeviceFilter: () => false,
      setError,
      t: (key: string) => key
    })

    draft.commandIdentify.value = 'reboot'
    draft.scheduledAt.value = Date.parse('2026-07-19T23:59:59.999Z')

    expect(draft.validateFleetCommandPayload()).toBe(false)
    expect(setError).toHaveBeenLastCalledWith('custom.commandCenter.scheduleMustBeFuture')
  })
})
