/**
 * 文件用途：覆盖 operation-log/modules/message-block.vue 的载荷美化与折叠交互。
 * 核心逻辑：验证 JSON 美化、非 JSON 原文兜底、空内容占位与超长文本折叠按钮的状态切换。
 * 关键注意事项：折叠阈值与样式类名被审计页面展开行测试间接依赖，调整时需同步。
 */
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import MessageBlock from '../message-block.vue'

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const mountBlock = (props: { label: string; message: string | null }) => {
  const wrapper = mount(MessageBlock, { props })
  mountedWrappers.push(wrapper)
  return wrapper
}

describe('management/operation-log/modules/message-block.vue', () => {
  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('pretty-prints JSON payloads and keeps short content unfolded', () => {
    const wrapper = mountBlock({ label: '请求内容', message: '{"name":"dev-1","tags":["a"]}' })

    const body = wrapper.get('pre')
    expect(body.text()).toContain('"name": "dev-1"')
    expect(body.classes()).not.toContain('is-folded')
    // 短内容不渲染折叠按钮
    expect(wrapper.find('.operation-log-message-block__toggle').exists()).toBe(false)
  })

  it('falls back to raw text for non-JSON payloads and shows empty placeholder', () => {
    const rawWrapper = mountBlock({ label: '响应内容', message: 'plain-text-response' })
    expect(rawWrapper.get('pre').text()).toContain('plain-text-response')

    const emptyWrapper = mountBlock({ label: '响应内容', message: null })
    expect(emptyWrapper.find('pre').exists()).toBe(false)
    expect(emptyWrapper.text()).toContain('custom.management.operationLog.detail.empty')
  })

  it('folds oversized payloads by default and toggles through the expand button', async () => {
    const longText = `x${'y'.repeat(500)}`
    const wrapper = mountBlock({ label: '响应内容', message: longText })

    const toggle = wrapper.get('.operation-log-message-block__toggle')
    expect(toggle.text()).toBe('custom.management.operationLog.detail.expand')
    expect(wrapper.get('pre').classes()).toContain('is-folded')

    await toggle.trigger('click')
    expect(wrapper.get('pre').classes()).not.toContain('is-folded')
    expect(wrapper.get('.operation-log-message-block__toggle').text()).toBe(
      'custom.management.operationLog.detail.collapse'
    )

    await wrapper.find('.operation-log-message-block__toggle').trigger('click')
    expect(wrapper.get('pre').classes()).toContain('is-folded')
  })
})
