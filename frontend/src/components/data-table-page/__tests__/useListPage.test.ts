import { computed, effectScope, nextTick, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import {
  buildTableColumns,
  createLazyOptionsLoader,
  emptySearchValue,
  fromFlatResponse,
  normalizeListResponse,
  rowKeySignature,
  serializeDates,
  useListPage
} from '../useListPage'
import type { ListBaseColumn, ListColumnSpec } from '../useListPage'

type Row = { id: string }

function deferred<T>() {
  let resolve!: (v: T) => void
  const promise = new Promise<T>((r) => {
    resolve = r
  })
  return { promise, resolve }
}

describe('useListPage', () => {
  it('drops stale responses and aborts the previous request', async () => {
    const first = deferred<{ list: Row[]; total: number }>()
    const second = deferred<{ list: Row[]; total: number }>()
    const signals: AbortSignal[] = []
    const fetcher = vi
      .fn()
      .mockImplementationOnce((_p, ctx) => {
        signals.push(ctx.signal)
        return first.promise
      })
      .mockImplementationOnce((_p, ctx) => {
        signals.push(ctx.signal)
        return second.promise
      })
    const list = useListPage<Row>({ fetcher })
    const p1 = list.load()
    const p2 = list.setPage(2)
    expect(signals[0].aborted).toBe(true)
    second.resolve({ list: [{ id: 'b' }], total: 30 })
    await p2
    first.resolve({ list: [{ id: 'a' }], total: 1 })
    expect(await p1).toBe(false)
    expect(list.rows.value).toEqual([{ id: 'b' }])
    expect(list.total.value).toBe(30)
    expect(list.loading.value).toBe(false)
    expect(fetcher.mock.calls[1][0]).toMatchObject({ page: 2, page_size: 10 })
  })

  it('search resets page, pageSize change resets page, reset restores filters', async () => {
    const fetcher = vi.fn().mockResolvedValue({ list: [{ id: 'x' }], total: 100 })
    const list = useListPage<Row, { name: string }>({ fetcher, initialQuery: () => ({ name: '' }) })
    await list.setPage(3)
    list.query.name = 'abc'
    await list.search()
    expect(fetcher.mock.lastCall![0]).toEqual({ name: 'abc', page: 1, page_size: 10 })
    await list.setPage(4)
    await list.setPageSize(50)
    expect(fetcher.mock.lastCall![0]).toMatchObject({ page: 1, page_size: 50 })
    await list.reset()
    expect(list.query.name).toBe('')
  })

  it('keeps rows when the fetcher reports failure', async () => {
    const fetcher = vi.fn().mockResolvedValueOnce({ list: [{ id: 'a' }], total: 1 }).mockResolvedValueOnce(null)
    const list = useListPage<Row>({ fetcher })
    await list.load()
    expect(await list.load()).toBe(false)
    expect(list.rows.value).toEqual([{ id: 'a' }])
  })

  it('keeps the current rows visible while a reload is in flight', async () => {
    const pending = deferred<{ list: Row[]; total: number }>()
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce({ list: [{ id: 'a' }], total: 1 })
      .mockImplementationOnce(() => pending.promise)
    const list = useListPage<Row>({ fetcher })

    await list.load()
    const reload = list.load()

    // load() 刻意不清空 rows（避免翻页/刷新时列表闪空）；清空是 clear() 的职责。
    expect(list.rows.value).toEqual([{ id: 'a' }])
    expect(list.total.value).toBe(1)
    expect(list.loading.value).toBe(true)

    pending.resolve({ list: [{ id: 'b' }], total: 2 })
    expect(await reload).toBe(true)
    expect(list.rows.value).toEqual([{ id: 'b' }])
    expect(list.loading.value).toBe(false)
  })

  it('steps back when the current page no longer exists', async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce({ list: [], total: 10 })
      .mockResolvedValueOnce({ list: [{ id: 'z' }], total: 10 })
    const list = useListPage<Row>({ fetcher, initialPage: 2 })
    await list.load()
    expect(list.page.value).toBe(1)
    expect(list.rows.value).toEqual([{ id: 'z' }])
  })

  it('prunes selection to rows still visible', async () => {
    const fetcher = vi
      .fn()
      .mockResolvedValueOnce({ list: [{ id: 'a' }, { id: 'b' }], total: 2 })
      .mockResolvedValueOnce({ list: [{ id: 'b' }], total: 1 })
    const list = useListPage<Row>({ fetcher })
    await list.load()
    list.setSelectedKeys(['a', 'b'])
    expect(list.selectedRows.value).toHaveLength(2)
    await list.load()
    expect(list.selectedKeys.value).toEqual(['b'])
  })

  it('reads and writes route query when routeSync is on', async () => {
    const replace = vi.fn()
    const route = { query: { page: '3', page_size: '20', search: 'dev', other: 'keep' } as Record<string, unknown> }
    const fetcher = vi.fn().mockResolvedValue({ list: [{ id: 'a' }], total: 100 })
    const list = useListPage<Row, { search: string }>({
      fetcher,
      initialQuery: () => ({ search: '' }),
      routeSync: { route, router: { replace }, keys: ['search'] }
    })
    expect(list.page.value).toBe(3)
    expect(list.pageSize.value).toBe(20)
    expect(list.query.search).toBe('dev')
    await list.load()
    expect(replace).not.toHaveBeenCalled()
    list.query.search = ''
    await list.search()
    expect(replace).toHaveBeenLastCalledWith({ query: { page_size: '20', other: 'keep' } })
  })

  it('cancels in-flight work when the scope is disposed', async () => {
    const pending = deferred<{ list: Row[]; total: number }>()
    let signal: AbortSignal | undefined
    const scope = effectScope()
    const list = scope.run(() =>
      useListPage<Row>({
        fetcher: (_p, ctx) => {
          signal = ctx.signal
          return pending.promise
        }
      })
    )!
    const p = list.load()
    scope.stop()
    expect(signal?.aborted).toBe(true)
    pending.resolve({ list: [{ id: 'a' }], total: 1 })
    expect(await p).toBe(false)
    await nextTick()
    expect(list.rows.value).toEqual([])
  })

  it('pagination object stays in sync and drives loads', async () => {
    const fetcher = vi.fn().mockResolvedValue({ list: [{ id: 'a' }], total: 42 })
    const list = useListPage<Row>({ fetcher })
    await list.load()
    expect(list.pagination.itemCount).toBe(42)
    list.pagination.onUpdatePage!(2)
    await Promise.resolve()
    expect(list.pagination.page).toBe(2)
    expect(fetcher.mock.lastCall![0]).toMatchObject({ page: 2 })
  })

  it('helpers normalize shapes and dates', () => {
    expect(normalizeListResponse({ list: [1], total: 5 })).toEqual({ list: [1], total: 5 })
    expect(normalizeListResponse({ data: { list: [1, 2] } })).toEqual({ list: [1, 2], total: 2 })
    expect(normalizeListResponse([1])).toEqual({ list: [1], total: 1 })
    expect(fromFlatResponse({ error: new Error('x') })).toBeNull()
    expect(fromFlatResponse({ data: { list: [], total: 0 } })).toEqual({ list: [], total: 0 })
    const d = new Date('2024-01-01T00:00:00Z')
    expect(serializeDates({ a: d, b: [d], c: 'x' })).toEqual({
      a: d.toISOString(),
      b: [d.toISOString()],
      c: 'x'
    })
  })
})

