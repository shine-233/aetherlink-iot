/**
 * 文件用途: 覆盖测试在设备场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可抽取稳定的挂载工厂和业务 fixture，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, reactive } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  acceptRdiSharedDevice: vi.fn(),
  routerPush: vi.fn(),
  routerBack: vi.fn()
}))

const routeQuery = reactive({ share_token: '' as string | string[] })

vi.mock('@/service/api/rdi', () => ({
  acceptRdiSharedDevice: hoisted.acceptRdiSharedDevice
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: routeQuery }),
  useRouter: () => ({
    push: hoisted.routerPush,
    back: hoisted.routerBack
  })
}))

import DeviceShare from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountDeviceShare = () => {
  const wrapper = shallowMount(DeviceShare, {
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : [])
          }
        }),
        NSpin: defineComponent({ setup() { return () => h('div', { class: 'spin-stub' }) } }),
        NResult: defineComponent({
          props: {
            status: { type: String, default: '' },
            title: { type: String, default: '' },
            description: { type: String, default: '' }
          },
          setup(props, { slots }) {
            return () =>
              h('div', { class: 'result-stub', 'data-status': props.status }, [
                h('div', { class: 'result-title' }, props.title),
                h('div', { class: 'result-description' }, props.description),
                slots.footer ? slots.footer() : null
              ])
          }
        })
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/share/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeQuery.share_token = 'share-token'
    hoisted.acceptRdiSharedDevice.mockResolvedValue({
      data: {
        device: { device_id: 'dev-1' },
        already_accepted: true,
        shared_with_me: false
      }
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('accepts a valid share token on mount', async () => {
    const wrapper = mountDeviceShare()
    await flushPromises()

    const state = getSetupState(wrapper)
    expect(hoisted.acceptRdiSharedDevice).toHaveBeenCalledWith('share-token')
    expect(state.status).toBe('success')
    expect(state.deviceId).toBe('dev-1')
    expect(state.alreadyAccepted).toBe(true)
    expect(state.sharedWithMe).toBe(false)
  })

  it('shows a missing-token error without calling the API', async () => {
    routeQuery.share_token = ''
    const wrapper = mountDeviceShare()
    await flushPromises()

    const state = getSetupState(wrapper)
    expect(hoisted.acceptRdiSharedDevice).toHaveBeenCalledTimes(0)
    expect(state.status).toBe('error')
    expect(state.errorMessage).toBe('rdi.share.missingToken')
  })

  it('clears stale device state when a later refresh has no token', async () => {
    const wrapper = mountDeviceShare()
    await flushPromises()

    const state = getSetupState(wrapper)
    expect(state.deviceId).toBe('dev-1')
    expect(state.alreadyAccepted).toBe(true)
    expect(state.sharedWithMe).toBe(false)

    routeQuery.share_token = ''
    await state.acceptShare()
    await flushPromises()

    expect(state.status).toBe('error')
    expect(state.errorMessage).toBe('rdi.share.missingToken')
    expect(state.deviceId).toBe('')
    expect(state.alreadyAccepted).toBe(false)
    expect(state.sharedWithMe).toBe(false)

    state.goSharedWithMe()
    expect(hoisted.routerPush).toHaveBeenLastCalledWith({
      path: '/device/shared-with-me',
      query: {}
    })
  })

  it('only opens device details when a device id exists', async () => {
    const wrapper = mountDeviceShare()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.goDeviceDetails()
    expect(hoisted.routerPush).toHaveBeenCalledWith({
      name: 'device_details',
      query: { d_id: 'dev-1' }
    })

    vi.clearAllMocks()
    state.deviceId = ''
    state.goDeviceDetails()
    expect(hoisted.routerPush).toHaveBeenCalledTimes(0)
  })

  it('distinguishes same-tenant already-owned results from shared-with-me results', async () => {
    const wrapper = mountDeviceShare()
    await flushPromises()

    const state = getSetupState(wrapper)
    expect(state.resultTitle).toBe('rdi.share.alreadyOwned')
    expect(state.resultDescription).toBe('rdi.share.alreadyOwnedDescription')
    expect(state.showSharedWithMeAction).toBe(false)

    hoisted.acceptRdiSharedDevice.mockResolvedValueOnce({
      data: {
        device: { device_id: 'dev-2' },
        already_accepted: true,
        shared_with_me: true
      }
    })

    await state.acceptShare()
    await flushPromises()

    expect(state.resultTitle).toBe('rdi.share.alreadyShared')
    expect(state.resultDescription).toBe('rdi.share.successDescription')
    expect(state.showSharedWithMeAction).toBe(true)
  })

  it('goBack delegates to router.back', async () => {
    const wrapper = mountDeviceShare()
    await flushPromises()

    const state = getSetupState(wrapper)
    state.goBack()
    expect(hoisted.routerBack).toHaveBeenCalledTimes(1)
  })
})
