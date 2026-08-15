/**
 * 文件用途: 测试物模型完成步骤。
 * 核心逻辑: 模拟物模型详情接口，验证完成页文案和数据展示。
 * 关键注意事项: 完成页测试要覆盖接口失败和缺少物模型 ID 的安全状态。
 * 重构建议: 与基础信息步骤共用物模型详情 fixture。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  getTemplat: vi.fn()
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/utils/common/discrete', () => ({
  message: { info: vi.fn(), error: vi.fn(), success: vi.fn() }
}))

vi.mock('@/service/api/system-data', () => ({
  getTemplat: hoisted.getTemplat
}))

import Component from '../complete.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = (props = {}) => {
  const wrapper = shallowMount(Component, {
    props: { stepCurrent: 5, modalVisible: false, deviceTemplateId: 'tpl-1', ...props },
    global: {
      stubs: {
        NCard: true,
        NScrollbar: true,
        NCode: true,
        NButton: true,
        NSpace: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

describe('device/template/components/step/complete.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getTemplat.mockResolvedValue({
      data: {
        id: 'tpl-1',
        name: 'Telemetry Model 1',
        app_chart_config: JSON.stringify({ widgets: [{ id: 'app-widget' }], refreshInterval: 5000 }),
        web_chart_config: JSON.stringify({ widgets: [{ id: 'web-widget' }], refreshInterval: 30000 })
      },
      error: null
    })
  })
  afterEach(() => {
    document.body.innerHTML = ''
    ;(window as any).$message = undefined
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads final template JSON with parsed app and web chart configs', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const code = JSON.parse(state.code)

    expect(hoisted.getTemplat).toHaveBeenCalledWith('tpl-1')
    expect(code.id).toBe('tpl-1')
    expect(code.name).toBe('Telemetry Model 1')
    expect(code.app_chart_config).toEqual({ widgets: [{ id: 'app-widget' }], refreshInterval: 5000 })
    expect(code.web_chart_config).toEqual({ widgets: [{ id: 'web-widget' }], refreshInterval: 30000 })
  })

  it('keeps malformed chart config strings visible in final template JSON', async () => {
    hoisted.getTemplat.mockResolvedValue({
      data: {
        id: 'tpl-1',
        name: 'Telemetry Model 1',
        app_chart_config: '{bad-json',
        web_chart_config: '[bad-json'
      },
      error: null
    })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const code = JSON.parse(state.code)

    expect(code.app_chart_config).toBe('{bad-json')
    expect(code.web_chart_config).toBe('[bad-json')
  })

  it('back emits update:stepCurrent with 4', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.back()
    expect(wrapper.emitted('update:stepCurrent')).toEqual([[4]])
  })

  it('loads template data on mount', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.getTemplat).toHaveBeenCalledWith('tpl-1')
  })

  it('copyText writes code content to clipboard in secure context', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(window, 'isSecureContext', { configurable: true, value: true })
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    document.body.innerHTML = '<div id="text-to-copy">{"temperature":26}</div>'

    const wrapper = mountComponent()
    await flushPromises()
    getSetupState(wrapper).copyText()
    await flushPromises()

    const { message } = await import('@/utils/common/discrete')
    expect(writeText).toHaveBeenCalledWith('{"temperature":26}')
    expect(message.info).toHaveBeenCalledWith('common.copiedClipboard')
    expect(message.error).toHaveBeenCalledTimes(0)
  })

  it('copyText falls back to document selection and window message outside secure clipboard', async () => {
    const execCommand = vi.fn().mockReturnValue(true)
    const success = vi.fn()
    Object.defineProperty(window, 'isSecureContext', { configurable: true, value: false })
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: undefined })
    Object.defineProperty(document, 'execCommand', { configurable: true, value: execCommand })
    ;(window as any).$message = { success }
    document.body.innerHTML = '<div id="text-to-copy">{"humidity":60}</div>'

    const wrapper = mountComponent()
    await flushPromises()
    getSetupState(wrapper).copyText()

    expect(execCommand).toHaveBeenCalledWith('Copy')
    expect(success).toHaveBeenCalledWith('theme.configOperation.copySuccess')
  })
})
