/**
 * 文件用途：验证交互预览组件的执行、重置和用户反馈行为。
 * 核心逻辑：挂载预览组件后驱动按钮与交互数据，断言预览状态和消息调用。
 * 关键注意事项：测试重点是组件可见行为，不应依赖真实动画或浏览器计时细节。
 * 重构建议：可将预览执行器抽离后补充纯函数单测，降低组件测试复杂度。
 */
import { defineComponent, h, nextTick } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  messageSuccess: vi.fn(),
  t: (key: string, params?: Record<string, unknown>) => {
    return params ? `${key}:${JSON.stringify(params)}` : key
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: hoisted.t }),
  createI18n: () => ({ global: { t: hoisted.t, locale: { value: 'en-US' } } })
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({
    success: hoisted.messageSuccess
  }),
  NSpace: defineComponent({
    props: ['justify', 'align', 'size'],
    setup(_, { slots }) {
      return () => h('div', { class: 'n-space-stub' }, slots.default?.())
    }
  }),
  NText: defineComponent({
    props: ['strong', 'depth'],
    setup(_, { slots }) {
      return () => h('span', { class: 'n-text-stub' }, slots.default?.())
    }
  }),
  NButton: defineComponent({
    props: {
      disabled: Boolean,
      quaternary: Boolean,
      size: String,
      type: String
    },
    emits: ['click'],
    setup(props, { emit, slots }) {
      return () =>
        h(
          'button',
          {
            class: ['n-button-stub', props.type ? `n-button-${props.type}` : ''],
            disabled: props.disabled,
            type: 'button',
            onClick: () => {
              if (!props.disabled) emit('click')
            }
          },
          [slots.icon?.(), slots.default?.()]
        )
    }
  }),
  NIcon: defineComponent({
    props: ['size'],
    setup(_, { slots }) {
      return () => h('i', { class: 'n-icon-stub' }, slots.default?.())
    }
  }),
  NCard: defineComponent({
    props: ['size'],
    setup(_, { slots }) {
      return () => h('section', { class: 'n-card-stub' }, [slots.header?.(), slots.default?.()])
    }
  }),
  NTag: defineComponent({
    props: ['type', 'size', 'round'],
    setup(props, { slots }) {
      return () =>
        h(
          'span',
          {
            class: 'n-tag-stub',
            'data-type': props.type
          },
          slots.default?.()
        )
    }
  }),
  NSwitch: defineComponent({
    props: {
      value: Boolean,
      size: String
    },
    emits: ['update:value'],
    setup(props, { emit }) {
      return () =>
        h(
          'button',
          {
            class: 'n-switch-stub',
            'data-enabled': String(props.value),
            type: 'button',
            onClick: () => emit('update:value', !props.value)
          },
          String(props.value)
        )
    }
  })
}))

vi.mock('@vicons/ionicons5', () => ({
  FlashOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-flash' }) }),
  PlayCircleOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-play-circle' }) }),
  PlayOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-play' }) }),
  RefreshOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-refresh' }) })
}))

import InteractionPreview from './InteractionPreview.vue'

type PreviewInteraction = {
  event: string
  enabled: boolean
  name?: string
  priority?: number
  responses: PreviewResponse[]
}

type PreviewResponse = {
  action: string
  value: any
  duration?: number
  delay?: number
  easing?: string
}

const mountedWrappers: VueWrapper[] = []

const response = (action: string, value: any, overrides: Partial<PreviewResponse> = {}): PreviewResponse => ({
  action,
  value,
  duration: 60,
  ...overrides
})

const interaction = (overrides: Partial<PreviewInteraction> = {}): PreviewInteraction => ({
  event: 'click',
  enabled: true,
  name: 'Click interaction',
  priority: 0,
  responses: [response('changeContent', 'clicked')],
  ...overrides
})

