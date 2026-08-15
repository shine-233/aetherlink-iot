/**
 * 文件用途：验证交互模板选择器的分类筛选、预览、选择和文件导入流程。
 * 核心逻辑：通过 mock 模板预览组件、上传控件和 FileReader，覆盖用户选择与导入路径。
 * 关键注意事项：上传相关测试依赖全局 FileReader 替身，结束后必须恢复全局对象。
 * 重构建议：可抽出模板上传测试工具，减少不同导入场景的重复 mock。
 */
import { defineComponent, h, nextTick } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  uploadFile: null as File | null,
  uploadResult: undefined as unknown,
  t: (key: string, params?: Record<string, unknown>) => {
    return params ? `${key}:${JSON.stringify(params)}` : key
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: hoisted.t }),
  createI18n: () => ({ global: { t: hoisted.t, locale: { value: 'en-US' } } })
}))

vi.mock('@/core/interaction-system/components/InteractionTemplatePreview.vue', () => ({
  default: defineComponent({
    props: ['template'],
    emits: ['close'],
    setup(props, { emit }) {
      return () =>
        h('div', { class: 'template-preview-stub' }, [
          h('span', props.template?.name || ''),
          h(
            'button',
            {
              type: 'button',
              onClick: () => emit('close')
            },
            'close preview'
          )
        ])
    }
  })
}))

vi.mock('naive-ui', () => ({
  createDiscreteApi: () => ({
    message: {
      success: hoisted.messageSuccess,
      error: hoisted.messageError
    },
    notification: {},
    dialog: {},
    loadingBar: {}
  }),
  useMessage: () => ({
    success: hoisted.messageSuccess,
    error: hoisted.messageError
  }),
  NTabs: defineComponent({
    props: ['value', 'type', 'size'],
    emits: ['update:value'],
    setup(props, { emit, slots }) {
      return () =>
        h('div', { class: 'n-tabs-stub', 'data-active': props.value }, [
          h(
            'button',
            {
              class: 'tab-user-trigger',
              type: 'button',
              onClick: () => emit('update:value', 'user')
            },
            'user tab'
          ),
          slots.default?.()
        ])
    }
  }),
  NTabPane: defineComponent({
    props: ['name', 'tab'],
    setup(props, { slots }) {
      return () =>
        h('section', { class: 'n-tab-pane-stub', 'data-name': props.name }, [
          h('span', { class: 'tab-label-stub' }, props.tab),
          slots.default?.()
        ])
    }
  }),
  NCard: defineComponent({
    props: ['size', 'hoverable', 'bordered'],
    emits: ['click'],
    setup(_, { emit, slots }) {
      return () =>
        h(
          'article',
          {
            class: 'n-card-stub template-card',
            onClick: () => emit('click')
          },
          [slots.header?.(), slots.default?.(), slots.action?.()]
        )
    }
  }),
  NIcon: defineComponent({
    props: ['color', 'class'],
    setup(props, { slots }) {
      return () => h('i', { class: ['n-icon-stub', props.class], style: { color: props.color } }, slots.default?.())
    }
  }),
  NTag: defineComponent({
    props: ['type', 'size', 'round'],
    setup(props, { slots }) {
      return () => h('span', { class: 'n-tag-stub', 'data-type': props.type }, slots.default?.())
    }
  }),
  NText: defineComponent({
    props: ['depth'],
    setup(_, { slots }) {
      return () => h('span', { class: 'n-text-stub' }, slots.default?.())
    }
  }),
  NSpace: defineComponent({
    props: ['vertical', 'size', 'justify'],
    setup(_, { slots }) {
      return () => h('div', { class: 'n-space-stub' }, slots.default?.())
    }
  }),
  NButton: defineComponent({
    props: {
      disabled: Boolean,
      dashed: Boolean,
      size: String,
      type: String,
      quaternary: Boolean
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
            onClick: (event: MouseEvent) => {
              if (!props.disabled) emit('click', event)
            }
          },
          [slots.icon?.(), slots.default?.()]
        )
    }
  }),
  NDivider: defineComponent({
    props: ['dashed'],
    setup(_, { slots }) {
      return () => h('hr', { class: 'n-divider-stub' }, slots.default?.())
    }
  }),
  NUpload: defineComponent({
    props: ['showFileList', 'accept', 'beforeUpload'],
    setup(props, { slots }) {
      return () =>
        h('div', { class: 'n-upload-stub' }, [
          h(
            'button',
            {
              class: 'upload-json-trigger',
              type: 'button',
              onClick: () => {
                hoisted.uploadResult = props.beforeUpload?.({ file: { file: hoisted.uploadFile } })
              }
            },
            'upload json'
          ),
          slots.default?.()
        ])
    }
  }),
  NInput: defineComponent({
    props: ['value', 'type', 'rows', 'placeholder', 'size'],
    emits: ['update:value'],
    setup(props, { emit }) {
      return () =>
        h('textarea', {
          class: 'n-input-stub',
          value: props.value,
          placeholder: props.placeholder,
          rows: props.rows,
          onInput: (event: Event) => emit('update:value', (event.target as HTMLTextAreaElement).value)
        })
    }
  }),
  NModal: defineComponent({
    props: {
      show: Boolean,
      title: String
    },
    emits: ['update:show'],
    setup(props, { slots }) {
      return () =>
        props.show ? h('div', { class: 'n-modal-stub', 'data-title': props.title }, slots.default?.()) : null
    }
  })
}))

