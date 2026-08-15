/**
 * 文件用途: 覆盖测试在自动化场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  sceneGet: vi.fn(),
  sceneDel: vi.fn(),
  sceneActive: vi.fn(),
  sceneLog: vi.fn(),
  routerPushByKey: vi.fn(),
  dialogWarning: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api/automation', () => ({
  sceneGet: hoisted.sceneGet,
  sceneDel: hoisted.sceneDel,
  sceneActive: hoisted.sceneActive,
  sceneLog: hoisted.sceneLog
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({
    routerPushByKey: hoisted.routerPushByKey
  })
}))

vi.mock('@/utils/common/datetime', () => ({
  formatDateTime: (val: string) => `formatted-${val}`
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('naive-ui', async importOriginal => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useDialog: () => ({
      warning: hoisted.dialogWarning
    }),
    useMessage: () => ({
      success: hoisted.messageSuccess,
      error: vi.fn()
    })
  }
})

import SceneManage from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(SceneManage, {
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NFlex: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ setup() { return () => h('input') } }),
        NIcon: true,
        NDataTable: true,
        NPagination: true,
        NPopconfirm: true,
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NModal: true,
        NTable: true,
        NEmpty: true,
        NSelect: true,
        NDatePicker: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('scene-manage/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.sceneGet.mockResolvedValue({
      data: {
        list: [
          { id: 'scene-1', name: 'Scene 1', description: 'desc 1', created_at: '2024-01-01', updated_at: '2024-01-02' }
        ],
        total: 1
      }
    })
    hoisted.sceneLog.mockResolvedValue({
      data: {
        list: [{ executed_at: '2024-01-01', detail: 'ok', execution_result: 'S' }],
        total: 1
      }
    })
    hoisted.sceneActive.mockResolvedValue({ error: null })
    hoisted.sceneDel.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads scene list on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    expect(hoisted.sceneGet).toHaveBeenCalledTimes(1)
    expect(hoisted.sceneGet).toHaveBeenCalledWith({
      name: '',
      page: 1,
      page_size: 10
    })
    const setupState = getSetupState(wrapper)
    expect(setupState.tableData).toHaveLength(1)
    expect(setupState.tableData[0].name).toBe('Scene 1')
    expect(setupState.dataTotal).toBe(1)
  })

  it('sceneAdd navigates to scene-edit route', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    getSetupState(wrapper).sceneAdd()
    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('automation_scene-edit')
  })

  it('sceneEdit navigates with id query', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    getSetupState(wrapper).sceneEdit({ id: 'scene-2' })
    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('automation_scene-edit', { query: { id: 'scene-2' } })
  })

  it('sceneActivation calls sceneActive and refreshes list', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    hoisted.sceneGet.mockClear()
    await getSetupState(wrapper).sceneActivation({ id: 'scene-1' })

    expect(hoisted.sceneActive).toHaveBeenCalledWith('scene-1')
    expect(hoisted.sceneGet).toHaveBeenCalledTimes(1)
    expect(hoisted.sceneGet).toHaveBeenCalledWith({
      name: '',
      page: 1,
      page_size: 10
    })
  })

  it('handleQuery resets page and fetches data', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.queryData.page = 3
    hoisted.sceneGet.mockClear()

    await setupState.handleQuery()

    expect(setupState.queryData.page).toBe(1)
    expect(hoisted.sceneGet).toHaveBeenCalledTimes(1)
    expect(hoisted.sceneGet).toHaveBeenCalledWith({
      name: '',
      page: 1,
      page_size: 10
    })
  })

  it('openLog sets id, fetches log list and shows modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    await setupState.openLog({ id: 'scene-1' })
    await flushPromises()

    expect(setupState.logQuery.id).toBe('scene-1')
    expect(setupState.showLog).toBe(true)
    expect(setupState.logData).toHaveLength(1)
    expect(setupState.logDataTotal).toBe(1)
  })

  it('getLogList formats execution time range', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.logQuery.time = [1700000000000, 1700000001000]
    hoisted.sceneLog.mockClear()

    await setupState.getLogList()

    expect(hoisted.sceneLog).toHaveBeenCalledTimes(1)
    const callArg = hoisted.sceneLog.mock.calls[0][0]
    expect(callArg.execution_start_time).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/)
    expect(callArg.execution_end_time).toMatch(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}/)
    expect(callArg.execution_start_time).not.toBe(callArg.execution_end_time)
  })

  it('deleteScene opens dialog and deletes on positive click', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    hoisted.sceneGet.mockClear()

    setupState.deleteScene({ id: 'scene-1' })

    expect(hoisted.dialogWarning).toHaveBeenCalledTimes(1)
    const dialogOptions = hoisted.dialogWarning.mock.calls[0][0]
    expect(dialogOptions.title).toBe('common.deletePrompt')

    hoisted.sceneGet.mockClear()
    await dialogOptions.onPositiveClick()
    await flushPromises()

    expect(hoisted.sceneDel).toHaveBeenCalledWith('scene-1')
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('custom.grouping_details.operationSuccess')
    expect(hoisted.sceneGet).toHaveBeenCalledTimes(1)
    expect(hoisted.sceneGet).toHaveBeenCalledWith({
      name: '',
      page: 1,
      page_size: 10
    })
  })

  it('queryLog resets page and fetches log list', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.logQuery.page = 5
    hoisted.sceneLog.mockClear()

    setupState.queryLog()

    expect(setupState.logQuery.page).toBe(1)
    expect(hoisted.sceneLog).toHaveBeenCalledTimes(1)
    expect(hoisted.sceneLog.mock.calls[0][0]).toMatchObject({
      id: '',
      page: 1,
      page_size: 10,
      execution_result: ''
    })
  })

  it('logClose resets log query state', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    setupState.logQuery.id = 'scene-1'
    setupState.logQuery.execution_result = 'S'

    setupState.logClose()

    expect(setupState.logQuery.id).toBe('')
    expect(setupState.logQuery.execution_result).toBe('')
    expect(setupState.logQuery.page).toBe(1)
  })

  it('columns render functions produce expected output', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const setupState = getSetupState(wrapper)
    const columns = setupState.columns

    const createdCol = columns.find((c: any) => c.key === 'created_at')
    expect(createdCol.render({ created_at: '2024-01-01' })).toBe('formatted-2024-01-01')

    const updatedCol = columns.find((c: any) => c.key === 'updated_at')
    expect(updatedCol.render({ updated_at: '2024-01-02' })).toBe('formatted-2024-01-02')

    const nameCol = columns.find((c: any) => c.key === 'name')
    expect(nameCol.title).toBe('generate.scene-name')

    const actionsCol = columns.find((c: any) => c.key === 'actions')
    expect(typeof actionsCol.title).toBe('string')
    expect(actionsCol.title).toBe('common.actions')
  })
})
