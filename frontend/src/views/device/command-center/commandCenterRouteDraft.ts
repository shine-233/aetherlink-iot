import type { LocationQueryRaw } from 'vue-router'
import { normalizeQueryValue } from './commandCenterState'

export type CommandCenterRouteDraft = {
  identify: string
  value: string
  source: string
  timeoutSeconds: number | null
  signature: string
  hasDraft: boolean
}

const MAX_IDENTIFIER_LENGTH = 160
const MAX_VALUE_LENGTH = 4000

function compactRouteDraftText(value: unknown, maxLength: number) {
  const text = String(normalizeQueryValue(value as any)).trim()
  if (!text) return ''
  return text.length > maxLength ? text.slice(0, maxLength) : text
}

function normalizeTimeoutSeconds(value: unknown) {
  const raw = compactRouteDraftText(value, 16)
  if (!raw) return null
  const parsed = Number(raw)
  return Number.isFinite(parsed) && parsed >= 1 ? Math.round(parsed) : null
}

export function parseCommandCenterRouteDraft(query: LocationQueryRaw): CommandCenterRouteDraft {
  const identify = compactRouteDraftText(query.command_identify, MAX_IDENTIFIER_LENGTH)
  const value = compactRouteDraftText(query.command_value, MAX_VALUE_LENGTH)
  const source = compactRouteDraftText(query.command_source, 80) || 'route'
  const timeoutSeconds = normalizeTimeoutSeconds(query.timeout_seconds)
  const hasDraft = Boolean(identify || value)

  return {
    identify,
    value,
    source,
    timeoutSeconds,
    hasDraft,
    signature: [source, identify, value, timeoutSeconds || ''].join('|')
  }
}
