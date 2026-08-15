import { computed, reactive, ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useDistributionSubmitFlow } from '../useDistributionSubmitFlow'

const hoisted = vi.hoisted(() => ({
  commandDataPub: vi.fn(),
  messageError: vi.fn(),
  messageSuccess: vi.fn(),
  messageWarning: vi.fn(),
  isJSON: vi.fn((value: string) => {
    try {
      JSON.parse(value)
      return true
    } catch {
      return false
    }
  })
}))

vi.mock('@/service/api', () => ({
  commandDataPub: hoisted.commandDataPub
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/tool', () => ({
  isJSON: hoisted.isJSON
}))

vi.mock('dayjs', () => ({
  default: vi.fn(() => ({ format: vi.fn(() => '2024-01-01T00:00:00+00:00') }))
}))

const createFlow = (overrides: Partial<Parameters<typeof useDistributionSubmitFlow>[0]> = {}) => {
  const submitApi = vi.fn().mockResolvedValue({ error: null })
  const expectApi = vi.fn().mockResolvedValue({ error: null })
  const fetchData = vi.fn().mockResolvedValue(undefined)
  const closeDialog = vi.fn()
  const logger = { error: vi.fn() }
  const directMethodApi = vi.fn().mockResolvedValue({ error: null })
  const onDirectMethodResult = vi.fn()
  const onSubmitTracking = vi.fn()
  const formModel = reactive({
    commandValue: 'reboot',
    textValue: '',
    expected: false,
    time: null as number | null,
    timeoutSeconds: 10,
    waitForResponse: false
  })
  const paramsData = ref<any[]>([{ data_identifier: 'delay', delay: 5 }])
  const attributeList = ref<any[]>([
    { data_identifier: 'mode', inputValue: 'auto', attributeType: 'string', checked: true }
  ])

  const flow = useDistributionSubmitFlow({
    activeTab: ref('visual'),
    attributeList,
    closeDialog,
    deviceId: () => 'device-1',
    directMethodApi: () => directMethodApi,
    expectApi: () => expectApi,
    fetchData,
    formModel,
    formRef: ref({ validate: vi.fn().mockResolvedValue(undefined) } as any),
    hasAttributeSelection: computed(() => attributeList.value.some((item) => item.checked)),
    isCommand: () => true,
    logger,
    onDirectMethodResult,
    onSubmitTracking,
    paramsData,
    submitApi: () => submitApi,
    ...overrides
  })

  return {
    attributeList,
    closeDialog,
    directMethodApi,
    expectApi,
    fetchData,
    flow,
    formModel,
    logger,
    onDirectMethodResult,
    onSubmitTracking,
    paramsData,
    submitApi
  }
}

describe('useDistributionSubmitFlow', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(window as any).$message = {
      error: hoisted.messageError,
      success: hoisted.messageSuccess,
      warning: hoisted.messageWarning
    }
    hoisted.commandDataPub.mockResolvedValue({ error: null })
  })

  it('submits visual command payload through the immediate command API', async () => {
    const { closeDialog, fetchData, flow, onSubmitTracking, submitApi } = createFlow()
    submitApi.mockResolvedValueOnce({ error: null, data: { message_id: 'cmd-track-1' } })

    await flow.submit()

    expect(submitApi).toHaveBeenCalledWith({
      device_id: 'device-1',
      identify: 'reboot',
      value: JSON.stringify({ delay: 5 })
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledWith(
      'generate.commandSubmittedWithMessageId cmd-track-1'
    )
    expect(onSubmitTracking).toHaveBeenCalledWith({ logRecorded: undefined, messageId: 'cmd-track-1', status: undefined })
    expect(fetchData).toHaveBeenCalled()
    expect(closeDialog).toHaveBeenCalled()
  })

  it('submits expected attribute payload with expiry and attribute send type', async () => {
    const { expectApi, flow, formModel } = createFlow({
      isCommand: () => false
    })
    formModel.expected = true
    formModel.time = 2

    await flow.submit()

    expect(expectApi).toHaveBeenCalledWith({
      device_id: 'device-1',
      expiry: '2024-01-01T00:00:00+00:00',
      identify: null,
      payload: JSON.stringify({ mode: 'auto' }),
      send_type: 'attribute'
    })
  })

  it('uses the direct method API and preserves the correlated device result', async () => {
    const { directMethodApi, flow, formModel, onDirectMethodResult, onSubmitTracking, submitApi } = createFlow()
    formModel.waitForResponse = true
    formModel.timeoutSeconds = 12
    directMethodApi.mockResolvedValueOnce({
      error: null,
      data: {
        message_id: 'direct-1',
        device_id: 'device-1',
        identify: 'reboot',
        status: '3',
        outcome: 'device_succeeded',
        published: true,
        log_recorded: true,
        device_responded: true,
        device_succeeded: true,
        timed_out: false,
        response_payload: '{"result":0}',
        timeout_seconds: 12,
        elapsed_ms: 81
      }
    })

    await flow.submit()

    expect(submitApi).not.toHaveBeenCalled()
    expect(directMethodApi).toHaveBeenCalledWith({
      device_id: 'device-1',
      identify: 'reboot',
      timeout_seconds: 12,
      value: JSON.stringify({ delay: 5 })
    })
    expect(onSubmitTracking).toHaveBeenCalledWith({ logRecorded: true, messageId: 'direct-1', status: '3' })
    expect(onDirectMethodResult).toHaveBeenCalledWith(expect.objectContaining({
      message_id: 'direct-1',
      outcome: 'device_succeeded',
      response_payload: '{"result":0}'
    }))
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('generate.directMethodSucceeded')
  })

  it('keeps the dialog open when attribute submit has no selected attributes', async () => {
    const { attributeList, closeDialog, flow, submitApi } = createFlow({
      isCommand: () => false
    })
    attributeList.value = [{ data_identifier: 'mode', inputValue: 'auto', attributeType: 'string', checked: false }]

    await flow.submit()

    expect(submitApi).not.toHaveBeenCalled()
    expect(closeDialog).not.toHaveBeenCalled()
  })

  it('guards quick command duplicate clicks and refreshes after a successful publish', async () => {
    const { fetchData, flow, onSubmitTracking } = createFlow()
    hoisted.commandDataPub.mockResolvedValueOnce({ error: null, data: { message_id: 'quick-track-1' } })

    await flow.onCommandChange({ id: 'cmd-1', instruct: '1', data_identifier: 'switch' })

    expect(hoisted.commandDataPub).toHaveBeenCalledWith({
      device_id: 'device-1',
      identify: 'switch',
      value: '1'
    })
    expect(hoisted.messageSuccess).toHaveBeenCalledWith(
      'generate.commandSubmittedWithMessageId quick-track-1'
    )
    expect(onSubmitTracking).toHaveBeenCalledWith({
      logRecorded: undefined,
      messageId: 'quick-track-1',
      status: undefined
    })
    expect(fetchData).toHaveBeenCalled()
    expect(flow.quickCommandLoadingId.value).toBe('')
  })

  it('does not imply command-log tracking when backend accepted publish but did not record the log', async () => {
    const { flow, submitApi } = createFlow()
    submitApi.mockResolvedValueOnce({ error: null, data: { log_recorded: false, message_id: 'cmd-no-log' } })

    await flow.submit()

    expect(hoisted.messageSuccess).toHaveBeenCalledWith(
      'generate.commandSubmittedLogUnavailable cmd-no-log'
    )
  })
})
