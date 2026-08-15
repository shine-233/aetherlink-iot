import { flushPromises } from '@vue/test-utils'
import { effectScope, nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useTelemetrySimulationDialog } from '../useTelemetrySimulationDialog'

const isJSON = (value: string) => {
  try {
    JSON.parse(value)
    return true
  } catch {
    return false
  }
}

const createDialog = (overrides: Partial<Parameters<typeof useTelemetrySimulationDialog>[0]> = {}) => {
  const scope = effectScope()
  const getSimulationInitRequest = vi.fn().mockResolvedValue({ data: null, error: new Error('offline') })
  const sendSimulationDataRequest = vi.fn().mockResolvedValue({ error: null })
  let dialog!: ReturnType<typeof useTelemetrySimulationDialog>

  scope.run(() => {
    dialog = useTelemetrySimulationDialog({
      getDeviceId: () => 'device-1',
      getSimulationInitRequest,
      sendSimulationDataRequest,
      translate: (key) => key,
      isJSON,
      ...overrides
    })
  })

  return {
    dialog,
    getSimulationInitRequest,
    scope,
    sendSimulationDataRequest
  }
}

describe('useTelemetrySimulationDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(window as any).$message = {
      error: vi.fn(),
      success: vi.fn(),
      warning: vi.fn()
    }
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('opens the dialog and falls back to built-in payloads when init fails', async () => {
    const { dialog, getSimulationInitRequest, scope } = createDialog()

    dialog.openUpLog()
    await flushPromises()

    expect(dialog.showLogDialog.value).toBe(true)
    expect(dialog.showAdvanced.value).toBe(false)
    expect(getSimulationInitRequest).toHaveBeenCalledWith({ device_id: 'device-1' })
    expect(dialog.simulationForm.default_data).toBe(
      '{"temperature":25.5,"humidity":60,"rssi":-52,"online":true,"alarm_count":0}'
    )
    expect(dialog.simulationForm.event_default_data).toBe(
      '{"method":"report_alarm","params":{"alarm_code":"over_temperature","level":"warning","value":38.5}}'
    )

    scope.stop()
  })

  it('backfills broker credentials and topic payloads from init data', async () => {
    const getSimulationInitRequest = vi.fn().mockResolvedValue({
      data: {
        username: 'sim-user',
        password: 'sim-pass',
        client_id: 'sim-client',
        server: 'sim.server',
        port: 8883,
        topic: 'devices/telemetry',
        topic_options: [{ label: 'telemetry', value: 'devices/telemetry' }],
        default_data: '{"sim":1}',
        event_default_data: '{"method":"Ping"}'
      },
      error: null
    })
    const { dialog, scope } = createDialog({ getSimulationInitRequest })

    dialog.openUpLog()
    await flushPromises()

    expect(dialog.simulationForm.username).toBe('sim-user')
    expect(dialog.simulationForm.password).toBe('sim-pass')
    expect(dialog.simulationForm.client_id).toBe('sim-client')
    expect(dialog.simulationForm.server).toBe('sim.server')
    expect(dialog.simulationForm.port).toBe(8883)
    expect(dialog.simulationForm.topic).toBe('devices/telemetry')
    expect(dialog.simulationForm.topic_options).toEqual([{ label: 'telemetry', value: 'devices/telemetry' }])
    expect(dialog.simulationForm.default_data).toBe('{"sim":1}')
    expect(dialog.simulationForm.normal_default_data).toBe('{"sim":1}')
    expect(dialog.simulationForm.event_default_data).toBe('{"method":"Ping"}')

    scope.stop()
  })

  it('remembers separate default payloads for telemetry and event topics', async () => {
    const { dialog, scope } = createDialog()

    dialog.simulationForm.default_data = '{"normal":1}'
    await nextTick()

    dialog.simulationForm.topic = 'devices/event/test'
    await nextTick()
    expect(dialog.simulationForm.default_data).toBe(
      '{"method":"report_alarm","params":{"alarm_code":"over_temperature","level":"warning","value":38.5}}'
    )

    dialog.simulationForm.default_data = '{"event":2}'
    await nextTick()

    dialog.simulationForm.topic = 'devices/telemetry'
    await nextTick()
    expect(dialog.simulationForm.default_data).toBe('{"normal":1}')

    dialog.simulationForm.topic = 'devices/event/test'
    await nextTick()
    expect(dialog.simulationForm.default_data).toBe('{"event":2}')

    scope.stop()
  })

  it('submits payloads, closes on success, and keeps the dialog open on failure', async () => {
    const sendSimulationDataRequest = vi
      .fn()
      .mockResolvedValueOnce({ error: null })
      .mockResolvedValueOnce({
        error: { message: 'send failed' }
      })
    const { dialog, scope } = createDialog({ sendSimulationDataRequest })

    dialog.openUpLog()
    await flushPromises()
    await dialog.sendSimulationDataByForm()

    expect(sendSimulationDataRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        data: '{"temperature":25.5,"humidity":60,"rssi":-52,"online":true,"alarm_count":0}',
        device_id: 'device-1',
        port: 1883,
        topic: 'devices/telemetry'
      })
    )
    expect(dialog.showLogDialog.value).toBe(false)
    expect((window as any).$message.success).toHaveBeenCalledWith('custom.devicePage.success')

    dialog.showLogDialog.value = true
    await dialog.sendSimulationDataByForm()

    expect(dialog.showLogDialog.value).toBe(true)
    expect(dialog.showError.value).toBe(true)
    expect(dialog.erroMessage.value).toBe('send failed')

    scope.stop()
  })

  it('resets error and advanced state when opening, then can toggle and close the dialog', async () => {
    const { dialog, scope } = createDialog()

    dialog.showError.value = true
    dialog.erroMessage.value = 'old error'
    dialog.showAdvanced.value = true

    dialog.openUpLog()
    await flushPromises()

    expect(dialog.showLogDialog.value).toBe(true)
    expect(dialog.showError.value).toBe(false)
    expect(dialog.showAdvanced.value).toBe(false)

    dialog.toggleAdvanced()
    expect(dialog.showAdvanced.value).toBe(true)
    dialog.toggleAdvanced()
    expect(dialog.showAdvanced.value).toBe(false)

    dialog.closeSimulationDialog()
    expect(dialog.showLogDialog.value).toBe(false)

    scope.stop()
  })

  it('keeps loading true while a simulation submit is pending and clears it after throw', async () => {
    let rejectRequest!: (error: Error) => void
    const sendSimulationDataRequest = vi.fn(
      () =>
        new Promise((_, reject) => {
          rejectRequest = reject
        })
    )
    const { dialog, scope } = createDialog({ sendSimulationDataRequest })

    const pending = dialog.sendSimulationDataByForm()
    expect(dialog.simulationLoading.value).toBe(true)

    rejectRequest(new Error('network down'))
    await pending

    expect(dialog.simulationLoading.value).toBe(false)
    expect(dialog.showError.value).toBe(true)
    expect(dialog.erroMessage.value).toBe('network down')

    scope.stop()
  })

  it('prefers server response error messages over generic error messages', async () => {
    const sendSimulationDataRequest = vi.fn().mockResolvedValue({
      error: {
        message: 'generic',
        response: {
          data: {
            message: 'server says no'
          }
        }
      }
    })
    const { dialog, scope } = createDialog({ sendSimulationDataRequest })

    await dialog.sendSimulationDataByForm()

    expect(dialog.showError.value).toBe(true)
    expect(dialog.erroMessage.value).toBe('server says no')

    scope.stop()
  })

  it('blocks empty payloads before sending', async () => {
    const { dialog, scope, sendSimulationDataRequest } = createDialog()

    dialog.simulationForm.default_data = ''
    await dialog.sendSimulationDataByForm()

    expect((window as any).$message.error).toHaveBeenCalledWith('custom.device_details.sendInputData')
    expect(sendSimulationDataRequest).not.toHaveBeenCalled()

    scope.stop()
  })

  it('formats, clears, and copies the editable simulation payload', async () => {
    const { dialog, scope } = createDialog()
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', {
      clipboard: { writeText }
    } as unknown as Navigator)

    dialog.simulationForm.default_data = '{"b":2,"a":1}'
    dialog.formatSimulationData()
    expect(dialog.simulationForm.default_data).toBe(JSON.stringify({ b: 2, a: 1 }, null, 2))

    await dialog.copySimulationData()
    expect(writeText).toHaveBeenCalledWith(JSON.stringify({ b: 2, a: 1 }, null, 2))
    expect((window as any).$message.success).toHaveBeenCalledWith('theme.configOperation.copySuccess')

    dialog.clearSimulationData()
    expect(dialog.simulationForm.default_data).toBe('')

    dialog.formatSimulationData()
    await dialog.copySimulationData()
    expect(writeText).toHaveBeenCalledTimes(1)

    scope.stop()
  })

  it('warns when formatting non-json and reports clipboard failures', async () => {
    const { dialog, scope } = createDialog()
    const writeText = vi.fn().mockRejectedValue(new Error('copy failed'))
    vi.stubGlobal('navigator', {
      clipboard: { writeText }
    } as unknown as Navigator)

    dialog.simulationForm.default_data = 'not-json'
    dialog.formatSimulationData()
    expect(dialog.simulationForm.default_data).toBe('not-json')
    expect((window as any).$message.warning).toHaveBeenCalledWith('custom.device_details.notJsonNoFormat')

    dialog.simulationForm.default_data = '{"copy":1}'
    await dialog.copySimulationData()
    expect((window as any).$message.error).toHaveBeenCalledWith('common.copyFailed')

    scope.stop()
  })

  it('leaves the current payload unchanged when topic is empty', async () => {
    const { dialog, scope } = createDialog()

    dialog.simulationForm.default_data = '{"before":1}'
    dialog.simulationForm.topic = ''
    await nextTick()

    expect(dialog.simulationForm.default_data).toBe('{"before":1}')

    scope.stop()
  })
})
