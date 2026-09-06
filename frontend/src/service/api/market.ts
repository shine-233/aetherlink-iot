/**
 * 文件用途: 物模型市场登录、发布、查询和安装相关 API wrapper。
 * 核心逻辑: 封装市场账号登录、项目发布、物模型列表详情和安装请求。
 * 关键注意事项: 市场物模型 ID、认证信息和安装 payload 连接外部分发流程，失败分支不能静默吞掉。
 * 重构建议: 明确市场账号与物模型安装类型，并补充登录失败、安装失败和分页参数测试。
 */
import { request } from '../request'

/** 市场登录 */
export const marketLogin = async (data: { username: string; password: string }) => {
  return await request.post<{ token: string }>('/device/template/market/login', data)
}

/** 发布设备配置到市场（同时发布 DeviceConfig 凭证协议 + DeviceTemplate 物模型 + 展示配置） */
export const publishToMarket = async (data: {
  device_config_id: string
  market_token: string
  market_name?: string
  brand?: string
  model?: string
  category?: string
  version?: string
  author?: string
  description?: string
}) => {
  return await request.post<{ market_template_id: string }>('/device/template/market/publish', data)
}

/** 获取市场物模型列表（通过后端代理） */
export const getMarketTemplates = async (params: {
  keyword?: string
  category?: string
  sort_by?: string
  page: number
  page_size: number
}) => {
  return await request.get<Record<string, unknown>>('/device/template/market/list', { params })
}

/** 获取市场物模型详情 */
export const getMarketTemplateDetail = async (marketId: string) => {
  return await request.get<Record<string, unknown>>(`/device/template/market/detail/${marketId}`)
}

/** 从市场安装物模型 */
export const installFromMarket = async (data: {
  market_template_id: string
  version?: string
  market_token: string
}) => {
  return await request.post<Record<string, unknown>>('/device/template/market/install', data)
}

// PHASE-D-D10 BEGIN 模板市场运营化：本地分类目录 + 按行业打包导出
export interface MarketCatalogEntry {
  type_key: string
  template_count: number
  download_count: number
}

/** 行业分类目录（租户内 distinct type_key + 计数） */
export const getMarketCatalog = async () => {
  return await request.get<MarketCatalogEntry[]>('/device/template/market/catalog')
}

/** 按行业打包导出（base64 信封，前端解码为文件下载） */
export const getMarketBundle = async (typeKey: string) => {
  return await request.get<{ file_name: string; content_base64: string; count: number }>(
    '/device/template/market/bundle',
    { params: typeKey ? { type_key: typeKey } : {} }
  )
}

/** 模板导入（导出载荷回放，同租户同名同版本幂等） */
export const importDeviceTemplate = async (data: unknown) => {
  return await request.post('/device/template/import', data)
}

/** 本地模板分页列表（浏览页卡片数据源，后端支持 type_key 过滤） */
export const getLocalTemplateList = async (params: { page: number; page_size: number; type_key?: string }) => {
  return await request.get('/device/template', { params })
}

/** 解码 base64 打包载荷并触发浏览器下载 */
export function downloadMarketBundle(bundle: { file_name: string; content_base64: string }) {
  const binary = atob(bundle.content_base64)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i)
  const blob = new Blob([bytes], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = bundle.file_name
  anchor.click()
  URL.revokeObjectURL(url)
}
// PHASE-D-D10 END
