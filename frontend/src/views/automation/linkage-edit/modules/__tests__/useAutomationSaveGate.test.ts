import { describe, expect, it, vi } from 'vitest'
import { useAutomationSaveGate } from '../useAutomationSaveGate'

const t = (key: string) => key

describe('useAutomationSaveGate', () => {
  it('allows save after backend dry-run says the draft can be saved', async () => {
    const runBackendDryRunForPayload = vi.fn().mockResolvedValue({ can_save: true })
    const gate = useAutomationSaveGate({ runBackendDryRunForPayload, t })

    await expect(gate.ensureBackendDryRunCanSave({ actions: [] })).resolves.toBe(true)

    expect(runBackendDryRunForPayload).toHaveBeenCalledWith({ actions: [] })
    expect(gate.isSaveDryRunLoading.value).toBe(false)
  })

  it('blocks save with the backend dry-run blocker message', async () => {
    const error = vi.fn()
    const originalWindow = globalThis.window
    vi.stubGlobal('window', { ...originalWindow, $message: { error } })
    const runBackendDryRunForPayload = vi.fn().mockResolvedValue({ can_save: false, blockers: ['fix condition'] })
    const gate = useAutomationSaveGate({ runBackendDryRunForPayload, t })

    await expect(gate.ensureBackendDryRunCanSave({ actions: [] })).resolves.toBe(false)

    expect(error).toHaveBeenCalledWith('fix condition')
    expect(gate.isSaveDryRunLoading.value).toBe(false)
    vi.unstubAllGlobals()
  })
})