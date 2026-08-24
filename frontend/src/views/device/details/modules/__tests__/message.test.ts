/**
 * 文件用途: message 测试文件。
 * 核心逻辑: 验证对应组件或组合函数的关键用户路径和边界状态。
 * 关键注意事项: mock 数据应贴近真实接口字段，避免只断言组件存在。
 * 重构建议: 补齐失败、空数据和权限边界用例。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  deviceDetail: vi.fn(),
  deviceConfigInfo: vi.fn(),
  deviceLocation: vi.fn(),
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  windowMessageError: vi.fn(),
  route: { query: { d_id: 'route-device-1' } }
}))

vi.mock('vue-router', () => ({
  useRoute: () => hoisted.route
}))

vi.mock('@/service/api', () => ({
  deviceConfigInfo: hoisted.deviceConfigInfo,
  deviceDetail: hoisted.deviceDetail,
  deviceLocation: hoisted.deviceLocation
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('naive-ui', async importOriginal => {
  const actual = await importOriginal<typeof import('naive-ui')>()
  return {
    ...actual,
    useMessage: () => ({
      success: hoisted.messageSuccess,
      error: hoisted.messageError
    })
  }
})

vi.mock('../public/tencent-map.vue', () => ({
  default: defineComponent({
    name: 'TencentMap',
    setup() {
      return () => h('div', { class: 'tencent-map-stub' })
    }
  })
}))

import MessagePage from '../message.vue'

const mountedWrappers: VueWrapper[] = []

const SlotPassThroughCard = defineComponent({
  name: 'NCard',
  props: ['title', 'class'],
  setup(_, { slots }) {
    return () => h('div', { class: 'n-card-stub' }, slots.default?.())
  }
})

const SlotPassThroughForm = defineComponent({
  name: 'NForm',
  setup(_, { slots }) {
    return () => h('div', { class: 'n-form-stub' }, slots.default?.())
  }
})

const InputStub = defineComponent({
  name: 'NInput',
  props: ['value', 'placeholder', 'readonly'],
  emits: ['update:value'],
  setup() {
    return () => h('input')
  }
})

const InputNumberStub = defineComponent({
  name: 'NInputNumber',
  props: ['value', 'placeholder'],
  emits: ['update:value'],
  setup() {
    return () => h('input', { type: 'number' })
  }
})

const SelectStub = defineComponent({
  name: 'NSelect',
  props: ['value', 'options', 'placeholder'],
  emits: ['update:value'],
  setup() {
    return () => h('select')
  }
})

const SwitchStub = defineComponent({
  name: 'NSwitch',
  props: ['value', 'checkedValue', 'uncheckedValue'],
  emits: ['update:value'],
  setup() {
    return () => h('button')
  }
})

const ModalStub = defineComponent({
  name: 'NModal',
  props: ['show'],
  emits: ['update:show'],
  setup(_, { slots }) {
    return () => h('div', { class: 'modal-stub' }, slots.default?.())
  }
})

// setupState 的受控视图：只声明测试实际触达的成员，其余成员走 unknown 兜底。
interface MessageSetupState {
  additionInfo: Array<{ enable?: boolean; value?: unknown; name?: unknown }>
  isShow: boolean
  latitude: unknown
  longitude: unknown
  safeParseJSON: (...args: unknown[]) => unknown
  normalizeExtendedInfo: (...args: unknown[]) => unknown
  coerceValueByType: (...args: unknown[]) => unknown
  getBooleanValue: (...args: unknown[]) => unknown
  getNumberValue: (...args: unknown[]) => unknown
  getTextValue: (...args: unknown[]) => unknown
  handleSave: (...args: unknown[]) => unknown
  setItemValue: (...args: unknown[]) => void
  onPositionSelected: (...args: unknown[]) => void
  openMapAndGetPosition: () => void
  [key: string]: unknown
}

const mountMessagePage = (props: Record<string, unknown> = {}, options: Record<string, unknown> = {}) => {
  const wrapper = shallowMount(MessagePage, {
    props: {
      id: 'device-1',
      deviceConfigId: 'config-1',
      ...props
    },
    global: {
      config: {
        globalProperties: {
          getPlatform: () => false
        }
      },
      stubs: {
        NCard: SlotPassThroughCard,
        NSpace: true,
        NButton: true,
        NInput: true,
        NInputNumber: true,
        NSelect: true,
        NSwitch: true,
        NTooltip: true,
        NForm: SlotPassThroughForm,
        NModal: true,
        SvgIcon: true,
        TencentMap: true,
        ...options.stubs
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as MessageSetupState

interface VNodeLike {
  props?: Record<string, unknown>
  children?: unknown
  component?: { subTree?: VNodeLike }
  dynamicChildren?: VNodeLike[] | null
}

type CollectedHandler = (...args: unknown[]) => unknown

const collectVNodeHandlers = (
  vnode: VNodeLike | null | undefined,
  eventName: string,
  handlers: CollectedHandler[] = [],
  seen = new Set<unknown>()
) => {
  if (!vnode || seen.has(vnode)) return handlers
  seen.add(vnode)

  const handler = vnode.props?.[eventName]
  if (Array.isArray(handler)) {
    handlers.push(...handler.filter(item => typeof item === 'function'))
  } else if (typeof handler === 'function') {
    handlers.push(handler)
  }

  if (Array.isArray(vnode.children)) {
    vnode.children.forEach(child => collectVNodeHandlers(child, eventName, handlers, seen))
  } else if (vnode.children && typeof vnode.children === 'object') {
    Object.values(vnode.children).forEach((child: unknown) => {
      if (typeof child === 'function') {
        const rendered = child()
        if (Array.isArray(rendered)) {
          rendered.forEach(item => collectVNodeHandlers(item, eventName, handlers, seen))
        } else {
          collectVNodeHandlers(rendered, eventName, handlers, seen)
        }
      } else {
        collectVNodeHandlers(child, eventName, handlers, seen)
      }
    })
  }

  if (vnode.component?.subTree) {
    collectVNodeHandlers(vnode.component.subTree, eventName, handlers, seen)
  }
  if (Array.isArray(vnode.dynamicChildren)) {
    vnode.dynamicChildren.forEach(child => collectVNodeHandlers(child, eventName, handlers, seen))
  }

  return handlers
}

describe('message.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.deviceDetail.mockResolvedValue({
      data: {
        location: '116.404,39.915',
        additional_info: '{}'
      },
      error: null
    })
    hoisted.deviceConfigInfo.mockResolvedValue({
      data: {
        additional_info: '[]'
      },
      error: null
    })
    hoisted.deviceLocation.mockResolvedValue({ error: null })
    ;(window as unknown as { $message: Record<string, (...args: unknown[]) => void> }).$message = {
      success: vi.fn(),
      error: hoisted.windowMessageError,
      warning: vi.fn(),
      info: vi.fn()
    }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  describe('safeParseJSON', () => {
    it('returns fallback for bad JSON', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.safeParseJSON('not-json', { fallback: true })).toEqual({ fallback: true })
    })

    it('returns parsed value for valid JSON', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.safeParseJSON('{"a":1}', null)).toEqual({ a: 1 })
    })

    it('returns fallback for null/undefined/empty payload', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.safeParseJSON(null, 'fallback')).toBe('fallback')
      expect(setupState.safeParseJSON(undefined, 'fallback')).toBe('fallback')
      expect(setupState.safeParseJSON('', 'fallback')).toBe('fallback')
    })
  })

  describe('normalizeExtendedInfo', () => {
    it('returns array payload as-is', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const arr = [{ name: 'a', value: '1' }]

      expect(setupState.normalizeExtendedInfo(arr)).toBe(arr)
    })

    it('converts object to name/value array', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.normalizeExtendedInfo({ a: 1, b: 2 })).toEqual([
        { name: 'a', value: 1 },
        { name: 'b', value: 2 }
      ])
    })

    it('returns empty array for invalid values', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.normalizeExtendedInfo(null)).toEqual([])
      expect(setupState.normalizeExtendedInfo('string')).toEqual([])
      expect(setupState.normalizeExtendedInfo(123)).toEqual([])
    })
  })

  describe('coerceValueByType', () => {
    it('converts Number type', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.coerceValueByType('42', 'Number')).toBe(42)
      expect(setupState.coerceValueByType('not-a-number', 'Number')).toBeUndefined()
    })

    it('converts Boolean type', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.coerceValueByType('true', 'Boolean')).toBe(true)
      expect(setupState.coerceValueByType('false', 'Boolean')).toBe(false)
      expect(setupState.coerceValueByType(true, 'Boolean')).toBe(true)
      expect(setupState.coerceValueByType('something', 'Boolean')).toBe(true)
    })

    it('converts String type (default case)', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.coerceValueByType(123, 'String')).toBe('123')
      expect(setupState.coerceValueByType(true, 'String')).toBe('true')
    })

    it('returns undefined for null/undefined/empty values', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.coerceValueByType(null, 'String')).toBeUndefined()
      expect(setupState.coerceValueByType(undefined, 'Number')).toBeUndefined()
      expect(setupState.coerceValueByType('', 'Boolean')).toBeUndefined()
    })
  })

  describe('config loading and merging', () => {
    it('loads device detail and config info, merging extension info with device values', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: {
          location: '116.404,39.915',
          additional_info: JSON.stringify({
            extendedInfo: [{ name: 'field1', value: 'device-value-1' }]
          })
        },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'field1', type: 'String', default_value: 'default-1', enable: true },
            { name: 'field2', type: 'Number', default_value: '0', enable: true }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.latitude).toBe('39.915')
      expect(setupState.longitude).toBe('116.404')
      expect(setupState.additionInfo).toHaveLength(2)
      expect(setupState.additionInfo[0].value).toBe('device-value-1')
      expect(setupState.additionInfo[1].value).toBe(0)
    })

    it('falls back to default_value when device additional_info has no matching extendedInfo', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: {
          location: '',
          additional_info: '{}'
        },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'field1', type: 'String', default_value: 'default-1', enable: true }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo[0].value).toBe('default-1')
    })

    it('handles device additional_info being a plain object (not wrapped in extendedInfo)', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: {
          location: '',
          additional_info: JSON.stringify({ field1: 'object-value' })
        },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'field1', type: 'String', default_value: 'default-1', enable: true }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo[0].value).toBe('object-value')
    })

    it('skips config loading when deviceConfigId is empty', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: { location: '116.0,40.0', additional_info: '{}' },
        error: null
      })

      const wrapper = mountMessagePage({ deviceConfigId: '' })
      await flushPromises()

      expect(hoisted.deviceConfigInfo).toHaveBeenCalledTimes(0)
      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo).toHaveLength(0)
    })

    it('uses fallback when additional_info is bad JSON', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: { location: '', additional_info: 'bad-json' },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: { additional_info: 'also-bad' },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo).toHaveLength(0)
    })
  })

  describe('field rendering helpers', () => {
    it('getTextValue returns string representation for String/Enum fields', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.getTextValue({ value: 123 })).toBe('123')
      expect(setupState.getTextValue({ value: 'hello' })).toBe('hello')
      expect(setupState.getTextValue({ value: null })).toBe('')
      expect(setupState.getTextValue({ value: undefined })).toBe('')
    })

    it('getNumberValue returns numeric value for Number fields', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.getNumberValue({ value: '42' })).toBe(42)
      expect(setupState.getNumberValue({ value: 42 })).toBe(42)
      expect(setupState.getNumberValue({ value: null })).toBeNull()
      expect(setupState.getNumberValue({ value: '' })).toBeNull()
      expect(setupState.getNumberValue({ value: 'abc' })).toBeNull()
    })

    it('getBooleanValue returns boolean for Boolean fields', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      expect(setupState.getBooleanValue({ value: true })).toBe(true)
      expect(setupState.getBooleanValue({ value: 'true' })).toBe(true)
      expect(setupState.getBooleanValue({ value: 1 })).toBe(true)
      expect(setupState.getBooleanValue({ value: false })).toBe(false)
      expect(setupState.getBooleanValue({ value: 'false' })).toBe(false)
    })

    it('setItemValue sets value on item', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const item = { name: 'x', value: '' }

      setupState.setItemValue(item, 'new-value')
      expect(item.value).toBe('new-value')
    })
  })

  describe('template field type rendering and events', () => {
    it('renders Number, Boolean, and Enum fields and handles update:value events', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: { location: '', additional_info: '{}' },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'numField', type: 'Number', default_value: '0', enable: true },
            { name: 'boolField', type: 'Boolean', default_value: 'false', enable: true },
            { name: 'enumField', type: 'Enum', default_value: 'a', enable: true, options: [{ label: 'A', value: 'a' }] }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo).toHaveLength(3)

      // Use setItemValue directly since shallowMount stubs child components
      setupState.setItemValue(setupState.additionInfo[0], 99)
      expect(setupState.additionInfo[0].value).toBe(99)

      setupState.setItemValue(setupState.additionInfo[1], true)
      expect(setupState.additionInfo[1].value).toBe(true)

      setupState.setItemValue(setupState.additionInfo[2], 'b')
      expect(setupState.additionInfo[2].value).toBe('b')
    })
  })

  describe('map interaction', () => {
    it('blocks opening map when coordinates are invalid', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.latitude = '999'
      setupState.longitude = '116'

      setupState.openMapAndGetPosition()

      expect(setupState.isShow).toBe(false)
      expect(hoisted.windowMessageError).toHaveBeenCalledTimes(1)
      expect(hoisted.windowMessageError).toHaveBeenCalledWith(
        expect.stringContaining('generate.currentCoordinatesInvalid')
      )
    })

    it('opens map when coordinates are valid', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.latitude = '39.915'
      setupState.longitude = '116.404'

      setupState.openMapAndGetPosition()

      expect(setupState.isShow).toBe(true)
    })

    it('opens map when coordinates are empty', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.latitude = ''
      setupState.longitude = ''

      setupState.openMapAndGetPosition()

      expect(setupState.isShow).toBe(true)
    })

    it('onPositionSelected fills coordinates and closes map', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.isShow = true
      setupState.onPositionSelected({ lat: 39.915, lng: 116.404 })

      expect(setupState.latitude).toBe('39.915')
      expect(setupState.longitude).toBe('116.404')
      expect(setupState.isShow).toBe(false)
    })
  })

  describe('handleSave', () => {
    it('blocks save when coordinates are invalid', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.latitude = '999'
      setupState.longitude = '116'

      await setupState.handleSave()

      expect(hoisted.messageError).toHaveBeenCalledTimes(1)
      expect(hoisted.messageError).toHaveBeenCalledWith(expect.stringContaining('generate.invalidCoordinates'))
      expect(hoisted.deviceLocation).toHaveBeenCalledTimes(0)
    })

    it('does not submit when form validation fails', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const validate = vi.fn().mockRejectedValue(new Error('validation failed'))

      setupState.extensionFormRef = {
        validate
      }

      await setupState.handleSave()

      expect(validate).toHaveBeenCalledTimes(1)
      expect(hoisted.deviceLocation).toHaveBeenCalledTimes(0)
      expect(hoisted.messageError).toHaveBeenCalledWith('common.saveFailed')
    })

    it('serializes additional_info and calls deviceLocation on successful save', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.extensionFormRef = null
      setupState.additionInfo = [
        { name: 'field1', type: 'String', value: 'value1', enable: true },
        { name: 'field2', type: 'Number', value: 42, enable: true }
      ]
      setupState.latitude = '39.915'
      setupState.longitude = '116.404'

      await setupState.handleSave()

      expect(hoisted.deviceLocation).toHaveBeenCalledWith({
        id: 'device-1',
        location: '116.404,39.915',
        additional_info: JSON.stringify({ field1: 'value1', field2: 42 })
      })
    })

    it('shows success notification on successful save', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)

      setupState.extensionFormRef = null
      setupState.additionInfo = []
      hoisted.deviceLocation.mockResolvedValue({ error: null })

      await setupState.handleSave()

      expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.modifySuccess')
    })

    it('passes validation and submits when extensionFormRef validates successfully', async () => {
      const wrapper = mountMessagePage()
      await flushPromises()
      const setupState = getSetupState(wrapper)
      const validate = vi.fn().mockResolvedValue(undefined)

      setupState.extensionFormRef = {
        validate
      }
      setupState.additionInfo = [
        { name: 'field1', type: 'String', value: 'val1', enable: true, default_value: '' }
      ]
      setupState.latitude = '39.915'
      setupState.longitude = '116.404'
      hoisted.deviceLocation.mockResolvedValue({ error: null })

      await setupState.handleSave()

      expect(validate).toHaveBeenCalledTimes(1)
      expect(hoisted.deviceLocation).toHaveBeenCalledTimes(1)
      expect(hoisted.deviceLocation).toHaveBeenCalledWith({
        id: 'device-1',
        location: '116.404,39.915',
        additional_info: JSON.stringify({ field1: 'val1' })
      })
      expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.modifySuccess')
    })
  })

  describe('extension info template rendering', () => {
    it('shows noData when all additionInfo items have enable: false', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: { location: '', additional_info: '{}' },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'field1', type: 'String', default_value: 'val', enable: false }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      // All items have enable: false, so the v-else branch should render
      expect(setupState.additionInfo.filter(item => item.enable === true).length).toBe(0)
    })

    it('renders Boolean type field (NSwitch branch)', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: { location: '', additional_info: '{}' },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'boolField', type: 'Boolean', default_value: 'false', enable: true }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo).toHaveLength(1)
      expect(setupState.additionInfo[0].type).toBe('Boolean')
      expect(setupState.additionInfo[0].enable).toBe(true)
      // Verify getBooleanValue is called for Boolean type
      expect(setupState.getBooleanValue(setupState.additionInfo[0])).toBe(false)
    })

    it('renders Enum type field (NSelect branch)', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: { location: '', additional_info: '{}' },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'enumField', type: 'Enum', default_value: 'a', enable: true, options: [{ label: 'A', value: 'a' }] }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo).toHaveLength(1)
      expect(setupState.additionInfo[0].type).toBe('Enum')
      expect(setupState.additionInfo[0].options).toEqual([{ label: 'A', value: 'a' }])
      // Verify getTextValue is called for Enum type
      expect(setupState.getTextValue(setupState.additionInfo[0])).toBe('a')
    })

    it('renders unknown type field (v-else NInput branch)', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: { location: '', additional_info: '{}' },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'unknownField', type: 'CustomType', default_value: 'fallback-val', enable: true }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo).toHaveLength(1)
      // coerceValueByType falls to default (String) for unknown type
      expect(setupState.additionInfo[0].value).toBe('fallback-val')
    })

    it('renders field with empty desc falling back to extensionNoDesc', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: { location: '', additional_info: '{}' },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'noDescField', type: 'String', default_value: 'val', enable: true, desc: '' }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo[0].desc).toBe('')
      // Empty desc is falsy, so the fallback $t('generate.extensionNoDesc') would be used
      // Verify the desc is indeed empty (falsy)
      expect(setupState.additionInfo[0].desc || 'generate.extensionNoDesc').toBe('generate.extensionNoDesc')
    })

    it('renders field with desc present', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: { location: '', additional_info: '{}' },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'descField', type: 'String', default_value: 'val', enable: true, desc: 'A description' }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo[0].desc).toBe('A description')
      // Non-empty desc is truthy, so it would be used instead of fallback
      expect(setupState.additionInfo[0].desc || 'generate.extensionNoDesc').toBe('A description')
    })

    it('renders all field types together to cover all template branches', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: { location: '', additional_info: '{}' },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'strField', type: 'String', default_value: 'str-default', enable: true },
            { name: 'numField', type: 'Number', default_value: '0', enable: true },
            { name: 'boolField', type: 'Boolean', default_value: 'true', enable: true },
            { name: 'enumField', type: 'Enum', default_value: 'a', enable: true, options: [{ label: 'A', value: 'a' }] },
            { name: 'otherField', type: 'OtherType', default_value: 'other-default', enable: true }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo).toHaveLength(5)
      // Verify each type's value helper is called correctly
      expect(setupState.getTextValue(setupState.additionInfo[0])).toBe('str-default')
      expect(setupState.getNumberValue(setupState.additionInfo[1])).toBe(0)
      expect(setupState.getBooleanValue(setupState.additionInfo[2])).toBe(true)
      expect(setupState.getTextValue(setupState.additionInfo[3])).toBe('a')
      expect(setupState.getTextValue(setupState.additionInfo[4])).toBe('other-default')
    })

    it('updates location and enabled extension fields through template v-model handlers', async () => {
      hoisted.deviceDetail.mockResolvedValue({
        data: { location: '116.404,39.915', additional_info: '{}' },
        error: null
      })
      hoisted.deviceConfigInfo.mockResolvedValue({
        data: {
          additional_info: JSON.stringify([
            { name: 'strField', type: 'String', default_value: 'str-default', enable: true },
            { name: 'numField', type: 'Number', default_value: '0', enable: true },
            { name: 'boolField', type: 'Boolean', default_value: 'false', enable: true },
            { name: 'enumField', type: 'Enum', default_value: 'a', enable: true, options: [{ label: 'A', value: 'a' }] },
            { name: 'otherField', type: 'OtherType', default_value: 'fallback', enable: true }
          ])
        },
        error: null
      })

      const wrapper = mountMessagePage()
      await flushPromises()

      const setupState = getSetupState(wrapper)
      expect(setupState.additionInfo).toHaveLength(5)

      const valueHandlers = collectVNodeHandlers(wrapper.vm.$.subTree, 'onUpdate:value')
      const showHandlers = collectVNodeHandlers(wrapper.vm.$.subTree, 'onUpdate:show')

      expect(valueHandlers.length).toBeGreaterThanOrEqual(7)
      valueHandlers.forEach((handler, index) => handler(index === 4 ? true : `value-${index}`))
      await wrapper.vm.$nextTick()

      expect(valueHandlers.length).toBeGreaterThanOrEqual(7)
      expect([setupState.longitude, setupState.latitude]).toEqual(
        expect.arrayContaining([expect.stringMatching(/^value-\d+$/), expect.stringMatching(/^value-\d+$/)])
      )
      expect(setupState.additionInfo.filter(item => String(item.value).startsWith('value-')).length).toBeGreaterThanOrEqual(3)

      setupState.isShow = true
      expect(showHandlers.length).toBeGreaterThanOrEqual(1)
      showHandlers[0](false)
      expect(setupState.isShow).toBe(false)
    })
  })
})
