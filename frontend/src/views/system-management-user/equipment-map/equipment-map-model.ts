/*
 * Shared types and pure helpers for the equipment map page.
 */

export interface DeviceRecord {
  id: string
  device_number?: string
  name?: string
  pid_number?: string
  location?: string
  is_online?: number
  warn_status?: string
}

export interface TelemetryItem {
  key: string
  label?: string | null
  value?: string | number | boolean | null
  unit?: string | null
}

export interface MapTelemetry {
  device_id?: string
  device_name?: string
  is_online?: number
  last_push_time?: string | null
  telemetry_data?: TelemetryItem[]
}

export interface ParsedLocation {
  raw: string
  lat?: number
  lng?: number
}

/** Accepts a JSON location object or a plain "lat,lng" string. */
export function parseLocation(value?: string): ParsedLocation | null {
  const raw = String(value || '').trim()
  if (!raw) return null

  try {
    const obj = JSON.parse(raw)
    const lat = Number(obj.lat ?? obj.latitude)
    const lng = Number(obj.lng ?? obj.lon ?? obj.longitude)
    if (Number.isFinite(lat) && Number.isFinite(lng)) return { raw, lat, lng }
  } catch {
    // Plain text locations are allowed.
  }

  const match = raw.match(/(-?\d+(?:\.\d+)?)\s*[, ]\s*(-?\d+(?:\.\d+)?)/)
  if (match) {
    const lat = Number(match[1])
    const lng = Number(match[2])
    if (Number.isFinite(lat) && Number.isFinite(lng)) return { raw, lat, lng }
  }

  return { raw }
}

export function toLngLat(location: ParsedLocation | null): [number, number] | null {
  if (!location || location.lat === undefined || location.lng === undefined) return null
  return [location.lng, location.lat]
}

/** Percentage position used by the fallback grid marker. */
export function toMarkerStyle(location: ParsedLocation | null) {
  if (!location || location.lat === undefined || location.lng === undefined) {
    return { left: '50%', top: '50%' }
  }
  const left = ((location.lng + 180) / 360) * 100
  const top = ((90 - location.lat) / 180) * 100
  return {
    left: `${Math.min(96, Math.max(4, left))}%`,
    top: `${Math.min(92, Math.max(8, top))}%`
  }
}

export function formatTime(value?: string | null) {
  if (!value) return '-'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString()
}

export function telemetryLabel(item: TelemetryItem) {
  return item.label || item.key
}

export function telemetryValue(item: TelemetryItem) {
  const value = item.value === undefined || item.value === null || item.value === '' ? '-' : String(item.value)
  return item.unit ? `${value} ${item.unit}` : value
}
