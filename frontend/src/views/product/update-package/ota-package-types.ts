export interface DeviceConfigOption {
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
