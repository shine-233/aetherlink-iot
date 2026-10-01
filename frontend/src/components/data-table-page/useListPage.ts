/*
 * useListPage: the single list-page state machine shared by search-form + paginated-table pages.
 *
 * Owns: filter state, page/pageSize/total, rows, loading, stale-request cancellation,
 * optional route-query sync, row selection, and a ready-to-bind naive-ui pagination object.
 *
 * Stale requests: every load bumps a sequence number and aborts the previous AbortController.
 * A response whose sequence is not the latest is dropped, so a slow page-1 reply can never
 * overwrite a fast page-2 reply, and loading only turns off for the latest request.
 */
import { computed, getCurrentScope, onScopeDispose, reactive, ref, shallowRef, toRef } from 'vue'
import type { Ref, VNodeChild } from 'vue'
import type { PaginationProps } from 'naive-ui'

export type ListRowKey = string | number

export interface ListPageResult<Row> {
  list: Row[]
  total: number
}

export interface ListPageFetchContext {
  /** Aborted as soon as a newer request starts or the owning scope is disposed. */
  signal: AbortSignal
}

/**
 * Fetcher contract: receive the flattened params (filters + page + page_size) and return either
 * `{ list, total }` or `null` for "request failed, keep current rows".
 */
export type ListPageFetcher<Row, Params> = (
  params: Params & { page: number; page_size: number },
  ctx: ListPageFetchContext
) => Promise<ListPageResult<Row> | null | undefined>

/** Minimal router surface so callers pass `useRoute()`/`useRouter()` and tests pass plain objects. */
export interface ListPageRouteSync {
  route: { query: Record<string, unknown> }
  router: { replace: (to: { query: Record<string, unknown> }) => unknown }
  /** Filter keys mirrored into the URL. page/page_size are always mirrored when route sync is on. */
  keys?: string[]
}

export interface UseListPageOptions<Row, Params extends Record<string, any>> {
  fetcher: ListPageFetcher<Row, Params>
  /** Initial filter values; also the values `reset()` restores unless `resetValues` is given. */
  initialQuery?: () => Params
  /** Values applied by `reset()`; defaults to `initialQuery()`. */
  resetValues?: () => Partial<Params>
  initialPage?: number
  initialPageSize?: number
  pageSizes?: number[]
  rowKey?: (row: Row) => ListRowKey
  /** Deep-reactive rows so in-place field mutation re-renders. Default false (shallowRef). */
  deepRows?: boolean
  /** Mirror page/page_size/filters into route.query (router.replace, no history entries). */
  routeSync?: ListPageRouteSync
  /** Transform filters right before the request (e.g. Date -> ISO string, trim, drop empties). */
  serialize?: (query: Params) => Record<string, any>
  /** Called with the latest successful result, after rows/total are applied. */
  onLoaded?: (result: ListPageResult<Row>, params: Params & { page: number; page_size: number }) => void
}

const DEFAULT_PAGE_SIZES = [10, 20, 50, 100]

function firstQueryValue(value: unknown): unknown {
  return Array.isArray(value) ? value[0] : value
}

function toPositiveInt(value: unknown, fallback: number) {
  const parsed = Number(firstQueryValue(value))
  return Number.isInteger(parsed) && parsed > 0 ? parsed : fallback
}

function isAbortError(error: unknown) {
  const name = (error as { name?: string } | null)?.name
  return name === 'AbortError' || name === 'CanceledError'
}

/** Shallow Date -> ISO conversion, including arrays of dates (range pickers). */
export function serializeDates<T extends Record<string, any>>(query: T): Record<string, any> {
  return Object.fromEntries(
    Object.entries(query).map(([key, value]) => {
      if (Array.isArray(value)) return [key, value.map((v) => (v instanceof Date ? v.toISOString() : v))]
      return [key, value instanceof Date ? value.toISOString() : value]
    })
  )
}

