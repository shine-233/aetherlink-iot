/**
 * 文件用途: 预注册设备列表查询组合函数——筛选、分页与远程数据加载。
 * 核心逻辑: 交由 useListPage 统一承担 page/pageSize/total/rows/loading 与过期请求丢弃,
 *          本文件只保留 device/preRegister 的后端契约: 筛选值序列化与 {data,error} 载荷适配。
 * 关键注意事项: 后端契约 page/page_size 必填, product/batch/activate 为空时必须整键省略
 *              (请求参数必须严格等于 `{page, page_size}`,多一个空字符串键后端会按空值过滤)。
 */
import { computed } from 'vue'
import { useListPage, fromFlatResponse } from '@/components/data-table-page/useListPage'
import { getPreProductList } from '@/service/product/list'
import type { PreRegisterRecord } from './types'

interface PreRegisterQuery {
  product_id: string
  batch_number: string
  activate_flag: string | null
}

export function usePreRegisterList() {
  const listPage = useListPage<PreRegisterRecord, PreRegisterQuery>({
    initialQuery: () => ({ product_id: '', batch_number: '', activate_flag: null }),
    initialPageSize: 10,
    pageSizes: [10, 20, 50],
    serialize: query => ({
      ...(query.product_id ? { product_id: query.product_id } : {}),
      ...(query.batch_number.trim() ? { batch_number: query.batch_number.trim() } : {}),
      ...(query.activate_flag ? { activate_flag: query.activate_flag } : {})
    }),
    fetcher: async params => fromFlatResponse<PreRegisterRecord>(await getPreProductList(params))
  })

  const hasActiveFilters = computed(() =>
    Boolean(
      listPage.query.product_id || listPage.query.batch_number.trim() || listPage.query.activate_flag
    )
  )

  return {
    loading: listPage.loading,
    tableData: listPage.rows,
    queryParams: listPage.query,
    pagination: listPage.pagination,
    hasActiveFilters,
    fetchList: listPage.search,
    resetQuery: listPage.reset,
    search: listPage.search,
    refresh: listPage.refresh
  }
}
