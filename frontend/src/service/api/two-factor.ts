/**
 * 文件用途: 2FA（TOTP）设置相关接口——状态/绑定准备/激活/停用（ROADMAP C7）。
 * 注意: 激活成功仅此一次返回恢复码明文；停用需当前 TOTP 或一个未用恢复码。
 */
import { request } from '../request'

interface TotpStatusData {
  enabled: boolean
}

interface TotpSetupData {
  secret: string
  uri: string
  account: string
  issuer: string
  enabled: boolean
}

interface TotpActivateData {
  codes: string[]
}

/** 查询当前用户 2FA 状态 */
export function fetchTotpStatus() {
  return request.get<TotpStatusData | null>('/user/totp/status')
}

/** 生成一次性绑定材料（otpauth URI） */
export function fetchTotpSetup() {
  return request.get<TotpSetupData | null>('/user/totp/setup')
}

/** 用验证码激活 2FA，返回一次性恢复码 */
export function fetchTotpActivate(code: string) {
  return request.post<TotpActivateData | null>('/user/totp/activate', { code })
}

/** 停用 2FA（需当前 TOTP 或未用恢复码） */
export function fetchTotpDisable(code: string) {
  return request.post<null>('/user/totp/disable', { code })
}
