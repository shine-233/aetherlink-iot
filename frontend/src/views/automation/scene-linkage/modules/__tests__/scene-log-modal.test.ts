/**
 * 文件用途：覆盖 scene-log-modal.vue（从 dataList.vue 拆出的执行日志弹窗）。
 * 核心逻辑：mock sceneAutomationsLog，验证打开即按场景 id 拉取第 1 页、时间范围/结果过滤
 *   在请求前转换为后端字段、筛选回第一页、关闭时清空查询。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  sceneAutomationsLog: vi.fn()
}))

vi.mock('@/service/api/automation', () => ({
  sceneAutomationsLog: hoisted.sceneAutomationsLog
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('dayjs', () => {
  const fn = (v?: any) => ({
    subtract: () => ({ valueOf: () => 1000 }),
    format: () => '2024-01-01T00:00:00',
    valueOf: () => v || 1000
  })
  return { default: fn }
})

vi.mock('vue', async () => {
  const actual = await vi.importActual('vue')
  return {
    ...actual,
    getCurrentInstance: () => ({ proxy: { getPlatform: () => false } })
  }
})

import SceneLogModal from '../scene-log-modal.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = { show: false, sceneAutomationId: '' }) => {
  const wrapper = shallowMount(SceneLogModal, {
    props,
    global: {
      stubs: {
        NModal: defineComponent({
          props: { show: Boolean },
          emits: ['update:show'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NFlex: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        NButton: defineComponent({
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default?.())
          }
        }),
        NSelect: defineComponent({
          props: { value: { default: null }, options: { default: () => [] } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NDatePicker: defineComponent({
          props: { value: { default: null } },
          emits: ['update:value'],
          setup() {
            return () => h('div')
          }
        }),
        NTable: defineComponent({
          setup(_, { slots }) {
            return () => h('table', slots.default?.())
          }
        }),
        NPagination: defineComponent({
          props: { page: { type: Number, default: 1 }, pageSize: { type: Number, default: 10 } },
          emits: ['update:page'],
          setup() {
            return () => h('div')
          }
        }),
        NEmpty: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('SceneLogModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.sceneAutomationsLog.mockResolvedValue({ data: { list: [], total: 0 }, error: null })
  })

  afterEach(() => {
    mountedWrappers.forEach((w) => w.unmount())
    mountedWrappers.length = 0
  })

  it('loads the first page for the target scene when opened', async () => {
    mountComponent({ show: true, sceneAutomationId: 'scene-1' })
    await flushPromises()

    expect(hoisted.sceneAutomationsLog).toHaveBeenCalledTimes(1)
    expect(hoisted.sceneAutomationsLog).toHaveBeenCalledWith({
      page: 1,
      page_size: 10,
      scene_automation_id: 'scene-1',
      execution_result: '',
      execution_start_time: '2024-01-01T00:00:00',
      execution_end_time: '2024-01-01T00:00:00'
    })
  })

  it('does not load while closed', async () => {
    mountComponent({ show: false, sceneAutomationId: 'scene-1' })
    await flushPromises()

    expect(hoisted.sceneAutomationsLog).not.toHaveBeenCalled()
  })

  it('resets to the first page and re-queries on queryLog', async () => {
    const wrapper = mountComponent({ show: true, sceneAutomationId: 'scene-1' })
    await flushPromises()

    const state = getState(wrapper)
    state.pagination.page = 5
    state.logQuery.execution_result = 'F'
    await state.queryLog()
    await flushPromises()

    expect(state.pagination.page).toBe(1)
    expect(hoisted.sceneAutomationsLog).toHaveBeenLastCalledWith(
      expect.objectContaining({
        page: 1,
        page_size: 10,
        scene_automation_id: 'scene-1',
        execution_result: 'F',
        execution_start_time: '2024-01-01T00:00:00',
        execution_end_time: '2024-01-01T00:00:00'
      })
    )
  })

  it('exposes the three execution result options', () => {
    const wrapper = mountComponent({ show: false, sceneAutomationId: '' })
    const state = getState(wrapper)
    expect(state.execution_result_options).toHaveLength(3)
  })

  it('clears the query state when the modal closes', async () => {
    const wrapper = mountComponent({ show: true, sceneAutomationId: 'scene-1' })
    await flushPromises()

    const state = getState(wrapper)
    state.logQuery.execution_result = 'S'
    await wrapper.setProps({ show: false })
    await flushPromises()

    expect(state.logQuery.execution_result).toBe('')
    expect(state.pagination.page).toBe(1)
    expect(state.logDataTotal).toBe(0)
  })
})