const mountPreview = (interactions: PreviewInteraction[]) => {
  const wrapper = mount(InteractionPreview, {
    props: {
      interactions,
      componentId: 'component-preview'
    },
    attachTo: document.body
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const previewElement = (wrapper: VueWrapper) => wrapper.get<HTMLElement>('.preview-element')

const buttonByText = (wrapper: VueWrapper, text: string) => {
  const button = wrapper.findAll('button').find(item => item.text().includes(text))
  if (!button) throw new Error(`Button not found: ${text}`)
  return button
}

const logEntries = (wrapper: VueWrapper) => {
  return wrapper.findAll('.log-entry').map(entry => ({
    classes: entry.classes(),
    message: entry.get('.log-message').text()
  }))
}

const chronologicalLogMessages = (wrapper: VueWrapper) => logEntries(wrapper).map(entry => entry.message).reverse()

describe('InteractionPreview.vue', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T02:00:00.000Z'))
    vi.clearAllMocks()
    document.body.innerHTML = ''
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    document.body.innerHTML = ''
    vi.useRealTimers()
  })

  it('disables batch controls when every interaction is disabled and logs unmatched events', async () => {
    const interactions = [
      interaction({
        enabled: false,
        name: 'Disabled click',
        responses: [response('changeBackgroundColor', '#ff0000')]
      })
    ]
    const wrapper = mountPreview(interactions)
    await nextTick()

    expect(buttonByText(wrapper, 'interaction.reset').attributes('disabled')).toBe('')
    expect(buttonByText(wrapper, 'interaction.preview.runAll').attributes('disabled')).toBe('')

    await previewElement(wrapper).trigger('click')

    const [latest] = logEntries(wrapper)
    expect(latest.classes).toContain('warning')
    expect(latest.message).toContain('interaction.preview.noEnabledInteractions')
    expect(latest.message).toContain('interaction.events.click')
  })

  it('executes enabled click interactions by priority, applies delayed responses, and clears active state', async () => {
    const interactions = [
      interaction({
        name: 'Low priority',
        priority: 1,
        responses: [response('changeTextColor', 'rgb(1, 2, 3)', { delay: 20, duration: 40 })]
      }),
      interaction({
        name: 'High priority',
        priority: 9,
        responses: [response('changeBackgroundColor', 'rgb(9, 8, 7)', { duration: 80 })]
      })
    ]
    const wrapper = mountPreview(interactions)
    await nextTick()

    await previewElement(wrapper).trigger('click')

    const chronological = chronologicalLogMessages(wrapper)
    expect(chronological.findIndex(message => message.includes('High priority'))).toBeLessThan(
      chronological.findIndex(message => message.includes('Low priority'))
    )
    expect(wrapper.findAll('.interaction-item')[1].classes()).toContain('active')

    await vi.advanceTimersByTimeAsync(0)
    expect(previewElement(wrapper).element.style.backgroundColor).toBe('rgb(9, 8, 7)')

    await vi.advanceTimersByTimeAsync(20)
    expect(previewElement(wrapper).element.style.color).toBe('rgb(1, 2, 3)')

    await vi.advanceTimersByTimeAsync(80)
    expect(wrapper.findAll('.interaction-item')[1].classes()).not.toContain('active')
  })

  it('applies every built-in response action and formats the response list for each action branch', async () => {
    const responses = [
      response('changeBackgroundColor', 'rgb(10, 20, 30)'),
      response('changeTextColor', 'rgb(30, 20, 10)'),
      response('changeBorderColor', '#010101'),
      response('changeSize', { width: 321, height: 123 }),
      response('changeOpacity', 0.42),
      response('changeTransform', 'scale(1.25)'),
      response('changeVisibility', 'hidden'),
      response('changeContent', 'Updated preview content'),
      response('triggerAnimation', 'pulse', { duration: 90, easing: 'linear' }),
      response('custom', { marginTop: '12px', zIndex: '4' })
    ]
    const wrapper = mountPreview([interaction({ responses })])
    await nextTick()

    expect(wrapper.text()).toContain('42%')
    expect(wrapper.text()).toContain('Updated preview cont...')

    await previewElement(wrapper).trigger('click')
    await vi.advanceTimersByTimeAsync(0)

    const element = previewElement(wrapper).element
    expect(element.style.backgroundColor).toBe('rgb(10, 20, 30)')
    expect(element.style.color).toBe('rgb(30, 20, 10)')
    expect(element.style.borderTopColor).toBe('#010101')
    expect(element.style.borderRightColor).toBe('#010101')
    expect(element.style.borderBottomColor).toBe('#010101')
    expect(element.style.borderLeftColor).toBe('#010101')
    expect(element.style.width).toBe('321px')
    expect(element.style.height).toBe('123px')
    expect(element.style.opacity).toBe('0.42')
    expect(element.style.transform).toBe('scale(1.25)')
    expect(element.style.visibility).toBe('hidden')
    expect(element.style.animation).toBe('pulse 90ms linear')
    expect(element.style.marginTop).toBe('12px')
    expect(element.style.zIndex).toBe('4')
    expect(wrapper.text()).toContain('Updated preview content')
  })

  it('runs all enabled event types, skips disabled interactions, and logs hover end separately', async () => {
    const interactions = [
      interaction({ event: 'click', name: 'Click branch', responses: [response('changeContent', 'click ran')] }),
      interaction({ event: 'hover', name: 'Hover branch', responses: [response('changeContent', 'hover ran')] }),
      interaction({ event: 'focus', name: 'Focus branch', responses: [response('changeContent', 'focus ran')] }),
      interaction({ event: 'blur', name: 'Blur branch', responses: [response('changeContent', 'blur ran')] }),
      interaction({ event: 'custom', name: 'Custom branch', responses: [response('changeContent', 'custom ran')] }),
      interaction({
        event: 'custom',
        enabled: false,
        name: 'Disabled custom branch',
        responses: [response('changeContent', 'disabled ran')]
      })
    ]
    const wrapper = mountPreview(interactions)
    await nextTick()

    await buttonByText(wrapper, 'interaction.preview.runAll').trigger('click')
    await vi.advanceTimersByTimeAsync(0)

    const combinedLogs = logEntries(wrapper)
      .map(entry => entry.message)
      .join('\n')
    expect(combinedLogs).toContain('interaction.preview.startExecutingAll')
    expect(combinedLogs).toContain('Click branch')
    expect(combinedLogs).toContain('Hover branch')
    expect(combinedLogs).toContain('Focus branch')
    expect(combinedLogs).toContain('Blur branch')
    expect(combinedLogs).toContain('Custom branch')
    expect(combinedLogs).not.toContain('Disabled custom branch')
    expect(wrapper.text()).toContain('custom ran')

    await previewElement(wrapper).trigger('mouseleave')
    expect(logEntries(wrapper)[0].message).toContain('interaction.preview.hoverEnd')
  })

  it('toggles an interaction, executes the card test button, resets preview, and clears the log', async () => {
    const interactions = [
      interaction({
        enabled: false,
        name: 'Switchable interaction',
        responses: [response('changeContent', 'single test ran')]
      })
    ]
    const wrapper = mountPreview(interactions)
    await nextTick()

    await wrapper.get('.n-switch-stub').trigger('click')
    expect(interactions[0].enabled).toBe(true)
    expect(logEntries(wrapper)[0].message).toContain('interaction.preview.interactionToggled')

    await buttonByText(wrapper, 'interaction.preview.test').trigger('click')
    await vi.advanceTimersByTimeAsync(0)
    expect(wrapper.text()).toContain('single test ran')

    await buttonByText(wrapper, 'interaction.reset').trigger('click')
    expect(wrapper.text()).toContain('interaction.editor.previewElement')
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('interaction.messages.previewReset')

    await buttonByText(wrapper, 'interaction.clear').trigger('click')
    const entries = logEntries(wrapper)
    expect(entries).toHaveLength(1)
    expect(entries[0].message).toBe('interaction.messages.logCleared')
  })

  it('keeps only the latest 100 log entries when a high-volume interaction runs', async () => {
    const responses = Array.from({ length: 101 }, (_, index) =>
      response('changeContent', `bulk-${index}`, { delay: 0, duration: 1 })
    )
    const wrapper = mountPreview([interaction({ responses })])
    await nextTick()

    await previewElement(wrapper).trigger('click')
    await vi.advanceTimersByTimeAsync(0)

    expect(logEntries(wrapper)).toHaveLength(100)
    expect(logEntries(wrapper).some(entry => entry.message.includes('interaction.preview.previewStarted'))).toBe(false)
  }, 10_000)

  it('logs a failed action when a custom style object throws during application', async () => {
    const brokenStyle: Record<string, unknown> = {}
    const wrapper = mountPreview([interaction({ responses: [response('custom', brokenStyle)] })])
    await nextTick()
    Object.defineProperty(brokenStyle, 'color', {
      enumerable: true,
      get() {
        throw new Error('style getter failed')
      }
    })

    await previewElement(wrapper).trigger('click')
    await vi.advanceTimersByTimeAsync(0)

    const [latest] = logEntries(wrapper)
    expect(latest.classes).toContain('error')
    expect(latest.message).toContain('interaction.preview.actionFailed')
    expect(latest.message).toContain('interaction.actions.custom')
  })
})
