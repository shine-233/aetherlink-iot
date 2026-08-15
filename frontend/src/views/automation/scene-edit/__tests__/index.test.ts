/**
 * 文件用途: 覆盖测试在自动化场景下的前端行为与契约。
 * 核心逻辑: 通过 Vitest、Vue Test Utils 和必要的接口 mock，验证关键渲染、交互和数据流。
 * 关键注意事项: Mock 数据要贴近真实接口字段，避免只证明组件能挂载。
 * 重构建议: 后续可继续抽取稳定的 mount 工厂与交互 helper，减少重复路径拼接。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import {
  applyActionParamSelection,
  applyActionParamTypeChange,
  clearActionValueValidationState,
  markInvalidJsonActionValue,
  validateJsonActionValue
} from '../scene-action-form-state'

const hoisted = vi.hoisted(() => ({
  sceneAdd: vi.fn(),
  sceneEdit: vi.fn(),
  sceneGet: vi.fn(),
  sceneDryRun: vi.fn(),
  sceneInfo: vi.fn(),
  deviceGroupTree: vi.fn(),
  deviceListAll: vi.fn(),
  deviceConfigAll: vi.fn(),
  deviceConfigMetricsMenu: vi.fn(),
  deviceMetricsMenu: vi.fn(),
  warningMessageList: vi.fn(),
  messageError: vi.fn(),
  dialogWarning: vi.fn(),
  routeQuery: {}
}))

vi.mock('@/service/api/automation', () => ({
  sceneAdd: hoisted.sceneAdd,
  sceneEdit: hoisted.sceneEdit,
  sceneGet: hoisted.sceneGet,
  sceneDryRun: hoisted.sceneDryRun,
  sceneInfo: hoisted.sceneInfo,
  deviceConfigAll: hoisted.deviceConfigAll,
  deviceConfigMetricsMenu: hoisted.deviceConfigMetricsMenu,
  deviceListAll: hoisted.deviceListAll,
  deviceMetricsMenu: hoisted.deviceMetricsMenu
}))

vi.mock('@/service/api', () => ({
  deviceGroupTree: hoisted.deviceGroupTree
}))

vi.mock('@/service/api/alarm', () => ({
  warningMessageList: hoisted.warningMessageList
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ query: hoisted.routeQuery }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() })
}))

vi.mock('@/store/modules/tab', () => ({
  useTabStore: () => ({ removeTab: vi.fn() })
}))

vi.mock('naive-ui', async () => {
  const actual = await vi.importActual('naive-ui')
  return {
    ...actual,
    useDialog: () => ({ warning: hoisted.dialogWarning }),
    useMessage: () => ({ success: vi.fn(), error: hoisted.messageError })
  }
})

import SceneEdit from '../index.vue'

const CardStub = defineComponent({
  name: 'NCard',
  setup(_, { attrs, slots }) {
    return () => h('div', attrs, slots.default?.())
  }
})

const FlexStub = defineComponent({
  name: 'NFlex',
  setup(_, { attrs, slots }) {
    return () => h('div', attrs, slots.default?.())
  }
})

const ButtonStub = defineComponent({
  name: 'NButton',
  props: {
    type: { type: String, default: '' }
  },
  emits: ['click'],
  setup(props, { attrs, emit, slots }) {
    return () =>
      h(
        'button',
        {
          ...attrs,
          'data-type': props.type,
          onClick: () => emit('click')
        },
        slots.default?.()
      )
  }
})

const InputStub = defineComponent({
  name: 'NInput',
  props: {
    value: { default: '' },
    placeholder: { type: String, default: '' },
    type: { type: String, default: 'text' }
  },
  emits: ['update:value', 'blur', 'click'],
  setup(props, { attrs, emit }) {
    return () =>
      h('input', {
        ...attrs,
        type: props.type === 'textarea' ? 'text' : props.type,
        value: props.value ?? '',
        placeholder: props.placeholder,
        onInput: (event: Event) => emit('update:value', (event.target as HTMLInputElement).value),
        onBlur: () => emit('blur'),
        onClick: () => emit('click')
      })
  }
})

const InputNumberStub = defineComponent({
  name: 'NInputNumber',
  props: {
    value: { default: null },
    placeholder: { type: String, default: '' }
  },
  emits: ['update:value'],
  setup(props, { attrs, emit }) {
    return () =>
      h('input', {
        ...attrs,
        type: 'number',
        value: props.value ?? '',
        placeholder: props.placeholder,
        onInput: (event: Event) => {
          const nextValue = (event.target as HTMLInputElement).value
          emit('update:value', nextValue === '' ? null : Number(nextValue))
        }
      })
  }
})

const RadioStub = defineComponent({
  name: 'NRadio',
  props: {
    value: { default: null }
  },
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const RadioGroupStub = defineComponent({
  name: 'NRadioGroup',
  props: {
    value: { default: null }
  },
  emits: ['update:value'],
  setup(props, { attrs, slots }) {
    return () =>
      h(
        'div',
        {
          ...attrs,
          'data-value': props.value == null ? '' : String(props.value)
        },
        slots.default?.()
      )
  }
})

const SpaceStub = defineComponent({
  name: 'NSpace',
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const EmptyStub = defineComponent({
  name: 'NEmpty',
  setup(_, { slots }) {
    return () => h('div', { class: 'empty-stub' }, slots.default?.())
  }
})

const AlertStub = defineComponent({
  name: 'NAlert',
  setup(_, { slots }) {
    return () => h('div', { class: 'alert-stub' }, slots.default?.())
  }
})

const TagStub = defineComponent({
  name: 'NTag',
  setup(_, { slots }) {
    return () => h('span', { class: 'tag-stub' }, slots.default?.())
  }
})

const DividerStub = defineComponent({
  name: 'NDivider',
  setup(_, { attrs }) {
    return () => h('hr', attrs)
  }
})

const SimpleContainerStub = (name: string) =>
  defineComponent({
    name,
    setup(_, { attrs, slots }) {
      return () => h('div', attrs, slots.default?.())
    }
  })

const SelectStub = defineComponent({
  name: 'NSelect',
  props: {
    value: { default: null },
    options: { type: Array, default: () => [] },
    valueField: { type: String, default: 'value' },
    labelField: { type: String, default: 'label' }
  },
  emits: ['update:value', 'search'],
  setup(props, { attrs, slots }) {
    return () =>
      h(
        'div',
        {
          ...attrs,
          'data-value': props.value == null ? '' : String(props.value),
          'data-option-count': String(props.options.length)
        },
        [slots.header?.(), slots.default?.()]
      )
  }
})

const FormStub = defineComponent({
  name: 'NForm',
  setup(_, { attrs, expose, slots }) {
    const validate = () => Promise.resolve()
    expose({ validate })
    return () => h('div', attrs, slots.default?.())
  }
})

const FormItemStub = defineComponent({
  name: 'NFormItem',
  props: {
    path: { type: String, default: '' },
    validationStatus: { type: String, default: undefined },
    feedback: { type: String, default: '' }
  },
  setup(props, { attrs, slots }) {
    return () =>
      h(
        'div',
        {
          ...attrs,
          'data-path': props.path || undefined,
          'data-validation-status': props.validationStatus,
          'data-feedback': props.feedback || undefined
        },
        slots.default?.()
      )
  }
})

const SceneOperateDeviceActionGroupEditorStub = defineComponent({
  name: 'SceneOperateDeviceActionGroupEditor',
  props: {
    actionGroupItem: { type: Object, required: true },
    actionGroupIndex: { type: Number, required: true },
    actionTypeOptions: { type: Array, default: () => [] },
    configFormRules: { type: Object, default: () => ({}) },
    deviceConfigOption: { type: Array, default: () => [] },
    deviceGroupOptions: { type: Array, default: () => [] },
    deviceOptions: { type: Array, default: () => [] },
    loadingSelect: { type: Boolean, default: false },
    queryDevice: { type: Object, required: true },
    actionTargetChange: { type: Function, required: true },
    actionTypeChange: { type: Function, required: true },
    createInstruction: { type: Function, required: true }
  },
  setup(props) {
    const updateActionType = async (item: Record<string, any>, value: string) => {
      item.action_type = value
      await props.actionTypeChange(item, value)
    }

    const updateActionTarget = async (item: Record<string, any>, value: string) => {
      item.action_target = value
      await props.actionTargetChange(item)
    }

    const updateActionParamType = (item: Record<string, any>, value: string) => {
      item.action_param_type = value
      applyActionParamTypeChange(item, value)
    }

    const updateActionParam = (item: Record<string, any>, value: string) => {
      item.action_param = value
      applyActionParamSelection(item, value)
    }

    const validateActionValue = (item: Record<string, any>) => {
      if (validateJsonActionValue(item.action_param_type, item.actionValue)) {
        clearActionValueValidationState(item)
        return
      }

      hoisted.messageError('common.enterJson')
      markInvalidJsonActionValue(item, 'common.enterJson')
    }

    const addInstruction = () => {
      ;(props.actionGroupItem as any).actionInstructList.push(props.createInstruction())
    }

    const deleteInstruction = (index: number) => {
      ;(props.actionGroupItem as any).actionInstructList.splice(index, 1)
    }

    return () =>
      h(
        'div',
        { class: 'scene-operate-device-action-group-editor-stub' },
        ((props.actionGroupItem as any).actionInstructList || []).map((item: Record<string, any>, instructIndex: number) =>
          h('div', { class: 'scene-operate-device-action-group-editor-stub__row' }, [
            h(
              FormItemStub,
              {
                path: instructionFieldPath(props.actionGroupIndex, instructIndex, 'action_type')
              },
              {
                default: () =>
                  h(SelectStub, {
                    value: item.action_type,
                    options: props.actionTypeOptions,
                    'onUpdate:value': (value: string) => void updateActionType(item, value)
                  })
              }
            ),
            item.action_type === '10'
              ? h(
                  FormItemStub,
                  {
                    path: instructionFieldPath(props.actionGroupIndex, instructIndex, 'action_target')
                  },
                  {
                    default: () =>
                      h(SelectStub, {
                        value: item.action_target,
                        options: props.deviceOptions,
                        'onUpdate:value': (value: string) => void updateActionTarget(item, value)
                      })
                  }
                )
              : null,
            item.action_type === '11'
              ? h(
                  FormItemStub,
                  {
                    path: instructionFieldPath(props.actionGroupIndex, instructIndex, 'action_target')
                  },
                  {
                    default: () =>
                      h(SelectStub, {
                        value: item.action_target,
                        options: props.deviceConfigOption,
                        'onUpdate:value': (value: string) => void updateActionTarget(item, value)
                      })
                  }
                )
              : null,
            item.action_type
              ? h(
                  FormItemStub,
                  {
                    path: instructionFieldPath(props.actionGroupIndex, instructIndex, 'action_param_type')
                  },
                  {
                    default: () =>
                      h(SelectStub, {
                        value: item.action_param_type,
                        options: item.actionParamTypeOptions || [],
                        'onUpdate:value': (value: string) => updateActionParamType(item, value)
                      })
                  }
                )
              : null,
            item.action_type && item.showSubSelect
              ? h(
                  FormItemStub,
                  {
                    path: instructionFieldPath(props.actionGroupIndex, instructIndex, 'action_param')
                  },
                  {
                    default: () =>
                      h(SelectStub, {
                        value: item.action_param,
                        options: item.actionParamOptions || [],
                        'onUpdate:value': (value: string) => updateActionParam(item, value)
                      })
                  }
                )
              : null,
            item.action_type
              ? h(
                  FormItemStub,
                  {
                    path: instructionFieldPath(props.actionGroupIndex, instructIndex, 'actionValue'),
                    validationStatus: item.inputValidationStatus,
                    feedback: item.inputFeedback
                  },
                  {
                    default: () =>
                      h(InputStub, {
                        value: item.actionValue ?? '',
                        'onUpdate:value': (value: string) => {
                          item.actionValue = value
                        },
                        onBlur: () => validateActionValue(item)
                      })
                  }
                )
              : null,
            instructIndex === 0
              ? h(
                  ButtonStub,
                  { type: 'primary', class: 'absolute right-5', onClick: addInstruction },
                  { default: () => 'generate.add-row' }
                )
              : h(
                  ButtonStub,
                  {
                    type: 'error',
                    class: 'absolute right-5',
                    onClick: () => deleteInstruction(instructIndex)
                  },
                  { default: () => 'common.delete' }
                )
          ])
        )
      )
  }
})

const mountedWrappers: Array<ReturnType<typeof mount>> = []

const mountSceneEdit = () => {
  const wrapper = mount(SceneEdit, {
    global: {
      stubs: {
        NCard: CardStub,
        'n-card': CardStub,
        NFlex: FlexStub,
        'n-flex': FlexStub,
        NButton: ButtonStub,
        'n-button': ButtonStub,
        NInput: InputStub,
        'n-input': InputStub,
        NInputNumber: InputNumberStub,
        'n-input-number': InputNumberStub,
        NRadio: RadioStub,
        'n-radio': RadioStub,
        NRadioGroup: RadioGroupStub,
        'n-radio-group': RadioGroupStub,
        NSpace: SpaceStub,
        'n-space': SpaceStub,
        NSelect: SelectStub,
        'n-select': SelectStub,
        NForm: FormStub,
        'n-form': FormStub,
        NFormItem: FormItemStub,
        'n-form-item': FormItemStub,
        NEmpty: EmptyStub,
        'n-empty': EmptyStub,
        NAlert: AlertStub,
        'n-alert': AlertStub,
        NTag: TagStub,
        'n-tag': TagStub,
        NDivider: DividerStub,
        'n-divider': DividerStub,
        PopUp: SimpleContainerStub('PopUp'),
        'pop-up': SimpleContainerStub('PopUp'),
        AutomationDryRunPreview: SimpleContainerStub('AutomationDryRunPreview'),
        'automation-dry-run-preview': SimpleContainerStub('AutomationDryRunPreview'),
        LinkageActionExecutionSummary: SimpleContainerStub('LinkageActionExecutionSummary'),
        'linkage-action-execution-summary': SimpleContainerStub('LinkageActionExecutionSummary'),
        SceneOperateDeviceActionGroupEditor: SceneOperateDeviceActionGroupEditorStub,
        'scene-operate-device-action-group-editor': SceneOperateDeviceActionGroupEditorStub
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

type SceneEditWrapper = ReturnType<typeof mountSceneEdit>

const actionGroupPath = (groupIndex: number) => `actions[${groupIndex}].actionType`
const instructionFieldPath = (groupIndex: number, instructionIndex: number, field: string) =>
  `actions[${groupIndex}].actionInstructList[${instructionIndex}].${field}`

const getFormItem = (wrapper: SceneEditWrapper, path: string) => wrapper.get(`[data-path="${path}"]`)

const getRenderedFormItemPaths = (wrapper: SceneEditWrapper) =>
  wrapper
    .findAll('[data-path]')
    .map(item => item.attributes('data-path'))
    .filter((path): path is string => Boolean(path))

const getSelectByPath = (wrapper: SceneEditWrapper, path: string) => getFormItem(wrapper, path).getComponent(SelectStub)

const getSelectOptions = (wrapper: SceneEditWrapper, path: string) =>
  getSelectByPath(wrapper, path).props('options') as Array<Record<string, any>>

const emitSelectValue = async (wrapper: SceneEditWrapper, path: string, value: string) => {
  getSelectByPath(wrapper, path).vm.$emit('update:value', value)
  await flushPromises()
}

const getButtons = (wrapper: SceneEditWrapper) => wrapper.findAll('button, button-stub, n-button-stub')
type ButtonWrapper = ReturnType<typeof getButtons>[number]

const findButton = (wrapper: SceneEditWrapper, predicate: (button: ButtonWrapper) => boolean) => {
  const button = getButtons(wrapper).find(predicate)
  if (!button) throw new Error('expected scene editor button was not rendered')
  return button!
}

const clickButton = async (button: ButtonWrapper) => {
  await button.trigger('click')
  await flushPromises()
}

const getAddActionGroupButton = (wrapper: SceneEditWrapper) =>
  findButton(wrapper, button => button.text().includes('generate.add-execution-action'))

const getDeleteActionGroupButton = (wrapper: SceneEditWrapper) =>
  findButton(wrapper, button => button.text().includes('generate.delete-execution-action'))

const getAddInstructionButton = (wrapper: SceneEditWrapper) =>
  findButton(wrapper, button => button.text().includes('generate.add-row'))

const getDeleteInstructionButton = (wrapper: SceneEditWrapper) =>
  findButton(wrapper, button => button.text().includes('common.delete'))

const getActionValueInput = (wrapper: SceneEditWrapper, path: string) =>
  getFormItem(wrapper, path).getComponent(InputStub)

const getActionValueField = (wrapper: SceneEditWrapper, path: string) =>
  getActionValueInput(wrapper, path).get('input')

const prepareCommandValueEditor = async (wrapper: SceneEditWrapper) => {
  await emitSelectValue(wrapper, actionGroupPath(0), '1')
  await emitSelectValue(wrapper, instructionFieldPath(0, 0, 'action_type'), '10')
  await emitSelectValue(wrapper, instructionFieldPath(0, 0, 'action_target'), 'device-1')
  await emitSelectValue(wrapper, instructionFieldPath(0, 0, 'action_param_type'), 'command')
  await emitSelectValue(wrapper, instructionFieldPath(0, 0, 'action_param'), 'methodA')
}

describe('SceneEdit', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.routeQuery = {}
    hoisted.sceneGet.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.sceneInfo.mockResolvedValue({
      data: {
        info: { id: '', name: '', description: '', actions: [] },
        actions: []
      }
    })
    hoisted.deviceGroupTree.mockResolvedValue({
      data: [{ group: { id: 'group-1', name: 'Group 1' } }]
    })
    hoisted.deviceListAll.mockResolvedValue({
      data: [
        { id: 'device-1', name: 'Device 1' },
        { id: 'device-2', name: 'Device 2' }
      ]
    })
    hoisted.deviceConfigAll.mockResolvedValue({
      data: [{ id: 'config-1', name: 'Config 1' }]
    })
    hoisted.warningMessageList.mockResolvedValue({ data: { list: [], total: 0 } })
    hoisted.sceneAdd.mockResolvedValue({ error: null })
    hoisted.sceneEdit.mockResolvedValue({ error: null })
    hoisted.sceneDryRun.mockResolvedValue({ data: { can_save: true } })
    hoisted.deviceMetricsMenu.mockImplementation(({ device_id }: { device_id: string }) =>
      Promise.resolve({
        data: [
          {
            data_source_type: 'command',
            label: device_id === 'device-2' ? 'secondary' : 'primary',
            options: [
              {
                key: device_id === 'device-2' ? 'methodB' : 'methodA',
                label: 'Method',
                data_type: 'string'
              }
            ]
          }
        ]
      })
    )
    hoisted.deviceConfigMetricsMenu.mockResolvedValue({
      data: [
        {
          data_source_type: 'attributes',
          label: 'Attributes',
          options: [{ key: 'temperature', label: 'Temperature', data_type: 'number' }]
        }
      ]
    })
  })

  afterEach(() => {
    mountedWrappers.forEach(wrapper => wrapper.unmount())
    mountedWrappers.length = 0
  })

  it('renders one empty action group selector without eager catalog requests', async () => {
    const wrapper = mountSceneEdit()

    await flushPromises()

    expect(hoisted.deviceGroupTree).toHaveBeenCalledTimes(0)
    expect(hoisted.deviceListAll).toHaveBeenCalledTimes(0)
    expect(hoisted.warningMessageList).toHaveBeenCalledTimes(0)
    expect(hoisted.sceneGet).toHaveBeenCalledTimes(0)
    expect(hoisted.deviceConfigAll).toHaveBeenCalledTimes(0)
    expect(getSelectOptions(wrapper, actionGroupPath(0)).map(option => option.value)).toEqual(['1'])
    expect(getRenderedFormItemPaths(wrapper)).not.toContain(instructionFieldPath(0, 0, 'action_type'))
  })

  it('loads an edit scene and submits through sceneEdit after the rendered save confirmation', async () => {
    hoisted.routeQuery = { id: 'scene-42' }
    hoisted.sceneInfo.mockResolvedValue({
      data: {
        info: { id: 'scene-42', name: 'Existing scene', description: 'Loaded description' },
        actions: [
          {
            action_type: '10',
            action_target: 'device-1',
            action_param_type: 'command',
            action_param: 'methodA',
            action_value: JSON.stringify({ method: 'methodA', params: '{"target":18}' })
          }
        ]
      }
    })

    const wrapper = mountSceneEdit()
    await flushPromises()

    expect(hoisted.sceneInfo).toHaveBeenCalledWith('scene-42')
    expect((wrapper.get('input[placeholder="generate.enterSceneName"]').element as HTMLInputElement).value).toBe(
      'Existing scene'
    )
    expect(
      (wrapper.get('input[placeholder="generate.enter-description"]').element as HTMLInputElement).value
    ).toBe('Loaded description')

    await prepareCommandValueEditor(wrapper)
    await getActionValueField(wrapper, instructionFieldPath(0, 0, 'actionValue')).setValue('{"target":18}')
    await clickButton(
      findButton(wrapper, button => button.text().includes('generate.save-scene-configuration'))
    )

    expect(hoisted.sceneDryRun).toHaveBeenCalledTimes(1)
    const confirmation = hoisted.dialogWarning.mock.calls[0][0] as { onPositiveClick: () => Promise<void> }
    await confirmation.onPositiveClick()

    expect(hoisted.sceneEdit).toHaveBeenCalledTimes(1)
    expect(hoisted.sceneAdd).not.toHaveBeenCalled()
    expect(hoisted.sceneEdit.mock.calls[0][0]).toMatchObject({
      id: 'scene-42',
      name: 'Existing scene',
      description: 'Loaded description',
      actions: [expect.objectContaining({ action_type: '10', action_target: 'device-1', action_param: 'methodA' })]
    })
  })

  it('replays a single-class device action and loads its config metrics through the rendered controls', async () => {
    hoisted.routeQuery = { id: 'scene-11' }
    hoisted.sceneInfo.mockResolvedValue({
      data: {
        info: { id: 'scene-11', name: 'Class action', description: 'Use the device class' },
        actions: [
          {
            action_type: '11',
            action_target: 'config-1',
            action_param_type: 'attributes',
            action_param: 'temperature',
            action_value: JSON.stringify({ temperature: 23 })
          }
        ]
      }
    })

    const wrapper = mountSceneEdit()
    await flushPromises()

    const actionTypePath = instructionFieldPath(0, 0, 'action_type')
    const actionTargetPath = instructionFieldPath(0, 0, 'action_target')
    const actionParamTypePath = instructionFieldPath(0, 0, 'action_param_type')
    const actionParamPath = instructionFieldPath(0, 0, 'action_param')
    const actionValuePath = instructionFieldPath(0, 0, 'actionValue')

    expect(hoisted.sceneInfo).toHaveBeenCalledWith('scene-11')
    expect(getSelectByPath(wrapper, actionTypePath).props('value')).toBe('11')
    expect(getSelectOptions(wrapper, actionTargetPath).map(option => option.id)).toEqual(['config-1'])
    expect(hoisted.deviceConfigAll).toHaveBeenCalledWith({ device_config_name: '' })
    expect(hoisted.deviceConfigMetricsMenu).toHaveBeenCalledWith({ device_config_id: 'config-1' })
    expect(getSelectOptions(wrapper, actionParamTypePath).map(option => option.value)).toEqual(['attributes'])
    expect(getSelectOptions(wrapper, actionParamPath).map(option => option.value)).toEqual(['temperature'])
    expect(getActionValueInput(wrapper, actionValuePath).props('value')).toBe(23)
  })

  it('adds and deletes action groups through rendered action buttons', async () => {
    const wrapper = mountSceneEdit()

    await flushPromises()
    await clickButton(getAddActionGroupButton(wrapper))

    expect(getRenderedFormItemPaths(wrapper).filter(path => path.endsWith('.actionType'))).toEqual([
      actionGroupPath(0),
      actionGroupPath(1)
    ])

    await clickButton(getDeleteActionGroupButton(wrapper))

    expect(getRenderedFormItemPaths(wrapper).filter(path => path.endsWith('.actionType'))).toEqual([
      actionGroupPath(0)
    ])
  })

  it('adds and removes instruction rows through child emits and row buttons', async () => {
    const wrapper = mountSceneEdit()

    await flushPromises()
    await emitSelectValue(wrapper, actionGroupPath(0), '1')

    expect(getRenderedFormItemPaths(wrapper)).toContain(instructionFieldPath(0, 0, 'action_type'))
    expect(
      getSelectOptions(wrapper, instructionFieldPath(0, 0, 'action_type')).map(option => option.value)
    ).toEqual(['10', '11'])

    await clickButton(getAddInstructionButton(wrapper))

    expect(
      getRenderedFormItemPaths(wrapper).filter(path => path.endsWith('.action_type'))
    ).toEqual([
      instructionFieldPath(0, 0, 'action_type'),
      instructionFieldPath(0, 1, 'action_type')
    ])

    await clickButton(getDeleteInstructionButton(wrapper))

    expect(
      getRenderedFormItemPaths(wrapper).filter(path => path.endsWith('.action_type'))
    ).toEqual([instructionFieldPath(0, 0, 'action_type')])
  })

  it('reloads the correct target catalog when the instruction type changes', async () => {
    const wrapper = mountSceneEdit()

    await flushPromises()
    await emitSelectValue(wrapper, actionGroupPath(0), '1')
    vi.clearAllMocks()

    await emitSelectValue(wrapper, instructionFieldPath(0, 0, 'action_type'), '10')

    expect(hoisted.deviceListAll).toHaveBeenCalledWith({
      group_id: null,
      device_name: null,
      bind_config: 0
    })
    expect(
      getSelectOptions(wrapper, instructionFieldPath(0, 0, 'action_target')).map(option => option.id)
    ).toEqual(['device-1', 'device-2'])

    await emitSelectValue(wrapper, instructionFieldPath(0, 0, 'action_type'), '11')

    expect(hoisted.deviceConfigAll).toHaveBeenCalledWith({
      device_config_name: ''
    })
    expect(
      getSelectOptions(wrapper, instructionFieldPath(0, 0, 'action_target')).map(option => option.id)
    ).toEqual(['config-1'])
  })

  it('resets downstream parameter controls when the action target changes', async () => {
    const wrapper = mountSceneEdit()
    const actionTargetPath = instructionFieldPath(0, 0, 'action_target')
    const actionParamTypePath = instructionFieldPath(0, 0, 'action_param_type')
    const actionParamPath = instructionFieldPath(0, 0, 'action_param')
    const actionValuePath = instructionFieldPath(0, 0, 'actionValue')

    await flushPromises()
    await emitSelectValue(wrapper, actionGroupPath(0), '1')
    await emitSelectValue(wrapper, instructionFieldPath(0, 0, 'action_type'), '10')
    await emitSelectValue(wrapper, actionTargetPath, 'device-1')

    expect(hoisted.deviceMetricsMenu).toHaveBeenLastCalledWith({ device_id: 'device-1' })
    expect(getSelectOptions(wrapper, actionParamTypePath).map(option => option.value)).toEqual(['command'])

    await emitSelectValue(wrapper, actionParamTypePath, 'command')
    expect(getSelectOptions(wrapper, actionParamPath).map(option => option.value)).toEqual(['methodA'])

    await emitSelectValue(wrapper, actionParamPath, 'methodA')
    expect(getRenderedFormItemPaths(wrapper)).toContain(actionValuePath)

    await emitSelectValue(wrapper, actionTargetPath, 'device-2')

    expect(hoisted.deviceMetricsMenu).toHaveBeenLastCalledWith({ device_id: 'device-2' })
    expect(getSelectByPath(wrapper, actionParamTypePath).props('value')).toBeNull()
    expect(getRenderedFormItemPaths(wrapper)).toContain(actionParamPath)
    expect(getSelectOptions(wrapper, actionParamPath)).toEqual([])
    expect(getActionValueInput(wrapper, actionValuePath).props('value')).toBe('')
  })

  it('surfaces JSON validation state from the rendered action value input', async () => {
    const wrapper = mountSceneEdit()
    const actionValuePath = instructionFieldPath(0, 0, 'actionValue')

    await flushPromises()
    await prepareCommandValueEditor(wrapper)

    await getActionValueField(wrapper, actionValuePath).setValue('not-json')
    await getActionValueField(wrapper, actionValuePath).trigger('blur')
    await flushPromises()

    expect(hoisted.messageError).toHaveBeenCalledWith('common.enterJson')
    expect(getFormItem(wrapper, actionValuePath).attributes('data-validation-status')).toBe('error')

    hoisted.messageError.mockClear()

    await getActionValueField(wrapper, actionValuePath).setValue('{"key":"val"}')
    await getActionValueField(wrapper, actionValuePath).trigger('blur')
    await flushPromises()

    expect(hoisted.messageError).not.toHaveBeenCalled()
    expect(getFormItem(wrapper, actionValuePath).attributes('data-validation-status')).toBeUndefined()
  })

  it('uses the action-only scene dry-run as a save gate before opening confirmation', async () => {
    const wrapper = mountSceneEdit()
    const actionValuePath = instructionFieldPath(0, 0, 'actionValue')
    hoisted.sceneDryRun.mockResolvedValueOnce({ data: { can_save: false, blockers: ['select a valid target'] } })

    await flushPromises()
    await prepareCommandValueEditor(wrapper)
    await getActionValueField(wrapper, actionValuePath).setValue('{"delay":1}')
    await clickButton(
      findButton(
        wrapper,
        button => button.text().includes('generate.save-scene-configuration')
      )
    )
    await flushPromises()

    expect(hoisted.sceneDryRun).toHaveBeenCalledTimes(1)
    expect(hoisted.sceneDryRun.mock.calls[0][0]).toMatchObject({
      actions: expect.any(Array)
    })
    expect(hoisted.sceneDryRun.mock.calls[0][0]).not.toHaveProperty('trigger_condition_groups')
    expect(hoisted.messageError).toHaveBeenCalledWith('select a valid target')
    expect(hoisted.dialogWarning).not.toHaveBeenCalled()
  })

  it('blocks an incomplete rendered action locally before invoking backend dry-run', async () => {
    const wrapper = mountSceneEdit()

    await flushPromises()
    await clickButton(
      findButton(wrapper, button => button.text().includes('generate.save-scene-configuration'))
    )
    await flushPromises()

    expect(hoisted.sceneDryRun).not.toHaveBeenCalled()
    expect(hoisted.messageError).toHaveBeenCalledWith('generate.sceneDryRunIncompleteActionBlocker')
    expect(hoisted.dialogWarning).not.toHaveBeenCalled()
  })

  it('submits the confirmed scene with the flattened command payload approved by dry-run', async () => {
    const wrapper = mountSceneEdit()
    const actionValuePath = instructionFieldPath(0, 0, 'actionValue')

    await flushPromises()
    await wrapper.get('input[placeholder="generate.enterSceneName"]').setValue('Night cooling')
    await wrapper.get('input[placeholder="generate.enter-description"]').setValue('Reduce the room temperature')
    await prepareCommandValueEditor(wrapper)

    await clickButton(getAddActionGroupButton(wrapper))
    await clickButton(getDeleteActionGroupButton(wrapper))
    await clickButton(getAddInstructionButton(wrapper))
    await clickButton(getDeleteInstructionButton(wrapper))

    expect(getRenderedFormItemPaths(wrapper).filter(path => path.endsWith('.actionType'))).toEqual([
      actionGroupPath(0)
    ])
    expect(
      getRenderedFormItemPaths(wrapper).filter(path => path.endsWith('.action_type'))
    ).toEqual([instructionFieldPath(0, 0, 'action_type')])

    await getActionValueField(wrapper, actionValuePath).setValue('{"target":18}')
    await clickButton(
      findButton(wrapper, button => button.text().includes('generate.save-scene-configuration'))
    )

    expect(hoisted.sceneDryRun).toHaveBeenCalledTimes(1)
    const dryRunPayload = hoisted.sceneDryRun.mock.calls[0][0]
    expect(dryRunPayload).toMatchObject({
      id: '',
      name: 'Night cooling',
      description: 'Reduce the room temperature',
      actions: [
        expect.objectContaining({
          action_type: '10',
          action_target: 'device-1',
          action_param_type: 'command',
          action_param: 'methodA',
          action_value: '{"method":"methodA","params":"{\\"target\\":18}"}'
        })
      ]
    })
    expect(dryRunPayload.actions).toHaveLength(1)
    expect(hoisted.dialogWarning).toHaveBeenCalledWith({
      title: 'common.tip',
      content: 'common.saveSceneInfo',
      positiveText: 'device_template.confirm',
      negativeText: 'common.cancel',
      onPositiveClick: expect.any(Function)
    })
    expect(hoisted.sceneAdd).not.toHaveBeenCalled()

    const confirmation = hoisted.dialogWarning.mock.calls[0][0] as {
      onPositiveClick: () => Promise<void>
    }
    await confirmation.onPositiveClick()

    expect(hoisted.sceneAdd).toHaveBeenCalledTimes(1)
    const submitPayload = hoisted.sceneAdd.mock.calls[0][0]
    expect(submitPayload).toMatchObject({
      id: '',
      name: 'Night cooling',
      description: 'Reduce the room temperature',
      actions: [
        expect.objectContaining({
          action_type: '10',
          action_target: 'device-1',
          action_param_type: 'command',
          action_param: 'methodA',
          action_value: '{"method":"methodA","params":"{\\"target\\":18}"}'
        })
      ]
    })
    expect(submitPayload.actions).toHaveLength(1)
    expect(hoisted.sceneEdit).not.toHaveBeenCalled()
  })
})
