/**
 * Device onboarding, connection diagnostics, and debug-log API wrappers.
 *
 * Keep these contracts aligned with the backend `/device/:id/...` onboarding
 * routes because they drive the customer-facing first connection workflow.
 */
import { request } from '../request'

export const getDeviceConnectionDiagnostics = async (
  deviceId: string,
  params?: {
    debug_log_limit?: number
  }
) => {
  return await request.get<any>(`/device/${deviceId}/connection/diagnostics`, { params })
}

export interface DeviceConnectionGuideQuery {
  debug_log_limit?: number
  command_log_limit?: number
}

export interface DeviceConnectionGuideTwinSummary {
  desiredCount?: number
  reportedCount?: number
  matchedCount?: number
  deltaCount?: number
  unavailableCount?: number
}

export interface DeviceConnectionGuideCommandSummary {
  level?: string
  code?: string
  summary?: string
  latest_status?: string
  latest_message_id?: string
  next_actions?: string[]
}

export interface DeviceConnectionGuideWarning {
  component?: string
  reason?: string
}

export interface DeviceConnectionGuideResponse {
  device_id?: string
  evaluated_at?: string
  access?: Record<string, any> & {
    connection_profile?: {
      protocol?: string
      endpoint?: string
      host?: string
      port?: string
      tls_enabled?: boolean
      credential_mode?: string
      credential_required?: boolean
      device_type?: string
      device_number?: string
      client_id?: string
      username?: string
      telemetry_topic?: string
      command_topic?: string
      test_payload?: string
      sample_payload?: string
      http_address?: string
      sub_topic_prefix?: string
    }
  }
  readiness?: Record<string, any> & {
    level?: string
    code?: string
    summary?: string
    online?: boolean
    ready?: boolean
    latest_telemetry_at?: string
    next_actions?: string[]
    evidence?: string[]
  }
  last_connection_error?: Record<string, any> & {
    code?: string
    summary?: string
    evidence?: string[]
  }
  twin_summary?: DeviceConnectionGuideTwinSummary
  command_summary?: DeviceConnectionGuideCommandSummary
  next_steps?: Array<{
    key?: string
    title?: string
    description?: string
    status?: string
  }>
  partial_results?: DeviceConnectionGuideWarning[]
}

export const getDeviceConnectionGuide = async (deviceId: string, params?: DeviceConnectionGuideQuery) => {
  return await request.get<DeviceConnectionGuideResponse | any>(`/device/${deviceId}/onboarding/connection-guide`, {
    params
  })
}

export interface DeviceDebugStatus {
  enabled?: boolean
  expire_at?: number
  remaining_seconds?: number
  config?: {
    enabled?: boolean
    expire_at?: number
    max_items?: number
    payload_max_bytes?: number
  }
}

export interface DeviceDebugLogEntry {
  ts?: string
  protocol?: string
  direction?: string
  action?: string
  outcome?: string
  error?: string
  event?: string
  stage?: string
  meta?: Record<string, unknown>
}

export interface DeviceDebugLogsResponse {
  total?: number
  offset?: number
  limit?: number
  list?: DeviceDebugLogEntry[]
}

export type DeviceMQTTDebugAction = 'subscribe' | 'unsubscribe' | 'publish'

export interface DeviceMQTTDebugMessage {
  sequence: number
  timestamp: string
  direction: 'system' | 'inbound' | 'outbound' | string
  topic?: string
  qos?: number
  retained?: boolean
  duplicate?: boolean
  payload?: string
  truncated?: boolean
  outcome?: string
  source?: string
}

export interface DeviceMQTTDebugSubscription {
  topic: string
  mode: 'broker_subscription' | 'accepted_application_uplink_observer' | string
  qos?: number
}

export interface DeviceMQTTDebugSnapshot {
  session_id: string
  device_id: string
  connected: boolean
  platform_device_online?: boolean
  created_at: string
  expires_at: string
  subscriptions: string[]
  subscription_details?: DeviceMQTTDebugSubscription[]
  messages: DeviceMQTTDebugMessage[]
  last_sequence: number
  dropped_messages: number
  message_capacity: number
  payload_max_bytes: number
  subscription_limit: number
  uplink_observer_dropped_messages?: number
}

export interface DeviceMQTTDebugCommand {
  action: DeviceMQTTDebugAction
  topic: string
  qos?: number
  payload?: string
}

export const setDeviceDebug = async (
  deviceId: string,
  params: {
    enabled: boolean
    duration?: number
    max_items?: number
    payload_max_bytes?: number
  }
) => {
  return await request.post<DeviceDebugStatus>(`/device/${deviceId}/debug`, params)
}

export const getDeviceDebugStatus = async (deviceId: string) => {
  return await request.get<DeviceDebugStatus>(`/device/${deviceId}/debug/status`)
}

export const getDeviceDebugLogs = async (
  deviceId: string,
  params?: {
    offset?: number
    limit?: number
  }
) => {
  return await request.get<DeviceDebugLogsResponse>(`/device/${deviceId}/debug/logs`, { params })
}

export const setDeviceDebugStatus = async (deviceId: string, data: { enabled: boolean }) => {
  return await setDeviceDebug(deviceId, data)
}

export const openDeviceMQTTDebugSession = async (deviceId: string) => {
  return await request.post<DeviceMQTTDebugSnapshot>(`/device/${encodeURIComponent(deviceId)}/mqtt-debug/session`)
}

export const getDeviceMQTTDebugSession = async (
  deviceId: string,
  sessionId: string,
  params?: { after_sequence?: number; limit?: number },
  options?: { silentError?: boolean }
) => {
  return await request.get<DeviceMQTTDebugSnapshot>(
    `/device/${encodeURIComponent(deviceId)}/mqtt-debug/session/${encodeURIComponent(sessionId)}`,
    { params, silentError: options?.silentError }
  )
}

export const applyDeviceMQTTDebugCommand = async (
  deviceId: string,
  sessionId: string,
  command: DeviceMQTTDebugCommand
) => {
  return await request.post<DeviceMQTTDebugSnapshot>(
    `/device/${encodeURIComponent(deviceId)}/mqtt-debug/session/${encodeURIComponent(sessionId)}/command`,
    command
  )
}

export const closeDeviceMQTTDebugSession = async (deviceId: string, sessionId: string) => {
  return await request.delete<Api.BaseApi.Data>(
    `/device/${encodeURIComponent(deviceId)}/mqtt-debug/session/${encodeURIComponent(sessionId)}`
  )
}
