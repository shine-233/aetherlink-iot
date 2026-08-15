import { defineComponent, h, nextTick, reactive } from 'vue'
import { flushPromises, shallowMount, type VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  execute: vi.fn(),
  getDashboard: vi.fn(),
  updateDashboard: vi.fn(),
  routerPushByKey: vi.fn(),
  message: { error: vi.fn(), success: vi.fn() },
  route: undefined as unknown as { query: Record<string, unknown> },
  userInfo: undefined as unknown as { authority: string; roles: string[] }
}))

vi.mock('vue-router', () => ({ useRoute: () => hoisted.route }))
vi.mock('@/service/visualization-provider/composition', () => ({
  getDefaultVisualizationProviderFacade: () => ({ execute: hoisted.execute })
}))
vi.mock('@/hooks/common/router', () => ({ useRouterPush: () => ({ routerPushByKey: hoisted.routerPushByKey }) }))
vi.mock('@/store/modules/auth', () => ({ useAuthStore: () => ({ userInfo: hoisted.userInfo }) }))
vi.mock('@/locales', () => ({ $t: (key: string) => key }))
vi.mock('naive-ui', () => {
  const container = (name: string) =>
    defineComponent({ name, inheritAttrs: false, setup(_, { attrs, slots }) { return () => h('div', attrs, slots.default?.()) } })
  const button = defineComponent({
    name: 'NButton', inheritAttrs: false,
    setup(_, { attrs, slots }) { return () => h('button', attrs, slots.default?.()) }
  })
  const input = defineComponent({
    name: 'NInput', props: ['value'], emits: ['update:value'], inheritAttrs: false,
    setup(props, { attrs, emit }) {
      return () => h('input', { ...attrs, value: props.value, onInput: (event: Event) => emit('update:value', (event.target as HTMLInputElement).value) })
    }
  })
  return {
    createDiscreteApi: () => ({ message: hoisted.message, notification: {}, dialog: {}, loadingBar: {} }),
    NButton: button,
    NCard: container('NCard'),
    NInput: input,
    NInputNumber: container('NInputNumber'),
    NSelect: container('NSelect'),
    NSpin: container('NSpin'),
    useMessage: () => hoisted.message
  }
})

import NativeBoardEditor from '../index.vue'

const dashboard = { version: 1 as const, columns: 24, rowHeight: 60, widgets: [] }
const board = (id = 'board-1', overrides: Record<string, unknown> = {}) => ({
  id,
  name: 'Native board',
  description: 'Description',
  thumbnail: null,
  version: 1,
  canvasConfig: { mode: 'responsive', width: 1920, height: 1080, background: null },
  nodes: [],
  dataSources: [],
  rendererData: dashboard,
  published: false,
  publishedAt: null,
  shareToken: null,
  projectId: 'native-boards',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
  ...overrides
})
const success = (data = board()) => ({ ok: true as const, data })
const wrappers: VueWrapper[] = []

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

function mountPage(id: unknown = ' board-1 ', authority = 'SYS_ADMIN', roles: string[] = [], hasId = true) {
  hoisted.route = reactive({ query: hasId ? { id } : {} })
  hoisted.userInfo = reactive({ authority, roles })
  const wrapper = shallowMount(NativeBoardEditor)
  wrappers.push(wrapper)
  return wrapper
}

function vm(wrapper: VueWrapper) {
  return wrapper.vm as unknown as {
    boardName: string
    boardDescription: string
    dashboard: typeof dashboard | null
    saving: boolean
    failed: boolean
    handleSave: () => Promise<void>
  }
}

