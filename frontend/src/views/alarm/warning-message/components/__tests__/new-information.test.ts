/**
 * 文件用途：覆盖 new-information 在 告警消息管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  warningMessageList: vi.fn(),
  delInfo: vi.fn(),
  editInfo: vi.fn(),
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  currentInstanceProxy: {
    getPlatform: () => false
  }
}))

vi.mock('@/service/api/alarm', () => ({
  warningMessageList: hoisted.warningMessageList,
  delInfo: hoisted.delInfo,
  editInfo: hoisted.editInfo
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('~/packages/hooks', () => ({
  useBoolean: () => ({
    bool: ref(false),
    setTrue: vi.fn(() => {}),
    setFalse: vi.fn(() => {}),
    setBool: vi.fn(() => {})
  })
}))

vi.mock('vue', async importOriginal => {
  const actual = await importOriginal<typeof import('vue')>()
  return {
    ...actual,
    getCurrentInstance: () => ({
      proxy: hoisted.currentInstanceProxy
    })
  }
})

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      success: hoisted.messageSuccess,
      error: hoisted.messageError,
      warning: vi.fn(),
      info: vi.fn()
    })
  }
})

vi.mock('../pop-up.vue', () => ({
  default: defineComponent({
    name: 'PopUpStub',
    setup() {
      return () => h('div', { class: 'pop-up-stub' })
    }
  })
}))

import NewInformation from '../new-information.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(NewInformation, {
    global: {
      stubs: {
        NButton: true,
        NDataTable: true,
        NPopconfirm: true,
        IconIcRoundPlus: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) =>
  wrapper.vm.$.setupState as Record<string, any>

describe('new-information.vue', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    hoisted.warningMessageList.mockResolvedValue({
      data: {
        list: [
          { id: 'a1', name: 'Alarm 1', description: 'desc 1', alarm_level: 'H', enabled: 'Y', notification_group_name: 'Group 1' },
          { id: 'a2', name: 'Alarm 2', description: 'desc 2', alarm_level: 'L', enabled: 'N', notification_group_name: 'Group 2' }
        ],
        total: 2
      }
    })
    hoisted.delInfo.mockResolvedValue({ data: null })
    hoisted.editInfo.mockResolvedValue({ data: { id: 'a1' } })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    vi.useRealTimers()
  })

  describe('initial load', () => {
    it('calls list() on mount with the current pagination', async () => {
      mountComponent()
      await flushPromises()

      expect(hoisted.warningMessageList).toHaveBeenCalledTimes(1)
      expect(hoisted.warningMessageList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    })

    it('populates tableData after setTimeout completes', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.tableData).toHaveLength(2)
      expect(setupState.tableData[0].name).toBe('Alarm 1')
      expect(setupState.tableData[1].name).toBe('Alarm 2')
    })

    it('sets loading to true during fetch and false after setTimeout', async () => {
      let resolveList: (value: any) => void = () => undefined
      hoisted.warningMessageList.mockImplementationOnce(
        () =>
          new Promise(resolve => {
            resolveList = resolve
          })
      )
      const wrapper = mountComponent()

      const setupState = getSetupState(wrapper)
      expect(setupState.loading).toBe(true)

      resolveList({ data: { list: [], total: 0 } })
      await flushPromises()

      expect(setupState.loading).toBe(false)
    })

    it('assigns operatorBtn with disable option for enabled=Y items', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      const setupState = getSetupState(wrapper)
      const enabledItem = setupState.tableData.find((item: any) => item.enabled === 'Y')
      expect(enabledItem.operatorBtn).toHaveLength(3)
      const types = enabledItem.operatorBtn.map((b: any) => b.type)
      expect(types).toContain('edit')
      expect(types).toContain('enable')
      expect(types).toContain('delete')
      // For enabled=Y, the enable button should say "disable"
      const enableBtn = enabledItem.operatorBtn.find((b: any) => b.type === 'enable')
      expect(enableBtn.btnName).toBe('page.manage.common.status.disable')
    })

    it('assigns operatorBtn with enable option for enabled=N items', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      const setupState = getSetupState(wrapper)
      const disabledItem = setupState.tableData.find((item: any) => item.enabled === 'N')
      const enableBtn = disabledItem.operatorBtn.find((b: any) => b.type === 'enable')
      expect(enableBtn.btnName).toBe('page.manage.common.status.enable')
    })

    it('sets pagination.itemCount from total', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.pagination.itemCount).toBe(2)
    })

    it('does not update tableData when data is falsy', async () => {
      hoisted.warningMessageList.mockResolvedValue({ data: null })
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.tableData).toEqual([])
    })
  })

  describe('rowKey', () => {
    it('returns row.id', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.rowKey({ id: 'test-id' })).toBe('test-id')
    })
  })

  describe('setModalType', () => {
    it('sets modalType to the given value', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.setModalType('edit')
      expect(setupState.modalType).toBe('edit')

      setupState.setModalType('add')
      expect(setupState.modalType).toBe('add')
    })
  })

  describe('addWarningMessageBut', () => {
    it('opens modal and sets type to add', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.addWarningMessageBut()
      expect(setupState.modalType).toBe('add')
    })
  })

  describe('newEdit', () => {
    it('calls list() to refresh data', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()

      const setupState = getSetupState(wrapper)
      hoisted.warningMessageList.mockResolvedValue({ data: { list: [], total: 0 } })
      await setupState.newEdit()
      await flushPromises()

      expect(hoisted.warningMessageList).toHaveBeenCalledTimes(1)
      expect(hoisted.warningMessageList).toHaveBeenLastCalledWith({ page: 1, page_size: 10 })
    })
  })

  describe('handleEditPwd', () => {
    it('sets editData, modalType, and opens modal for edit type', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const row = { id: 'a1', name: 'Alarm 1', enabled: 'Y' }
      setupState.handleEditPwd(row, 'edit')

      expect(setupState.editData).toEqual(row)
      expect(setupState.modalType).toBe('edit')
    })

    it('toggles enabled to N and calls editInfos for enable type when currently Y', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      const row = { id: 'a1', name: 'Alarm 1', enabled: 'Y' }
      hoisted.editInfo.mockResolvedValue({ data: { id: 'a1' } })
      setupState.handleEditPwd(row, 'enable')
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      expect(hoisted.editInfo).toHaveBeenCalledWith(
        expect.objectContaining({
          ID: 'a1',
          enabled: 'N'
        })
      )
    })

    it('toggles enabled to Y and calls editInfos for enable type when currently N', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      const row = { id: 'a2', name: 'Alarm 2', enabled: 'N' }
      hoisted.editInfo.mockResolvedValue({ data: { id: 'a2' } })
      setupState.handleEditPwd(row, 'enable')
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      expect(hoisted.editInfo).toHaveBeenCalledWith(
        expect.objectContaining({
          ID: 'a2',
          enabled: 'Y'
        })
      )
    })
  })

  describe('editInfos', () => {
    it('shows startSuccess when enabled=Y and data is truthy', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      // Set params to enabled=Y
      setupState.params.ID = 'a1'
      setupState.params.enabled = 'Y'
      hoisted.editInfo.mockResolvedValue({ data: { id: 'a1' } })

      await setupState.editInfos()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.startSuccess')
    })

    it('shows stopSuccess when enabled=N and data is truthy', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      setupState.params.ID = 'a1'
      setupState.params.enabled = 'N'
      hoisted.editInfo.mockResolvedValue({ data: { id: 'a1' } })

      await setupState.editInfos()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.stopSuccess')
    })

    it('shows startFail when enabled=Y and data is falsy', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      setupState.params.ID = 'a1'
      setupState.params.enabled = 'Y'
      hoisted.editInfo.mockResolvedValue({ data: null })

      await setupState.editInfos()
      await flushPromises()

      expect(hoisted.messageError).toHaveBeenCalledWith('common.startFail')
    })

    it('shows stopFail when enabled=N and data is falsy', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      setupState.params.ID = 'a1'
      setupState.params.enabled = 'N'
      hoisted.editInfo.mockResolvedValue({ data: null })

      await setupState.editInfos()
      await flushPromises()

      expect(hoisted.messageError).toHaveBeenCalledWith('common.stopFail')
    })
  })

  describe('handleDeleteTable', () => {
    it('sets deleteId and calls deleteInfo', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      hoisted.delInfo.mockResolvedValue({ data: null })
      setupState.handleDeleteTable({ id: 'del-1' })
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      expect(hoisted.delInfo).toHaveBeenCalledWith('del-1')
    })
  })

  describe('deleteInfo', () => {
    it('shows deleteSuccess when data is falsy', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      setupState.deleteId = 'del-1'
      hoisted.delInfo.mockResolvedValue({ data: null })

      await setupState.deleteInfo()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.deleteSuccess')
    })

    it('shows deleteFail when data is truthy', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      setupState.deleteId = 'del-1'
      hoisted.delInfo.mockResolvedValue({ data: { error: 'something' } })

      await setupState.deleteInfo()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      expect(hoisted.messageError).toHaveBeenCalledWith('common.deleteFail')
    })

    it('calls list() after delete to refresh data', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      setupState.deleteId = 'del-1'
      hoisted.delInfo.mockResolvedValue({ data: null })
      hoisted.warningMessageList.mockResolvedValue({ data: { list: [], total: 0 } })

      await setupState.deleteInfo()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      expect(hoisted.warningMessageList).toHaveBeenCalledTimes(1)
      expect(hoisted.warningMessageList).toHaveBeenLastCalledWith({ page: 1, page_size: 10 })
    })
  })

  describe('pagination', () => {
    it('onChange updates page and calls list()', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      hoisted.warningMessageList.mockResolvedValue({ data: { list: [], total: 0 } })
      setupState.pagination.onChange(3)
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      expect(setupState.pagination.page).toBe(3)
      expect(hoisted.warningMessageList).toHaveBeenCalledTimes(1)
      expect(hoisted.warningMessageList).toHaveBeenCalledWith({ page: 3, page_size: 10 })
    })

    it('onUpdatePageSize updates pageSize, resets page to 1, and calls list()', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.advanceTimersByTime(1000)
      vi.clearAllMocks()
      const setupState = getSetupState(wrapper)

      hoisted.warningMessageList.mockResolvedValue({ data: { list: [], total: 0 } })
      setupState.pagination.onUpdatePageSize(20)
      await flushPromises()
      vi.advanceTimersByTime(1000)
      await flushPromises()

      expect(setupState.pagination.pageSize).toBe(20)
      expect(setupState.pagination.page).toBe(1)
      expect(hoisted.warningMessageList).toHaveBeenCalledTimes(1)
      expect(hoisted.warningMessageList).toHaveBeenCalledWith({ page: 1, page_size: 20 })
    })
  })

  describe('list', () => {
    it('refreshes table rows and total from the alarm API', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.clearAllMocks()

      hoisted.warningMessageList.mockResolvedValue({
        data: { list: [{ id: 'g2', name: 'Alarm 2', enabled: 'N' }], total: 1 }
      })
      await getSetupState(wrapper).list()

      expect(hoisted.warningMessageList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
      expect(getSetupState(wrapper).tableData).toEqual([
        expect.objectContaining({ id: 'g2', name: 'Alarm 2' })
      ])
      expect(getSetupState(wrapper).pagination.itemCount).toBe(1)
    })
  })

  describe('columns', () => {
    it('has 6 columns including actions', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.columns).toHaveLength(6)
      const keys = setupState.columns.map((c: any) => c.key)
      expect(keys).toEqual(['name', 'description', 'alarm_level', 'notification_group_name', 'enabled', 'actions'])
    })

    it('alarm_level column render returns high for H', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const alarmLevelCol = setupState.columns.find((c: any) => c.key === 'alarm_level')
      expect(alarmLevelCol.render({ alarm_level: 'H' })).toBe('common.high')
      expect(alarmLevelCol.render({ alarm_level: 'M' })).toBe('common.middle')
      expect(alarmLevelCol.render({ alarm_level: 'L' })).toBe('common.low')
      expect(alarmLevelCol.render({ alarm_level: 'X' })).toBe('common.low')
    })

    it('enabled column render returns enable for Y and disable for N', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      const enabledCol = setupState.columns.find((c: any) => c.key === 'enabled')
      expect(enabledCol.render({ enabled: 'Y' })).toBe('page.manage.common.status.enable')
      expect(enabledCol.render({ enabled: 'N' })).toBe('page.manage.common.status.disable')
    })
  })

  describe('getPlatform computed', () => {
    it('returns value from proxy.getPlatform()', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.getPlatform).toBe(false)
    })

    it('returns true when proxy.getPlatform returns true', async () => {
      hoisted.currentInstanceProxy.getPlatform = () => true
      const wrapper = mountComponent()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.getPlatform).toBe(true)
      // Reset for other tests
      hoisted.currentInstanceProxy.getPlatform = () => false
    })
  })
})
