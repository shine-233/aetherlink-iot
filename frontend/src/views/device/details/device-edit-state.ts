export type DeviceDetailData = Record<string, any>

export type DeviceUpdatePayload = {
  id: unknown
  name: unknown
  device_number: unknown
  label: string
  description: unknown
}

export type DeviceEditValidationMessages = {
  nameRequired: string
  numberRequired: string
  numberMax: string
}

export type DeviceEditValidationResult = {
  valid: boolean
  message?: string
}

export type DeviceDetailStateSnapshot = {
  labels: string[]
  deviceNumber: unknown
  isOnline: unknown
  name: unknown
}

export type DeviceUpdateQueryTarget = {
  id: string
  name: string
  device_number: string
  label: string
  description: string
}

export function normalizeDeviceLabels(rawLabel: unknown) {
  if (typeof rawLabel !== 'string' || !rawLabel) return []
  return rawLabel.includes(',') ? rawLabel.split(',') : [rawLabel]
}

export function normalizeDeviceDetailState(data: DeviceDetailData): DeviceDetailStateSnapshot {
  return {
    labels: normalizeDeviceLabels(data.label),
    deviceNumber: data.device_number,
    isOnline: data.is_online,
    name: data.name
  }
}

export function validateDeviceUpdate(
  deviceData: DeviceDetailData,
  messages: DeviceEditValidationMessages
): DeviceEditValidationResult {
  if (!deviceData?.name) return { valid: false, message: messages.nameRequired }
  if (!deviceData?.device_number) return { valid: false, message: messages.numberRequired }
  if (String(deviceData.device_number).length > 100) return { valid: false, message: messages.numberMax }
  return { valid: true }
}

export function createDeviceUpdatePayload(deviceData: DeviceDetailData, labels: string[]): DeviceUpdatePayload {
  return {
    id: deviceData?.id,
    name: deviceData?.name,
    device_number: deviceData?.device_number,
    label: labels.join(','),
    description: deviceData?.description
  }
}

export function syncDeviceUpdateQueryTarget(target: DeviceUpdateQueryTarget, payload: DeviceUpdatePayload) {
  target.id = payload.id as string
  target.name = payload.name as string
  target.device_number = payload.device_number as string
  target.label = payload.label
  target.description = payload.description as string
}
