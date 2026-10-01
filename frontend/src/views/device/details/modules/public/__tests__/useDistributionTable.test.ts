import { flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  commandDataById: vi.fn(),
  deviceCustomCommandsIdList: vi.fn(),
  getAttributeDataSet: vi.fn()
}))

vi.mock('@/service/api', () => hoisted)
vi.mock('@/locales', () => ({ $t: (key: string) => key }))
vi.mock('@/utils/logger', () => ({ createLogger: () => ({ warn: vi.fn(), error: vi.fn(), info: vi.fn() }) }))
vi.mock('@aetherlink/hooks', () => ({
  useLoading: () => {
    const loading = ref(false)
    return {
      loading,
      startLoading: () => {
        loading.value = true
      },
      endLoading: () => {
        loading.value = false
      }
    }
  }
}))

import { useDistributionDialogState, useDistributionTable } from '../useDistributionTable'

describe('useDistributionTable', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('queries paged data, normalizes rows and loads quick commands for command mode', async () => {
    const fetchDataApi = vi.fn().mockResolvedValue({ data: { list: [{ id: 1 }], total: 9 }, error: null })
    hoisted.deviceCustomCommandsIdList.mockResolvedValue({ data: [{ id: 'c1' }] })
    const table = useDistributionTable({
      deviceId: () => 'dev-1',
      noRefresh: () => false,
      isCommand: () => true,
      fetchDataApi: () => fetchDataApi
    })

    table.initialize()
    await flushPromises()
    expect(fetchDataApi).toHaveBeenCalledWith({ page: 1, page_size: 4, device_id: 'dev-1' })
    expect(table.tableData.value).toEqual([{ id: 1 }])
    expect(table.page_coune.value).toBe(3)
    expect(table.commandList.value).toEqual([{ id: 'c1' }])
    expect(table.loading.value).toBe(false)

    table.updatePage(2)
    await flushPromises()
    expect(fetchDataApi).toHaveBeenLastCalledWith({ page: 2, page_size: 4, device_id: 'dev-1' })
    table.refresh()
    expect(table.the_page.value).toBe(1)
  })

  it('omits paging params in noRefresh mode and survives fetch errors', async () => {
    const fetchDataApi = vi.fn().mockRejectedValue(new Error('boom'))
    const table = useDistributionTable({
      deviceId: () => 'dev-1',
      noRefresh: () => true,
      isCommand: () => false,
      fetchDataApi: () => fetchDataApi
    })
    await table.fetchDataFunction()
    expect(fetchDataApi).toHaveBeenCalledWith({ page: undefined, page_size: undefined, device_id: 'dev-1' })
    expect(hoisted.deviceCustomCommandsIdList).not.toHaveBeenCalled()
    expect(table.loading.value).toBe(false)
  })
})

describe('useDistributionDialogState', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  const create = (overrides: { isCommand?: boolean; online?: boolean } = {}) =>
    useDistributionDialogState({
      deviceId: () => 'dev-1',
      isCommand: () => overrides.isCommand,
      directMethodOnline: () => overrides.online
    })

  it('loads attribute rows with the device_id contract when opening attribute dispatch', async () => {
    hoisted.getAttributeDataSet.mockResolvedValue({ data: [{ key: 'mode', data_name: 'Mode' }], error: null })
    const state = create({ isCommand: false })
    await state.openDialog()
    expect(hoisted.getAttributeDataSet).toHaveBeenCalledWith({ device_id: 'dev-1' })
    expect(state.showDialog.value).toBe(true)
    expect(state.attributeList.value[0]).toMatchObject({ key: 'mode', checked: false, inputValue: '' })
    expect(state.hasAttributeSelection.value).toBe(false)
    state.attributeList.value[0].checked = true
    expect(state.hasAttributeSelection.value).toBe(true)

    state.closeDialog()
    expect(state.showDialog.value).toBe(false)
    expect(state.attributeList.value).toEqual([])
  })

  it('loads command options and applies parameter templates', async () => {
    hoisted.commandDataById.mockResolvedValue({
      data: [{ data_identifier: 'reboot', params: [{ id: 'p1', data_identifier: 'force', param_type: 'Boolean' }] }]
    })
    const state = create({ isCommand: true })
    await state.loadCommandOptions(false)
    expect(hoisted.commandDataById).not.toHaveBeenCalled()
    await state.loadCommandOptions(true)
    state.handleCommandInput('reboot')
    expect(state.formModel.commandValue).toBe('reboot')
    expect(state.paramsData.value.length).toBeGreaterThanOrEqual(0)
    expect(state.rules.value.commandValue).toBeDefined()
  })

  it('flags invalid JSON and blocks direct-method submits with bad timeout or offline device', () => {
    const state = create({ isCommand: false, online: false })
    state.formModel.textValue = '{bad'
    expect(state.jsonInvalid.value).toBe(true)
    expect(state.isSubmitDisabled.value).toBe(true)

    state.formModel.textValue = '{"a":1}'
    expect(state.jsonInvalid.value).toBe(false)
    expect(state.isSubmitDisabled.value).toBe(false)

    state.formModel.waitForResponse = true
    expect(state.isSubmitDisabled.value).toBe(true)

    const online = create({ isCommand: false, online: true })
    online.formModel.textValue = '{"a":1}'
    online.formModel.waitForResponse = true
    online.formModel.timeoutSeconds = 45
    expect(online.isSubmitDisabled.value).toBe(true)
    online.formModel.timeoutSeconds = 10
    expect(online.isSubmitDisabled.value).toBe(false)
  })
})
