import { defineComponent, h } from 'vue'
import { shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  authStore: {
    isLogin: true,
    userInfo: {
      name: 'Alice',
      userName: 'LegacyName',
      additional_info: '{"user_icon":"/uploads/avatar.png"}',
      avatar_url: ''
    },
    requestLogout: vi.fn()
  },
  routerPushByKey: vi.fn(),
  toLogin: vi.fn(),
  clearMarketToken: vi.fn(),
  routerPush: vi.fn()
}))

vi.mock('@/utils/common/tool', () => ({
  getPlatformApiBaseUrl: vi.fn(() => 'http://localhost/api/v1')
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: vi.fn(() => hoisted.authStore)
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: vi.fn(() => ({
    routerPushByKey: hoisted.routerPushByKey,
    toLogin: hoisted.toLogin
  }))
}))

vi.mock('@/views/device/config/composables/use-market-auth', () => ({
  useMarketAuth: vi.fn(() => ({
    clearToken: hoisted.clearMarketToken
  }))
}))

vi.mock('vue-router', () => ({
  useRouter: vi.fn(() => ({
    push: hoisted.routerPush
  }))
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@aetherlink/hooks', () => ({
  useSvgIconRender: vi.fn(() => ({
    SvgIconVNode: vi.fn(() => h('span'))
  }))
}))

vi.mock('@/components/custom/svg-icon.vue', () => ({
  default: defineComponent({
    setup() {
      return () => h('span')
    }
  })
}))

import UserAvatar from '../user-avatar.vue'

const mountComponent = () =>
  shallowMount(UserAvatar, {
    global: {
      stubs: {
        NButton: defineComponent({
          emits: ['click'],
          setup(_, { slots, emit }) {
            return () => h('button', { onClick: () => emit('click') }, slots.default?.())
          }
        }),
        NDropdown: defineComponent({
          props: ['options'],
          emits: ['select'],
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        }),
        ButtonIcon: defineComponent({
          setup(_, { slots }) {
            return () => h('div', slots.default?.())
          }
        })
      }
    }
  })

describe('UserAvatar', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.authStore.isLogin = true
    hoisted.authStore.userInfo.name = 'Alice'
    hoisted.authStore.userInfo.userName = 'LegacyName'
    hoisted.authStore.userInfo.additional_info = '{"user_icon":"/uploads/avatar.png"}'
    hoisted.authStore.userInfo.avatar_url = ''
  })

  it('renders the cached uploaded avatar in the global header', () => {
    const wrapper = mountComponent()
    const img = wrapper.get('img')

    expect(img.attributes('src')).toBe('http://localhost/uploads/avatar.png')
    expect(wrapper.text()).toContain('Alice')
  })

  it('falls back to avatar_url when additional_info has no user_icon', () => {
    hoisted.authStore.userInfo.name = ''
    hoisted.authStore.userInfo.userName = 'FallbackName'
    hoisted.authStore.userInfo.additional_info = '{}'
    hoisted.authStore.userInfo.avatar_url = '/uploads/from-avatar.png'

    const wrapper = mountComponent()
    const img = wrapper.get('img')

    expect(img.attributes('src')).toBe('http://localhost/uploads/from-avatar.png')
    expect(wrapper.text()).toContain('FallbackName')
  })

  it('renders login button when the user is not logged in', () => {
    hoisted.authStore.isLogin = false

    const wrapper = mountComponent()

    expect(wrapper.text()).toContain('page.login.common.loginOrRegister')
    expect(wrapper.find('img').exists()).toBe(false)
  })
})
