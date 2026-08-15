/**
 * 文件用途: RDI 设备配置、遥测、历史、命令和分享相关 API wrapper。
 * 核心逻辑: 为设备详情 RDI 面板和分享页面提供统一的后端调用入口。
 * 关键注意事项: 分享 token、命令 payload、温度/告警字段和配置字段是高风险兼容契约，需与后端 RDI service 同步验证。
 * 重构建议: 按配置、遥测历史、命令、分享拆分函数组，并补失败分支与权限边界测试。
 */
import { request } from '../request'

export interface RDIThingModelItem {
  identifier: string
  name: string
  kind?: string
  data_type: string
  unit?: string
  range?: string
  read_write?: string
  enum?: string[]
  default?: unknown
  required: boolean
  description?: string
}

export interface RDIServiceModelItem {
  identifier: string
  name: string
  call_type: string
  inputs: string[]
  outputs: string[]
  description?: string
}

export interface RDIThingModel {
  telemetry: RDIThingModelItem[]
  properties: RDIThingModelItem[]
  events: RDIThingModelItem[]
  services: RDIServiceModelItem[]
}

export interface RDIConfig {
  data_collection_interval: number
  alarm_sensor_1_enabled: boolean
  alarm_sensor_2_enabled: boolean
  sensor_1_upper: number
  sensor_1_lower: number
  sensor_2_upper: number
  sensor_2_lower: number
  sensor_1_duration: number
  sensor_2_duration: number
  switch_1_alarm_mode: 'powered_on' | 'powered_off' | 'disabled'
  switch_2_alarm_mode: 'powered_on' | 'powered_off' | 'disabled'
  switch_1_alarm_duration: number
  switch_2_alarm_duration: number
  dry_contact_alarm_level: 'high' | 'low'
  dry_contact_normal_level: 'high' | 'low'
  dry_contact_alarm_delay: number
  dry_contact_normal_delay: number
  notification_enabled: boolean
  notification_temperature_alarm: boolean
  notification_switch_alarm: boolean
  notification_warranty_alarm: boolean
  sensor_alarm_emails: string
  switch_alarm_emails: string
  warranty_alarm_emails: string
  sensor_1_alarm_emails: string
  sensor_2_alarm_emails: string
  switch_1_alarm_emails: string
  switch_2_alarm_emails: string
  field_setting?: Record<string, unknown>
}

export interface RDISystemInfo {
  installation_location?: string
  address?: string
  installation_date?: string
  installer_company?: string
  installer_contact?: string
  installer_name?: string
  installer_phone?: string
  installer_email?: string
  controller_serial_number?: string
  maintenance_technician?: string
  customer_name?: string
  contact_email?: string
  contact_phone?: string
  warranty_status?: string
  extra_fields?: Record<string, unknown>
}

export interface RDIDeviceConfigResponse {
  device_id: string
  pid_number: string
  device_name: string
  firmware_version: string
  online: boolean
  connection_type: string
  config: RDIConfig
  system_info: RDISystemInfo
  additional_info: Record<string, unknown>
  thing_model: RDIThingModel
  command_tracking?: RDICommandTracking
}

export interface RDICommandTracking {
  message_id: string
  status: string
  device_id: string
  identifier: string
  operation_type: string
  log_recorded: boolean
}

export type RDICommandIdentifier =
  | 'set_dry_contact'
  | 'set_alarm_config'
  | 'set_field_setting'
  | 'test_dry_contact'
  | 'ota_upgrade'
  | 'unbind_device'
  | 'factory_reset'

export interface RDICommandReq {
  identifier: RDICommandIdentifier
  params?: Record<string, unknown>
}

export interface RDISendCommandResponse {
  device_id: string
  identifier: string
  params?: Record<string, unknown>
  status: string
  message_id?: string
  log_recorded?: boolean
  operation_type?: string
  tracking_status?: string
  command_tracking?: RDICommandTracking
}

export interface RDIShareTokenResponse {
  device_id: string
  token: string
  share_path: string
  accept_path?: string
  expires_at: number
}

export interface RDIAcceptShareResponse {
  device: RDIDeviceConfigResponse
  accepted_at: number
  already_accepted: boolean
  shared_with_me: boolean
}

