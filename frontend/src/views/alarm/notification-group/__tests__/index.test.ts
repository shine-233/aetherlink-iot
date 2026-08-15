/**
 * 文件用途：覆盖 index 在 告警通知组管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getNotificationGroupList: vi.fn(),
  getNotificationGroupDetail: vi.fn(),
  deleteNotificationGroup: vi.fn(),
  putNotificationGroup: vi.fn(),
  startLoading: vi.fn(),
  endLoading: vi.fn(),
  setTrue: vi.fn(),
  setFalse: vi.fn(),
}))

vi.mock('@/service/api/notification', () => ({
  getNotificationGroupList: hoisted.getNotificationGroupList,
  getNotificationGroupDetail: hoisted.getNotificationGroupDetail,
  deleteNotificationGroup: hoisted.deleteNotificationGroup,
  putNotificationGroup: hoisted.putNotificationGroup,
}))

vi.mock('@/constants/business', () => ({
  notificationOptions: [
    { label: 'Email', value: 'email' },
    { label: 'SMS', value: 'sms' }
  ]
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('~/packages/hooks', () => ({
  useLoading: () => ({ loading: { value: false }, startLoading: hoisted.startLoading, endLoading: hoisted.endLoading }),
  useBoolean: () => ({ bool: { value: false }, setTrue: hoisted.setTrue, setFalse: hoisted.setFalse }),
  useContext: vi.fn(() => ({ setupStore: vi.fn(), useStore: vi.fn() })),
}))

vi.mock('../components/table-action-modal.vue', () => ({
  default: defineComponent({
    name: 'TableActionModal',
    props: ['visible', 'type', 'editData'],
    emits: ['getTableData', 'update:visible'],
    setup() {
      return () => h('div', { 'data-test': 'table-action-modal' })
    }
  })
}))

import NotificationGroup from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const nCardStub = defineComponent({
    name: 'NCard',
    props: ['title'],
    setup(componentProps, { slots }) {
      return () =>
        h('div', { title: componentProps.title as string }, [slots['header-extra']?.(), slots.default?.()])
    }
  })

  const wrapper = shallowMount(NotificationGroup, {
    props,
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        NCard: nCardStub,
        'n-card': nCardStub,
        NButton: true,
        'n-button': true,
        NDataTable: true,
        'n-data-table': true,
        NPagination: true,
        'n-pagination': true,
        NSpace: true,
        'n-space': true,
        NPopconfirm: true,
        'n-popconfirm': true,
        NSwitch: true,
        'n-switch': true,
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('NotificationGroup', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  describe('mount and initial data fetch', () => {
    it('should mount and fetch table data on init', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      expect(hoisted.getNotificationGroupList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    })

    it('should call startLoading and endLoading during fetch', async () => {
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      mountComponent()
      expect(hoisted.startLoading).toHaveBeenCalledTimes(1)
      await flushPromises()
      expect(hoisted.endLoading).toHaveBeenCalledTimes(1)
    })

    it('should populate table data on successful fetch', async () => {
      const mockData = [
        { id: '1', name: 'Group1', notification_type: 'email', status: 'OPEN' },
        { id: '2', name: 'Group2', notification_type: 'sms', status: 'CLOSE' }
      ]
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: mockData, total: 2 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      expect(state.tableData).toEqual(mockData)
      expect(state.total).toBe(2)
    })

    it('should handle empty list response', async () => {
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      expect(state.tableData).toEqual([])
      expect(state.total).toBe(0)
    })

    it('should handle response without data', async () => {
      hoisted.getNotificationGroupList.mockResolvedValue(null)
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      expect(state.tableData).toEqual([])
    })

    it('should handle response with data but no list', async () => {
      hoisted.getNotificationGroupList.mockResolvedValue({ data: {} })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      expect(state.tableData).toEqual([])
      expect(state.total).toBe(0)
    })
  })

  describe('rendering', () => {
    it('should render NCard with notification-group title', async () => {
      const wrapper = mountComponent()
      await flushPromises()

      expect(wrapper.getComponent({ name: 'NCard' }).attributes('title')).toBe('generate.notification-group')
      expect(wrapper.findAllComponents({ name: 'NDataTable' })).toHaveLength(1)
      expect(wrapper.findAllComponents({ name: 'NPagination' })).toHaveLength(1)
      expect(wrapper.findAllComponents({ name: 'TableActionModal' })).toHaveLength(1)
      expect(wrapper.html()).toContain('device_template.add')
    })

    it('should render TableActionModal component', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      expect(wrapper.html()).toContain('table-action-modal')
    })

    it('should render HTML containing notification-group i18n key', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      expect(wrapper.html()).toContain('generate.notification-group')
    })

    it('should render NButton for add action', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      expect(wrapper.html()).toContain('button-stub')
    })
  })

  describe('columns', () => {
    it('should have name column with correct title', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      const nameCol = state.columns.find(c => c.key === 'name')
      expect(nameCol).toMatchObject({
        title: 'generate.notification-group-name',
        minWidth: '140px',
        align: 'left'
      })
    })

    it('should have notification_type column that renders label from options', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      const typeCol = state.columns.find(c => c.key === 'notification_type')
      expect(typeCol).toMatchObject({
        title: 'generate.notification-type',
        align: 'left',
        minWidth: '140px'
      })
      expect(typeCol?.render({ notification_type: 'email' })).toBe('Email')
      expect(typeCol?.render({ notification_type: 'sms' })).toBe('SMS')
      expect(typeCol?.render({ notification_type: 'unknown' })).toBe('')
    })

    it('should have status column with NSwitch render', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      const statusCol = state.columns.find(c => c.key === 'status')
      expect(statusCol).toMatchObject({
        title: 'generate.status',
        align: 'left',
        minWidth: '140px'
      })
    })

    it('should have actions column with correct title', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      const actionsCol = state.columns.find(c => c.key === 'actions')
      expect(actionsCol).toMatchObject({
        title: 'common.actions',
        align: 'left',
        width: '200px'
      })
    })
  })

  describe('handleSwitchChange', () => {
    it('should set status to OPEN when value is true', async () => {
      hoisted.putNotificationGroup.mockResolvedValue({ error: null })
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      const row = { id: '1', status: 'CLOSE', notification_type: 'email', name: 'G1' }
      await state.handleSwitchChange(row, true)
      expect(row.status).toBe('OPEN')
    })

    it('should set status to CLOSE when value is false', async () => {
      hoisted.putNotificationGroup.mockResolvedValue({ error: null })
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      const row = { id: '1', status: 'OPEN', notification_type: 'email', name: 'G1' }
      await state.handleSwitchChange(row, false)
      expect(row.status).toBe('CLOSE')
    })

    it('should call putNotificationGroup with row data and id', async () => {
      hoisted.putNotificationGroup.mockResolvedValue({ error: null })
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      const row = { id: '1', status: 'CLOSE', notification_type: 'email', name: 'G1' }
      await state.handleSwitchChange(row, true)
      await flushPromises()
      expect(hoisted.putNotificationGroup).toHaveBeenCalledTimes(1)
      expect(hoisted.putNotificationGroup).toHaveBeenCalledWith({
        status: 'OPEN',
        notification_type: 'email',
        name: 'G1'
      }, '1')
      expect(hoisted.getNotificationGroupList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    })

    it('should handle row without id', async () => {
      hoisted.putNotificationGroup.mockResolvedValue({ error: null })
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      const row = { status: 'CLOSE', notification_type: 'email', name: 'G1' }
      await state.handleSwitchChange(row, true)
      await flushPromises()
      expect(hoisted.putNotificationGroup).toHaveBeenCalledWith(expect.anything(), '')
    })
  })

  describe('handleDeleteTable', () => {
    it('should call deleteNotificationGroup with correct id', async () => {
      hoisted.deleteNotificationGroup.mockResolvedValue({ error: null })
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      await state.handleDeleteTable('test-id')
      expect(hoisted.deleteNotificationGroup).toHaveBeenCalledWith({ id: 'test-id' })
    })

    it('should call getTableData after delete', async () => {
      hoisted.deleteNotificationGroup.mockResolvedValue({ error: null })
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      vi.clearAllMocks()
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const state = getState(wrapper)
      await state.handleDeleteTable('test-id')
      await flushPromises()
      expect(hoisted.getNotificationGroupList).toHaveBeenCalledTimes(1)
      expect(hoisted.getNotificationGroupList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    })

    it('should call window.$message.info after delete', async () => {
      const messageInfo = vi.fn()
      const originalMessage = window.$message
      window.$message = { info: messageInfo } as any

      hoisted.deleteNotificationGroup.mockResolvedValue({ error: null })
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      await state.handleDeleteTable('test-id')
      expect(messageInfo).toHaveBeenCalledWith('generate.notificationGroup')

      window.$message = originalMessage
    })
  })

  describe('handleEditTable', () => {
    it('should call getNotificationGroupDetail with correct id', async () => {
      hoisted.getNotificationGroupDetail.mockResolvedValue({ data: { id: '1', name: 'G1' } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      await state.handleEditTable('1')
      expect(hoisted.getNotificationGroupDetail).toHaveBeenCalledWith({ id: '1' })
    })

    it('should set editData and open modal when detail data exists', async () => {
      const detailData = { id: '1', name: 'G1', notification_type: 'email' }
      hoisted.getNotificationGroupDetail.mockResolvedValue({ data: detailData })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      await state.handleEditTable('1')
      await flushPromises()
      expect(state.editData).toEqual(detailData)
      expect(state.modalType).toBe('edit')
      expect(hoisted.setTrue).toHaveBeenCalledTimes(1)
    })

    it('should not open modal when detail data is null', async () => {
      hoisted.getNotificationGroupDetail.mockResolvedValue({ data: null })
      const wrapper = mountComponent()
      await flushPromises()
      vi.clearAllMocks()
      const state = getState(wrapper)
      await state.handleEditTable('1')
      await flushPromises()
      expect(hoisted.setTrue).toHaveBeenCalledTimes(0)
    })

    it('should not open modal when response has no data', async () => {
      hoisted.getNotificationGroupDetail.mockResolvedValue(null)
      const wrapper = mountComponent()
      await flushPromises()
      vi.clearAllMocks()
      const state = getState(wrapper)
      await state.handleEditTable('1')
      await flushPromises()
      expect(hoisted.setTrue).toHaveBeenCalledTimes(0)
    })
  })

  describe('handleAddTable', () => {
    it('should open modal and set modalType to add', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      vi.clearAllMocks()
      const state = getState(wrapper)
      state.handleAddTable()
      expect(hoisted.setTrue).toHaveBeenCalledTimes(1)
      expect(state.modalType).toBe('add')
    })
  })

  describe('setModalType', () => {
    it('should set modalType to add', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      state.setModalType('add')
      expect(state.modalType).toBe('add')
    })

    it('should set modalType to edit', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      state.setModalType('edit')
      expect(state.modalType).toBe('edit')
    })
  })

  describe('pagination', () => {
    it('should have correct initial pagination values', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      expect(state.pagination.page).toBe(1)
      expect(state.pagination.pageSize).toBe(10)
      expect(state.pagination.showSizePicker).toBe(true)
      expect(state.pagination.pageSizes).toEqual([10, 15, 20, 25, 30])
    })

    it('should update page on onChange', async () => {
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      state.pagination.onChange(3)
      expect(state.pagination.page).toBe(3)
    })

    it('should reset page to 1 and update pageSize on onUpdatePageSize', async () => {
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      state.pagination.page = 5
      state.pagination.onUpdatePageSize(20)
      expect(state.pagination.pageSize).toBe(20)
      expect(state.pagination.page).toBe(1)
    })
  })

  describe('getTableData', () => {
    it('should use pagination values for API call', async () => {
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      state.pagination.page = 2
      state.pagination.pageSize = 20
      await state.getTableData()
      await flushPromises()
      expect(hoisted.getNotificationGroupList).toHaveBeenCalledWith({ page: 2, page_size: 20 })
    })

    it('should default to page 1 and pageSize 10 if pagination values are falsy', async () => {
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 0 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      state.pagination.page = 0
      state.pagination.pageSize = 0
      await state.getTableData()
      await flushPromises()
      expect(hoisted.getNotificationGroupList).toHaveBeenCalledWith({ page: 1, page_size: 10 })
    })

    it('should set total from response', async () => {
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [], total: 42 } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      expect(state.total).toBe(42)
    })

    it('should default total to 0 when not provided', async () => {
      hoisted.getNotificationGroupList.mockResolvedValue({ data: { list: [] } })
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      expect(state.total).toBe(0)
    })
  })

  describe('setTableData', () => {
    it('should set table data', async () => {
      const wrapper = mountComponent()
      await flushPromises()
      const state = getState(wrapper)
      const data = [{ id: '1', name: 'Test' }]
      state.setTableData(data)
      expect(state.tableData).toEqual(data)
    })
  })
})