describe('native board editor page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.getDashboard.mockResolvedValue(success())
    hoisted.updateDashboard.mockResolvedValue(success())
    hoisted.execute.mockImplementation((operation: (provider: {
      getDashboard: typeof hoisted.getDashboard
      updateDashboard: typeof hoisted.updateDashboard
    }) => unknown) => operation({ getDashboard: hoisted.getDashboard, updateDashboard: hoisted.updateDashboard }))
  })
  afterEach(() => { wrappers.forEach(wrapper => wrapper.unmount()); wrappers.length = 0 })

  it('loads a trimmed native board through the provider and renders a readonly safe preview', async () => {
    const wrapper = mountPage()
    await flushPromises()
    expect(hoisted.getDashboard).toHaveBeenCalledWith('board-1')
    expect(vm(wrapper).boardName).toBe('Native board')
    expect(vm(wrapper).boardDescription).toBe('Description')
    const preview = wrapper.findComponent({ name: 'LocalVisualizationViewer' })
    expect(preview.props('dashboard')).toEqual(dashboard)
    expect(preview.props('fields')).toEqual({})
    expect(wrapper.find('textarea').exists()).toBe(false)
  })

  it.each([
    ['missing id', undefined, 'SYS_ADMIN', [], false],
    ['array id', ['board-1'], 'SYS_ADMIN', [], true],
    ['blank id', '  ', 'SYS_ADMIN', [], true],
    ['tenant user', 'board-1', 'TENANT_USER', [], true]
  ])('fails closed without a provider request for %s', async (_label, id, authority, roles, hasId) => {
    const wrapper = mountPage(id, authority, roles as string[], hasId as boolean)
    await flushPromises()
    expect(hoisted.execute).not.toHaveBeenCalled()
    expect(hoisted.getDashboard).not.toHaveBeenCalled()
    expect(vm(wrapper).failed).toBe(true)
  })

  it.each([
    ['provider error', { ok: false, error: { code: 'provider-failure', message: 'failed' } }],
    ['wrong id', success(board('other'))],
    ['missing renderer data', success(board('board-1', { rendererData: undefined }))],
    ['invalid renderer data', success(board('board-1', { rendererData: '{' }))],
    ['unsupported widget', success(board('board-1', { rendererData: { version: 1, widgets: [{ id: 'x', x: 0, y: 0, w: 1, h: 1, type: 'future', config: {} }] } }))]
  ])('fails closed for %s', async (_label, response) => {
    hoisted.getDashboard.mockResolvedValue(response)
    const wrapper = mountPage()
    await flushPromises()
    expect(vm(wrapper).failed).toBe(true)
  })

  it('does not let an older result overwrite a newer route request', async () => {
    const older = deferred<ReturnType<typeof success>>()
    hoisted.getDashboard.mockReturnValueOnce(older.promise).mockResolvedValueOnce(success(board('board-2', { name: 'Newer' })))
    const wrapper = mountPage('board-1')
    hoisted.route.query.id = 'board-2'
    await nextTick(); await flushPromises()
    older.resolve(success(board('board-1', { name: 'Older' })))
    await flushPromises()
    expect(vm(wrapper).boardName).toBe('Newer')
  })

  it('submits canonical renderer data and neutral metadata through the provider', async () => {
    const wrapper = mountPage()
    await flushPromises()
    vm(wrapper).boardName = '  Renamed  '
    vm(wrapper).boardDescription = 'Updated description'
    hoisted.updateDashboard.mockResolvedValue(success(board('board-1', { name: 'Renamed', description: 'Updated description' })))
    await vm(wrapper).handleSave()

    expect(hoisted.updateDashboard).toHaveBeenCalledWith('board-1', {
      name: 'Renamed',
      description: 'Updated description',
      rendererData: dashboard
    })
    expect(hoisted.routerPushByKey).toHaveBeenCalledWith('visualization_native-board', { query: { id: 'board-1' } })
  })

  it.each(['', '   ', 'x'.repeat(256)])('rejects invalid name %j', async name => {
    const wrapper = mountPage(); await flushPromises(); vm(wrapper).boardName = name
    await vm(wrapper).handleSave()
    expect(hoisted.updateDashboard).not.toHaveBeenCalled()
  })

  it('rejects an overlong description', async () => {
    const wrapper = mountPage(); await flushPromises(); vm(wrapper).boardDescription = 'x'.repeat(501)
    await vm(wrapper).handleSave()
    expect(hoisted.updateDashboard).not.toHaveBeenCalled()
  })

  it('prevents duplicate save submissions', async () => {
    const pending = deferred<ReturnType<typeof success>>()
    hoisted.updateDashboard.mockReturnValue(pending.promise)
    const wrapper = mountPage(); await flushPromises()
    const first = vm(wrapper).handleSave(); const second = vm(wrapper).handleSave()
    expect(hoisted.updateDashboard).toHaveBeenCalledTimes(1)
    pending.resolve(success()); await first; await second
  })

  it.each([
    ['provider error', { ok: false, error: { code: 'provider-failure', message: 'failed' } }],
    ['wrong id', success(board('other'))]
  ])('stays on editor for invalid save response: %s', async (_label, response) => {
    hoisted.updateDashboard.mockResolvedValue(response)
    const wrapper = mountPage(); await flushPromises(); await vm(wrapper).handleSave()
    expect(hoisted.routerPushByKey).not.toHaveBeenCalled()
    expect(hoisted.message.error).toHaveBeenCalled()
  })
})