describe('emptySearchValue', () => {
  it('resets per control type', () => {
    expect(emptySearchValue({ type: 'date-range' })).toEqual([])
    expect(emptySearchValue({ type: 'tree-select', multiple: true })).toEqual([])
    expect(emptySearchValue({ type: 'tree-select', multiple: false })).toBeNull()
    expect(emptySearchValue({ type: 'select' })).toBeNull()
    expect(emptySearchValue({ type: 'input' })).toBe('')
    expect(emptySearchValue({ type: 'date' })).toBe('')
  })
})

describe('createLazyOptionsLoader', () => {
  it('loads once, shares the in-flight request and appends to existing options', async () => {
    const gate = deferred<{ label: string; value: number }[]>()
    const loadOptions = vi.fn(() => gate.promise)
    const config = { key: 'k', type: 'select', options: [{ label: 'pre', value: 0 }], loadOptions }
    const loader = createLazyOptionsLoader()

    const a = loader.ensure(config)
    const b = loader.ensure(config)
    expect(loadOptions).toHaveBeenCalledTimes(1)
    // select loaders receive the empty search pattern
    expect(loadOptions).toHaveBeenCalledWith('')
    gate.resolve([{ label: 'x', value: 1 }])
    await Promise.all([a, b])

    expect(config.options).toEqual([
      { label: 'pre', value: 0 },
      { label: 'x', value: 1 }
    ])
    expect(loader.isLoaded('k')).toBe(true)
    await loader.ensure(config)
    expect(loadOptions).toHaveBeenCalledTimes(1)
  })

  it('calls tree-select loaders without args and retries after a failure', async () => {
    const loadOptions = vi
      .fn()
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce([{ key: 't', label: 'T' }])
    const config: { key: string; type: string; options?: unknown[]; loadOptions: typeof loadOptions } = {
      key: 'tree',
      type: 'tree-select',
      loadOptions
    }
    const loader = createLazyOptionsLoader()

    await expect(loader.ensure(config)).rejects.toThrow('boom')
    expect(loader.isLoaded('tree')).toBe(false)
    expect(loadOptions.mock.calls[0]).toEqual([])

    await loader.ensure(config)
    expect(config.options).toEqual([{ key: 't', label: 'T' }])
    expect(loadOptions).toHaveBeenCalledTimes(2)
  })

  it('ignores configs without a loader', async () => {
    const loader = createLazyOptionsLoader()
    await expect(loader.ensure({ key: 'plain', type: 'input' })).resolves.toBeUndefined()
    await expect(loader.ensure(undefined)).resolves.toBeUndefined()
  })
})

