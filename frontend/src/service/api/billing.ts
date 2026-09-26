/**
 * 文件用途：Billing（TB-17 租户 API 日配额 + TB-17R 传输维度）前端 API 客户端。
 * 核心逻辑：封装 GET /api/v1/billing/api-quota——API 维度（今日调用数/限额/剩余）与
 * TB-17R 扩展的传输维度（MQTT 连接认证计量，transport_* 系列字段）。
 * 关键注意事项：remaining/transport_remaining=-1 与 *_per_day<=0 是"不限量"哨兵语义，前端不得当数字展示；
 * 传输维度的限额/状态与 broker 执法同口径（后端读 broker 使用的 Redis 缓存）。
 * 重构建议：若套餐订购/账单等能力入前端，在本文件平级扩展而不是塞进同一接口。
 */
import { request } from '../request'

export interface ApiQuotaReport {
  tenant_id: string
  /** 计量日（UTC，YYYY-MM-DD），与后端执法口径一致 */
  date: string
  plan_code: string
  api_calls_today: number
  /** 套餐日调用限额；<=0 表示未设执法阈值（不限量） */
  max_api_calls_per_day: number
  /** 剩余额度；-1 表示不限量 */
  remaining: number
  usage_pct: number
  /** normal | warning | exceeded | unlimited */
  quota_status: 'normal' | 'warning' | 'exceeded' | 'unlimited'
  /** ---- TB-17R 传输维度（MQTT 连接认证计量，展示与 broker 执法同口径） ---- */
  /** 今日 MQTT 连接认证次数（被拒同样计入） */
  transport_events_today: number
  /** broker 执法用限额缓存值；0 表示未配置执法阈值（不限量） */
  max_transport_per_day: number
  /** 剩余连接额度；-1 表示不限量 */
  transport_remaining: number
  transport_usage_pct: number
  transport_quota_status: 'normal' | 'warning' | 'exceeded' | 'unlimited'
}

/** 获取当前租户今日 API 配额（SYS_ADMIN 可传 tenant_id 查看其他租户） */
export const getApiQuota = async (params?: { tenant_id?: string }) => {
  return await request.get<ApiQuotaReport>('/billing/api-quota', { params })
}
