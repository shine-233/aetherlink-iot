/**
 * 文件用途：验证交互模板预览组件的模板展示、演示和导出行为。
 * 核心逻辑：使用组件桩模拟 Naive UI，传入模板数据并断言渲染内容与事件。
 * 关键注意事项：模板字段与导出格式变化时，需要同步更新测试数据和断言。
 * 重构建议：可将模板示例数据抽成 fixture，供选择器和预览测试共享。
 */
import { defineComponent, h, nextTick } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
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
    success: hoisted.messageSuccess,
    error: hoisted.messageError
  }),
  NIcon: defineComponent({
    props: ['color', 'size'],
    setup(props, { slots }) {
      return () => h('i', { class: 'n-icon-stub', style: { color: props.color } }, slots.default?.())
    }
  }),
  NText: defineComponent({
    props: ['strong', 'depth'],
    setup(_, { slots }) {
      return () => h('span', { class: 'n-text-stub' }, slots.default?.())
    }
  }),
  NSpace: defineComponent({
    props: ['justify', 'align', 'size'],
    setup(_, { slots }) {
      return () => h('div', { class: 'n-space-stub' }, slots.default?.())
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
      return () => h('span', { class: 'n-tag-stub', 'data-type': props.type }, slots.default?.())
    }
  }),
  NSwitch: defineComponent({
    props: {
      value: Boolean,
      disabled: Boolean,
      size: String
    },
    setup(props) {
      return () =>
        h(
          'button',
          {
            class: 'n-switch-stub',
            disabled: props.disabled,
            'data-enabled': String(props.value),
            type: 'button'
          },
          String(props.value)
        )
    }
  }),
  NButton: defineComponent({
    props: {
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
            type: 'button',
            onClick: () => emit('click')
          },
          [slots.icon?.(), slots.default?.()]
        )
    }
  })
}))

vi.mock('@vicons/ionicons5', () => ({
  CheckmarkOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-check' }) }),
  DownloadOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-download' }) }),
  FlashOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-flash' }) }),
  PlayOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-play' }) }),
  RefreshOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-refresh' }) })
}))

import InteractionTemplatePreview from './InteractionTemplatePreview.vue'

type PreviewResponse = {
  action: string
  value: any
  duration?: number
  delay?: number
  easing?: string
}

type PreviewConfig = {
  event: string
  enabled: boolean
  name?: string
  priority?: number
  responses: PreviewResponse[]
}

type PreviewTemplate = {
  id: string
  name: string
  description: string
  category: string
  icon: any
  color: string
  tags?: string[]
  config: PreviewConfig[]
}

const FixtureIcon = defineComponent({ setup: () => () => h('svg', { class: 'template-fixture-icon' }) })
const mountedWrappers: VueWrapper[] = []

const response = (action: string, value: any, overrides: Partial<PreviewResponse> = {}): PreviewResponse => ({
  action,
  value,
  duration: 80,
  ...overrides
})

const config = (overrides: Partial<PreviewConfig> = {}): PreviewConfig => ({
  event: 'click',
  enabled: true,
  name: 'Click config',
  priority: 0,
  responses: [response('changeContent', 'clicked')],
  ...overrides
})

const templateFixture = (overrides: Partial<PreviewTemplate> = {}): PreviewTemplate => ({
  id: 'template-a',
  name: 'Template A',
  description: 'Template description',
  category: 'basic',
  icon: FixtureIcon,
  color: '#18a058',
  tags: ['preview', 'interaction'],
  config: [config()],
  ...overrides
})