/** Normalize the backend `{ data: { list, total } }` / `{ list, total }` / bare-array shapes. */
export function normalizeListResponse<Row>(data: any): ListPageResult<Row> {
  const container = data && typeof data === 'object' && !Array.isArray(data) && data.data ? data.data : data
  const list: Row[] = Array.isArray(container?.list) ? container.list : Array.isArray(container) ? container : []
  const total = Number(container?.total ?? data?.total ?? list.length)
  return { list, total: Number.isFinite(total) ? total : list.length }
}

/** Adapt the project's `{ data, error }` flat-request result to a ListPageFetcher result. */
export function fromFlatResponse<Row>(response: { data?: any; error?: any } | null | undefined) {
  if (!response || response.error) return null
  return normalizeListResponse<Row>(response.data)
}

export function useListPage<Row, Params extends Record<string, any> = Record<string, any>>(
  options: UseListPageOptions<Row, Params>
) {
  const sync = options.routeSync
  const syncedKeys = sync?.keys ?? []
  const routeQuery = sync?.route.query ?? {}

  const baseQuery = (options.initialQuery?.() ?? {}) as Params
  if (sync) {
    for (const key of syncedKeys) {
      const raw = firstQueryValue(routeQuery[key])
      if (raw !== undefined && raw !== null && raw !== '') (baseQuery as Record<string, any>)[key] = raw
    }
  }

  const query = reactive(baseQuery) as Params
  const page = ref(sync ? toPositiveInt(routeQuery.page, options.initialPage ?? 1) : (options.initialPage ?? 1))
  const pageSize = ref(
    sync ? toPositiveInt(routeQuery.page_size, options.initialPageSize ?? 10) : (options.initialPageSize ?? 10)
  )
  const total = ref(0)
  // Shallow by default: large pages skip deep proxying. Callers that patch row fields in place
  // (e.g. live online-status pushes) opt into deep reactivity with `deepRows: true`.
  const rows = (options.deepRows ? ref<Row[]>([]) : shallowRef<Row[]>([])) as Ref<Row[]>
  const loading = ref(false)
  const loaded = ref(false)
  const selectedKeys = ref<ListRowKey[]>([])

  const rowKey = options.rowKey ?? ((row: Row) => (row as any)?.id as ListRowKey)
  let seq = 0
  let controller: AbortController | null = null

  const rowByKey = computed(() => {
    const map = new Map<string, Row>()
    for (const row of rows.value) map.set(String(rowKey(row)), row)
    return map
  })
  const selectedRows = computed(
    () => selectedKeys.value.map((key) => rowByKey.value.get(String(key))).filter(Boolean) as Row[]
  )
  const pageCount = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

  function buildParams() {
    const filters = options.serialize ? options.serialize(query) : { ...query }
    return { ...filters, page: page.value, page_size: pageSize.value } as Params & { page: number; page_size: number }
  }

  function writeRoute() {
    if (!sync) return
    const next: Record<string, unknown> = { ...sync.route.query }
    next.page = page.value > 1 ? String(page.value) : undefined
    next.page_size = pageSize.value !== (options.initialPageSize ?? 10) ? String(pageSize.value) : undefined
    for (const key of syncedKeys) {
      const value = (query as Record<string, any>)[key]
      next[key] = value === undefined || value === null || value === '' ? undefined : String(value)
    }
    const cleaned = Object.fromEntries(Object.entries(next).filter(([, v]) => v !== undefined))
    const current = sync.route.query
    const same =
      Object.keys(cleaned).length === Object.keys(current).filter((k) => current[k] !== undefined).length &&
      Object.entries(cleaned).every(([k, v]) => String(firstQueryValue(current[k])) === v)
    if (!same) void sync.router.replace({ query: cleaned })
  }

  function pruneSelection() {
    if (selectedKeys.value.length === 0) return
    const available = rowByKey.value
    const kept = selectedKeys.value.filter((key) => available.has(String(key)))
    if (kept.length !== selectedKeys.value.length) selectedKeys.value = kept
  }

  /** Load the current page. Resolves to true when this call's result was applied. */
  async function load(): Promise<boolean> {
    const requestSeq = ++seq
    controller?.abort()
    const current = typeof AbortController === 'undefined' ? null : new AbortController()
    controller = current
    const params = buildParams()
    loading.value = true
    writeRoute()
    try {
      const result = await options.fetcher(params, { signal: current?.signal as AbortSignal })
      if (requestSeq !== seq) return false
      if (!result) return false
      rows.value = result.list
      total.value = result.total
      loaded.value = true
      pruneSelection()
      // Deleting the last row of the last page leaves an empty page; step back once.
      const lastPage = Math.max(1, Math.ceil(result.total / pageSize.value))
      if (result.list.length === 0 && result.total > 0 && page.value > lastPage) {
        page.value = lastPage
        return load()
      }
      options.onLoaded?.(result, params)
      return true
    } catch (error) {
      if (requestSeq !== seq || isAbortError(error)) return false
      throw error
    } finally {
      if (requestSeq === seq) loading.value = false
    }
  }

  /** Apply filters: jump to page 1 and reload. */
  function search() {
    page.value = 1
    return load()
  }

  function reset() {
    const values = (options.resetValues?.() ?? options.initialQuery?.() ?? {}) as Record<string, any>
    for (const key of Object.keys(query)) {
      ;(query as Record<string, any>)[key] = key in values ? values[key] : undefined
    }
    return search()
  }

  /** Patch filters (e.g. from a preset or deep link) and search. */
  function patchQuery(patch: Partial<Params>, { reload = true, resetPage = true } = {}) {
    Object.assign(query, patch)
    if (resetPage) page.value = 1
    return reload ? load() : Promise.resolve(false)
  }

  function setPage(next: number) {
    page.value = next
    return load()
  }

  function setPageSize(next: number) {
    pageSize.value = next
    page.value = 1
    return load()
  }

  function setSelectedKeys(keys: ListRowKey[]) {
    selectedKeys.value = keys
  }

  function clearSelection() {
    selectedKeys.value = []
  }

  /** Invalidate any in-flight request without starting a new one. */
  function cancel() {
    seq++
    controller?.abort()
    controller = null
    loading.value = false
  }

  /** Clear rows/total (e.g. when a required parent id disappears). */
  function clear() {
    cancel()
    rows.value = []
    total.value = 0
    selectedKeys.value = []
  }

  const pagination = reactive({
    page,
    pageSize,
    itemCount: total,
    showSizePicker: true,
    pageSizes: options.pageSizes ?? DEFAULT_PAGE_SIZES,
    onUpdatePage: (next: number) => void setPage(next),
    onUpdatePageSize: (next: number) => void setPageSize(next)
  }) as unknown as PaginationProps

  /**
   * Flat two-way view `{ ...filters, page, page_size }` for pages whose templates/tests bind
   * `queryParams.page` style fields. Writes go straight to the composable's state.
   */
  const flatQuery = reactive({
    ...Object.fromEntries(Object.keys(query).map((key) => [key, toRef(query as Record<string, any>, key)])),
    page,
    page_size: pageSize
  }) as Params & { page: number; page_size: number }

  if (getCurrentScope()) onScopeDispose(cancel)

  return {
    query,
    flatQuery,
    page,
    pageSize,
    pageCount,
    total,
    rows,
    loading,
    loaded,
    pagination,
    selectedKeys,
    selectedRows,
    rowKey,
    load,
    refresh: load,
    search,
    reset,
    patchQuery,
    setPage,
    setPageSize,
    setSelectedKeys,
    clearSelection,
    cancel,
    clear
  }
}

