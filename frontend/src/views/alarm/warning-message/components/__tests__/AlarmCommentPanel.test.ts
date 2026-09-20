/**
 * 文件用途：验证告警评论面板的加载、空态、提交、删除与失败分支。
 * 核心逻辑：mock 掉三个 alarm 评论 wrapper、auth store 与 naive-ui 离散组件，
 *   用可控的 Promise 断言真实交互路径（输入 -> 提交 -> 重新拉取），而不是只断言能 mount。
 * 关键注意事项：
 *   1. 面板在 setup 里就调用 useMessage()，必须 mock naive-ui，否则缺少 provider 会直接抛错。
 *   2. 加载走 watch(immediate)，每个用例都要 await flushPromises() 才能断言渲染结果。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  messageError: vi.fn(),
  currentUserId: 'user-1',
  listResult: { data: { list: [] as any[] }, error: null as unknown },
  createResult: { data: {}, error: null as unknown },
  deleteResult: { data: {}, error: null as unknown },
  listAlarmComments: vi.fn(),
  createAlarmComment: vi.fn(),
  deleteAlarmComment: vi.fn()
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({ userInfo: { id: hoisted.currentUserId } })
}))

vi.mock('@/service/api/alarm', () => ({
  listAlarmComments: (...args: unknown[]) => {
    hoisted.listAlarmComments(...args)
    return Promise.resolve(hoisted.listResult)
  },
  createAlarmComment: (...args: unknown[]) => {
    hoisted.createAlarmComment(...args)
    return Promise.resolve(hoisted.createResult)
  },
  deleteAlarmComment: (...args: unknown[]) => {
    hoisted.deleteAlarmComment(...args)
    return Promise.resolve(hoisted.deleteResult)
  }
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({ error: hoisted.messageError }),
  NSpin: defineComponent({
    name: 'NSpin',
    setup(_, { slots }) {
      return () => h('div', { class: 'n-spin-stub' }, slots.default?.())
    }
  }),
  NEmpty: defineComponent({
    name: 'NEmpty',
    props: { description: { type: String, default: '' } },
    setup(props) {
      return () => h('div', { class: 'n-empty-stub' }, props.description)
    }
  }),
  NInput: defineComponent({
    name: 'NInput',
    props: { value: { type: String, default: '' } },
    emits: ['update:value'],
    setup(props, { attrs, emit }) {
      return () =>
        h('textarea', {
          ...attrs,
          value: props.value,
          onInput: (event: Event) => emit('update:value', (event.target as HTMLTextAreaElement).value)
        })
    }
  }),
  NButton: defineComponent({
    name: 'NButton',
    props: { disabled: { type: Boolean, default: false } },
    emits: ['click'],
    setup(props, { attrs, emit, slots }) {
      return () =>
        h(
          'button',
          {
            ...attrs,
            disabled: props.disabled,
            onClick: () => {
              if (!props.disabled) emit('click')
            }
          },
          slots.default?.()
        )
    }
  })
}))

import AlarmCommentPanel from '../AlarmCommentPanel.vue'

const ALARM_ID = 'alarm-history-1'

const comment = (overrides: Record<string, unknown> = {}) => ({
  id: 'comment-1',
  tenant_id: 'tenant-1',
  alarm_history_id: ALARM_ID,
  content: 'rebooted the gateway',
  author_user_id: 'user-1',
  created_at: '2026-09-14T08:00:00.000Z',
  ...overrides
})

const mountPanel = () => mount(AlarmCommentPanel, { props: { alarmHistoryId: ALARM_ID } })

const textarea = (wrapper: VueWrapper) => wrapper.get('textarea')
const submitButton = (wrapper: VueWrapper) =>
  wrapper.findAll('button').find((button) => button.text() === 'custom.alarmComment.submit')!
const deleteButtons = (wrapper: VueWrapper) =>
  wrapper.findAll('button').filter((button) => button.text() === 'common.delete')

describe('AlarmCommentPanel', () => {
  beforeEach(() => {
    hoisted.messageError.mockClear()
    hoisted.listAlarmComments.mockClear()
    hoisted.createAlarmComment.mockClear()
    hoisted.deleteAlarmComment.mockClear()
    hoisted.currentUserId = 'user-1'
    hoisted.listResult = { data: { list: [] }, error: null }
    hoisted.createResult = { data: {}, error: null }
    hoisted.deleteResult = { data: {}, error: null }
  })

  it('loads and renders the comments of the focused alarm history', async () => {
    hoisted.listResult = {
      data: { list: [comment({ content: 'first note' }), comment({ id: 'comment-2', content: 'second note' })] },
      error: null
    }

    const wrapper = mountPanel()
    await flushPromises()

    expect(hoisted.listAlarmComments).toHaveBeenCalledWith(ALARM_ID)
    expect(wrapper.text()).toContain('first note')
    expect(wrapper.text()).toContain('second note')
  })

  it('renders the empty state when the alarm has no comment', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.get('.n-empty-stub').text()).toBe('custom.alarmComment.empty')
  })

  it('submits the trimmed comment and reloads the list', async () => {
    hoisted.listResult = { data: { list: [] }, error: null }
    const wrapper = mountPanel()
    await flushPromises()
    hoisted.listAlarmComments.mockClear()

    await textarea(wrapper).setValue('  escalated to on-call  ')
    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(hoisted.createAlarmComment).toHaveBeenCalledWith(ALARM_ID, 'escalated to on-call')
    // 提交成功后要重新拉一次，否则列表不会刷新。
    expect(hoisted.listAlarmComments).toHaveBeenCalledWith(ALARM_ID)
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe('')
  })

  it('does not send a request when the draft is blank', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    await textarea(wrapper).setValue('   ')
    expect(submitButton(wrapper).attributes('disabled')).toBeDefined()

    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(hoisted.createAlarmComment).not.toHaveBeenCalled()
  })

  it('deletes an own comment and reloads the list', async () => {
    hoisted.listResult = { data: { list: [comment()] }, error: null }
    const wrapper = mountPanel()
    await flushPromises()
    hoisted.listAlarmComments.mockClear()

    expect(deleteButtons(wrapper)).toHaveLength(1)
    await deleteButtons(wrapper)[0].trigger('click')
    await flushPromises()

    expect(hoisted.deleteAlarmComment).toHaveBeenCalledWith(ALARM_ID, 'comment-1')
    expect(hoisted.listAlarmComments).toHaveBeenCalledWith(ALARM_ID)
  })

  it('hides the delete action on comments written by someone else', async () => {
    hoisted.listResult = { data: { list: [comment({ author_user_id: 'user-2' })] }, error: null }

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.text()).toContain('rebooted the gateway')
    expect(deleteButtons(wrapper)).toHaveLength(0)
  })

  it('shows the load failure text when listing fails', async () => {
    hoisted.listResult = { data: null, error: new Error('boom') }

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.get('.n-empty-stub').text()).toBe('custom.alarmComment.loadFailed')
  })

  it('reports a failed submit through the message api and keeps the draft', async () => {
    hoisted.createResult = { data: null, error: new Error('boom') }
    const wrapper = mountPanel()
    await flushPromises()

    await textarea(wrapper).setValue('will not be saved')
    await submitButton(wrapper).trigger('click')
    await flushPromises()

    expect(hoisted.messageError).toHaveBeenCalledWith('custom.alarmComment.submitFailed')
    expect((textarea(wrapper).element as HTMLTextAreaElement).value).toBe('will not be saved')
  })

  it('reports a failed delete through the message api', async () => {
    hoisted.listResult = { data: { list: [comment()] }, error: null }
    hoisted.deleteResult = { data: null, error: new Error('boom') }

    const wrapper = mountPanel()
    await flushPromises()

    await deleteButtons(wrapper)[0].trigger('click')
    await flushPromises()

    expect(hoisted.messageError).toHaveBeenCalledWith('custom.alarmComment.deleteFailed')
  })

  it('reloads when the alarm history changes instead of reusing the previous list', async () => {
    hoisted.listResult = { data: { list: [comment({ content: 'old alarm note' })] }, error: null }
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.text()).toContain('old alarm note')

    hoisted.listResult = { data: { list: [comment({ content: 'new alarm note' })] }, error: null }
    await wrapper.setProps({ alarmHistoryId: 'alarm-history-2' })
    await flushPromises()

    expect(hoisted.listAlarmComments).toHaveBeenLastCalledWith('alarm-history-2')
    expect(wrapper.text()).toContain('new alarm note')
    expect(wrapper.text()).not.toContain('old alarm note')
  })
})
