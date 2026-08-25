/**
 * 文件用途: 操作审计日志查询 API wrapper。
 * 核心逻辑: 按分页和筛选条件读取当前租户范围内的操作日志列表，供审计页面展示请求轨迹。
 * 关键注意事项: 后端按登录 claims 做租户隔离；request_message/response_message 可能包含敏感载荷，
 *   页面展示时不要写入日志或上报到第三方。时间参数使用 RFC3339（如 2026-08-25T10:00:00+08:00）。
 * 重构建议: 若后端后续支持服务端排序/更多筛选，把排序参数并入 OperationLogListParams 并补充分页契约测试。
 */
import { request } from '../request'

/** 操作日志筛选参数（与后端 GetOperationLogListByPageReq 的 form 标签一一对应） */
export interface OperationLogListParams {
  page?: number
  page_size?: number
  /** 客户端 IP 关键字（模糊匹配） */
  ip?: string
  /** 操作人账号关键字（模糊匹配 users.name） */
  username?: string
  /** 请求方法（后端按接口名称列精确匹配） */
  method?: string
  /** 请求路径关键字（模糊匹配） */
  path?: string
  /** 开始时间，RFC3339 格式 */
  start_time?: string
  /** 结束时间，RFC3339 格式 */
  end_time?: string
}

/**
 * 操作日志行结构（对应后端 GetOperationLogListByPageRsp）。
 * 说明：中间件写入日志时 name 列当前存的是 HTTP 方法名（GET/POST 等），
 * username/email 来自联表 users 的展示字段。
 */
export interface OperationLogRow {
  id: string
  ip: string
  path: string | null
  user_id: string
  name: string | null
  created_at: string | null
  /** 耗时(ms) */
  latency: number
  request_message: string | null
  response_message: string | null
  tenant_id: string
  remark: string | null
  username: string | null
  email: string | null
}

export interface OperationLogListResult {
  list: OperationLogRow[]
  total: number
}

/** 分页查询操作审计日志 */
export const fetchOperationLogs = async (params: OperationLogListParams) => {
  return await request.get<OperationLogListResult>('/operation_logs', { params })
}
