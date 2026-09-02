/**
 * 文件用途: 预注册设备页面的类型定义，与后端 device/preRegister 契约字段一一对应。
 */
export interface PreRegisterRecord {
  id: string
  name: string
  device_number: string
  activate_flag: string
  activate_at?: string | null
  batch_number: string
  current_version?: string | null
  created_at?: string | null
}

export interface PreRegisterCreatedDevice {
  id: string
  device_number: string
  name: string
  voucher: string
}

export interface PreRegisterImportResult {
  created_count: number
  devices: PreRegisterCreatedDevice[]
  skipped_existing: string[]
  skipped_duplicate_rows: string[]
}