const mountPreview = (template: PreviewTemplate) => {
  const wrapper = mount(InteractionTemplatePreview, {
    props: { template },
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

describe('InteractionTemplatePreview.vue', () => {
  let originalCreateObjectURL: typeof URL.createObjectURL | undefined
  let originalRevokeObjectURL: typeof URL.revokeObjectURL | undefined
  let clickSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T03:00:00.000Z'))
    vi.clearAllMocks()
    document.body.innerHTML = ''
    originalCreateObjectURL = URL.createObjectURL
    originalRevokeObjectURL = URL.revokeObjectURL
    URL.createObjectURL = vi.fn(() => 'blob:template-preview')
    URL.revokeObjectURL = vi.fn()
    clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined)
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    URL.createObjectURL = originalCreateObjectURL as typeof URL.createObjectURL
    URL.revokeObjectURL = originalRevokeObjectURL as typeof URL.revokeObjectURL
    clickSpy.mockRestore()
    document.body.innerHTML = ''
    vi.useRealTimers()
  })

  it('renders template metadata, statistics, event tags, priority, switches, and response formatting branches', async () => {
    const circularValue: Record<string, unknown> = {}
    circularValue.self = circularValue
    const template = templateFixture({
      config: [
        config({
          name: '',
          priority: 3,
          event: 'click',
          responses: [
            response('changeBackgroundColor', '#111111', { delay: 20, easing: 'ease-in' }),
            response('changeSize', { width: 200, height: 90 }),
            response('changeOpacity', 0.625),
            response('changeVisibility', 'visible'),
            response('changeContent', 'This text is intentionally longer than twenty chars'),
            response('custom', circularValue)
          ]
        }),
        config({
          event: 'mqtt',
          enabled: false,
          name: 'Unknown event config',
          responses: [response('unknownAction', 'raw-value')]
        })
      ]
    })

    const wrapper = mountPreview(template)
    await nextTick()

    expect(wrapper.text()).toContain('Template A')
    expect(wrapper.text()).toContain('Template description')
    expect(wrapper.text()).toContain('interaction.template.interactionCount')
    expect(wrapper.text()).toContain('interaction.template.actionCount')
    expect(wrapper.text()).toContain('interaction.template.eventTypeCount')
    expect(wrapper.text()).toContain('interaction.template.configIndex:{"index":1}')
    expect(wrapper.text()).toContain('interaction.template.priorityLabel:{"priority":3}')
    expect(wrapper.text()).toContain('200×90')
    expect(wrapper.text()).toContain('63%')
    expect(wrapper.text()).toContain('interaction.visibility.visible')
    expect(wrapper.text()).toContain('This text is intenti...')
    expect(wrapper.text()).toContain('[object Object]')
    expect(wrapper.text()).toContain('mqtt')
    expect(wrapper.text()).toContain('raw-value')
    expect(wrapper.findAll('.n-switch-stub').map(item => item.attributes('disabled'))).toEqual(['', ''])
    expect(wrapper.findAll('.n-tag-stub').map(item => item.attributes('data-type'))).toEqual(
      expect.arrayContaining(['success', 'default', 'info'])
    )
  })

  it('runs preview interactions by event, priority, and delay while ignoring disabled configs', async () => {
    const template = templateFixture({
      config: [
        config({
          name: 'Slow low priority',
          priority: 1,
          responses: [response('changeContent', 'low priority result', { delay: 30 })]
        }),
        config({
          name: 'Fast high priority',
          priority: 10,
          responses: [response('changeBackgroundColor', 'rgb(1, 2, 3)', { delay: 0 })]
        }),
        config({
          event: 'click',
          enabled: false,
          name: 'Disabled config',
          responses: [response('changeContent', 'disabled result')]
        })
      ]
    })
    const wrapper = mountPreview(template)
    await nextTick()

    await previewElement(wrapper).trigger('click')
    await vi.advanceTimersByTimeAsync(0)

    expect(previewElement(wrapper).element.style.backgroundColor).toBe('rgb(1, 2, 3)')
    expect(previewElement(wrapper).text()).not.toContain('disabled result')

    await vi.advanceTimersByTimeAsync(30)
    expect(previewElement(wrapper).text()).toContain('low priority result')
  })

  it('applies all preview response action styles and restores the preview on reset or mouse leave', async () => {
    const template = templateFixture({
      config: [
        config({
          responses: [
            response('changeBackgroundColor', 'rgb(10, 20, 30)'),
            response('changeTextColor', 'rgb(30, 20, 10)'),
            response('changeBorderColor', '#010101'),
            response('changeSize', { width: 222, height: 111 }),
            response('changeOpacity', 0.34),
            response('changeTransform', 'scale(1.2)'),
            response('changeVisibility', 'hidden'),
            response('changeContent', 'content action result'),
            response('triggerAnimation', 'bounce', { duration: 70, easing: 'linear' }),
            response('custom', { paddingLeft: '14px', zIndex: '7' })
          ]
        })
      ]
    })
    const wrapper = mountPreview(template)
    await nextTick()

    await previewElement(wrapper).trigger('click')
    await vi.advanceTimersByTimeAsync(0)

    const element = previewElement(wrapper).element
    expect(element.style.backgroundColor).toBe('rgb(10, 20, 30)')
    expect(element.style.color).toBe('rgb(30, 20, 10)')
    expect(element.style.borderTopColor).toBe('#010101')
    expect(element.style.borderRightColor).toBe('#010101')
    expect(element.style.borderBottomColor).toBe('#010101')
    expect(element.style.borderLeftColor).toBe('#010101')
    expect(element.style.width).toBe('222px')
    expect(element.style.height).toBe('111px')
    expect(element.style.opacity).toBe('0.34')
    expect(element.style.transform).toBe('scale(1.2)')
    expect(element.style.visibility).toBe('hidden')
    expect(element.style.animation).toBe('bounce 70ms linear')
    expect(element.style.paddingLeft).toBe('14px')
    expect(element.style.zIndex).toBe('7')
    expect(wrapper.text()).toContain('content action result')

    await previewElement(wrapper).trigger('mouseleave')
    expect(wrapper.text()).toContain('interaction.template.previewTarget')

    await previewElement(wrapper).trigger('click')
    await vi.advanceTimersByTimeAsync(0)
    await buttonByText(wrapper, 'interaction.reset').trigger('click')
    expect(wrapper.text()).toContain('interaction.template.previewTarget')
  })

  it('runs every enabled preview event in sequence and resets after the final scheduled step', async () => {
    const template = templateFixture({
      config: [
        config({ event: 'click', responses: [response('changeContent', 'click preview')] }),
        config({ event: 'hover', responses: [response('changeContent', 'hover preview')] }),
        config({ event: 'focus', responses: [response('changeContent', 'focus preview')] }),
        config({ event: 'blur', responses: [response('changeContent', 'blur preview')] }),
        config({ event: 'custom', responses: [response('changeContent', 'custom preview')] })
      ]
    })
    const wrapper = mountPreview(template)
    await nextTick()

    await buttonByText(wrapper, 'interaction.template.previewAll').trigger('click')

    await vi.advanceTimersByTimeAsync(0)
    expect(wrapper.text()).toContain('click preview')
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.text()).toContain('hover preview')
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.text()).toContain('focus preview')
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.text()).toContain('blur preview')
    await vi.advanceTimersByTimeAsync(1000)
    expect(wrapper.text()).toContain('custom preview')
    await vi.advanceTimersByTimeAsync(2000)
    expect(wrapper.text()).toContain('interaction.template.previewTarget')
  })

  it('emits close and select events and reports template application success', async () => {
    const template = templateFixture()
    const wrapper = mountPreview(template)
    await nextTick()

    await buttonByText(wrapper, 'interaction.cancel').trigger('click')
    await buttonByText(wrapper, 'interaction.template.selectTemplate').trigger('click')

    expect(wrapper.emitted('close')).toHaveLength(1)
    expect(wrapper.emitted('select')).toEqual([[template]])
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('interaction.messages.templateApplied')
  })

  it('exports a template JSON file with tags and timestamp, then revokes the object URL', async () => {
    const template = templateFixture()
    const wrapper = mountPreview(template)
    await nextTick()

    await buttonByText(wrapper, 'interaction.template.export').trigger('click')

    expect(URL.createObjectURL).toHaveBeenCalledWith(expect.any(Blob))
    expect(clickSpy).toHaveBeenCalledTimes(1)
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:template-preview')
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('interaction.messages.templateExported')
  })

  it('reports export failure when the browser download URL cannot be created', async () => {
    ;(URL.createObjectURL as any).mockImplementationOnce(() => {
      throw new Error('download blocked')
    })
    const wrapper = mountPreview(templateFixture())
    await nextTick()

    await buttonByText(wrapper, 'interaction.template.export').trigger('click')

    expect(hoisted.messageError).toHaveBeenCalledWith('interaction.messages.exportFailed')
    expect(clickSpy).toHaveBeenCalledTimes(0)
  })
})
