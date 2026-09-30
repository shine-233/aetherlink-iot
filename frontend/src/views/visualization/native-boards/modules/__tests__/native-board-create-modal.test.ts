/*
 * 文件用途：守护 NativeBoardCreateModal 的创建契约。
 * 核心逻辑：这些用例原挂在 native-boards/__tests__/index.test.ts 上，创建流程在
 *          2026-09-29 的重构中从 index.vue 下沉到本组件（模块化拆分），因此契约随之下沉。
 * 关键注意事项：SYS_ADMIN 必须显式选租户；prefill 由页面在打开时注入，组件只在
 *              show 由 false 变 true 时读取（见组件里的 watch），所以用例都要先 open()。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  execute: vi.fn(),
  createDashboard: vi.fn(),
  message: { error: vi.fn(), success: vi.fn() }
}))

vi.mock('@/service/visualization-provider/composition', () => ({
  getDefaultVisualizationProviderFacade: () => ({ execute: hoisted.execute })
}))

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

vi.mock('naive-ui', () => {
  const container = (name: string) =>
    defineComponent({
      name,
      inheritAttrs: false,
      setup(_, { attrs, slots }) {
        return () => h('div', attrs, [...(slots.default?.() ?? []), ...(slots.footer?.() ?? [])])
      }
    })
  const button = defineComponent({
    name: 'NButton',
    props: ['disabled', 'loading'],
    emits: ['click'],
    inheritAttrs: false,
    setup(_, { attrs, emit, slots }) {
      return () => h('button', { ...attrs, onClick: () => emit('click') }, slots.default?.())
    }
  })
  const input = defineComponent({
    name: 'NInput',
    props: ['value'],
    emits: ['update:value'],
    inheritAttrs: false,
    setup(props, { attrs, emit }) {
      return () =>
        h('input', {
          ...attrs,
          value: props.value,
          onInput: (event: Event) => emit('update:value', (event.target as HTMLInputElement).value)
        })
    }
  })
  const select = defineComponent({
    name: 'NSelect',
    props: ['value', 'options', 'loading'],
    emits: ['update:value'],
    inheritAttrs: false,
    setup(props, { attrs, emit }) {
      return () =>
        h(
          'select',
          {
            ...attrs,
            value: props.value ?? '',
            onChange: (event: Event) => emit('update:value', (event.target as HTMLSelectElement).value)
          },
          ((props.options as Array<{ label: string; value: string }> | undefined) ?? []).map((option) =>
            h('option', { value: option.value }, option.label)
          )
        )
    }
  })
  const modal = defineComponent({
    name: 'NModal',
    props: ['show'],
    emits: ['update:show'],
    inheritAttrs: false,
    setup(props, { attrs, slots }) {
      return () =>
        props.show ? h('div', attrs, [...(slots.default?.() ?? []), ...(slots.footer?.() ?? [])]) : null
    }
  })
  return {
    NButton: button,
    NInput: input,
    NSelect: select,
    NModal: modal,
    NForm: container('NForm'),
    NFormItem: container('NFormItem'),
    useMessage: () => hoisted.message
  }
})

import NativeBoardCreateModal from '../native-board-create-modal.vue'

const success = <T>(data: T) => ({ ok: true as const, data })
const failure = () => ({ ok: false as const, error: { code: 'provider-failure' as const, message: 'failed' } })
const createdResult = (id = 'created-1') => success({ id })

const TENANT_OPTIONS = [
  { label: 'Tenant one (tenant-1)', value: 'tenant-1' },
  { label: 'Tenant two (tenant-2)', value: 'tenant-2' }
]

const wrappers: VueWrapper[] = []

type ModalProps = {
  show: boolean
  isSysAdmin: boolean
  tenantOptions: Array<{ label: string; value: string }>
  loadingTenants: boolean
  prefillTenantId: string | null
}

/** 挂载时 show 固定为 false —— 组件的 prefill 只在 show 由 false 变 true 时生效。 */
function mountModal(overrides: Partial<ModalProps> = {}) {
  const wrapper = mount(NativeBoardCreateModal, {
    props: {
      show: false,
      isSysAdmin: true,
      tenantOptions: TENANT_OPTIONS,
      loadingTenants: false,
      prefillTenantId: 'tenant-1',
      ...overrides
    } as ModalProps
  })
  wrappers.push(wrapper)
  return wrapper
}

async function open(wrapper: VueWrapper) {
  await wrapper.setProps({ show: true })
  await flushPromises()
}

function vm(wrapper: VueWrapper) {
  return wrapper.vm as unknown as {
    createForm: { name: string; description: string }
    createTenantId: string | null
    creating: boolean
    handleCreate: () => Promise<void>
  }
}

