/**
 * 文件用途: 看板菜单配置 API wrapper。
 * 核心逻辑: 按看板 ID 获取、保存和删除菜单配置，支撑可视化看板的菜单持久化。
 * 关键注意事项: `dashboardId` 同时参与路径和配置归属，改动时要避免跨看板覆盖或删除。
 * 重构建议: 提取菜单配置请求类型，并补充保存 payload 和删除路径的测试。
 */
import { request } from '../request'

export interface DashboardMenuConfig {
  dashboard_id: string
  menu_name: string
  sort: number
  enabled: boolean
  parent_code: string
}

export function fetchDashboardMenuConfig(dashboardId: string) {
  return request.get<DashboardMenuConfig | null>(`/dashboard-menu/${dashboardId}`)
}

export function fetchDashboardMenuConfigs(dashboardIds: string[]) {
  return request.post<Record<string, DashboardMenuConfig | null>>('/dashboard-menu/batch', {
    dashboard_ids: dashboardIds
  })
}

export function saveDashboardMenuConfig(
  dashboardId: string,
  payload: {
    menu_name: string
    dashboard_name?: string
    sort?: number
    enabled?: boolean
  }
) {
  return request.put<DashboardMenuConfig | null>(`/dashboard-menu/${dashboardId}`, payload)
}

export function deleteDashboardMenuConfig(dashboardId: string) {
  return request.delete(`/dashboard-menu/${dashboardId}`)
}
