/**
 * 文件用途: 升级包列表的查询状态机——基于共享 useListPage 配置的分页/筛选/加载。
 * 核心逻辑: 筛选(name/version/device_config_id) + 服务端分页全部收口在 useListPage；
 *   请求载荷与迁移前保持一致(device_config_id 空值传 '')，响应兼容 list / data.list / records 形状。
 * 关键注意事项: fetcher 失败返回 null 时 useListPage 会保留当前行；过期请求由 useListPage 丢弃。
 * 重构建议: 后端若统一分页契约，可去掉 records 兜底直接用 normalizeListResponse。
 */
import { normalizeListResponse, useListPage } from '@/components/data-table-page/useListPage'
import type { ListPageResult } from '@/components/data-table-page/useListPage'
import { getOtaPackageList } from '@/service/product/update-package'
import type { OtaPackageRecord } from './ota-package-types'

export interface OtaPackageQueryParams {
  name: string
  version: string
  device_config_id: string | null
}

/** normalizeListResponse 之外的 records 兜底（老接口可能返回 { records, total }）。 */
function normalizePackageListResponse(data: unknown): ListPageResult<OtaPackageRecord> {
  const record = data as { records?: unknown[]; total?: unknown } | null | undefined
  if (record && typeof record === 'object' && !Array.isArray(record) && Array.isArray(record.records)) {
    const total = Number(record.total ?? record.records.length)
    return {
      list: record.records as OtaPackageRecord[],
      total: Number.isFinite(total) ? total : record.records.length
    }
  }
  return normalizeListResponse<OtaPackageRecord>(data)
}

export function useOtaPackageList() {
  return useListPage<OtaPackageRecord, OtaPackageQueryParams>({
    initialQuery: () => ({ name: '', version: '', device_config_id: null }),
    initialPageSize: 10,
    pageSizes: [10, 20, 50],
    fetcher: async (params) => {
      const { data, error } = await getOtaPackageList({
        page: params.page,
        page_size: params.page_size,
        name: params.name,
        version: params.version,
        device_config_id: params.device_config_id || ''
      })
      if (error) return null
      return normalizePackageListResponse(data)
    }
  })
}

export type OtaPackageListPage = ReturnType<typeof useOtaPackageList>