vi.mock('@vicons/ionicons5', () => ({
  CloudUploadOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-upload' }) }),
  ColorPaletteOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-palette' }) }),
  EyeOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-eye' }) }),
  FlashOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-flash' }) }),
  HeartOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-heart' }) }),
  PlayOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-play' }) }),
  SettingsOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-settings' }) }),
  StarOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-star' }) })
}))

import InteractionTemplateSelector from './InteractionTemplateSelector.vue'

const mountedWrappers: VueWrapper[] = []

const mountSelector = () => {
  const wrapper = mount(InteractionTemplateSelector, {
    attachTo: document.body
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const buttonByText = (wrapper: VueWrapper, text: string) => {
  const button = wrapper.findAll('button').find(item => item.text().includes(text))
  if (!button) throw new Error(`Button not found: ${text}`)
  return button
}

const importJson = async (wrapper: VueWrapper, value: string) => {
  const textarea = wrapper.get<HTMLTextAreaElement>('textarea')
  await textarea.setValue(value)
  await nextTick()
}

describe('InteractionTemplateSelector.vue', () => {
  let originalFileReader: typeof FileReader | undefined

  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T04:00:00.000Z'))
    vi.clearAllMocks()
    localStorage.clear()
    document.body.innerHTML = ''
    hoisted.uploadFile = null
    hoisted.uploadResult = undefined
    originalFileReader = globalThis.FileReader
    globalThis.FileReader = class {
      onload: ((event: ProgressEvent<FileReader>) => void) | null = null

      readAsText(file: File) {
        this.onload?.({
          target: {
            result: (file as any).__text || ''
          }
        } as ProgressEvent<FileReader>)
      }
    } as unknown as typeof FileReader
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    document.body.innerHTML = ''
    globalThis.FileReader = originalFileReader as typeof FileReader
    vi.useRealTimers()
  })

  it('renders all predefined categories, templates, events, action counts, and tag types', async () => {
    const wrapper = mountSelector()
    await nextTick()

    expect(wrapper.text()).toContain('interaction.template.basic')
    expect(wrapper.text()).toContain('interaction.template.visual')
    expect(wrapper.text()).toContain('interaction.template.animation')
    expect(wrapper.text()).toContain('interaction.template.complex')
    expect(wrapper.text()).toContain('interaction.template.user')
    expect(wrapper.text()).toContain('interaction.template.predefined.clickHighlight')
    expect(wrapper.text()).toContain('interaction.template.predefined.rainbowBorder')
    expect(wrapper.text()).toContain('interaction.template.predefined.pulseAnimation')
    expect(wrapper.text()).toContain('interaction.template.predefined.completeFeedback')
    expect(wrapper.text()).toContain('3 interaction.template.actions')
    expect(wrapper.findAll('.n-tag-stub').map(item => item.attributes('data-type'))).toEqual(
      expect.arrayContaining(['success', 'info', 'warning', 'default'])
    )
  })

  it('selects predefined templates from the card and from the select button without exposing source references', async () => {
    const wrapper = mountSelector()
    await nextTick()

    await wrapper.findAll('.template-card')[0].trigger('click')
    const firstSelection = wrapper.emitted('select')?.[0]?.[0] as Record<string, unknown>

    expect(firstSelection).toMatchObject({
      event: 'click',
      enabled: true,
      priority: 1,
      name: 'interaction.template.predefined.clickHighlightEffect'
    })

    firstSelection.enabled = false
    await buttonByText(wrapper, 'interaction.template.select').trigger('click')
    const secondSelection = wrapper.emitted('select')?.[1]?.[0] as Record<string, unknown>
    expect(secondSelection.enabled).toBe(true)
    expect(secondSelection).not.toBe(firstSelection)
  })

  it('emits one selected config for every config inside a complex template', async () => {
    const wrapper = mountSelector()
    await nextTick()

    const complexCard = wrapper
      .findAll('.template-card')
      .find(card => card.text().includes('interaction.template.predefined.completeFeedback'))
    if (!complexCard) throw new Error('Complex template card not found')

    await complexCard.trigger('click')

    const selectedEvents = wrapper.emitted('select')?.map(eventArgs => (eventArgs[0] as any).event)
    expect(selectedEvents).toEqual(['hover', 'click', 'focus'])
  })

  it('opens template preview dialog and closes it from the preview child', async () => {
    const wrapper = mountSelector()
    await nextTick()

    await buttonByText(wrapper, 'interaction.template.preview').trigger('click')
    await nextTick()

    expect(wrapper.get('.n-modal-stub').attributes('data-title')).toContain(
      'interaction.template.predefined.clickHighlight'
    )
    expect(wrapper.get('.template-preview-stub').text()).toContain('interaction.template.predefined.clickHighlight')

    await buttonByText(wrapper, 'close preview').trigger('click')
    await nextTick()
    expect(wrapper.findAll('.template-preview-stub')).toHaveLength(0)
  })

  it('imports a valid custom template, switches to the user category, persists it, and allows selection', async () => {
    const wrapper = mountSelector()
    await nextTick()

    await importJson(
      wrapper,
      JSON.stringify({
        name: 'User template',
        config: [
          {
            event: 'custom',
            enabled: true,
            priority: 4,
            name: 'User custom config',
            responses: [{ action: 'changeContent', value: 'custom content' }]
          }
        ],
        tags: ['user']
      })
    )
    await buttonByText(wrapper, 'interaction.template.importConfig').trigger('click')
    await nextTick()

    expect(hoisted.messageSuccess).toHaveBeenCalledWith('interaction.messages.templateImported')
    expect(wrapper.get('.n-tabs-stub').attributes('data-active')).toBe('user')
    expect(wrapper.text()).toContain('User template')
    expect(wrapper.get('textarea').element.value).toBe('')
    expect(JSON.parse(localStorage.getItem('interaction-user-templates') || '[]')).toEqual([
      expect.objectContaining({
        id: 'user-1782532800000',
        name: 'User template',
        category: 'user',
        tags: ['user']
      })
    ])

    const userCard = wrapper.findAll('.template-card').find(card => card.text().includes('User template'))
    if (!userCard) throw new Error('Imported user template card not found')
    await userCard.trigger('click')
    expect(wrapper.emitted('select')?.at(-1)?.[0]).toMatchObject({
      event: 'custom',
      name: 'User custom config'
    })
  })

  it('uses a default custom-template description and empty tags when optional fields are absent', async () => {
    const wrapper = mountSelector()
    await nextTick()

    await importJson(
      wrapper,
      JSON.stringify({
        name: 'Minimal user template',
        config: [{ event: 'click', enabled: true, responses: [] }]
      })
    )
    await buttonByText(wrapper, 'interaction.template.importConfig').trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain('interaction.template.predefined.userCustomTemplate')
    expect(JSON.parse(localStorage.getItem('interaction-user-templates') || '[]')[0]).toMatchObject({
      description: 'interaction.template.predefined.userCustomTemplate',
      tags: []
    })
  })

  it('rejects malformed custom-template JSON and invalid template shapes', async () => {
    const wrapper = mountSelector()
    await nextTick()

    await importJson(wrapper, '{bad json')
    await buttonByText(wrapper, 'interaction.template.importConfig').trigger('click')

    await importJson(wrapper, JSON.stringify({ name: 'Missing config' }))
    await buttonByText(wrapper, 'interaction.template.importConfig').trigger('click')

    expect(hoisted.messageError).toHaveBeenCalledTimes(2)
    expect(hoisted.messageError).toHaveBeenNthCalledWith(1, 'interaction.messages.templateFormatError')
    expect(hoisted.messageError).toHaveBeenNthCalledWith(2, 'interaction.messages.templateFormatError')
  })

  it('clears the custom JSON input and disables import for blank content', async () => {
    const wrapper = mountSelector()
    await nextTick()

    expect(buttonByText(wrapper, 'interaction.template.importConfig').attributes('disabled')).toBe('')

    await importJson(wrapper, '{"name":"draft","config":[]}')
    expect(buttonByText(wrapper, 'interaction.template.importConfig').attributes('disabled')).toBeUndefined()

    await buttonByText(wrapper, 'interaction.template.clearInput').trigger('click')
    await nextTick()
    expect(wrapper.get('textarea').element.value).toBe('')
    expect(buttonByText(wrapper, 'interaction.template.importConfig').attributes('disabled')).toBe('')
  })

  it('loads saved user templates and tolerates corrupted persisted data', async () => {
    localStorage.setItem(
      'interaction-user-templates',
      JSON.stringify([
        {
          id: 'saved-template',
          name: 'Saved template',
          description: 'Saved description',
          category: 'user',
          color: '#666',
          config: [{ event: 'custom', enabled: true, responses: [{ action: 'changeContent', value: 'saved' }] }]
        }
      ])
    )
    const wrapper = mountSelector()
    await nextTick()

    expect(wrapper.text()).toContain('Saved template')

    wrapper.unmount()
    mountedWrappers.pop()
    localStorage.setItem('interaction-user-templates', '{bad json')

    const corruptedWrapper = mountSelector()
    await nextTick()
    expect(corruptedWrapper.get('.template-card').text()).toContain('interaction.template.predefined.clickHighlight')
    expect(corruptedWrapper.text()).not.toContain('Saved template')
    expect(corruptedWrapper.text()).toContain('interaction.template.predefined.clickHighlight')
    expect(corruptedWrapper.get('.n-tabs-stub').attributes('data-active')).toBe('basic')
  })

  it('loads custom-template JSON from an uploaded file and returns false to prevent default upload', async () => {
    hoisted.uploadFile = {
      __text: JSON.stringify({ name: 'Uploaded template', config: [{ event: 'click', enabled: true, responses: [] }] })
    } as unknown as File
    const wrapper = mountSelector()
    await nextTick()

    await wrapper.get('.upload-json-trigger').trigger('click')
    await vi.runAllTimersAsync()
    await nextTick()

    expect(hoisted.uploadResult).toBe(false)
    expect(wrapper.get('textarea').element.value).toContain('Uploaded template')
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('interaction.messages.templateFileLoaded')
  })
})
