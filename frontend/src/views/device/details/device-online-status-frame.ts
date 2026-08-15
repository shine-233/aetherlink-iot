/**
 * Device online-status websocket frame normalization.
 *
 * The broker/backend has emitted several frame shapes over time: arrays,
 * `data`/`payload` wrappers, camel/snake device IDs, boolean/number/string
 * online flags, and second/millisecond timestamps. Keeping that compatibility
 * here lets the device details page stay focused on UI orchestration.
 */

export function normalizeOnlineStatus(payload: unknown, targetDeviceId: string): number | null {
  if (Array.isArray(payload)) {
    for (const item of payload) {
      const status = normalizeOnlineStatus(item, targetDeviceId)
      if (status !== null) return status
    }
    return null
  }

  if (!payload || typeof payload !== 'object') return null
  const info = payload as Record<string, unknown>
  if (info.data !== undefined) return normalizeOnlineStatus(info.data, targetDeviceId)
  if (info.payload !== undefined) return normalizeOnlineStatus(info.payload, targetDeviceId)

  const frameDeviceId = info.device_id ?? info.deviceId
  if (frameDeviceId && String(frameDeviceId) !== String(targetDeviceId)) return null

  const rawStatus = info.is_online ?? info.isOnline
  if (typeof rawStatus === 'boolean') return rawStatus ? 1 : 0
  if (typeof rawStatus === 'number') return rawStatus === 1 ? 1 : 0
  if (typeof rawStatus === 'string') {
    const normalized = rawStatus.trim().toLowerCase()
    if (normalized === '1' || normalized === 'true' || normalized === 'online') return 1
    if (normalized === '0' || normalized === 'false' || normalized === 'offline') return 0
  }

  return null
}

function normalizeStatusTimestampValue(raw: unknown): string {
  if (raw instanceof Date) return raw.toISOString()
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    const millis = raw < 1_000_000_000_000 ? raw * 1000 : raw
    return new Date(millis).toISOString()
  }
  if (typeof raw !== 'string') return ''

  const trimmed = raw.trim()
  if (!trimmed) return ''

  const numeric = Number(trimmed)
  if (Number.isFinite(numeric) && /^\d+(\.\d+)?$/.test(trimmed)) {
    const millis = numeric < 1_000_000_000_000 ? numeric * 1000 : numeric
    return new Date(millis).toISOString()
  }

  return trimmed
}

export function normalizeOnlineStatusUpdatedAt(payload: unknown, targetDeviceId: string): string {
  if (Array.isArray(payload)) {
    for (const item of payload) {
      const updatedAt = normalizeOnlineStatusUpdatedAt(item, targetDeviceId)
      if (updatedAt) return updatedAt
    }
    return ''
  }

  if (!payload || typeof payload !== 'object') return ''
  const info = payload as Record<string, unknown>

  const nestedUpdatedAt =
    info.data !== undefined
      ? normalizeOnlineStatusUpdatedAt(info.data, targetDeviceId)
      : info.payload !== undefined
        ? normalizeOnlineStatusUpdatedAt(info.payload, targetDeviceId)
        : ''
  if (nestedUpdatedAt) return nestedUpdatedAt

  const frameDeviceId = info.device_id ?? info.deviceId
  if (frameDeviceId && String(frameDeviceId) !== String(targetDeviceId)) return ''

  for (const key of [
    'online_status_updated_at',
    'onlineStatusUpdatedAt',
    'status_updated_at',
    'statusUpdatedAt',
    'updated_at',
    'updatedAt',
    'timestamp',
    'time',
    'ts'
  ]) {
    const updatedAt = normalizeStatusTimestampValue(info[key])
    if (updatedAt) return updatedAt
  }

  return ''
}
