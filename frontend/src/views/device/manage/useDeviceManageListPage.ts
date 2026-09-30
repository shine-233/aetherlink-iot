/*
 * useDeviceManageListPage: the device-manage page's own list-state layer.
 *
 * Migrated off the shared <data-table-page> wrapper: the page now drives `useListPage`
 * (from `@/components/data-table-page/useListPage`) directly and this composable keeps the
 * old `tablePageRef` bridge contract — `dataList` / `selectedRows` / `handleSearch` /
 * `handleReset` / `forceChangeParamsByKey` / `clearSelection` — so the fleet-operations,
 * status-subscription and service-access-filter composables keep working unchanged.
 *
 * Behavior mirrored from <data-table-page>:
 * - initial query values come from the searchConfigs `initValue`s;
 * - rows are deep-reactive because the device status push mutates `is_online` in place;
 * - dates are serialized with `serializeDates` before the request;
 * - `onParamsUpdate` fires with the criteria (without page/page_size) before every fetch;
 * - after a reload with an active selection, the (pruned) selection is re-emitted;
 * - `handleReset` restores the per-control-type empty value (select -> null, tree-select ->
 *   []/null, date-range -> [], input -> ''), not `undefined`;
 * - `forceChangeParamsByKey` only patches keys present in the query contract and keeps the
 *   current page.
 */
import { computed, shallowRef, type Ref } from 'vue'
import { createLogger } from '@/utils/logger'
import { formatDateTime } from '@/utils/common/datetime'
import type { SearchConfig } from '@/components/data-table-page/types'
import { fromFlatResponse, serializeDates, useListPage } from '@/components/data-table-page/useListPage'

const logger = createLogger('DeviceManageListPage')

export type DeviceManageRow = Record<string, any> & {
  id?: string
  key?: string
  device_number?: string
}

export type DeviceManageFetchResponse = { data?: any; error?: any } | null | undefined

/** Row-key contract shared with the old data-table-page wrapper. */
export function deviceManageRowKey(row: DeviceManageRow): string | number {
  return row.id || row.key || row.device_number || ''
}

/** Reset value per control type (identical to data-table-page's handleReset semantics). */
export function emptyValueFor(config: SearchConfig) {
  if (config.type === 'date-range') return []
  if (config.type === 'tree-select') return config.multiple ? [] : null
  if (config.type === 'select') return null
  return ''
}

/** Clicks on interactive row content (buttons, selects, checkboxes…) must not trigger rowClick. */
const INTERACTIVE_ROW_TARGET_SELECTOR = [
  'button',
  'a',
  'input',
  'textarea',
  'select',
  '[role="button"]',
  '[role="checkbox"]',
  '.n-checkbox',
  '.n-button',
  '.n-dropdown',
  '.n-select',
  '.n-tree-select'
].join(',')

export function isInteractiveRowClickTarget(event: MouseEvent): boolean {
  const target = event.target
  if (!(target instanceof HTMLElement)) return false
  return Boolean(target.closest(INTERACTIVE_ROW_TARGET_SELECTOR))
}

/** Build the `row-props` function for the device table (cursor + guarded row click). */
export function deviceManageRowProps(rowClick?: (row: DeviceManageRow) => void) {
  if (!rowClick) {
    return () => ({})
  }
  return (row: DeviceManageRow) => ({
    style: 'cursor: pointer;',
    onClick: (event: MouseEvent) => {
      if (isInteractiveRowClickTarget(event)) return
      rowClick(row)
    }
  })
}

/**
 * Map the page column configs to NDataTable columns (label -> title, render wrapper,
 * formatted `ts` fallback) and prepend the selection column when rows are selectable.
 */
export function buildDeviceManageTableColumns(columnsToShow: any[], selectableRows: boolean) {
  let columns = columnsToShow.map((item) => {
    if (item.render) {
      return {
        ...item,
        title: item.label,
        key: item.key,
        render: (row: DeviceManageRow) => item.render(row)
      }
    }
    return {
      ...item,
      title: item.label,
      key: item.key,
      render: (row: DeviceManageRow) => {
        if (item.key === 'ts' && row[item.key]) {
          return formatDateTime(row[item.key])
        }
        return row[item.key]
      }
    }
  })

  if (selectableRows) {
    columns = [{ type: 'selection', fixed: 'left' }, ...columns]
  }

  return columns
}

/**
 * Bridge object exposed through `tablePageRef`: the same surface the old
 * <data-table-page> instance exposed via defineExpose.
 */
export interface DeviceManageTableBridge {
  /** Live rows (deep-reactive: the status push mutates `is_online` in place). */
  readonly dataList: DeviceManageRow[]
  /** Rows for the currently checked keys (pruned to visible rows). */
  readonly selectedRows: DeviceManageRow[]
  /** Back to page 1 and reload. */
  handleSearch: () => Promise<boolean>
  /** Restore per-control-type empty values, then handleSearch(). */
  handleReset: () => Promise<boolean>
  /** Patch query keys that exist in the query contract and reload, keeping the current page. */
  forceChangeParamsByKey: (params: Record<string, unknown>) => Promise<boolean>
  /** Clear checked rows and notify the page (mirrors the old `selectionUpdate([])` emit). */
  clearSelection: () => void
}

