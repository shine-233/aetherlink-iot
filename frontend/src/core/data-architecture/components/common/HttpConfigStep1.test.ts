/**
 * 文件用途: Http Config Step1 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { describe, expect, it, vi } from 'vitest'

import HttpConfigStep1 from './HttpConfigStep1.vue'

const { internalApi } = vi.hoisted(() => ({
  internalApi: {
    label: 'Device detail',
    value: '/device/{id}',
    url: '/device/{id}',
    method: 'GET',
    hasPathParams: true,
    pathParamNames: ['id']
  }
}))

vi.mock('@/core/data-architecture/data/internal-address-data', () => ({
  internalAddressOptions: [{ type: 'group', label: 'Device', key: 'device', children: [internalApi] }],
  getApiByValue: vi.fn((value: string) => (value === internalApi.value ? internalApi : undefined))
}))

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

function mountStep(modelValue: Record<string, unknown>) {
  return mount(HttpConfigStep1, {
    props: {
      modelValue,
      componentId: 'card-a'
    },
    global: {
      stubs: {
        'n-form': passthroughStub,
        'n-form-item': passthroughStub,
        'n-radio-group': passthroughStub,
        'n-radio': passthroughStub,
        'n-select': passthroughStub,
        'n-space': passthroughStub,
        'n-switch': passthroughStub,
        'n-text': passthroughStub,
        'n-input': passthroughStub,
        'n-button': passthroughStub,
        'n-input-number': passthroughStub,
        DynamicParameterEditor: {
          name: 'DynamicParameterEditor',
          props: ['modelValue'],
          template: '<div />'
        }
      }
    }
  })
}

describe('HttpConfigStep1 compatibility decisions', () => {
  it('hydrates compatibility pathParameter into the current pathParams editor state', async () => {
    const wrapper = mountStep({
      addressType: 'internal',
      selectedInternalAddress: internalApi.value,
      url: internalApi.url,
      method: 'GET',
      pathParameter: {
        value: 'legacy-device',
        isDynamic: true,
        variableName: 'deviceId',
        dataType: 'string',
        defaultValue: 'fallback-device'
      }
    })

    await nextTick()
    await flushPromises()

    const editor = wrapper.findComponent({ name: 'DynamicParameterEditor' })

    expect(editor.props('modelValue')).toEqual([
      expect.objectContaining({
        key: 'pathParam',
        value: 'legacy-device',
        valueMode: 'property',
        defaultValue: 'fallback-device'
      })
    ])
  })

  it('writes the compatibility pathParameter mirror without downgrading component bindings', async () => {
    const wrapper = mountStep({
      addressType: 'internal',
      selectedInternalAddress: internalApi.value,
      url: internalApi.url,
      method: 'GET',
      pathParams: [
        {
          key: 'id',
          value: 'card-a.component.deviceId',
          enabled: true,
          valueMode: 'component'
        }
      ],
      enableParams: true
    })

    await nextTick()
    await flushPromises()

    wrapper.findComponent({ name: 'DynamicParameterEditor' }).vm.$emit('update:modelValue', [
      {
        key: 'id',
        value: 'card-a.component.deviceId',
        enabled: true,
        valueMode: 'component',
        variableName: 'deviceId',
        dataType: 'string'
      }
    ])
    await nextTick()

    const emittedConfig = wrapper.emitted('update:modelValue')?.at(-1)?.[0] as any

    expect(emittedConfig.pathParams).toEqual([
      expect.objectContaining({
        key: 'id',
        valueMode: 'component'
      })
    ])
    expect(emittedConfig.pathParameter).toEqual(
      expect.objectContaining({
        key: 'id',
        value: 'card-a.component.deviceId',
        isDynamic: true
      })
    )
  })
})
