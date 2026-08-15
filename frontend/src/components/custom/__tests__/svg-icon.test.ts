/**
 * 文件用途：验证 自定义组件单元测试 的关键行为和回归边界。
 * 核心逻辑：通过 Vitest、组件挂载或模块 mock 构造输入，断言状态、事件、请求参数或可见输出。
 * 关键注意事项：测试数据和 mock 必须贴近真实契约，避免只证明代码能运行而没有业务断言。
 * 重构建议：可沉淀共享 fixture 与挂载工具，并补充异常、空数据和权限边界用例。
 */
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@iconify/vue', () => ({
  Icon: {
    name: 'Icon',
    props: ['icon'],
    template: '<span class="iconify-stub" :data-icon="icon"><slot /></span>'
  }
}))

import SvgIcon from '../svg-icon.vue'

describe('svg-icon.vue', () => {
  it('forwards class and style attrs to the rendered local svg root', () => {
    const wrapper = mount(SvgIcon, {
      props: {
        localIcon: 'defaultdevice'
      },
      attrs: {
        class: 'config-image',
        style: 'font-size: 18px;'
      }
    })

    const svg = wrapper.get('svg')
    expect(svg.classes()).toContain('config-image')
    expect(svg.attributes('style')).toContain('font-size: 18px;')
  })
})
