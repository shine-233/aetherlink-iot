/**
 * 文件用途: Dynamic Parameter Editor 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { defineComponent, h, inject, nextTick, provide, watch } from 'vue'
import { mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  loggerDebug: vi.fn(),
  loggerWarn: vi.fn(),
  loggerError: vi.fn(),
  generateVariableName: vi.fn((key: string) => `var_${key}`),
  getGroup: vi.fn(),
  getGroupParameters: vi.fn(),
  removeGroup: vi.fn(),
  t: (key: string) => key
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: hoisted.t }),
  createI18n: () => ({ global: { t: hoisted.t, locale: { value: 'en-US' } } })
}))

vi.mock('@/utils/logger', () => ({
  createLogger: () => ({
    debug: hoisted.loggerDebug,
    warn: hoisted.loggerWarn,
    error: hoisted.loggerError,
    info: vi.fn()
  })
}))

vi.mock('@/core/data-architecture/types/http-config', () => ({
  generateVariableName: hoisted.generateVariableName
}))

vi.mock('@/core/data-architecture/utils/device-parameter-generator', () => ({
  globalParameterGroupManager: {
    getGroup: hoisted.getGroup,
    getGroupParameters: hoisted.getGroupParameters,
    removeGroup: hoisted.removeGroup
  }
}))

vi.mock('@/core/data-architecture/components/common/templates/index', () => {
  const ParameterTemplateType = {
    MANUAL: 'manual',
    DROPDOWN: 'dropdown',
    PROPERTY: 'property',
    COMPONENT: 'component'
  }
  const templates = [
    { id: 'manual', name: 'Manual', type: ParameterTemplateType.MANUAL, defaultValue: '' },
    {
      id: 'content-types',
      name: 'Content Type',
      type: ParameterTemplateType.DROPDOWN,
      defaultValue: 'application/json',
      allowCustom: true,
      options: [{ label: 'json', value: 'application/json' }]
    },
    {
      id: 'component-property-binding',
      name: 'Component Property',
      type: ParameterTemplateType.COMPONENT,
      defaultValue: '',
      componentConfig: {
        component: 'ComponentPropertySelector',
        props: { placeholder: 'pick property' }
      }
    },
    {
      id: 'device-metrics-selector',
      name: 'Device Metrics',
      type: ParameterTemplateType.COMPONENT,
      defaultValue: '',
      componentConfig: {
        component: 'DeviceMetricsSelector',
        props: { mode: 'single' }
      }
    }
  ]
  return {
    ParameterTemplateType,
    getRecommendedTemplates: vi.fn(() => [templates[0], templates[2], templates[3]]),
    getTemplateById: vi.fn((id: string) => templates.find(template => template.id === id))
  }
})

function passthrough(className: string) {
  return defineComponent({
    props: ['value', 'modelValue', 'show', 'visible'],
    emits: [
      'click',
      'input',
      'update:value',
      'update:show',
      'update:visible',
      'change',
      'confirm',
      'cancel',
      'parameters-generated',
      'parameters-selected',
      'parameters-updated'
    ],
    setup(props, { emit, slots }) {
      return () =>
        h(
          'div',
          {
            class: className,
            onClick: () => emit('click')
          },
          [slots.header?.(), slots.default?.(), slots.action?.(), slots.footer?.()]
        )
    }
  })
}

const radioGroupContextKey = Symbol('radio-group-context')

vi.mock('naive-ui', () => ({
  useMessage: () => ({
    success: hoisted.messageSuccess,
    error: hoisted.messageError
  }),
  NButton: defineComponent({
    props: ['disabled', 'type', 'size', 'text', 'quaternary', 'circle'],
    emits: ['click'],
    setup(props, { emit, slots }) {
      return () =>
        h(
          'button',
          {
            class: ['n-button-stub', props.type ? `button-${props.type}` : ''],
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
  NCheckbox: defineComponent({
    props: ['checked'],
    emits: ['update:checked'],
    setup(props, { emit }) {
      return () => h('button', { class: 'n-checkbox-stub', type: 'button', onClick: () => emit('update:checked', !props.checked) })
    }
  }),
  NInput: defineComponent({
    props: ['value', 'placeholder', 'readonly'],
    emits: ['input', 'update:value', 'blur'],
    setup(props, { emit }) {
      return () =>
        h('input', {
          class: 'n-input-stub',
          value: props.value,
          placeholder: props.placeholder,
          readonly: props.readonly,
          onInput: (event: Event) => {
            const value = (event.target as HTMLInputElement).value
            emit('input', value)
            emit('update:value', value)
          },
          onBlur: () => emit('blur')
        })
    }
  }),
  NSelect: defineComponent({
    props: ['value', 'options'],
    emits: ['update:value'],
    setup(props, { emit }) {
      return () =>
        h(
          'select',
          {
            class: 'n-select-stub',
            value: props.value,
            onChange: (event: Event) => emit('update:value', (event.target as HTMLSelectElement).value)
          },
          (props.options || []).map((option: any) => h('option', { value: option.value }, option.label))
        )
    }
  }),
  NSpace: passthrough('n-space-stub'),
  NTag: passthrough('n-tag-stub'),
  NText: passthrough('n-text-stub'),
  NDrawer: defineComponent({
    props: ['show', 'onAfterLeave'],
    emits: ['update:show'],
    setup(props, { slots }) {
      watch(
        () => props.show,
        (show, previousShow) => {
          if (previousShow && !show) {
            props.onAfterLeave?.()
          }
        }
      )
      return () => (props.show ? h('aside', { class: 'n-drawer-stub' }, slots.default?.()) : null)
    }
  }),
  NDrawerContent: passthrough('n-drawer-content-stub'),
  NIcon: passthrough('n-icon-stub'),
  NDropdown: passthrough('n-dropdown-stub'),
  NAlert: passthrough('n-alert-stub'),
  NCard: passthrough('n-card-stub'),
  NForm: passthrough('n-form-stub'),
  NFormItem: passthrough('n-form-item-stub'),
  NRadioGroup: defineComponent({
    props: ['value'],
    emits: ['update:value'],
    setup(props, { emit, slots }) {
      provide(radioGroupContextKey, {
        getValue: () => props.value,
        updateValue: (value: string) => emit('update:value', value)
      })
      return () => h('div', { class: 'n-radio-group-stub' }, slots.default?.())
    }
  }),
  NRadio: defineComponent({
    props: ['value'],
    setup(props, { slots }) {
      const group = inject<{ getValue: () => string; updateValue: (value: string) => void } | null>(radioGroupContextKey, null)
      return () =>
        h('label', { class: 'n-radio-stub' }, [
          h('input', {
            class: 'n-radio-input-stub',
            type: 'radio',
            value: props.value,
            checked: group?.getValue() === props.value,
            onChange: () => group?.updateValue(props.value)
          }),
          slots.default?.()
        ])
    }
  }),
  NDivider: passthrough('n-divider-stub')
}))

vi.mock('@vicons/ionicons5', () => ({
  Sparkles: defineComponent({ setup: () => () => h('svg', { class: 'icon-sparkles' }) }),
  AddCircleOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-add' }) }),
  PhonePortraitOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-phone' }) }),
  CreateOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-edit' }) }),
  TrashOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-trash' }) }),
  LinkOutline: defineComponent({ setup: () => () => h('svg', { class: 'icon-link' }) })
}))

vi.mock('@/components/device-selectors/DeviceMetricsSelector.vue', () => ({
  default: defineComponent({ setup: () => () => h('div', { class: 'device-metrics-selector-stub' }) })
}))

vi.mock('@/components/device-selectors/DeviceDispatchSelector.vue', () => ({
  default: defineComponent({ setup: () => () => h('div', { class: 'device-dispatch-selector-stub' }) })
}))

vi.mock('@/core/data-architecture/components/common/ComponentPropertySelector.vue', () => ({
  default: defineComponent({
    emits: ['change'],
    setup(_, { emit }) {
      const propertyInfo = {
        componentId: 'target-card',
        componentName: 'Target Card',
        propertyName: 'styles.color',
        propertyLabel: 'Color'
      }
      return () =>
        h('div', [
          h(
            'button',
            {
              class: 'component-property-selector-stub',
              type: 'button',
              onClick: () => emit('change', 'target-card.component.styles.color', propertyInfo)
            },
            'property'
          ),
          h(
            'button',
            {
              class: 'component-property-selector-invalid-stub',
              type: 'button',
              onClick: () => emit('change', 'bad', undefined)
            },
            'invalid property'
          )
        ])
    }
  })
}))

vi.mock('@/core/data-architecture/components/common/AddParameterFromDevice.vue', () => ({
  default: defineComponent({
    emits: ['add', 'cancel'],
    setup(_, { emit }) {
      return () =>
        h('div', { class: 'add-parameter-from-device-stub' }, [
          h(
            'button',
            {
              class: 'add-parameter-from-device-confirm-stub',
              type: 'button',
              onClick: () =>
                emit('add', [
                  {
                    key: 'metric',
                    metricsId: 'metric',
                    source: {
                      deviceName: 'Pump A',
                      metricsName: 'Temperature'
                    }
                  },
                  {
                    key: 'overflow',
                    source: {
                      deviceName: 'Pump B',
                      metricsName: 'Humidity'
                    }
                  }
                ])
            },
            'confirm-add'
          ),
          h(
            'button',
            {
              class: 'add-parameter-from-device-cancel-stub',
              type: 'button',
              onClick: () => emit('cancel')
            },
            'cancel-add'
          )
        ])
    }
  })
}))

vi.mock('@/core/data-architecture/components/device-selectors/UnifiedDeviceConfigSelector.vue', () => ({
  default: defineComponent({
    emits: ['parametersGenerated'],
    setup(_, { emit }) {
      return () =>
        h(
          'button',
          {
            class: 'unified-device-config-selector-stub',
            type: 'button',
            onClick: () =>
              emit('parametersGenerated', [
                {
                  key: 'deviceId',
                  value: 'stub-device',
                  enabled: true,
                  valueMode: 'component',
                  selectedTemplate: 'device-metrics-selector',
                  dataType: 'string',
                  variableName: 'var_deviceId'
                },
                {
                  key: 'metric',
                  value: 'stub-device.temperature',
                  enabled: true,
                  valueMode: 'component',
                  selectedTemplate: 'device-metrics-selector',
                  dataType: 'string',
                  variableName: 'var_metric'
                }
              ])
          },
          'emit unified config'
        )
    }
  })
}))

vi.mock('@/core/data-architecture/components/device-selectors/DeviceParameterSelector.vue', () => ({
  default: defineComponent({
    emits: ['parameters-selected', 'parameters-updated', 'update:visible'],
    setup(_, { emit }) {
      return () =>
        h('div', { class: 'device-parameter-selector-stub' }, [
          h(
            'button',
            {
              class: 'device-parameter-selected-stub',
              type: 'button',
              onClick: () =>
                emit('parameters-selected', [
                  param({
                    key: 'selectedDevice',
                    value: 'device-2',
                    _id: 'selected-id',
                    valueMode: 'component'
                  })
                ])
            },
            'emit selected parameters'
          ),
          h(
            'button',
            {
              class: 'device-parameter-updated-stub',
              type: 'button',
              onClick: () =>
                emit('parameters-updated', {
                  groupId: 'group-1',
                  parameters: [param({ key: 'deviceId', value: 'replacement', _id: 'replacement-id' })]
                })
            },
            'emit updated parameters'
          )
        ])
    }
  })
}))

import DynamicParameterEditor from './DynamicParameterEditor.vue'
import DynamicParameterAddDrawer from './DynamicParameterAddDrawer.vue'
import DynamicParameterComponentDrawer from './DynamicParameterComponentDrawer.vue'
import { getParameterDisplayLabel } from './dynamicParameterEditorDeviceGroup'
import { getTemplateById } from './templates/index'

type Param = {
  key: string
  value: any
  enabled: boolean
  valueMode: 'manual' | 'dropdown' | 'property' | 'component'
  selectedTemplate?: string
  dataType: 'string' | 'number' | 'boolean' | 'json'
  variableName?: string
  _id?: string
  description?: string
  defaultValue?: any
  deviceContext?: any
  parameterGroup?: any
  isDynamic?: boolean
}

const mountedWrappers: VueWrapper[] = []

const param = (overrides: Partial<Param> = {}): Param => ({
  key: 'token',
  value: 'abc',
  enabled: true,
  valueMode: 'manual',
  selectedTemplate: 'manual',
  dataType: 'string',
  _id: `id-${overrides.key || 'token'}`,
  ...overrides
})

const mountEditor = (props: Record<string, unknown> = {}) => {
  const wrapper = mount(DynamicParameterEditor, {
    props: {
      modelValue: [],
      parameterType: 'query',
      currentComponentId: 'current-card',
      ...props
    },
    attachTo: document.body
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getState = (wrapper: VueWrapper) => wrapper.vm.$.setupState as Record<string, any>

const getAddDrawerState = (wrapper: VueWrapper) =>
  wrapper.findComponent(DynamicParameterAddDrawer).vm.$.setupState as Record<string, any>

const getComponentDrawerState = (wrapper: VueWrapper) =>
  wrapper.findComponent(DynamicParameterComponentDrawer).vm.$.setupState as Record<string, any>

const lastModelValue = (wrapper: VueWrapper) => wrapper.emitted('update:modelValue')?.at(-1)?.[0] as Param[]

const clickButton = async (buttons: ReturnType<VueWrapper['findAll']>, index: number) => {
  await buttons[index].trigger('click')
}

const clickHeaderAddButton = async (wrapper: VueWrapper) => {
  await clickButton(wrapper.findAll('.editor-header-enhanced .n-button-stub'), 0)
}

const clickHeaderTemplateButton = async (wrapper: VueWrapper) => {
  await clickButton(wrapper.findAll('.editor-header-enhanced .n-button-stub'), 1)
}

const clickHeaderButtons = async (wrapper: VueWrapper, index: number) => {
  await clickButton(wrapper.findAll('.editor-header-enhanced .n-button-stub'), index)
}

const getDrawerInputs = (wrapper: VueWrapper) => wrapper.findAll('.n-drawer-stub .n-input-stub')

const getDrawerButtons = (wrapper: VueWrapper) => wrapper.findAll('.n-drawer-stub .n-button-stub')

const selectDrawerConfigType = async (wrapper: VueWrapper, value: 'manual' | 'property' | 'device') => {
  await wrapper.get(`.n-drawer-stub .n-radio-input-stub[value="${value}"]`).setValue(true)
}

const setDrawerKey = async (wrapper: VueWrapper, value: string) => {
  const keyInput = getDrawerInputs(wrapper)[0]
  await keyInput.setValue(value)
  await nextTick()
}

const setDrawerManualFields = async (wrapper: VueWrapper, value: string, description: string) => {
  const inputs = getDrawerInputs(wrapper)
  await inputs[1].setValue(value)
  await inputs[2].setValue(description)
}

describe('DynamicParameterEditor.vue', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-06-27T09:00:00.000Z'))
    vi.clearAllMocks()
    document.body.innerHTML = ''
    hoisted.getGroup.mockReturnValue({
      sourceType: 'device-metric',
      sourceConfig: {
        selectedDevice: { id: 'device-1' },
        selectedMetric: { id: 'temperature' }
      }
    })
    hoisted.getGroupParameters.mockImplementation((groupId: string, params: Param[]) =>
      params.filter(item => item.parameterGroup?.groupId === groupId)
    )
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
    vi.useRealTimers()
    document.body.innerHTML = ''
  })

  it('validates manual drawer additions, rejects blank or duplicate keys, and emits a static parameter', async () => {
    const wrapper = mountEditor({ modelValue: [param({ key: 'token', value: 'existing' })] })
    const state = getState(wrapper)
    const drawerState = getAddDrawerState(wrapper)

    await clickHeaderAddButton(wrapper)
    expect(state.showAddParamDrawer).toBe(true)

    // Blank-key validation is not reachable via the real footer button because the UI disables it.
    drawerState.confirmNewParam()
    expect(hoisted.messageError).toHaveBeenCalledWith(expect.stringContaining('key'))
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()

    await setDrawerKey(wrapper, 'token')
    await clickButton(getDrawerButtons(wrapper), 1)
    expect(hoisted.messageError).toHaveBeenCalledWith(expect.stringContaining('token'))

    await setDrawerKey(wrapper, 'page')
    await setDrawerManualFields(wrapper, '1', 'page number')
    await clickButton(getDrawerButtons(wrapper), 1)

    expect(lastModelValue(wrapper)).toEqual([
      expect.objectContaining({ key: 'token', value: 'existing' }),
      expect.objectContaining({
        key: 'page',
        value: '1',
        description: 'page number',
        valueMode: 'manual',
        selectedTemplate: 'manual',
        isDynamic: false
      })
    ])
    expect(state.showAddParamDrawer).toBe(false)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith(expect.stringContaining('page'))
  })

  it('adds a property-bound parameter through the real child change event instead of direct property handlers', async () => {
    const wrapper = mountEditor({
      modelValue: [param({ key: 'token', value: 'existing' })],
      currentComponentId: 'current-card'
    })
    const state = getState(wrapper)

    await clickHeaderAddButton(wrapper)
    await setDrawerKey(wrapper, 'color')
    await selectDrawerConfigType(wrapper, 'property')
    await nextTick()

    await wrapper.get('.component-property-selector-stub').trigger('click')
    await clickButton(getDrawerButtons(wrapper), 1)

    expect(lastModelValue(wrapper)).toEqual([
      expect.objectContaining({ key: 'token', value: 'existing' }),
      expect.objectContaining({
        key: 'color',
        value: 'target-card.component.styles.color',
        selectedTemplate: 'component-property-binding',
        valueMode: 'component',
        isDynamic: true
      })
    ])
    expect(state.showAddParamDrawer).toBe(false)
  })

  it('imports API templates with query/path/header filtering and replaces duplicate keys without losing ids', async () => {
    const apiInfo = {
      url: '/api/devices/:deviceId/telemetry',
      pathParamNames: ['deviceId'],
      commonParams: [
        { name: 'deviceId', type: 'string', description: 'device id', example: 'd-1' },
        { name: 'page', type: 'number', description: 'page index', example: 1 },
        { name: 'active', type: 'boolean', description: 'active flag', example: true },
        { name: 'X-Trace-Id', type: 'string', paramType: 'header', description: 'trace header', example: 'trace-1' }
      ]
    }
    const existing = [param({ key: 'page', value: 'existing-page', _id: 'keep-page-id' })]
    const wrapper = mountEditor({ modelValue: existing, parameterType: 'query', currentApiInfo: apiInfo })

    await clickHeaderTemplateButton(wrapper)
    await nextTick()

    expect(lastModelValue(wrapper)).toEqual([
      expect.objectContaining({
        key: 'page',
        value: 1,
        dataType: 'number',
        _id: 'keep-page-id'
      }),
      expect.objectContaining({
        key: 'active',
        value: true,
        dataType: 'boolean'
      })
    ])
    expect(lastModelValue(wrapper).some(item => item.key === 'deviceId')).toBe(false)
    expect(lastModelValue(wrapper).some(item => item.key === 'X-Trace-Id')).toBe(false)

    const pathWrapper = mountEditor({ modelValue: [], parameterType: 'path', currentApiInfo: apiInfo })
    await clickHeaderTemplateButton(pathWrapper)
    expect(lastModelValue(pathWrapper)).toEqual([
      expect.objectContaining({
        key: 'deviceId',
        value: 'd-1',
        dataType: 'string'
      })
    ])

    const headerWrapper = mountEditor({ modelValue: [], parameterType: 'header', currentApiInfo: apiInfo })
    await clickHeaderTemplateButton(headerWrapper)
    expect(lastModelValue(headerWrapper).map(item => item.key)).toEqual(['X-Trace-Id'])
  })

  it('falls back to sensible default template params when API metadata is missing or has no common params', async () => {
    const missingInfo = mountEditor({ modelValue: [], parameterType: 'query' })
    await clickHeaderButtons(missingInfo, 1)
    expect(lastModelValue(missingInfo)).toEqual([expect.objectContaining({ key: 'deviceId', valueMode: 'manual' })])

    const groupInfo = mountEditor({
      modelValue: [],
      parameterType: 'query',
      currentApiInfo: {
        url: '/api/group/current',
        commonParams: []
      }
    })
    await clickHeaderTemplateButton(groupInfo)
    expect(lastModelValue(groupInfo)).toEqual([expect.objectContaining({ key: 'groupId', valueMode: 'manual' })])
  })

  it('adds device parameters through the drawer child event, enforces max slots, and merges unified device config with deduplication', async () => {
    const wrapper = mountEditor({
      modelValue: [param({ key: 'deviceId', value: 'existing-device' }), param({ key: 'keep', value: 'keep-value' })],
      maxParameters: 3
    })
    const state = getState(wrapper)

    state.isAddFromDeviceDrawerVisible = true
    await nextTick()
    await wrapper.get('.add-parameter-from-device-confirm-stub').trigger('click')
    await nextTick()

    expect(lastModelValue(wrapper)).toHaveLength(3)
    expect(lastModelValue(wrapper)[2]).toMatchObject({
      key: 'metric',
      value: 'Pump A.Temperature',
      valueMode: 'component',
      selectedTemplate: 'device-dispatch-selector',
      variableName: 'var_metric'
    })

    const newDeviceParams = [
      param({ key: 'deviceId', value: 'new-device', valueMode: 'component' }),
      param({ key: 'deviceStatus', value: 'online', valueMode: 'component' })
    ]
    state.handleUnifiedDeviceConfigGenerated(newDeviceParams)
    expect(lastModelValue(wrapper).map(item => [item.key, item.value])).toEqual([
      ['keep', 'keep-value'],
      ['deviceId', 'new-device'],
      ['deviceStatus', 'online']
    ])
    expect(state.isUnifiedDeviceConfigVisible).toBe(false)
  })

  it('appends parameters selected from the device-group child and handles empty or exhausted device slots', async () => {
    const wrapper = mountEditor({
      modelValue: [param({ key: 'keep', value: 'keep-value' })],
      maxParameters: 2
    })
    const state = getState(wrapper)

    state.isDeviceParameterSelectorVisible = true
    await nextTick()
    await wrapper.get('.device-parameter-selected-stub').trigger('click')
    await nextTick()

    expect(lastModelValue(wrapper)).toEqual([
      expect.objectContaining({ key: 'keep', value: 'keep-value' }),
      expect.objectContaining({ key: 'selectedDevice', value: 'device-2', _id: 'selected-id' })
    ])
    expect(state.isDeviceParameterSelectorVisible).toBe(false)

    await wrapper.setProps({ modelValue: lastModelValue(wrapper) })
    const updateCountAfterSelection = wrapper.emitted('update:modelValue')?.length ?? 0
    state.isAddFromDeviceDrawerVisible = true
    await nextTick()
    await wrapper.get('.add-parameter-from-device-confirm-stub').trigger('click')
    await nextTick()

    expect(wrapper.emitted('update:modelValue')?.length ?? 0).toBe(updateCountAfterSelection)
    expect(state.isAddFromDeviceDrawerVisible).toBe(true)

    state.handleAddFromDevice([])
    expect(state.isAddFromDeviceDrawerVisible).toBe(false)
  })

  it('treats an explicit zero maxParameters as having no available device slots', async () => {
    const wrapper = mountEditor({
      modelValue: [],
      maxParameters: 0
    })
    const state = getState(wrapper)

    state.isAddFromDeviceDrawerVisible = true
    await nextTick()
    await wrapper.get('.add-parameter-from-device-confirm-stub').trigger('click')
    await nextTick()

    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(state.isAddFromDeviceDrawerVisible).toBe(true)
  })

  it('updates existing device config by replacing device-related params before appending generated params', async () => {
    const wrapper = mountEditor({
      modelValue: [
        param({ key: 'deviceId', value: 'existing-device' }),
        param({ key: 'metric', value: 'existing-metric' }),
        param({ key: 'unrelated', value: 'stay' })
      ]
    })
    const state = getState(wrapper)

    await wrapper.get('.device-config-info .button-primary').trigger('click')
    expect(state.isEditingDeviceConfig).toBe(true)
    expect(state.isUnifiedDeviceConfigVisible).toBe(true)

    await wrapper.get('.unified-device-config-selector-stub').trigger('click')

    expect(lastModelValue(wrapper).map(item => [item.key, item.value])).toEqual([
      ['unrelated', 'stay'],
      ['deviceId', 'stub-device'],
      ['metric', 'stub-device.temperature']
    ])
    expect(state.isEditingDeviceConfig).toBe(false)
  })

  it('wires the add drawer device selector to generated parameters instead of a stale confirm event', async () => {
    const wrapper = mountEditor({
      modelValue: [param({ key: 'keep', value: 'keep-value' })],
      maxParameters: 4
    })
    const state = getState(wrapper)

    await clickHeaderAddButton(wrapper)
    await setDrawerKey(wrapper, 'deviceBinding')
    await selectDrawerConfigType(wrapper, 'device')
    await nextTick()

    await wrapper.get('.unified-device-config-selector-stub').trigger('click')

    expect(lastModelValue(wrapper).map(item => [item.key, item.value])).toEqual([
      ['keep', 'keep-value'],
      ['deviceId', 'stub-device'],
      ['metric', 'stub-device.temperature']
    ])
    expect(state.showAddParamDrawer).toBe(false)
    await clickHeaderAddButton(wrapper)
    const resetInputs = getDrawerInputs(wrapper)
    expect(resetInputs).toHaveLength(1)
    expect((resetInputs[0].element as HTMLInputElement).value).toBe('')
  })

  it('updates, de-duplicates, and resets parameter keys and values without mutating unrelated entries', async () => {
    const wrapper = mountEditor({
      modelValue: [
        param({ key: 'first', value: 'a', _id: 'first-id' }),
        param({ key: 'second', value: 'b', _id: 'second-id' })
      ]
    })

    const rows = wrapper.findAll('.parameter-item-inline')
    const firstRowInputs = rows[0].findAll('.n-input-stub')
    await firstRowInputs[1].setValue('changed')
    await wrapper.setProps({ modelValue: lastModelValue(wrapper) })
    expect(lastModelValue(wrapper)[0]).toMatchObject({ key: 'first', value: 'changed' })

    await firstRowInputs[0].setValue('second')
    await wrapper.setProps({ modelValue: lastModelValue(wrapper) })
    expect(lastModelValue(wrapper)[0]).toMatchObject({ key: 'second' })

    await rows[0].find('.param-key-input-inline').trigger('blur')
    expect(lastModelValue(wrapper)[0]).toMatchObject({ key: 'param1' })
    expect(hoisted.messageError).toHaveBeenCalledWith(expect.stringContaining('second'))

    const secondRowKeyInput = rows[1].find('.param-key-input-inline')
    await secondRowKeyInput.setValue('')
    await secondRowKeyInput.trigger('blur')
    expect(lastModelValue(wrapper)[1]).toMatchObject({ key: 'param2' })
  })

  it('switches templates, opens component drawers, recovers corrupted binding paths, and saves drawer changes', async () => {
    const wrapper = mountEditor({
      modelValue: [
        param({
          key: 'color',
          value: '',
          valueMode: 'manual',
          selectedTemplate: 'manual',
          variableName: 'targetCard_color'
        })
      ],
      currentComponentId: 'current-card'
    })
    const state = getState(wrapper)
    const firstRow = wrapper.get('.parameter-item-inline')
    await firstRow.get('.param-type-select-inline').setValue('component-property-binding')
    await nextTick()

    expect(lastModelValue(wrapper)[0]).toMatchObject({
      selectedTemplate: 'component-property-binding',
      valueMode: 'component',
      isDynamic: true
    })
    expect(state.editingIndex).toBe(0)
    expect(state.isDrawerVisible).toBe(true)

    const componentDrawerState = getComponentDrawerState(wrapper)
    expect(componentDrawerState.drawerParam).toMatchObject({ variableName: 'targetCard_color' })
    await wrapper.get('.component-property-selector-invalid-stub').trigger('click')
    expect(componentDrawerState.drawerParam.value).toBe('targetCard.base.color')

    await wrapper.get('.component-property-selector-stub').trigger('click')
    expect(componentDrawerState.drawerParam).toMatchObject({
      value: 'target-card.component.styles.color',
      variableName: 'target-card_styles.color'
    })

    await clickButton(getDrawerButtons(wrapper), 1)
    await nextTick()
    expect(lastModelValue(wrapper)[0]).toMatchObject({
      value: 'target-card.component.styles.color',
      variableName: 'target-card_styles.color'
    })
    expect(state.isDrawerVisible).toBe(false)
    expect(state.drawerParam).toBeNull()
  })

  it('opens, updates, and deletes device parameter groups through the group manager', async () => {
    const groupParams = [
      param({
        key: 'deviceId',
        value: 'device-1',
        _id: 'group-primary',
        valueMode: 'component',
        deviceContext: { sourceType: 'device-selection' },
        parameterGroup: { groupId: 'group-1', role: 'primary', isDerived: false }
      }),
      param({
        key: 'metric',
        value: 'temperature',
        _id: 'group-secondary',
        valueMode: 'component',
        deviceContext: { sourceType: 'device-selection' },
        parameterGroup: { groupId: 'group-1', role: 'secondary', isDerived: true }
      }),
      param({ key: 'page', value: '1', _id: 'standalone' })
    ]
    const wrapper = mountEditor({ modelValue: groupParams })
    const state = getState(wrapper)
    const primaryRow = wrapper.findAll('.parameter-item-inline')[0]

    expect(state.isDeviceParameterGroup(groupParams[0])).toBe(true)
    expect(getParameterDisplayLabel(groupParams[0], { getGroup: hoisted.getGroup })).toContain('deviceId')

    await clickButton(primaryRow.findAll('.n-button-stub'), 1)
    expect(state.editingGroupInfo).toMatchObject({
      groupId: 'group-1',
      preSelectedDevice: { id: 'device-1' },
      preSelectedMetric: { id: 'temperature' }
    })
    expect(state.isDeviceParameterSelectorVisible).toBe(true)

    await wrapper.get('.device-parameter-updated-stub').trigger('click')
    expect(lastModelValue(wrapper)).toEqual([
      expect.objectContaining({ key: 'page', _id: 'standalone' }),
      expect.objectContaining({ key: 'deviceId', value: 'replacement', _id: 'replacement-id' })
    ])
    expect(state.isDeviceParameterSelectorVisible).toBe(false)

    await clickButton(primaryRow.findAll('.n-button-stub'), 2)
    expect(lastModelValue(wrapper)).toEqual([expect.objectContaining({ key: 'page', _id: 'standalone' })])
    expect(hoisted.removeGroup).toHaveBeenCalledWith('group-1')
  })

  it('exposes stable ids and template helpers for property and dropdown parameters', () => {
    const wrapper = mountEditor({
      modelValue: [
        param({
          key: 'propertyBinding',
          value: 'card-1.base.title',
          valueMode: 'manual',
          selectedTemplate: 'manual',
          _id: undefined,
          isDynamic: undefined
        }),
        param({
          key: 'contentType',
          value: 'application/json',
          valueMode: 'dropdown',
          selectedTemplate: 'content-types'
        })
      ]
    })
    const state = getState(wrapper)

    expect(state.parametersWithStableIds[0]).toMatchObject({
      key: 'propertyBinding',
      isDynamic: true
    })
    expect(state.parametersWithStableIds[0]._id).toContain('param_stable_')
    expect(state.getCurrentTemplateOptions(wrapper.props('modelValue')[1])).toEqual([
      { label: 'json', value: 'application/json' }
    ])
    expect(state.isCustomInputAllowed(wrapper.props('modelValue')[1])).toBe(true)
    expect(getTemplateById('component-property-binding')).toMatchObject({
      componentConfig: {
        component: 'ComponentPropertySelector',
        props: expect.objectContaining({ placeholder: 'pick property' })
      }
    })
  })

  it('opens property and device add flows, respects max parameter limits, and resets cancelled drawer data', async () => {
    const fullWrapper = mountEditor({
      modelValue: [param({ key: 'only', value: '1' })],
      maxParameters: 1
    })
    const fullState = getState(fullWrapper)

    expect(fullState.canAddMoreParameters).toBe(false)
    const [addButton, templateButton] = fullWrapper.findAll<HTMLButtonElement>('.editor-header-enhanced .n-button-stub')
    expect(addButton.element.disabled).toBe(true)
    expect(templateButton.element.disabled).toBe(true)
    await addButton.trigger('click')
    await templateButton.trigger('click')
    await nextTick()
    expect(fullState.showAddParamDrawer).toBe(false)
    expect(fullWrapper.emitted('update:modelValue')).toBeUndefined()

    const propertyWrapper = mountEditor({ modelValue: [] })
    const propertyState = getState(propertyWrapper)
    const propertyDrawerState = getAddDrawerState(propertyWrapper)

    await clickHeaderButtons(propertyWrapper, 0)
    await setDrawerKey(propertyWrapper, 'propertyBinding')
    await selectDrawerConfigType(propertyWrapper, 'property')
    await nextTick()
    await propertyWrapper.get('.component-property-selector-stub').trigger('click')
    await clickButton(getDrawerButtons(propertyWrapper), 1)
    await nextTick()

    expect(lastModelValue(propertyWrapper)).toEqual([
      expect.objectContaining({
        selectedTemplate: 'component-property-binding',
        valueMode: 'component',
        isDynamic: true
      })
    ])
    expect(propertyState.editingIndex).toBe(-1)
    expect(propertyState.isDrawerVisible).toBe(false)

    await clickHeaderButtons(propertyWrapper, 0)
    await setDrawerKey(propertyWrapper, 'deviceBinding')
    await selectDrawerConfigType(propertyWrapper, 'device')
    await nextTick()
    await propertyWrapper.get('.unified-device-config-selector-stub').trigger('click')
    await nextTick()
    expect(propertyState.isUnifiedDeviceConfigVisible).toBe(false)
    expect(propertyState.isEditingDeviceConfig).toBe(false)

    await clickHeaderAddButton(propertyWrapper)
    await setDrawerKey(propertyWrapper, 'transient')
    await selectDrawerConfigType(propertyWrapper, 'device')
    const drawerInputs = getDrawerInputs(propertyWrapper)
    await drawerInputs[1].setValue('dirty description')
    propertyDrawerState.newParamConfig.value = 'dirty'
    propertyDrawerState.newParamConfig.propertyBinding = { dirty: true }
    propertyDrawerState.newParamConfig.deviceConfig = { deviceId: 'device-1' }

    await clickButton(getDrawerButtons(propertyWrapper), 0)

    expect(propertyState.showAddParamDrawer).toBe(false)
    expect(propertyDrawerState.newParamConfig).toEqual({
      key: '',
      configType: 'manual',
      value: '',
      description: '',
      propertyBinding: null,
      deviceConfig: null
    })
  })

  it('resets edit state when the edited parameter is removed and tracks the same row when a previous row is deleted', async () => {
    const wrapper = mountEditor({
      modelValue: [
        param({ key: 'first', value: 'a' }),
        param({
          key: 'second',
          value: '',
          valueMode: 'manual',
          selectedTemplate: 'manual'
        }),
        param({ key: 'third', value: 'c' })
      ]
    })
    const state = getState(wrapper)

    await wrapper.findAll('.parameter-item-inline')[1].get('.param-type-select-inline').setValue('component-property-binding')
    await nextTick()
    await wrapper.findAll('.parameter-item-inline')[0].get('.param-actions-inline .n-button-stub').trigger('click')
    await nextTick()
    await wrapper.setProps({ modelValue: lastModelValue(wrapper) })

    expect(lastModelValue(wrapper).map(item => item.key)).toEqual(['second', 'third'])
    expect(state.editingIndex).toBe(0)

    await wrapper.findAll('.parameter-item-inline')[0].get('.param-actions-inline .n-button-stub').trigger('click')
    await nextTick()

    expect(lastModelValue(wrapper).map(item => item.key)).toEqual(['third'])
    expect(state.editingIndex).toBe(-1)
  })
})
