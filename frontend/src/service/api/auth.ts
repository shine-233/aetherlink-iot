/**
 * 文件用途: 登录、用户信息、登出和认证辅助接口 wrapper。
 * 核心逻辑: 将 auth store 的认证流程绑定到后端认证接口，并提供用户信息转换所需数据。
 * 关键注意事项: token、密码加密盐、用户角色和退出清理会影响路由初始化与权限 guard。
 * 重构建议: 将请求函数与用户信息 normalize helper 分离，补充登录失败、空 token 和角色缺失测试。
 */
import { localStg } from '@/utils/storage'
import { request } from '../request'

/**
 * Login
 *
 * @param userName User name
 * @param password Password
 */

export function fetchLogin(email: string, password: string, salt: string | null) {
  return request.post<Api.Auth.LoginToken>('/login', { email, password, salt })
}

/** 2FA 第二因子登录（ticket 由 /login 的 step=totp 挑战下发） */
export function fetchLoginTotp(ticket: string, code: string) {
  return request.post<Api.Auth.LoginToken>('/login/totp', { ticket, code })
}

/** 登录页 SSO 提供方发现（公开，仅平台级启用项） */
export function fetchSsoProviders() {
  return request.get<Array<{ id: string; name: string }> | null>('/sso/providers')
}

/** Get user info */
export function fetchGetUserInfo() {
  return request.get<Api.Auth.UserInfo>('/user/detail')
}

// 登出接口
export function logout() {
  return request.get('/user/logout')
}
export function fetchEmailCode(email: string) {
  return request.get<{ email: string; is_register: number } | null>('/verification/code', {
    params: {
      email,
      is_register: 1,
      language: localStg.get('lang') || 'en-US'
    }
  })
}
export function fetchEmailCodeByEmail(email: string) {
  return request.get<{ email: string; is_register: number } | null>('/verification/code', {
    params: {
      email,
      is_register: 2,
      language: localStg.get('lang') || 'en-US'
    }
  })
}
/** 获取用户列表 */
export const fetchUserList = async (params: object) => {
  const data = await request.get<Api.UserManagement.Data | null>('/user', {
    params
  })
  return data
}

/** 添加用户 */
export const addUser = async (params: object) => {
  const data = await request.post<Api.BaseApi.Data>('/user', params)
  return data
}

/** 编辑用户 */
export const editUser = async (params: object) => {
  // delete params.password;
  const data = await request.put<Api.BaseApi.Data>('/user', params)
  return data
}

/** 删除用户 */
export const delUser = async (id: string) => {
  const data = await request.delete<Api.BaseApi.Data>(`/user/${id}`)
  return data
}

/** 切换用户 */
export const transformUser = async (params: { become_user_id: string }) => {
  const data = await request.post<Api.Auth.LoginToken>(`/user/transform`, params)
  return data
}
/** 修改密码 */
export const editUserPassWord = async (params: Record<string, unknown>) => {
  const data = await request.post<Api.BaseApi.Data>(`/reset/password`, params)
  return data
}

/** 发送密码重置链接 */
export const requestPasswordResetLink = async (params: { email: string; verify_code: string }) => {
  const data = await request.post<{ expires_in: number }>(`/reset/password/link`, params)
  return data
}

export const fetchCompatHomeConfig = async (params: Record<string, unknown>) => {
  const data = await request.get<{ config: string } | null>('/board/home', {
    params
  })
  return data
}

export function registerByEmail(data: {
  email: string // 邮箱
  verify_code: string // 邮箱验证码
  password: string // 用户密码
  phone_prefix: string // 手机前缀
  phone_number: string // 手机号码
}) {
  return request.post('/tenant/email/register', data, {
    headers: {
      'Content-Type': 'application/json' // 设置请求体类型为 application/json
    }
  })
}

/** 检查是否存在超管 */
export function fetchHasAdmin() {
  return request.get<{ has_admin: boolean }>('/tenant/has-admin')
}

/** 获取首次安装状态 */
export function fetchTenantSetupState() {
  return request.get<{
    has_admin: boolean
    has_tenant_admin?: boolean
    has_tenant?: boolean
    entry: 'login' | 'register'
    next_step?: 'create_super_admin' | 'create_tenant_admin' | 'login'
    market_base_url?: string
    market_register_url?: string
  }>('/tenant/setup-state')
}

export interface SuperAdminInitPayload {
  email: string
  password: string
  market_registered?: boolean
  market_email?: string
  market_source?: string
}

/** 首次安装超管初始化（语义化新接口） */
export async function fetchSuperAdminInit(data: SuperAdminInitPayload) {
  try {
    return await request.post('/tenant/super-admin/init', data, {
      headers: {
        'Content-Type': 'application/json'
      }
    })
  } catch (error: unknown) {
    // 请求层抛出的错误按 axios 响应结构鸭子类型判定（测试以普通对象模拟）。
    const response = (error as { response?: { status?: number; data?: { code?: number } } } | null)?.response
    const status = response?.status
    const code = response?.data?.code
    if (status === 404 || code === 100404) {
      return request.post('/tenant/market-register', data, {
        headers: {
          'Content-Type': 'application/json'
        }
      })
    }
    throw error
  }
}