describe('NativeBoardCreateModal', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.createDashboard.mockResolvedValue(createdResult())
    hoisted.execute.mockImplementation(
      (operation: (provider: { createDashboard: typeof hoisted.createDashboard }) => unknown) =>
        operation({ createDashboard: hoisted.createDashboard })
    )
  })

  afterEach(() => {
    wrappers.forEach((wrapper) => wrapper.unmount())
    wrappers.length = 0
  })

  it('creates through the neutral provider contract with the prefilled tenant', async () => {
    const wrapper = mountModal()
    await open(wrapper)

    vm(wrapper).createForm.name = '  New board  '
    vm(wrapper).createForm.description = ''
    await vm(wrapper).handleCreate()

    expect(hoisted.createDashboard).toHaveBeenCalledWith({
      name: 'New board',
      description: '',
      projectId: 'native-boards',
      rendererData: { version: 1, columns: 24, rowHeight: 60, widgets: [] },
      tenantId: 'tenant-1'
    })
    expect(wrapper.emitted('created')).toEqual([['created-1']])
    expect(wrapper.emitted('update:show')?.at(-1)).toEqual([false])
  })

  it('omits the tenant for non-admin users', async () => {
    const wrapper = mountModal({ isSysAdmin: false })
    await open(wrapper)

    vm(wrapper).createForm.name = 'Valid'
    await vm(wrapper).handleCreate()

    expect(hoisted.createDashboard).toHaveBeenCalledTimes(1)
    expect(hoisted.createDashboard.mock.calls[0][0]).not.toHaveProperty('tenantId')
  })

  it('requires an explicit tenant for SYS_ADMIN when more than one tenant is available', async () => {
    const wrapper = mountModal({ prefillTenantId: null })
    await open(wrapper)

    vm(wrapper).createForm.name = 'Valid'
    await vm(wrapper).handleCreate()

    expect(hoisted.createDashboard).not.toHaveBeenCalled()
    expect(wrapper.emitted('update:show')).toBeUndefined()
    expect(hoisted.message.error).toHaveBeenCalledWith('Select a tenant before creating a native board')
  })

  it('does not carry a tenant over when reopened without a prefill', async () => {
    const wrapper = mountModal()
    await open(wrapper)
    expect(vm(wrapper).createTenantId).toBe('tenant-1')

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, prefillTenantId: null })
    await flushPromises()

    expect(vm(wrapper).createTenantId).toBeNull()
  })

  it.each(['', '   ', 'x'.repeat(256)])('rejects invalid trimmed name %j', async (name) => {
    const wrapper = mountModal()
    await open(wrapper)

    vm(wrapper).createForm.name = name
    await vm(wrapper).handleCreate()

    expect(hoisted.createDashboard).not.toHaveBeenCalled()
    expect(hoisted.message.error).toHaveBeenCalledWith('custom.nativeBoards.nameInvalid')
  })

  it('rejects descriptions over 500 characters', async () => {
    const wrapper = mountModal()
    await open(wrapper)

    vm(wrapper).createForm.name = 'Valid'
    vm(wrapper).createForm.description = 'x'.repeat(501)
    await vm(wrapper).handleCreate()

    expect(hoisted.createDashboard).not.toHaveBeenCalled()
    expect(hoisted.message.error).toHaveBeenCalledWith('custom.nativeBoards.descriptionInvalid')
  })

  it('prevents duplicate create submissions', async () => {
    let resolve!: (value: ReturnType<typeof createdResult>) => void
    hoisted.createDashboard.mockReturnValue(
      new Promise<ReturnType<typeof createdResult>>((res) => {
        resolve = res
      })
    )

    const wrapper = mountModal()
    await open(wrapper)
    vm(wrapper).createForm.name = 'Valid'

    const first = vm(wrapper).handleCreate()
    const second = vm(wrapper).handleCreate()
    expect(hoisted.createDashboard).toHaveBeenCalledTimes(1)

    resolve(createdResult())
    await Promise.all([first, second])
  })

  it.each([
    ['provider failure', failure()],
    ['blank ID', createdResult(' ')]
  ])('keeps the create modal for %s', async (_label, result) => {
    hoisted.createDashboard.mockResolvedValue(result)
    const wrapper = mountModal()
    await open(wrapper)

    vm(wrapper).createForm.name = 'Valid'
    await vm(wrapper).handleCreate()

    expect(wrapper.emitted('created')).toBeUndefined()
    expect(wrapper.emitted('update:show')).toBeUndefined()
    expect(hoisted.message.error).toHaveBeenCalledWith('custom.nativeBoards.createFailed')
  })
})
