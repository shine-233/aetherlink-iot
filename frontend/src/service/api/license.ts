import { request } from '../request'

/** P3 商业许可证状态（SYS_ADMIN） */
export interface LicenseStatus {
  enabled: boolean
  required: boolean
  valid: boolean
  edition?: string
  issued_to?: string
  features?: string[]
  max_devices?: number
  max_tenants?: number
  not_before_ms?: number
  not_after_ms?: number
  fingerprint?: string
  reason?: string
}

export function fetchLicenseStatus() {
  return request.get<LicenseStatus>('/license/status')
}
