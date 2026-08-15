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
  return await request.get<any>('/device/template/market/list', { params })
}

/** 获取市场物模型详情 */
export const getMarketTemplateDetail = async (marketId: string) => {
  return await request.get<any>(`/device/template/market/detail/${marketId}`)
}

/** 从市场安装物模型 */
export const installFromMarket = async (data: {
  market_template_id: string
  version?: string
  market_token: string
}) => {
  return await request.post<any>('/device/template/market/install', data)
}
