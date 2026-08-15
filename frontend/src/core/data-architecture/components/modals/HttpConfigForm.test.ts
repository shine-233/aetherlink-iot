/**
 * 文件用途: Http Config Form 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { flushPromises, shallowMount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import HttpConfigForm from './HttpConfigForm.vue'

vi.mock('naive-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('naive-ui')>()

  return {
    ...actual,
    useMessage: () => ({
      success: vi.fn(),
      error: vi.fn(),
      warning: vi.fn(),
      info: vi.fn()
    })
  }
})

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const passthroughStub = {
  template: '<div><slot /></div>'
}

describe('HttpConfigForm compatibility decisions', () => {
  it('migrates compatibility pathParameter into pathParams while retaining the persisted mirror', async () => {
    const wrapper = shallowMount(HttpConfigForm, {
      props: {
        modelValue: {
          url: '/device/{id}',
          method: 'GET',
          addressType: 'internal',
          selectedInternalAddress: '/device/{id}',
          pathParameter: {
            value: 'legacy-device',
            isDynamic: true,
            variableName: 'deviceId',
            dataType: 'string'
          }
        }
      },
      global: {
        stubs: {
          'n-tabs': passthroughStub,
          'n-tab-pane': passthroughStub,
          'n-space': passthroughStub,
          'n-tag': passthroughStub,
          'n-tooltip': passthroughStub,
          'n-icon': passthroughStub,
          'n-alert': passthroughStub,
          'n-text': passthroughStub,
          'n-progress': passthroughStub
        }
      }
    })

    await nextTick()
    await flushPromises()

    const step = wrapper.findComponent({ name: 'HttpConfigStep1' })
    const stepConfig = step.props('modelValue') as any

    expect(stepConfig.pathParams).toEqual([
      expect.objectContaining({
        key: 'pathParam',
        value: 'legacy-device',
        valueMode: 'property'
      })
    ])
    expect(stepConfig.pathParameter).toEqual(
      expect.objectContaining({
        value: 'legacy-device',
        isDynamic: true
      })
    )
  })
})
