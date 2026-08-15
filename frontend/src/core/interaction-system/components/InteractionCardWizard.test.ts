/**
 * 文件用途：验证交互卡片向导的渲染、表单联动和配置同步行为。
 * 核心逻辑：通过 Vue Test Utils 挂载组件，模拟用户操作并断言 emit、消息和状态变化。
 * 关键注意事项：测试桩需要覆盖编辑器状态、配置桥接和国际化文本，避免依赖真实页面环境。
 * 重构建议：可沉淀通用 Naive UI 与编辑器 store mock，减少交互组件测试重复代码。
 */
import { defineComponent, h, nextTick } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  editorStore: {
    nodes: [] as any[]
  },
  getConfiguration: vi.fn(),
  fetchGetUserRoutes: vi.fn(),
  messageError: vi.fn(),
  t: (key: string) => key
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: hoisted.t }),
  createI18n: () => ({ global: { t: hoisted.t, locale: { value: 'en-US' } } })
}))

vi.mock('@/store/modules/editor', () => ({
  useEditorStore: () => hoisted.editorStore
}))

vi.mock('@/components/visual-editor/configuration/ConfigurationIntegrationBridge', () => ({
  configurationIntegrationBridge: {
    getConfiguration: hoisted.getConfiguration
  }
}))

vi.mock('@/service/api/route', () => ({
  fetchGetUserRoutes: hoisted.fetchGetUserRoutes
}))

vi.mock('../interaction-engine', () => ({
  createInteractionEngine: vi.fn(() => ({
    start: vi.fn(),
    stop: vi.fn()
  }))
}))

vi.mock('@/core/data-architecture/components/common/ComponentPropertySelector.vue', () => ({
  default: defineComponent({
    props: ['value', 'placeholder', 'currentComponentId'],
    emits: ['update:value', 'change'],
    setup(_, { emit }) {
      return () =>
        h(
          'button',
          {
            class: 'component-property-selector-stub',
            type: 'button',
            onClick: () => {
              emit('update:value', 'target-card.component.styles.backgroundColor')
              emit('change', 'target-card.component.styles.backgroundColor', {
                componentId: 'target-card',
                layer: 'component',
                propertyName: 'styles.backgroundColor'
              })
            }
          },
          'pick property'
        )
    }
  })
}))

vi.mock('naive-ui', () => ({
  useMessage: () => ({
    error: hoisted.messageError
  }),
  NSpace: defineComponent({
    props: ['justify', 'vertical', 'size'],
    setup(_, { slots }) {
      return () => h('div', { class: 'n-space-stub' }, slots.default?.())
    }
  }),
  NButton: defineComponent({
    props: {
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
            type: 'button',
            onClick: () => emit('click')
          },
          [slots.icon?.(), slots.default?.()]
        )
    }
  }),
  NIcon: defineComponent({
    setup(_, { slots }) {
      return () => h('i', { class: 'n-icon-stub' }, slots.default?.())
    }
  }),
  NInput: defineComponent({
    props: ['value', 'placeholder'],
    emits: ['update:value'],
    setup(props, { emit }) {
      return () =>
        h('input', {
          class: 'n-input-stub',
          placeholder: props.placeholder,
          value: props.value,
          onInput: (event: Event) => emit('update:value', (event.target as HTMLInputElement).value)
        })
    }
  }),
  NSelect: defineComponent({
    props: ['value', 'options', 'placeholder', 'loading', 'filterable'],
    emits: ['update:value'],
    setup(props, { emit }) {
      const flatten = (options: any[] = []): any[] => {
        return options.flatMap((option) => (option.children ? flatten(option.children) : option))
      }
      return () =>
        h(
          'select',
          {
            class: 'n-select-stub',
            'data-placeholder': props.placeholder,
            value: props.value || '',
            onChange: (event: Event) => emit('update:value', (event.target as HTMLSelectElement).value)
          },
          [
            h('option', { value: '' }, props.placeholder || ''),
            ...flatten(props.options || []).map((option: any) =>
              h('option', { value: option.value }, option.label || option.value)
            )
          ]
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
  }),
  NRadioGroup: defineComponent({
    props: ['value'],
    emits: ['update:value'],
    setup(_, { emit, slots }) {
      return () =>
        h(
          'div',
          {
            class: 'n-radio-group-stub',
            onClick: (event: MouseEvent) => {
              const target = event.target as HTMLElement
              const option = target.closest<HTMLElement>('[data-radio-value]')
              const value = option?.getAttribute('data-radio-value')
              if (value) emit('update:value', value)
            },
            ref: (element: Element | null) => {
              if (element) {
                ;(element as HTMLElement & { __radioUpdate?: (value: string) => void }).__radioUpdate = (value) =>
                  emit('update:value', value)
              }
            }
          },
          slots.default?.()
        )
    }
  }),
  NRadio: defineComponent({
    props: ['value'],
    setup(props, { slots }) {
      return () =>
        h(
          'button',
          {
            class: `n-radio-stub radio-${props.value}`,
            'data-radio-value': props.value,
            type: 'button'
          },
          slots.default?.() || props.value
        )
    }
  }),
  NModal: defineComponent({
    props: ['show', 'title'],
    emits: ['update:show'],
    setup(props, { slots }) {
      return () =>
        props.show ? h('div', { class: 'n-modal-stub', 'data-title': props.title }, slots.default?.()) : null
    }
  }),
  NCard: defineComponent({
    props: ['bordered'],
    setup(_, { slots }) {
      return () => h('section', { class: 'n-card-stub' }, [slots.default?.(), slots.footer?.()])
    }
  }),
  NForm: defineComponent({
    props: ['model', 'labelPlacement', 'labelWidth'],
    setup(_, { slots }) {
      return () => h('form', { class: 'n-form-stub' }, slots.default?.())
    }
  }),
  NFormItem: defineComponent({
    props: ['label'],
    setup(props, { slots }) {
      return () => h('div', { class: 'n-form-item-stub' }, [h('span', props.label), slots.default?.()])
    }
  })
}))