export type ListPage<Row, Params extends Record<string, any> = Record<string, any>> = ReturnType<
  typeof useListPage<Row, Params>
>

/* ------------------------------------------------------------------------------------------------
 * Search-form + table helpers shared by the generic <data-table-page> wrapper and pages that
 * rebuilt its surface on top of useListPage. Pure (no component instance), so they are unit-tested
 * directly instead of through a full naive-ui mount.
 * ---------------------------------------------------------------------------------------------- */

/** Reset value per control type: date-range -> [], tree-select -> []/null, select -> null, else ''. */
export function emptySearchValue(config: { type: string; multiple?: boolean }): unknown {
  if (config.type === 'date-range') return []
  if (config.type === 'tree-select') return config.multiple ? [] : null
  if (config.type === 'select') return null
  return ''
}

export interface LazyOptionsConfig<Option = unknown> {
  key: string
  type: string
  options?: Option[]
  /** select loaders receive the (empty) search pattern; tree-select loaders take no args. */
  loadOptions?: (pattern?: string) => Promise<Option[]>
}

/**
 * Single-flight, load-once option loader keyed by config.key. Concurrent `ensure` calls for the
 * same key share one request; a failed load is not marked loaded, so the next open retries.
 * Loaded options are appended to `config.options` in place (the config object is owned by the
 * parent page and read by the select control).
 */
