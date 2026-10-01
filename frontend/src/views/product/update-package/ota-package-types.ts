/* 用 type 而非 interface 声明: naive-ui 的 SelectBaseOption 带 `[k: string]: unknown` 索引签名,
   interface 声明的对象类型拿不到隐式索引签名, 会导致 NSelect 的 options 赋值报缺 type 属性。 */
export type DeviceConfigOption = {
  label: string
  value: string
}

export interface OtaPackageRecord {
  id: string
  name?: string
  version?: string
  target_version?: string
  device_config_id?: string
  device_config_name?: string
  module?: string
  package_type?: number
  signature_type?: string
  signature?: string
  package_url?: string
  additional_info?: string
  description?: string
  package_size?: number | string
  created_at?: string
  updated_at?: string
  remark?: string
}
