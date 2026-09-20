/**
 * 文件用途：验证告警指派面板的加载、当前处理人、指派/取消指派、用户选择器与失败分支。
 * 核心逻辑：mock 掉 alarm 指派 wrapper、notification 的 getUserList 与 naive-ui 离散组件，
 *   驱动真实交互（选人 -> 填备注 -> 提交 -> 重拉），断言 wrapper 的调用参数而不只是能 mount。
 * 关键注意事项：
 *   1. 面板在 setup 里就调用 useMessage()，必须 mock naive-ui，否则缺少 provider 会直接抛错。
 *   2. 加载走 watch(immediate)，选人走 setup 里的首次 loadUsers()，
 *      每个用例都要 await flushPromises() 才能断言渲染结果。
 *   3. NSelect 的 stub 用 input 承载搜索、用 button 承载选项，
 *      与 NInput（textarea）区分开，避免测试里选错元素。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  messageError: vi.fn(),
  listResult: { data: { list: [] as any[] }, error: null as unknown },
  assignResult: { data: {}, error: null as unknown },
  userResult: {
    data: {
      list: [
        { user_id: 'user-2', name: 'Alice' },
        { user_id: 'user-3', name: 'Bob' }
      ]
    },
    error: null as unknown
  },
  listAlarmAssignments: vi.fn(),
  assignAlarm: vi.fn(),
  getUserList: vi.fn()
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('@/service/api/alarm', () => ({
  listAlarmAssignments: (...args: unknown[]) => {
    hoisted.listAlarmAssignments(...args)
    return Promise.resolve(hoisted.listResult)
  },
  assignAlarm: (...args: unknown[]) => {
    hoisted.assignAlarm(...args)
    return Promise.resolve(hoisted.assignResult)
  }
}))

vi.mock('@/service/api/notification', () => ({
  getUserList: (...args: unknown[]) => {
    hoisted.getUserList(...args)
    return Promise.resolve(hoisted.userResult)
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
  NSelect: defineComponent({
    name: 'NSelect',
    props: {
      value: { type: [String, Number], default: null },
      options: { type: Array as unknown as () => { label: string; value: string }[], default: () => [] }
    },
    emits: ['update:value', 'search'],
    setup(props, { emit }) {
      return () =>
        h('div', { class: 'n-select-stub' }, [
          h('input', {
            class: 'n-select-search',
            onInput: (event: Event) => emit('search', (event.target as HTMLInputElement).value)
          }),
          ...props.options.map((option) =>
            h('button', { class: 'n-select-option', onClick: () => emit('update:value', option.value) }, option.label)
          )
        ])
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

import AlarmAssignmentPanel from '../AlarmAssignmentPanel.vue'

const ALARM_ID = 'alarm-history-1'

const assignment = (overrides: Record<string, unknown> = {}) => ({
  id: 'assignment-1',
  tenant_id: 'tenant-1',
  alarm_history_id: ALARM_ID,
  assignee_user_id: 'user-9',
  operator_user_id: 'user-1',
  remark: 'hand over to on-call',
  created_at: '2026-09-14T08:00:00.000Z',
  ...overrides
})

const mountPanel = () => mount(AlarmAssignmentPanel, { props: { alarmHistoryId: ALARM_ID } })

const currentAssignee = (wrapper: VueWrapper) => wrapper.get('[data-testid="alarm-current-assignee"]')
const remarkBox = (wrapper: VueWrapper) => wrapper.get('textarea')
const searchBox = (wrapper: VueWrapper) => wrapper.get('input.n-select-search')
const optionButton = (wrapper: VueWrapper, label: string) =>
  wrapper.findAll('button.n-select-option').find((button) => button.text() === label)!
const buttonByText = (wrapper: VueWrapper, text: string) =>
  wrapper.findAll('button').find((button) => button.text() === text)!

describe('AlarmAssignmentPanel', () => {
  beforeEach(() => {
    hoisted.messageError.mockClear()
    hoisted.listAlarmAssignments.mockClear()
    hoisted.assignAlarm.mockClear()
    hoisted.getUserList.mockClear()
    hoisted.listResult = { data: { list: [] }, error: null }
    hoisted.assignResult = { data: {}, error: null }
    hoisted.userResult = {
      data: {
        list: [
          { user_id: 'user-2', name: 'Alice' },
          { user_id: 'user-3', name: 'Bob' }
        ]
      },
      error: null
    }
  })

  it('loads the assignment history of the focused alarm and shows the latest assignee', async () => {
    hoisted.listResult = {
      data: {
        // 契约：created_at 倒序，最新在前，所以当前处理人是第 0 条。
        list: [
          assignment({ id: 'a-2', assignee_user_id: 'user-3', remark: 're-assigned' }),
          assignment({ id: 'a-1', assignee_user_id: 'user-2', remark: 'first owner' })
        ]
      },
      error: null
    }

    const wrapper = mountPanel()
    await flushPromises()

    expect(hoisted.listAlarmAssignments).toHaveBeenCalledWith(ALARM_ID)
    expect(currentAssignee(wrapper).text()).toBe('user-3')
    expect(wrapper.text()).toContain('re-assigned')
    expect(wrapper.text()).toContain('first owner')
  })

  it('renders the empty state and disables unassign when nobody is assigned', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.get('.n-empty-stub').text()).toBe('custom.alarmAssignment.empty')
    expect(currentAssignee(wrapper).text()).toBe('custom.alarmAssignment.unassigned')
    expect(buttonByText(wrapper, 'custom.alarmAssignment.unassign').attributes('disabled')).toBeDefined()
  })

  it('renders the unassigned state when the latest record clears the assignee', async () => {
    hoisted.listResult = {
      data: {
        list: [assignment({ id: 'a-2', assignee_user_id: null }), assignment({ id: 'a-1' })]
      },
      error: null
    }

    const wrapper = mountPanel()
    await flushPromises()

    expect(currentAssignee(wrapper).text()).toBe('custom.alarmAssignment.unassigned')
    expect(wrapper.text()).toContain('hand over to on-call')
  })

  it('loads the assignee options from the existing user selector on mount and on search', async () => {
    const wrapper = mountPanel()
    await flushPromises()

    expect(hoisted.getUserList).toHaveBeenLastCalledWith({ page: 1, page_size: 20, name: '' })
    expect(wrapper.findAll('button.n-select-option').map((button) => button.text())).toEqual(['Alice', 'Bob'])

    await searchBox(wrapper).setValue('ali')
    await flushPromises()

    expect(hoisted.getUserList).toHaveBeenLastCalledWith({ page: 1, page_size: 20, name: 'ali' })
  })

  it('assigns the selected user with the remark and reloads the history', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    hoisted.listAlarmAssignments.mockClear()

    expect(buttonByText(wrapper, 'custom.alarmAssignment.submit').attributes('disabled')).toBeDefined()

    await optionButton(wrapper, 'Alice').trigger('click')
    await remarkBox(wrapper).setValue('  take over  ')
    await buttonByText(wrapper, 'custom.alarmAssignment.submit').trigger('click')
    await flushPromises()

    expect(hoisted.assignAlarm).toHaveBeenCalledWith(ALARM_ID, {
      assignee_user_id: 'user-2',
      remark: 'take over'
    })
    // 提交成功后要重新拉一次，否则流水不会刷新。
    expect(hoisted.listAlarmAssignments).toHaveBeenCalledWith(ALARM_ID)
    expect((remarkBox(wrapper).element as HTMLTextAreaElement).value).toBe('')
  })

  it('unassigns by posting a null assignee', async () => {
    hoisted.listResult = { data: { list: [assignment()] }, error: null }
    const wrapper = mountPanel()
    await flushPromises()
    hoisted.listAlarmAssignments.mockClear()

    await buttonByText(wrapper, 'custom.alarmAssignment.unassign').trigger('click')
    await flushPromises()

    expect(hoisted.assignAlarm).toHaveBeenCalledWith(ALARM_ID, {
      assignee_user_id: null,
      remark: ''
    })
    expect(hoisted.listAlarmAssignments).toHaveBeenCalledWith(ALARM_ID)
  })

  it('shows the load failure text when listing the history fails', async () => {
    hoisted.listResult = { data: null, error: new Error('boom') }

    const wrapper = mountPanel()
    await flushPromises()

    expect(wrapper.get('.n-empty-stub').text()).toBe('custom.alarmAssignment.loadFailed')
    expect(currentAssignee(wrapper).text()).toBe('custom.alarmAssignment.unassigned')
  })

  it('reports a failed submit through the message api and keeps the remark', async () => {
    hoisted.assignResult = { data: null, error: new Error('boom') }
    const wrapper = mountPanel()
    await flushPromises()
    hoisted.listAlarmAssignments.mockClear()

    await optionButton(wrapper, 'Bob').trigger('click')
    await remarkBox(wrapper).setValue('will not be saved')
    await buttonByText(wrapper, 'custom.alarmAssignment.submit').trigger('click')
    await flushPromises()

    expect(hoisted.messageError).toHaveBeenCalledWith('custom.alarmAssignment.submitFailed')
    // 失败时不重拉，避免用旧流水覆盖错误提示的上下文。
    expect(hoisted.listAlarmAssignments).not.toHaveBeenCalled()
    expect((remarkBox(wrapper).element as HTMLTextAreaElement).value).toBe('will not be saved')
  })

  it('reports a failed user list load through the message api', async () => {
    hoisted.userResult = { data: null, error: new Error('boom') }

    const wrapper = mountPanel()
    await flushPromises()

    expect(hoisted.messageError).toHaveBeenCalledWith('custom.alarmAssignment.usersLoadFailed')
    expect(wrapper.findAll('button.n-select-option')).toHaveLength(0)
  })

  it('reloads when the alarm history changes instead of reusing the previous list', async () => {
    hoisted.listResult = { data: { list: [assignment({ remark: 'old alarm owner' })] }, error: null }
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.text()).toContain('old alarm owner')

    hoisted.listResult = {
      data: { list: [assignment({ remark: 'new alarm owner', assignee_user_id: 'user-4' })] },
      error: null
    }
    await wrapper.setProps({ alarmHistoryId: 'alarm-history-2' })
    await flushPromises()

    expect(hoisted.listAlarmAssignments).toHaveBeenLastCalledWith('alarm-history-2')
    expect(wrapper.text()).toContain('new alarm owner')
    expect(wrapper.text()).not.toContain('old alarm owner')
    expect(currentAssignee(wrapper).text()).toBe('user-4')
  })
})
