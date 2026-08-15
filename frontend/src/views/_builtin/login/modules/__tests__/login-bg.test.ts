/**
 * 文件用途：验证 frontend/src/views/_builtin/login/modules/__tests__/login-bg 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { describe, it, expect, vi } from 'vitest'
import { shallowMount } from '@vue/test-utils'
import LoginBg from '../login-bg.vue'

const stubs = {
  NImage: {
    name: 'NImage',
    props: {
      src: String,
      objectFit: String,
      previewDisabled: Boolean,
      imgProps: Object
    },
    template:
      '<img class="n-image-stub" :src="src" :data-fit="objectFit" :data-preview-disabled="String(previewDisabled)" />'
  },
  SvgIcon: {
    name: 'SvgIcon',
    props: ['localIcon'],
    template: '<i class="svg-icon-stub" :data-icon="localIcon" />'
  }
}

describe('LoginBg', () => {
  it('should render configured background image with preview disabled', () => {
    const wrapper = shallowMount(LoginBg, {
      props: {
        themeColor: '#6366f1',
        sysSetting: { home_background: 'https://example.com/bg.jpg' }
      },
      global: {
        stubs
      }
    })
    const image = wrapper.getComponent({ name: 'NImage' })

    expect(image.props('src')).toBe('https://example.com/bg.jpg')
    expect(image.props('objectFit')).toBe('cover')
    expect(image.props('previewDisabled')).toBe(true)
  })

  it('should render Wave fallback icon when no custom background is configured', () => {
    const wrapper = shallowMount(LoginBg, {
      props: {
        themeColor: '#6366f1',
        sysSetting: { home_background: '' }
      },
      global: {
        stubs
      }
    })
    expect(wrapper.getComponent({ name: 'SvgIcon' }).props('localIcon')).toBe('Wave')
  })

  it('should compute bgColor from sysSetting.home_background', () => {
    const wrapper = shallowMount(LoginBg, {
      props: {
        themeColor: '#6366f1',
        sysSetting: { home_background: 'https://example.com/bg.jpg' }
      },
      global: {
        stubs
      }
    })
    const vm = wrapper.vm as any
    expect(vm.bgColor).toBe('https://example.com/bg.jpg')
  })

  it('should return empty string for bgColor when home_background is not set', () => {
    const wrapper = shallowMount(LoginBg, {
      props: {
        themeColor: '#6366f1',
        sysSetting: { home_background: '' }
      },
      global: {
        stubs
      }
    })
    const vm = wrapper.vm as any
    expect(vm.bgColor).toBe('')
  })

  it('should pass correct src to NImage when bgColor is set', () => {
    const wrapper = shallowMount(LoginBg, {
      props: {
        themeColor: '#6366f1',
        sysSetting: { home_background: 'https://example.com/bg.jpg' }
      },
      global: {
        stubs
      }
    })
    const nImage = wrapper.findComponent({ name: 'NImage' })
    expect(nImage.attributes('src')).toBe('https://example.com/bg.jpg')
  })
})