vi.mock('@vicons/ionicons5', () => ({
  FlashOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-flash' }) }),
  TrashOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-trash' }) })
}))

import InteractionCardWizard from './InteractionCardWizard.vue'

const mountedWrappers: VueWrapper[] = []

const defaultRoutes = [
  {
    path: '/dashboard',
    meta: { title: 'Dashboard' },
    children: [
      { path: '/dashboard/device', meta: { title: 'Device' } },
      { path: '/dashboard/hidden', meta: { title: 'Hidden', hideInMenu: true } }
    ]
  }
]

const mountWizard = (props: Record<string, unknown> = {}) => {
  const wrapper = mount(InteractionCardWizard, {
    props: {
      modelValue: [],
      componentId: 'current-card',
      componentType: 'rdi-card',
      ...props
    },
    attachTo: document.body
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const buttonByText = (wrapper: VueWrapper, text: string) => {
  const button = wrapper.findAll('button').find((item) => item.text().includes(text))
  if (!button) throw new Error(`Button not found: ${text}`)
  return button
}

const editButtonAt = (wrapper: VueWrapper, index: number) => {
  const buttons = wrapper.findAll('button').filter((item) => item.text().includes('interaction.edit'))
  const button = buttons[index]
  if (!button) throw new Error(`Edit button not found at index ${index}`)
  return button
}

const selectByPlaceholder = (wrapper: VueWrapper, placeholder: string) => {
  return wrapper.get<HTMLSelectElement>(`select[data-placeholder="${placeholder}"]`)
}

const inputByPlaceholder = (wrapper: VueWrapper, placeholder: string) => {
  return wrapper.get<HTMLInputElement>(`input[placeholder="${placeholder}"]`)
}

const openAddModal = async (wrapper: VueWrapper) => {
  await buttonByText(wrapper, 'interaction.wizard.addInteraction').trigger('click')
  await nextTick()
}

const confirm = async (wrapper: VueWrapper) => {
  await buttonByText(wrapper, 'interaction.confirm').trigger('click')
  await flushPromises()
  await nextTick()
}

const ensureInternalMenuVisible = async (wrapper: VueWrapper) => {
  await wrapper.get('.radio-internal').trigger('click')
  await flushPromises()
  await nextTick()
  const menuSelect = selectByPlaceholder(wrapper, 'interaction.placeholders.selectMenuToJump')
  expect(menuSelect.findAll('option').map(option => option.attributes('value'))).toEqual([
    '',
    '/dashboard',
    '/dashboard/device'
  ])
}

const lastModelUpdate = (wrapper: VueWrapper) => {
  return wrapper.emitted('update:modelValue')?.at(-1)?.[0] as any[]
}

describe('InteractionCardWizard.vue', () => {
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>

  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
    hoisted.editorStore.nodes = [
      {
        id: 'current-card',
        type: 'rdi-card',
        metadata: {
          exposedProperties: {
            publicTitle: 'Current title',
            card2Definition: {
              interactionCapabilities: {
                watchableProperties: {
                  title: { label: 'Title', type: 'string', description: 'Card title' }
                }
              }
            }
          }
        }
      },
      {
        id: 'target-card',
        type: 'chart-card',
        metadata: {
          exposedProperties: {
            publicTitle: 'Target title'
          },
          card2Definition: {
            interactionCapabilities: {
              watchableProperties: {
                backgroundColor: { label: 'Background', type: 'string', description: 'Background color' }
              }
            }
          }
        }
      }
    ]
    hoisted.getConfiguration.mockReturnValue({
      base: {
        title: 'Runtime title',
        deviceId: 'device-1',
        metricsList: ['temperature']
      },
      component: {
        properties: {},
        styles: {}
      }
    })
    hoisted.fetchGetUserRoutes.mockResolvedValue({
      data: {
        list: defaultRoutes
      }
    })
    consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    consoleErrorSpy.mockRestore()
    document.body.innerHTML = ''
  })

  it('renders empty state and synchronizes external modelValue updates into summaries', async () => {
    const wrapper = mountWizard()

    expect(wrapper.text()).toContain('interaction.wizard.noInteractions')

    await wrapper.setProps({
      modelValue: [
        {
          event: 'click',
          enabled: true,
          priority: 1,
          responses: [{ action: 'jump', value: 'https://example.com' }]
        },
        {
          event: 'dataChange',
          enabled: true,
          priority: 2,
          watchedProperty: 'base.temperature',
          condition: { type: 'comparison', operator: 'greaterThan', value: '80' },
          responses: [{ action: 'modify', targetComponentId: 'target-card', targetProperty: 'component.color' }]
        },
        {
          event: 'hover',
          enabled: false,
          priority: 3,
          responses: [{ action: 'customAction' }]
        }
      ]
    })
    await nextTick()

    expect(wrapper.findAll('.interaction-item')).toHaveLength(3)
    expect(wrapper.findAll('.summary-badge').map((item) => item.classes())).toEqual(
      expect.arrayContaining([
        expect.arrayContaining(['click']),
        expect.arrayContaining(['condition']),
        expect.arrayContaining(['hover'])
      ])
    )
    expect(wrapper.text()).toContain('interaction.summary.pageJump')
    expect(wrapper.text()).toContain('interaction.summary.modifyProperty')
    expect(wrapper.text()).toContain('interaction.summary.customAction')
    expect(wrapper.text()).toContain('interaction.operators.greaterThan 80')
  })

  it('adds an external jump interaction with jumpConfig plus persisted compatibility fields', async () => {
    const wrapper = mountWizard()
    await openAddModal(wrapper)

    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectAction').setValue('jump')
    await inputByPlaceholder(wrapper, 'interaction.placeholders.enterUrl').setValue('https://aetherlink.example/docs')
    await wrapper.get('.radio-_self').trigger('click')
    await confirm(wrapper)

    expect(lastModelUpdate(wrapper)).toEqual([
      expect.objectContaining({
        event: 'click',
        enabled: true,
        priority: 1,
        responses: [
          expect.objectContaining({
            action: 'jump',
            value: 'https://aetherlink.example/docs',
            target: '_self',
            jumpConfig: {
              jumpType: 'external',
              target: '_self',
              url: 'https://aetherlink.example/docs'
            }
          })
        ]
      })
    ])
    expect(wrapper.findAll('.n-modal-stub')).toHaveLength(0)
  })

  it('adds an internal menu jump, flattens nested route options, and stores the selected internal path', async () => {
    const wrapper = mountWizard()
    await openAddModal(wrapper)

    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectAction').setValue('jump')
    await ensureInternalMenuVisible(wrapper)
    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectMenuToJump').setValue('/dashboard/device')
    await wrapper.get('.radio-_blank').trigger('click')
    await confirm(wrapper)

    expect(hoisted.fetchGetUserRoutes).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).not.toContain('/dashboard/hidden')
    expect(lastModelUpdate(wrapper)[0].responses[0]).toMatchObject({
      action: 'jump',
      value: '/dashboard/device',
      target: '_blank',
      jumpConfig: {
        jumpType: 'internal',
        target: '_blank',
        internalPath: '/dashboard/device'
      }
    })
  })

  it('adds a modify interaction with modifyConfig plus persisted compatibility fields', async () => {
    const wrapper = mountWizard()
    await openAddModal(wrapper)

    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectAction').setValue('modify')
    await wrapper.get('.component-property-selector-stub').trigger('click')
    await inputByPlaceholder(wrapper, 'interaction.placeholders.enterNewPropertyValue').setValue('#00ff00')
    await confirm(wrapper)

    expect(lastModelUpdate(wrapper)[0].responses[0]).toMatchObject({
      action: 'modify',
      targetComponentId: 'target-card',
      targetProperty: 'component.styles.backgroundColor',
      updateValue: '#00ff00',
      modifyConfig: {
        targetComponentId: 'target-card',
        targetProperty: 'component.styles.backgroundColor',
        updateValue: '#00ff00',
        updateMode: 'replace',
        bindingPath: 'target-card.component.styles.backgroundColor'
      }
    })
  })

  it('adds dataChange interactions with comparison, range, and expression conditions', async () => {
    const wrapper = mountWizard()

    await openAddModal(wrapper)
    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectTriggerCondition').setValue('dataChange')
    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectWatchedProperty').setValue('base.deviceId')
    await selectByPlaceholder(wrapper, 'interaction.placeholders.conditionType').setValue('comparison')
    await selectByPlaceholder(wrapper, 'interaction.placeholders.comparison').setValue('greaterThan')
    await inputByPlaceholder(wrapper, 'interaction.placeholders.value').setValue('80')
    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectAction').setValue('jump')
    await inputByPlaceholder(wrapper, 'interaction.placeholders.enterUrl').setValue('https://alarm.example')
    await confirm(wrapper)

    await openAddModal(wrapper)
    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectTriggerCondition').setValue('dataChange')
    await selectByPlaceholder(wrapper, 'interaction.placeholders.conditionType').setValue('range')
    await inputByPlaceholder(wrapper, 'interaction.placeholders.rangeValue').setValue('10-20')
    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectAction').setValue('modify')
    await wrapper.get('.component-property-selector-stub').trigger('click')
    await confirm(wrapper)

    await openAddModal(wrapper)
    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectTriggerCondition').setValue('dataChange')
    await selectByPlaceholder(wrapper, 'interaction.placeholders.conditionType').setValue('expression')
    await inputByPlaceholder(wrapper, 'interaction.placeholders.expressionValue').setValue('value !== null')
    await confirm(wrapper)

    const updates = lastModelUpdate(wrapper)
    expect(updates[0]).toMatchObject({
      event: 'dataChange',
      watchedProperty: 'base.deviceId',
      condition: {
        type: 'comparison',
        operator: 'greaterThan',
        value: '80'
      }
    })
    expect(updates[1]).toMatchObject({
      event: 'dataChange',
      condition: {
        type: 'range',
        value: '10-20'
      }
    })
    expect(updates[2]).toMatchObject({
      event: 'dataChange',
      condition: {
        type: 'expression',
        value: 'value !== null'
      },
      responses: []
    })
  })

  it('edits new-format internal jump interactions and preserves loaded menu plus compatibility fields', async () => {
    const wrapper = mountWizard({
      modelValue: [
        {
          event: 'click',
          enabled: true,
          priority: 1,
          responses: [
            {
              action: 'jump',
              jumpConfig: {
                jumpType: 'internal',
                target: '_self',
                internalPath: '/dashboard'
              },
              value: '/dashboard',
              target: '_self'
            }
          ]
        }
      ]
    })

    await editButtonAt(wrapper, 0).trigger('click')
    await flushPromises()
    await nextTick()
    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectMenuToJump').setValue('/dashboard/device')
    await confirm(wrapper)

    expect(hoisted.fetchGetUserRoutes).toHaveBeenCalledTimes(1)
    expect(lastModelUpdate(wrapper)).toHaveLength(1)
    expect(lastModelUpdate(wrapper)[0].responses[0]).toMatchObject({
      value: '/dashboard/device',
      target: '_self',
      jumpConfig: {
        jumpType: 'internal',
        internalPath: '/dashboard/device'
      }
    })
  })

  it('edits older-format navigate and modify interactions into the new response formats', async () => {
    const wrapper = mountWizard({
      modelValue: [
        {
          event: 'hover',
          enabled: true,
          priority: 2,
          responses: [{ action: 'navigateToUrl', value: 'https://classic.example', target: '_blank' }]
        },
        {
          event: 'click',
          enabled: true,
          priority: 3,
          responses: [
            {
              action: 'updateComponentData',
              targetComponentId: 'target-card',
              targetProperty: 'component.styles.color',
              updateValue: '#111111'
            }
          ]
        }
      ]
    })

    await editButtonAt(wrapper, 0).trigger('click')
    await inputByPlaceholder(wrapper, 'interaction.placeholders.enterUrl').setValue('https://rewritten.example')
    await confirm(wrapper)

    expect(lastModelUpdate(wrapper)[0].responses[0]).toMatchObject({
      action: 'jump',
      value: 'https://rewritten.example',
      jumpConfig: {
        jumpType: 'external',
        url: 'https://rewritten.example'
      }
    })

    await editButtonAt(wrapper, 1).trigger('click')
    await wrapper.get('.component-property-selector-stub').trigger('click')
    await inputByPlaceholder(wrapper, 'interaction.placeholders.enterNewPropertyValue').setValue('#222222')
    await confirm(wrapper)

    expect(lastModelUpdate(wrapper)[1].responses[0]).toMatchObject({
      action: 'modify',
      targetComponentId: 'target-card',
      targetProperty: 'component.styles.backgroundColor',
      updateValue: '#222222'
    })
  })

  it('preserves legacy modify targets when editing updateComponentData without re-selecting the property', async () => {
    const wrapper = mountWizard({
      modelValue: [
        {
          event: 'click',
          enabled: true,
          priority: 1,
          responses: [
            {
              action: 'updateComponentData',
              targetComponentId: 'target-card',
              targetProperty: 'component.styles.color',
              updateValue: '#111111'
            }
          ]
        }
      ]
    })

    await editButtonAt(wrapper, 0).trigger('click')
    await inputByPlaceholder(wrapper, 'interaction.placeholders.enterNewPropertyValue').setValue('#333333')
    await confirm(wrapper)

    expect(lastModelUpdate(wrapper)[0].responses[0]).toMatchObject({
      action: 'modify',
      targetComponentId: 'target-card',
      targetProperty: 'component.styles.color',
      updateValue: '#333333',
      modifyConfig: {
        targetComponentId: 'target-card',
        targetProperty: 'component.styles.color',
        updateValue: '#333333'
      }
    })
  })

  it('toggles and deletes list entries while emitting the remaining interaction list', async () => {
    const source = [
      {
        event: 'click',
        enabled: true,
        priority: 1,
        responses: [{ action: 'jump', value: 'https://example.com' }]
      },
      {
        event: 'hover',
        enabled: false,
        priority: 2,
        responses: [{ action: 'custom' }]
      }
    ]
    const wrapper = mountWizard({ modelValue: source })

    await wrapper.get('.n-switch-stub').trigger('click')
    expect(source[0].enabled).toBe(true)
    expect(wrapper.emitted('update:modelValue')?.[0]?.[0]).toEqual([
      {
        event: 'click',
        enabled: false,
        priority: 1,
        responses: [{ action: 'jump', value: 'https://example.com' }]
      },
      source[1]
    ])

    const deleteButtons = wrapper.findAll('button').filter((button) => button.html().includes('icon-trash'))
    await deleteButtons[0].trigger('click')

    expect(lastModelUpdate(wrapper)).toEqual([source[1]])
  })

  it('reports menu loading failures and abnormal menu payloads', async () => {
    hoisted.fetchGetUserRoutes.mockRejectedValueOnce(new Error('network down'))
    const wrapper = mountWizard()

    await openAddModal(wrapper)
    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectAction').setValue('jump')
    await wrapper.get('.radio-internal').trigger('click')
    await flushPromises()
    expect(hoisted.messageError).toHaveBeenCalledWith('interaction.messages.menuLoadFailed: network down')

    hoisted.fetchGetUserRoutes.mockResolvedValueOnce({ data: { list: [] } })
    await wrapper.get('.radio-external').trigger('click')
    await wrapper.get('.radio-internal').trigger('click')
    await flushPromises()
    expect(hoisted.messageError).toHaveBeenCalledWith('interaction.messages.menuDataProcessFailed')

    hoisted.fetchGetUserRoutes.mockResolvedValueOnce({ data: {} })
    await wrapper.get('.radio-external').trigger('click')
    await wrapper.get('.radio-internal').trigger('click')
    await flushPromises()
    expect(hoisted.messageError).toHaveBeenCalledWith('interaction.messages.menuDataAbnormal')
  })

  it('falls back safely when componentId or configuration data is unavailable', async () => {
    hoisted.getConfiguration.mockReturnValueOnce(null)
    const wrapper = mountWizard({ componentId: '' })

    await openAddModal(wrapper)
    await selectByPlaceholder(wrapper, 'interaction.placeholders.selectTriggerCondition').setValue('dataChange')

    expect(wrapper.text()).toContain('interaction.placeholders.selectWatchedProperty')
    expect(consoleErrorSpy).toHaveBeenCalledTimes(1)
  })
})
