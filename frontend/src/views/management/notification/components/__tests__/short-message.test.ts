/**
 * 文件用途：覆盖 short-message 在 通知服务管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchNotificationServicesSms: vi.fn(),
  editNotificationServices: vi.fn(),
  messageSuccess: vi.fn()
}))

vi.mock('@/service/api', () => ({
  fetchNotificationServicesSms: hoisted.fetchNotificationServicesSms,
  editNotificationServices: hoisted.editNotificationServices
}))

vi.mock('~/src/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@aetherlink/hooks', () => ({
  useLoading: (init = false) => {
    const loading = ref(init)
    return {
      loading,
      startLoading: vi.fn(() => {
        loading.value = true
      }),
      endLoading: vi.fn(() => {
        loading.value = false
      })
    }
  }
}))

vi.mock('@/utils/deep-clone', () => ({
  smartDeepClone: (obj: any) => JSON.parse(JSON.stringify(obj))
}))

vi.mock('@/utils/form/rule', () => ({
  createRequiredFormRule: (msg: string) => ({ required: true, message: msg, trigger: ['input', 'blur'] })
}))

import ShortMessage from '../short-message.vue'

const stub = (name: string) =>
  defineComponent({
    name,
    setup(_, { slots }) {
      return () => h('div', { 'data-stub': name }, slots.default?.())
    }
  })

const mountComponent = () =>
  shallowMount(ShortMessage, {
    global: {
      stubs: {
        NSpin: stub('NSpin'),
        NForm: stub('NForm'),
        NGrid: stub('NGrid'),
        NFormItemGridItem: stub('NFormItemGridItem'),
        NSelect: stub('NSelect'),
        NInput: stub('NInput'),
        NSwitch: stub('NSwitch'),
        NButton: stub('NButton')
      }
    }
  })

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('management/notification/components/short-message.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchNotificationServicesSms.mockResolvedValue({
      data: {
        id: 'sms-1',
        config:
          '{"provider":"ALIYUN","aliyun_sms_config":{"access_key_id":"ak","access_key_secret":"secret","endpoint":"dysmsapi.aliyuncs.com","sign_name":"AetherLink","template_code":"SMS_1"}}',
        sme_config: {
          provider: 'ALIYUN',
          aliyun_sms_config: {}
        },
        notice_type: 'SME_CODE',
        status: 'OPEN',
        remark: ''
      }
    })
    hoisted.editNotificationServices.mockResolvedValue({ error: null })
    ;(globalThis as any).$message = { success: hoisted.messageSuccess }
  })

  it('loads SME_CODE SMS service config into the form on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    expect(hoisted.fetchNotificationServicesSms).toHaveBeenCalledTimes(1)
    expect(state.formModel).toMatchObject({
      id: 'sms-1',
      notice_type: 'SME_CODE',
      status: 'OPEN',
      sme_config: {
        provider: 'ALIYUN',
        aliyun_sms_config: {
          access_key_id: 'ak',
          endpoint: 'dysmsapi.aliyuncs.com',
          sign_name: 'AetherLink',
          template_code: 'SMS_1'
        }
      }
    })
  })

  it('saves SMS service config without sending the backend config echo string', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)

    state.formRef = { validate: vi.fn() }
    state.formModel.sme_config.aliyun_sms_config.template_code = 'SMS_2'
    await state.handleSubmit()
    await flushPromises()

    expect(hoisted.editNotificationServices).toHaveBeenCalledWith(
      expect.objectContaining({
        notice_type: 'SME_CODE',
        status: 'OPEN',
        sme_config: expect.objectContaining({
          provider: 'ALIYUN',
          aliyun_sms_config: expect.objectContaining({
            template_code: 'SMS_2'
          })
        })
      })
    )
    expect(hoisted.editNotificationServices.mock.calls[0][0]).not.toHaveProperty('config')
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('success')
  })
})
