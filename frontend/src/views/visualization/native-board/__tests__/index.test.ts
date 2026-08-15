import { defineComponent, h, nextTick, reactive } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  execute: vi.fn(),
  getDashboard: vi.fn(),
  normalizeLocalDashboard: vi.fn(),
  route: undefined as unknown as { query: Record<string, unknown> }
}))

vi.mock('vue-router', () => ({
  useRoute: () => hoisted.route
}))

vi.mock('@/service/visualization-provider/composition', () => ({
  getDefaultVisualizationProviderFacade: () => ({ execute: hoisted.execute })
}))

vi.mock('@/components/local-visualization-viewer', () => ({
  normalizeLocalDashboard: hoisted.normalizeLocalDashboard,
  LocalVisualizationViewer: defineComponent({
    name: 'LocalVisualizationViewer',
    props: ['dashboard', 'fields'],
    setup(props) {
      return () => h('div', { 'data-dashboard': JSON.stringify(props.dashboard) })
    }
  })
}))

import NativeBoard from '../index.vue'

const validDashboard = {
  version: 1,
  columns: 12,
  rowHeight: 60,
  widgets: []
}

const schema = (id: string, rendererData: unknown = validDashboard) => ({
  id,
  name: 'Native board',
  description: 'Description',
  thumbnail: null,
  version: 1,
  canvasConfig: { mode: 'fixed', width: 1920, height: 1080, background: null },
  nodes: [],
  dataSources: [],
  rendererData,
  published: false,
  publishedAt: null,
  shareToken: null,
  projectId: 'native',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z'
})

const success = (id: string, rendererData: unknown = validDashboard) => ({
  ok: true as const,
  data: schema(id, rendererData)
})

const wrappers: Array<ReturnType<typeof shallowMount>> = []

function mountPage(id: unknown = ' board-1 ', hasId = true) {
  hoisted.route = reactive({ query: hasId ? { id } : {} })
  const wrapper = shallowMount(NativeBoard)
  wrappers.push(wrapper)
  return wrapper
}

