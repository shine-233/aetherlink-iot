/**
 * 文件用途: setting-info 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceConfigDel: vi.fn(),
  deviceConfigEdit: vi.fn(),
  removeTab: vi.fn(),
  routerPushByKey: vi.fn()
}))

vi.mock('@/service/api/device', () => ({
  deviceConfigDel: hoisted.deviceConfigDel,
  deviceConfigEdit: hoisted.deviceConfigEdit
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/store/modules/tab', () => ({
  useTabStore: () => ({ removeTab: hoisted.removeTab })
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ routerPushByKey: hoisted.routerPushByKey })
}))

vi.mock('@/utils/storage', () => ({
  localStg: { get: vi.fn(() => 'token') }
}))

vi.mock('@/utils/common/tool', () => ({
  getPlatformApiBaseUrl: () => 'http://localhost/api/v1'
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: { id: 'cfg-1' }, path: '/device/config-detail' })
}))

vi.mock('naive-ui', () => ({
  useDialog: () => ({
    warning: vi.fn(({ onPositiveClick }: any) => {
      if (onPositiveClick) onPositiveClick()
    })
  }),
  useMessage: () => ({ success: vi.fn(), error: vi.fn(), warning: vi.fn() }),
  NButton: defineComponent({
    emits: ['click'],
    setup(_, { slots, emit }) {
      return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
    }
  }),
  NFlex: defineComponent({
    setup(_, { slots }) {
      return () => h('div', slots.default ? slots.default() : [])
    }
  })
}))

import Component from '../setting-info.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: {
      configInfo: { id: 'cfg-1', auto_register: 0, template_secret: 'secret', other_config: '{}', image_url: '' },
      ...props
    },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        NCard: true,
        NForm: true,
        NFormItem: true,
        NInput: true,
        NSelect: true,
        NButton: true,
        NModal: defineComponent({
          props: { show: Boolean },
          emits: ['update:show'],
          setup(_, { slots }) {
            return () => h('div', slots.default ? slots.default() : [])
          }
        }),
        NSwitch: true,
        NInputNumber: true,
        NUpload: true,
        NUploadDragger: true,
        NIcon: true,
        NAlert: true,
        SvgIcon: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/config-detail/modules/setting-info.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceConfigDel.mockResolvedValue({ error: null })
    hoisted.deviceConfigEdit.mockResolvedValue({ error: null })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('initializes auto-register, image path and online timeout mutual-exclusion state', async () => {
    const wrapper = mountComponent({
      configInfo: {
        id: 'cfg-1',
        auto_register: 1,
        template_secret: 'secret',
        other_config: '{"online_timeout":30,"heartbeat":0}',
        image_url: 'uploads/config.png'
      }
    })
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.auto_register).toBe(true)
    expect(state.imagePath).toBe('http://localhost/uploads/config.png')
    expect(state.onlinejson).toMatchObject({ online_timeout: 0, heartbeat: 0 })
    expect(state.isTimeoutDisabled).toBe(false)
    expect(state.isHeartbeatDisabled).toBe(false)
    state.onOpenDialogModal(2)
    expect(state.onlinejson).toMatchObject({ online_timeout: 30, heartbeat: 0 })
    expect(state.isHeartbeatDisabled).toBe(true)
  })

  it('onDialogVisble toggles showModal', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    expect(state.showModal).toBe(false)
    state.onDialogVisble()
    expect(state.showModal).toBe(true)
  })

  it('onOpenDialogModal sets modalIndex and opens modal', () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.onOpenDialogModal(2)
    expect(state.modalIndex).toBe(2)
    expect(state.showModal).toBe(true)
  })

  it('onSubmit with modalIndex 1 calls deviceConfigEdit', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.modalIndex = 1
    await state.onSubmit()
    expect(hoisted.deviceConfigEdit).toHaveBeenCalledWith({ id: 'cfg-1', auto_register: 0 })
    expect(wrapper.emitted('change')).toEqual([[]])
  })

  it('onSubmit with modalIndex 2 saves other_config', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    state.modalIndex = 2
    await state.onSubmit()
    expect(hoisted.deviceConfigEdit).toHaveBeenCalledWith(
      expect.objectContaining({ id: 'cfg-1', other_config: expect.any(String) })
    )
  })
})