describe('buildTableColumns', () => {
  type DevRow = { id: string; name: string; ts: string | null }
  const asBase = (col: unknown) => col as ListBaseColumn<DevRow>

  it('maps label -> title, passes extra column props through and uses renderCell by default', () => {
    const renderCell = vi.fn((row: DevRow, key: string) => `${key}:${(row as Record<string, unknown>)[key]}`)
    const cols = buildTableColumns<DevRow>([{ key: 'name', label: 'Name', width: 120, ellipsis: true }], {
      renderCell
    })
    expect(cols).toHaveLength(1)
    const col = asBase(cols[0])
    expect(col).toMatchObject({ key: 'name', title: 'Name', width: 120, ellipsis: true })
    expect(col).not.toHaveProperty('label')
    expect(col.render({ id: '1', name: 'dev', ts: null })).toBe('name:dev')
    expect(renderCell).toHaveBeenCalledWith({ id: '1', name: 'dev', ts: null }, 'name')
  })

  it('prefers a custom render and prepends a selection column when selectable', () => {
    const renderCell = vi.fn()
    const custom = vi.fn((row: DevRow) => `custom-${row.id}`)
    const cols = buildTableColumns<DevRow>([{ key: 'id', label: () => 'ID', render: custom }], {
      selectable: true,
      renderCell
    })
    expect(cols[0]).toEqual({ type: 'selection', fixed: 'left' })
    expect(asBase(cols[1]).render({ id: '7', name: '', ts: null })).toBe('custom-7')
    expect(renderCell).not.toHaveBeenCalled()
  })

  it('is not rebuilt by row-data changes when keyed on specs only (perf contract)', async () => {
    const rows = ref<DevRow[]>([{ id: '1', name: 'a', ts: null }])
    const specs = ref<ListColumnSpec<DevRow>[]>([{ key: 'name', label: 'Name' }])
    const build = vi.fn(() =>
      buildTableColumns(specs.value, { renderCell: (row, key) => (row as Record<string, unknown>)[key] as string })
    )
    const columns = computed(build)

    const first = columns.value
    expect(build).toHaveBeenCalledTimes(1)

    // in-place status push + a full page refresh: columns stay the same array instance
    rows.value[0].name = 'b'
    rows.value = [{ id: '2', name: 'c', ts: null }]
    await nextTick()
    expect(columns.value).toBe(first)
    expect(build).toHaveBeenCalledTimes(1)
    // ...and the render callback still reads the live row value
    expect(asBase(first[0]).render(rows.value[0])).toBe('c')

    specs.value = [...specs.value, { key: 'id', label: 'ID' }]
    expect(columns.value).toHaveLength(2)
    expect(build).toHaveBeenCalledTimes(2)
  })
})

describe('rowKeySignature', () => {
  it('only changes when the row shape changes', () => {
    const rows = ref<Record<string, unknown>[]>([{ id: 1, name: 'a' }])
    const signature = computed(() => rowKeySignature(rows.value[0]))
    const derived = vi.fn(() => signature.value.split('\u0000'))
    const keys = computed(derived)

    expect(keys.value).toEqual(['id', 'name'])
    rows.value = [{ id: 2, name: 'b' }]
    expect(keys.value).toEqual(['id', 'name'])
    // same signature -> downstream computed is not re-evaluated
    expect(derived).toHaveBeenCalledTimes(1)

    rows.value = [{ id: 3, name: 'c', ts: 'x' }]
    expect(keys.value).toEqual(['id', 'name', 'ts'])
    expect(derived).toHaveBeenCalledTimes(2)
    expect(rowKeySignature(undefined)).toBe('')
    expect(rowKeySignature(null)).toBe('')
  })
})
