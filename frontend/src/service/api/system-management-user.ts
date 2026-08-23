/**
 * 文件用途: 系统日志列表 API wrapper。
 * 核心逻辑: 按系统管理页面的筛选条件请求后端日志分页数据。
 * 关键注意事项: 日志筛选条件和分页字段用于审计排查，改动时要避免丢失时间、用户或操作类型过滤。
 * 重构建议: 扩展为独立日志模块，补充日志查询参数类型和边界测试。
 */
import { request } from '../request'

export const getSystemLogList = async (params: Api.SystemManage.SystemLogSearchParams) => {
  return await request.get<{ list?: Api.SystemManage.SystemLogList[]; total?: number }>('/operation_logs', { params })
}
