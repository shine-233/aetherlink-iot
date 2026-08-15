export interface OtaPackageRecord {
  id: string
  name?: string
  version?: string
  target_version?: string
  device_config_id?: string
  device_config_name?: string
  signature?: string
}

export interface OtaTaskRecord {
  id: string
  name?: string
  description?: string
  ota_upgrade_package_id?: string
  device_count?: number
  target_mode?: 'explicit' | 'filter' | string
  target_filter?: string | Record<string, unknown> | null
  preview_total?: number | null
  selected_count?: number | null
  created_by?: string | null
  created_by_authority?: string | null
  created_at?: string
  remark?: string
}

export interface OtaTaskStatisticsItem {
  status?: string | number
  count?: number
}

export interface OtaTaskDetailRecord {
  id: string
  device_id?: string
  ota_upgrade_task_id?: string
  device_number?: string
  name?: string
  current_version?: string
  version?: string
  steps?: number
  status?: number
  status_description?: string
  updated_at?: string
}

export type RolloutSummaryTagType = 'default' | 'info' | 'warning' | 'success' | 'error'

export type RolloutSummaryItem = {
  key: string
  label: string
  value: number
  type: RolloutSummaryTagType
}

export type RolloutGuidanceItem = {
  key: string
  title: string
  description: string
  value: number
  type: RolloutSummaryTagType
}