function viewer(wrapper: ReturnType<typeof shallowMount>) {
  return wrapper.findComponent({ name: 'LocalVisualizationViewer' })
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

describe('native board viewer page', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.normalizeLocalDashboard.mockReturnValue({ ok: true, dashboard: validDashboard })
    hoisted.getDashboard.mockResolvedValue(success('board-1'))
    hoisted.execute.mockImplementation((operation: (provider: { getDashboard: typeof hoisted.getDashboard }) => unknown) =>
      operation({ getDashboard: hoisted.getDashboard })
    )
  })

  afterEach(() => {
    wrappers.forEach(wrapper => wrapper.unmount())
    wrappers.length = 0
  })

  it('loads a trimmed ID through the provider facade and renders its original renderer data', async () => {
    const rendererData = { ...validDashboard, columns: 8 }
    const normalized = { ...rendererData, columns: 24 }
    hoisted.getDashboard.mockResolvedValue(success('board-1', rendererData))
    hoisted.normalizeLocalDashboard.mockReturnValue({ ok: true, dashboard: normalized })

    const wrapper = mountPage()
    await flushPromises()

    expect(hoisted.execute).toHaveBeenCalledOnce()
    expect(hoisted.getDashboard).toHaveBeenCalledWith('board-1')
    expect(hoisted.normalizeLocalDashboard).toHaveBeenCalledWith(rendererData)
    expect(viewer(wrapper).props('dashboard')).toEqual(rendererData)
    expect(viewer(wrapper).props('dashboard')).not.toBe(normalized)
    expect(viewer(wrapper).props('fields')).toEqual({})
  })

  it.each([
    ['missing', undefined],
    ['array', ['board-1']],
    ['blank', '   ']
  ])('rejects a %s route query ID without invoking the provider', async (label, id) => {
    const wrapper = mountPage(id, label !== 'missing')
    await flushPromises()

    expect(hoisted.execute).not.toHaveBeenCalled()
    expect(hoisted.getDashboard).not.toHaveBeenCalled()
    expect(viewer(wrapper).exists()).toBe(false)
    expect(wrapper.text()).toContain('Unable to load dashboard')
  })

  it('fails closed for a provider error result', async () => {
    hoisted.getDashboard.mockResolvedValue({
      ok: false,
      error: { code: 'provider-unavailable', message: 'disabled' }
    })
    const wrapper = mountPage()
    await flushPromises()

    expect(viewer(wrapper).exists()).toBe(false)
    expect(wrapper.text()).toContain('Unable to load dashboard')
  })

  it('fails closed when facade execution throws', async () => {
    hoisted.execute.mockRejectedValue(new Error('failed'))
    const wrapper = mountPage()
    await flushPromises()

    expect(viewer(wrapper).exists()).toBe(false)
    expect(wrapper.text()).toContain('Unable to load dashboard')
  })

  it('rejects a provider response without renderer data', async () => {
    const result = success('board-1')
    delete result.data.rendererData
    hoisted.getDashboard.mockResolvedValue(result)
    const wrapper = mountPage()
    await flushPromises()

    expect(viewer(wrapper).exists()).toBe(false)
    expect(hoisted.normalizeLocalDashboard).not.toHaveBeenCalled()
  })

  it('rejects renderer data rejected by the local dashboard normalizer', async () => {
    hoisted.normalizeLocalDashboard.mockReturnValue({ ok: false, error: 'invalid' })
    const wrapper = mountPage()
    await flushPromises()

    expect(viewer(wrapper).exists()).toBe(false)
    expect(wrapper.text()).toContain('Unable to load dashboard')
  })

  it('ignores an older success that resolves after the newer request', async () => {
    const older = deferred<ReturnType<typeof success>>()
    const newerDashboard = { ...validDashboard, columns: 8 }
    hoisted.getDashboard
      .mockReturnValueOnce(older.promise)
      .mockResolvedValueOnce(success('board-2', newerDashboard))
    const wrapper = mountPage('board-1')

    hoisted.route.query.id = 'board-2'
    await nextTick()
    await flushPromises()
    older.resolve(success('board-1', { ...validDashboard, columns: 6 }))
    await flushPromises()

    expect(viewer(wrapper).props('dashboard')).toEqual(newerDashboard)
    expect(wrapper.text()).not.toContain('Unable to load dashboard')
  })

  it('does not let an older error overwrite a newer success', async () => {
    const older = deferred<ReturnType<typeof success>>()
    hoisted.getDashboard
      .mockReturnValueOnce(older.promise)
      .mockResolvedValueOnce(success('board-2'))
    const wrapper = mountPage('board-1')

    hoisted.route.query.id = 'board-2'
    await nextTick()
    await flushPromises()
    older.reject(new Error('old request failed'))
    await flushPromises()

    expect(viewer(wrapper).exists()).toBe(true)
    expect(wrapper.text()).not.toContain('Unable to load dashboard')
  })

  it('unmounts the previous viewer as soon as a new request starts', async () => {
    const nextRequest = deferred<ReturnType<typeof success>>()
    const wrapper = mountPage('board-1')
    await flushPromises()
    expect(viewer(wrapper).exists()).toBe(true)

    hoisted.getDashboard.mockReturnValueOnce(nextRequest.promise)
    hoisted.route.query.id = 'board-2'
    await nextTick()

    expect(viewer(wrapper).exists()).toBe(false)
    expect(wrapper.text()).toContain('Loading dashboard...')
  })

  it('invalidates an in-flight request when unmounted', async () => {
    const pending = deferred<ReturnType<typeof success>>()
    hoisted.getDashboard.mockReturnValueOnce(pending.promise)
    const wrapper = mountPage('board-1')

    wrapper.unmount()
    pending.resolve(success('board-1'))
    await flushPromises()

    expect(hoisted.normalizeLocalDashboard).not.toHaveBeenCalled()
  })
})
