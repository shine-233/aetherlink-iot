/**
 * 文件用途：覆盖 ReportScheduleFormModal（从 index.vue 拆出的创建/编辑弹窗）的展示契约。
 * 核心逻辑：表单数据与校验归 useReportSchedules；本套件验证：标题随 editing 切换、
 *   NForm 实例经 registerForm 回传（卸载置空，供页面 saveSchedule 校验）、
 *   取消走 update:show、保存走 save、csv/html/pdf 格式选项与 testid 不变、
 *   devices/keys 文本域经 defineModel 双向透传。
 */
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

vi.mock('naive-ui', () => {
  const stub = (name: string, tag = 'div') =>
    defineComponent({
      name,
      props: ['show', 'title', 'loading', 'disabled', 'options', 'value', 'model'],
      emits: ['click', 'update:value'],
      setup(_props, { slots, emit, expose }) {
        expose({ validate: () => Promise.resolve() })
        return () => h(tag, { onClick: () => emit('click') }, [slots.default?.(), slots.footer?.()])
      }
    })
  return {
    NModal: stub('NModal'),
    NForm: stub('NForm'),
    NFormItem: stub('NFormItem'),
    NInput: stub('NInput'),
    NInputNumber: stub('NInputNumber'),
    NSelect: stub('NSelect'),
    NSwitch: stub('NSwitch'),
    NButton: stub('NButton', 'button'),
    NSpace: stub('NSpace')
  }
})

import ReportScheduleFormModal from '../ReportScheduleFormModal.vue'
import { REPORT_FORMAT_OPTIONS } from '../report-helpers'

const form = {
  name: 'Daily fleet',
  cron_expr: '0 8 * * *',
  timezone: 'UTC',
  recipients: 'ops@example.com',
  device_ids: ['device-1'],
  keys: ['temperature'],
  lookback_hours: 24,
  format: 'csv',
  enabled: true
}

const mountModal = (overrides: Record<string, unknown> = {}) =>
  mount(ReportScheduleFormModal, {
    props: {
      show: true,
      editing: false,
      form,
      saving: false,
      deviceIdsText: '',
      keysText: '',
      registerForm: vi.fn(),
      ...overrides
    }
  })

describe('report schedule form modal', () => {
  it('switches the modal title between create and edit states', () => {
    const created = mountModal()
    expect(created.findComponent({ name: 'NModal' }).props('title')).toBe('report.form.createTitle')
    created.unmount()

    const editing = mountModal({ editing: true })
    expect(editing.findComponent({ name: 'NModal' }).props('title')).toBe('report.form.editTitle')
    editing.unmount()
  })

  it('hands the NForm instance to registerForm and clears it on unmount', async () => {
    const registerForm = vi.fn()
    const wrapper = mountModal({ registerForm })
    await wrapper.vm.$nextTick()

    expect(registerForm).toHaveBeenCalledTimes(1)
    expect(registerForm.mock.calls[0]?.[0]).toBeTruthy()

    wrapper.unmount()
    // 函数引用以 ref(instance, refs) 调用；卸载后首参置空，页面侧 formRef 归 null。
    expect(registerForm.mock.calls.at(-1)?.[0]).toBeNull()
  })

  it('emits update:show false on cancel and save on submit', async () => {
    const wrapper = mountModal()
    const buttons = wrapper.findAll('button')

    await buttons[0].trigger('click')
    expect(wrapper.emitted('update:show')).toEqual([[false]])

    await buttons[buttons.length - 1].trigger('click')
    expect(wrapper.emitted('save')).toHaveLength(1)
  })

  it('keeps the csv/html/pdf format options and the stable test id', () => {
    const wrapper = mountModal()
    const select = wrapper.findComponent({ name: 'NSelect' })

    expect(select.props('options')).toEqual(REPORT_FORMAT_OPTIONS)
    expect(select.attributes('data-testid')).toBe('report-format')
  })

  it('binds the devices/keys textareas through the defineModel passthrough', async () => {
    const wrapper = mountModal({ deviceIdsText: 'device-1', keysText: 'temperature' })
    const inputs = wrapper.findAllComponents({ name: 'NInput' })
    expect(inputs).toHaveLength(6)
    expect(inputs[4].props('value')).toBe('device-1')
    expect(inputs[5].props('value')).toBe('temperature')

    inputs[4].vm.$emit('update:value', 'device-1\ndevice-2')
    await wrapper.vm.$nextTick()
    expect(wrapper.emitted('update:deviceIdsText')).toEqual([['device-1\ndevice-2']])
  })
})
