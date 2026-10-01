/**
 * 文件用途: DeviceDiagnosticsOverview / DeviceDebugConsole 展示组件测试。
 * 覆盖: 错误态提示、空失败记录提示、刷新/复制/开关事件、日志渲染与贴底滚动。
 */
import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('@/service/api', () => ({}))
vi.mock('@/locales', () => ({ $t: (key: string) => key }))
vi.mock('@vicons/ionicons5', () => ({
  Refresh: defineComponent({ setup: () => () => h('span') }),
  HelpCircleOutline: defineComponent({ setup: () => () => h('span') })
}))

import DeviceDebugConsole from '../DeviceDebugConsole.vue'
import DeviceDiagnosticsOverview from '../DeviceDiagnosticsOverview.vue'
import { createEmptyStatistics } from '../useDeviceDiagnosticsStats'

const passthrough = (tag = 'div', props: string[] = []) =>
  defineComponent({
    props,
    inheritAttrs: true,
    setup(_, { slots }) {
      return () => h(tag, [slots.trigger?.(), slots.default?.(), slots.extra?.()])
    }
  })

const stubs = {
  NButton: defineComponent({
    props: ['bordered', 'loading', 'size', 'secondary', 'type'],
    emits: ['click'],
    setup(_, { slots, emit }) {
      return () => h('button', { onClick: () => emit('click') }, slots.default?.())
    }
  }),
  NSwitch: defineComponent({
    props: ['value', 'loading'],
    emits: ['update:value'],
    setup(props, { emit }) {
      return () => h('input', { type: 'checkbox', class: 'switch', onClick: () => emit('update:value', !props.value) })
    }
  }),
  NAlert: passthrough('div', ['type', 'title']),
  NIcon: passthrough('span', ['size']),
  NFlex: passthrough('div', ['gap', 'vertical']),
  NCard: passthrough('div', ['title', 'size']),
  NText: passthrough('span', ['type', 'depth']),
  NNumberAnimation: defineComponent({ props: ['from', 'to', 'precision'], setup: () => () => h('span') }),
  NDataTable: defineComponent({
    props: ['columns', 'data', 'maxHeight', 'remote', 'loading'],
    setup: () => () => h('table')
  }),
  NTooltip: passthrough('div', ['trigger']),
  NEmpty: passthrough('div', ['description']),
  NTimeline: passthrough('ul'),
  NTimelineItem: passthrough('li', ['type', 'time', 'title'])
}

describe('DeviceDiagnosticsOverview', () => {
  const base = { statistics: createEmptyStatistics(), failureRecords: [], nextSteps: ['step-1'] }

  it('shows an error alert with the error message instead of the empty-failures hint', () => {
    const wrapper = mount(DeviceDiagnosticsOverview, {
      props: { ...base, status: 'error', error: new Error('boom') },
      global: { stubs }
    })
    expect(wrapper.find('[data-testid="diagnostics-error"]').text()).toContain('boom')
    expect(wrapper.text()).not.toContain('custom.device_details.diagnosisNoFailureRecords')
  })

  it('shows the empty-failures hint when ready with no failures and emits refresh', async () => {
    const wrapper = mount(DeviceDiagnosticsOverview, { props: { ...base, status: 'ready' }, global: { stubs } })
    expect(wrapper.find('[data-testid="diagnostics-error"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('- step-1')
    await wrapper.find('[data-testid="diagnostics-refresh"]').trigger('click')
    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })
})

describe('DeviceDebugConsole', () => {
  const base = { logEnabled: false, debugLogs: [] as string[], timeline: [], nextSteps: ['step-1'] }

  it('renders logs, emits switch/copy, and surfaces fetch errors', async () => {
    const wrapper = mount(DeviceDebugConsole, {
      props: { ...base, debugLogs: ['[t] line-1'], logsError: new Error('502') },
      global: { stubs }
    })
    expect(wrapper.find('[data-testid="debug-log-container"]').text()).toContain('line-1')
    expect(wrapper.find('[data-testid="debug-console-error"]').text()).toContain('common.loadFailed: 502')

    await wrapper.find('.switch').trigger('click')
    expect(wrapper.emitted('update:logEnabled')).toEqual([[true]])
    await wrapper.find('[data-testid="copy-summary"]').trigger('click')
    expect(wrapper.emitted('copy-summary')).toHaveLength(1)
  })

  it('auto-scrolls only while the user is pinned to the bottom', async () => {
    const wrapper = mount(DeviceDebugConsole, { props: base, global: { stubs }, attachTo: document.body })
    const el = wrapper.find('[data-testid="debug-log-container"]').element as HTMLElement
    Object.defineProperty(el, 'scrollHeight', { configurable: true, value: 1000 })
    Object.defineProperty(el, 'clientHeight', { configurable: true, value: 400 })

    await wrapper.setProps({ debugLogs: ['a'] })
    await nextTick()
    expect(el.scrollTop).toBe(1000)

    // 用户向上翻看历史：离底部 > 阈值
    el.scrollTop = 100
    await wrapper.find('[data-testid="debug-log-container"]').trigger('scroll')
    await wrapper.setProps({ debugLogs: ['a', 'b'] })
    await nextTick()
    expect(el.scrollTop).toBe(100)
    wrapper.unmount()
  })
})
