/**
 * 文件用途：验证 frontend/src/views/_builtin/__tests__/exception-pages 相关页面或组件的关键行为。
 * 核心逻辑：通过 Vitest 和 Vue Test Utils 挂载目标组件，配合必要 mock 断言渲染、交互、事件和边界状态。
 * 关键注意事项：维护时优先断言用户可见结果和对外调用，不要让 mock 细节掩盖真实行为变化。
 * 重构建议：当准备逻辑继续膨胀时，抽出本测试文件内的工厂函数，保持每个用例只表达一个行为目标。
 */
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import ForbiddenPage from '../403/index.vue'
import NotFoundPage from '../404/index.vue'
import ServerErrorPage from '../500/index.vue'

const ExceptionBaseStub = defineComponent({
  name: 'ExceptionBase',
  props: {
    type: {
      type: String,
      required: true
    }
  },
  template: '<div class="exception-base-stub">{{ type }}</div>'
})

describe('builtin exception pages', () => {
  it.each([
    ['403', ForbiddenPage],
    ['404', NotFoundPage],
    ['500', ServerErrorPage]
  ])('passes %s to ExceptionBase', (type, component) => {
    const wrapper = mount(component, {
      global: {
        components: {
          ExceptionBase: ExceptionBaseStub
        }
      }
    })

    expect(wrapper.find('.exception-base-stub').text()).toBe(type)
  })
})