export interface RDISharedDeviceRecord {
  device: RDIDeviceConfigResponse
  accepted_at: number
}

export interface RDISharedDeviceListParams {
  page?: number
  page_size?: number
  device_id?: string
  device_name?: string
}

export interface RDISharedDeviceListResponse {
  total: number
  list: RDISharedDeviceRecord[]
}

// REQ-47：owner 主动撤销分享的返回结果，汇报本次实际清理掉的 token 与接收人数量。
export interface RDIRevokeShareResponse {
  device_id: string
  revoked_tokens: number
  revoked_recipients: number
  revoked_at: number
}

export interface RDIHistoryParams {
  key: string
  start_time: number
  end_time: number
  export_excel?: boolean
  export_format?: 'excel' | 'csv'
  page?: number
  page_size?: number
}

export interface RDIHistoryPoint {
  key: string
  ts: number | string
  value: unknown
}

export interface RDIHistoryResponse {
  total: number
  list: RDIHistoryPoint[]
  filePath?: string
  file_path?: string
  fileName?: string
  fileType?: 'excel' | 'csv'
}

export interface RDILatestFirmwareResponse {
  device_id: string
  current_version: string
  update_available: boolean
  package?: Record<string, unknown>
}

export const rdiThingModel = async () => {
  return await request.get<RDIThingModel>('/rdi/thing-model')
}

export const activateRdiDevice = async (params: { pid_number: string; name?: string }) => {
  return await request.post<RDIDeviceConfigResponse>('/rdi/devices/activate', params)
}

export const rdiDeviceConfig = async (deviceId: string, requestConfig: Record<string, unknown> = {}) => {
  return await request.get<RDIDeviceConfigResponse>(`/rdi/devices/${encodeURIComponent(deviceId)}/config`, requestConfig as any)
}

export const rdiDeviceHistory = async (
  deviceId: string,
  params: RDIHistoryParams,
  requestConfig: Record<string, unknown> = {}
) => {
  return await request.get<RDIHistoryResponse>(`/rdi/devices/${encodeURIComponent(deviceId)}/history`, {
    ...(requestConfig as any),
    params
  })
}

export const rdiLatestFirmware = async (deviceId: string) => {
  return await request.get<RDILatestFirmwareResponse>(`/rdi/devices/${encodeURIComponent(deviceId)}/latest-firmware`)
}

export const updateRdiDeviceConfig = async (
  deviceId: string,
  params: { config: RDIConfig; system_info?: RDISystemInfo; apply_to_device?: boolean }
) => {
  return await request.put<RDIDeviceConfigResponse>(`/rdi/devices/${encodeURIComponent(deviceId)}/config`, params)
}

export const sendRdiCommand = async (deviceId: string, params: RDICommandReq) => {
  return await request.post<RDISendCommandResponse>(`/rdi/devices/${encodeURIComponent(deviceId)}/commands`, params)
}

export const createRdiShareToken = async (deviceId: string, params: { expires_in?: number }) => {
  return await request.post<RDIShareTokenResponse>(`/rdi/devices/${encodeURIComponent(deviceId)}/share-token`, params)
}

export const acceptRdiSharedDevice = async (token: string) => {
  return await request.post<RDIAcceptShareResponse>(`/rdi/share-tokens/${encodeURIComponent(token)}/accept`)
}

export const rdiSharedWithMeDevices = async (params: RDISharedDeviceListParams = {}) => {
  return await request.get<RDISharedDeviceListResponse>('/rdi/shared-with-me/devices', { params })
}

// REQ-47：撤销整条分享链接。token 失效后凭它接受分享的接收人也会一并被清除。
export const revokeRdiShareToken = async (deviceId: string, token: string) => {
  return await request.delete<RDIRevokeShareResponse>(
    `/rdi/devices/${encodeURIComponent(deviceId)}/share-tokens/${encodeURIComponent(token)}`
  )
}

// REQ-47：只撤销单个接收人的访问权，分享链接本身保持有效。
export const revokeRdiShareRecipient = async (deviceId: string, userId: string) => {
  return await request.delete<RDIRevokeShareResponse>(
    `/rdi/devices/${encodeURIComponent(deviceId)}/share-recipients/${encodeURIComponent(userId)}`
  )
}
