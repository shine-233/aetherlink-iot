/**
 * 文件用途：验证 frontend/src/views/dashboard/workspace/__tests__/index 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => `t:${key}`
}))

import WorkspacePage from '../index.vue'

describe('dashboard/workspace/index.vue', () => {
  const mountPage = () =>
    mount(WorkspacePage, {
      global: {
        stubs: {
          RouterLink: {
            props: ['to'],
            template: '<a class="router-link-stub" :data-route="to"><slot /></a>'
          },
          'router-link': {
            props: ['to'],
            template: '<a class="router-link-stub" :data-route="to"><slot /></a>'
          }
        }
      }
    })

  it('renders the AetherLink visualization entry page', () => {
    const wrapper = mountPage()

    expect(wrapper.text()).toContain('t:custom.dashboardWorkspace.title')
    expect(wrapper.text()).toContain('t:custom.nativeBoards.title')
    expect(wrapper.text()).toContain('t:custom.dashboardWorkspace.workbenchTitle')
    expect(wrapper.findAll('.router-link-stub')).toHaveLength(2)
    expect(wrapper.findAll('.router-link-stub').map(link => link.attributes('data-route'))).toEqual([
      '/visualization/native-boards',
      '/dashboard/workbench'
    ])
  })

  it('uses path links so optional route-name registration cannot blank the page', () => {
    const wrapper = mountPage()

    expect(wrapper.findAll('.router-link-stub').every(link => link.attributes('data-route')?.startsWith('/'))).toBe(true)
  })
})
