/**
 * 文件用途: auth store 的 storage helper。
 * 核心逻辑: 读取、写入和清理 token/userInfo 等认证持久化数据。
 * 关键注意事项: 字段默认值必须与 auth store reset 行为一致，避免刷新后权限状态漂移。
 * 重构建议: 收敛 storage key 类型定义，并用 shared helper 单测覆盖空值和旧数据兼容。
 */
import { localStg } from '@/utils/storage'
/** Get token */
export function getToken() {
  return localStg.get('token') || ''
}

/** Get user info */
export function getUserInfo() {
  const emptyInfo: Api.Auth.UserInfo = {
    authority: '',
    id: '',
    userId: '',
    userName: '',
    roles: []
  }
  const userInfo = localStg.get('userInfo') || emptyInfo

  return userInfo
}

/** Check if token is expired */
export function isTokenExpired() {
  const tokenExpiresIn = localStg.get('token_expires_in')
  if (!tokenExpiresIn) return true

  const expiresTime = parseInt(tokenExpiresIn)
  const currentTime = Date.now()

  // 提前5分钟检查过期，避免在请求过程中过期
  return currentTime >= expiresTime - 5 * 60 * 1000
}

/** Clear auth storage */
export function clearAuthStorage() {
  localStg.remove('token')
  localStg.remove('userInfo')
  localStg.remove('token_expires_in')
}