/** Hooks filled in by index.vue after the provider composables exist (they need the bridge). */
export interface DeviceManageListPageHooks {
  /** Fired with the criteria (without page/page_size) right before every fetch. */
  onParamsUpdate?: (criteria: Record<string, unknown>) => void
  /** Fired whenever the checked rows change, including the post-reload re-emit. */
  onSelectionUpdate?: (rows: DeviceManageRow[]) => void
}

export interface UseDeviceManageListPageOptions {
  searchConfigs: Ref<SearchConfig[]>
  /** Shared ref filled with the bridge; passed on to the composables that consume it. */
  tablePageRef: Ref<DeviceManageTableBridge | undefined>
  /** The page's data loader (deviceList + cache + fleet sync + status subscription). */
  fetchData: (params: Record<string, any>) => Promise<DeviceManageFetchResponse>
  columnsToShow: any[]
  initPage?: number
  initPageSize?: number
  selectableRows?: boolean
  hooks?: DeviceManageListPageHooks
}

export function useDeviceManageListPage(options: UseDeviceManageListPageOptions) {
  // 与 data-table-page 的 getData/onLoaded 行为一致：加载前记录勾选状态，
  // 加载完成后（useListPage 已把勾选裁剪到当页可见行）把勾选回发给页面。
  let hadSelection = false

  const list = useListPage<DeviceManageRow, Record<string, any>>({
    initialQuery: () => Object.fromEntries(options.searchConfigs.value.map((item) => [item.key, item.initValue])),
    initialPage: options.initPage || 1,
    initialPageSize: options.initPageSize || 10,
    pageSizes: [10, 20, 30, 40, 50],
    rowKey: deviceManageRowKey,
    // 设备在线状态推送会原地修改行字段（useDeviceManageStatusSubscription），需要深响应。
    deepRows: true,
    serialize: serializeDates,
    fetcher: async (params) => {
      const { page: _page, page_size: _pageSize, ...criteria } = params
      options.hooks?.onParamsUpdate?.(criteria)
      const response = await options.fetchData(params)
      if (response?.error) logger.error({ 'Error fetching data:': response.error })
      return fromFlatResponse<DeviceManageRow>(response)
    },
    onLoaded: () => {
      if (hadSelection) options.hooks?.onSelectionUpdate?.(list.selectedRows.value)
    }
  })

  /** Every load goes through here so selection re-emission matches the old wrapper. */
  const loadPreservingSelection = () => {
    hadSelection = list.selectedKeys.value.length > 0
    return list.load()
  }

  const bridge: DeviceManageTableBridge = {
    get dataList() {
      return list.rows.value
    },
    get selectedRows() {
      return list.selectedRows.value
    },
    handleSearch: () => {
      list.page.value = 1
      return loadPreservingSelection()
    },
    handleReset: () => {
      const query = list.query as Record<string, unknown>
      const configs = options.searchConfigs.value
      Object.keys(query).forEach((key) => {
        const config = configs.find((item) => item.key === key)
        if (config) query[key] = emptyValueFor(config)
      })
      list.page.value = 1
      return loadPreservingSelection()
    },
    forceChangeParamsByKey: (params) => {
      const query = list.query as Record<string, unknown>
      Object.entries(params).forEach(([key, value]) => {
        if (key in query) query[key] = value
      })
      return loadPreservingSelection()
    },
    clearSelection: () => {
      list.clearSelection()
      options.hooks?.onSelectionUpdate?.([])
    }
  }

  // The consumer composables hold this ref from setup time; the bridge lands here when this
  // composable runs (before any load/interaction happens).
  options.tablePageRef.value = bridge

  const generatedColumns = computed(() =>
    buildDeviceManageTableColumns(options.columnsToShow, options.selectableRows ?? false)
  )

  const handleCheckedRowKeysUpdate = (keys: Array<string | number>) => {
    list.setSelectedKeys(keys)
    options.hooks?.onSelectionUpdate?.(list.selectedRows.value)
  }

  const setPage = (next: number) => {
    list.page.value = next
    return loadPreservingSelection()
  }

  const setPageSize = (next: number) => {
    list.pageSize.value = next
    list.page.value = 1
    return loadPreservingSelection()
  }

  return {
    /** Raw useListPage state machine (query, pagination, selection, load/search/reset…). */
    list,
    tablePageRef: options.tablePageRef,
    searchCriteria: list.query,
    rows: list.rows,
    loading: list.loading,
    total: list.total,
    page: list.page,
    pageSize: list.pageSize,
    selectedRowKeys: list.selectedKeys,
    selectedRows: list.selectedRows,
    rowKey: list.rowKey,
    generatedColumns,
    handleCheckedRowKeysUpdate,
    handleSearch: bridge.handleSearch,
    handleReset: bridge.handleReset,
    setPage,
    setPageSize,
    /** Initial/refresh load with the selection-preserving behavior. */
    load: loadPreservingSelection
  }
}
