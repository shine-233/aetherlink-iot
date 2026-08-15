/*
 * 文件用途：解析 ThingsVis 空间身份和展示标签。
 * 核心逻辑：从本地用户信息判断系统管理员、租户空间 ID 和空间标签。
 * 关键注意事项：空间 ID 决定数据隔离边界，不能跨用户或跨租户误用。
 * 重构建议：建议补充角色组合和空用户场景测试。
 */
import { localStg } from '@/utils/storage'

const SYS_ADMIN_ROLE = 'SYS_ADMIN'
const SYS_ADMIN_SPACE_ID = 'aetherlink-sys-admin'

export type UserInfoLike = Partial<Api.Auth.UserInfo> & {
  tenantId?: string
  tenant_id?: string
}

export function getCurrentUserInfo(): UserInfoLike | null {
  const userInfo = localStg.get('userInfo')
  return userInfo || null
}

export function isSysAdminUser(userInfo?: UserInfoLike | null): boolean {
  if (!userInfo) return false

  return userInfo.authority === SYS_ADMIN_ROLE || userInfo.roles?.includes(SYS_ADMIN_ROLE) === true
}

export function resolveThingsVisSpaceId(userInfo?: UserInfoLike | null): string {
  if (isSysAdminUser(userInfo)) {
    return SYS_ADMIN_SPACE_ID
  }

  const tenantId = String(userInfo?.tenantId || userInfo?.tenant_id || '').trim()
  return tenantId || 'default'
}

export function getThingsVisSpaceLabel(userInfo?: UserInfoLike | null): string {
  return isSysAdminUser(userInfo) ? '超管独立看板空间' : '租户看板空间'
}

export { SYS_ADMIN_ROLE, SYS_ADMIN_SPACE_ID }