export function createLazyOptionsLoader() {
  const loaded = new Set<string>()
  const pending = new Map<string, Promise<void>>()

  async function ensure<Option>(config: LazyOptionsConfig<Option> | null | undefined): Promise<void> {
    if (!config?.loadOptions || loaded.has(config.key)) return
    const inflight = pending.get(config.key)
    if (inflight) return inflight

    const loader = config.loadOptions
    const load = (async () => {
      const opts = config.type === 'select' ? await loader('') : await loader()
      config.options = [...(config.options ?? []), ...(opts ?? [])]
      loaded.add(config.key)
    })().finally(() => {
      pending.delete(config.key)
    })
    pending.set(config.key, load)
    return load
  }

  return { ensure, isLoaded: (key: string) => loaded.has(key) }
}

export type ListColumnLabel = string | (() => string) | undefined

export interface ListColumnSpec<Row> {
  key: string
  label?: ListColumnLabel
  /** Custom cell renderer; receives the row (legacy callers declare it with no parameters). */
  render?: (row: Row) => VNodeChild
  /** Any extra naive-ui column props (width, ellipsis, fixed…) pass through untouched. */
  [prop: string]: unknown
}

export interface ListBaseColumn<Row> {
  key: string
  title?: ListColumnLabel
  render: (row: Row) => VNodeChild
  [prop: string]: unknown
}

export type ListSelectionColumn = { type: 'selection'; fixed: 'left' }
export type ListTableColumn<Row> = ListBaseColumn<Row> | ListSelectionColumn

/**
 * Build the table column array once per column-spec change. Row-level formatting happens inside
 * the returned `render` callbacks, so callers must NOT key this on row data (a status push that
 * mutates a cell re-renders that cell only, instead of rebuilding every column + closure).
 */
export function buildTableColumns<Row>(
  specs: readonly ListColumnSpec<Row>[],
  options: { selectable?: boolean; renderCell: (row: Row, key: string) => VNodeChild }
): ListTableColumn<Row>[] {
  const columns: ListTableColumn<Row>[] = specs.map((spec) => {
    const { label, render, ...rest } = spec
    const custom = render
    return {
      ...rest,
      title: label,
      key: spec.key,
      render: custom ? (row: Row) => custom(row) : (row: Row) => options.renderCell(row, spec.key)
    }
  })
  return options.selectable ? [{ type: 'selection', fixed: 'left' }, ...columns] : columns
}

/**
 * Stable signature of a row's own keys, used by `columnsToShow: 'all'`: columns follow the
 * first row's shape and only rebuild when that shape changes, not on every data refresh.
 */
export function rowKeySignature(row: unknown): string {
  return row && typeof row === 'object' ? Object.keys(row).join('\u0000') : ''
}
