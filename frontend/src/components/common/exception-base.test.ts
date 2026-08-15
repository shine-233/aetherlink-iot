import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import ExceptionBase from './exception-base.vue'

const mocks = vi.hoisted(() => ({
  toLogin: vi.fn(),
  resetStore: vi.fn(),
  clearMarketToken: vi.fn()
}))

vi.mock('@/hooks/common/router', () => ({
  useRouterPush: () => ({ toLogin: mocks.toLogin })
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({ resetStore: mocks.resetStore })
}))

vi.mock('@/views/device/config/composables/use-market-auth', () => ({
  useMarketAuth: () => ({ clearToken: mocks.clearMarketToken })
}))

const passthrough = (tag: string) => defineComponent({
  name: `${tag}Stub`,
  template: `<${tag}><slot /></${tag}>`
})

describe('ExceptionBase', () => {
  it.each([
    ['403', 'no-permission'],
    ['404', 'not-found'],
    ['500', 'service-error']
  ])('renders a real URL for the %s illustration', (type, assetName) => {
    const wrapper = mount(ExceptionBase, {
      props: { type: type as '403' | '404' | '500' },
      global: {
        stubs: {
          ButtonIcon: passthrough('button'),
          SvgIcon: passthrough('svg'),
          NButton: passthrough('button'),
          RouterLink: passthrough('a')
        }
      }
    })

    const src = wrapper.get('img').attributes('src')
    expect(src).toEqual(expect.any(String))
    expect(src).not.toContain('[object Object]')
    expect(src).toMatch(new RegExp(`${assetName}\\.svg`))
  })
})
