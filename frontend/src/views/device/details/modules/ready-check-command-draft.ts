import type { LocationQueryRaw } from 'vue-router'

export type RecommendedCommandDraft = {
  identify: string
  value: string
  label: string
}

const MAX_COMMAND_VALUE_LENGTH = 4000

export function normalizeRecommendedCommandValue(value: unknown) {
  if (value === undefined || value === null) return ''
  const text = typeof value === 'string' ? value.trim() : JSON.stringify(value)
  if (!text || text.length > MAX_COMMAND_VALUE_LENGTH) return ''
  return text
}

export function buildRecommendedCommandDraft(commands: unknown): RecommendedCommandDraft | null {
  const list = Array.isArray(commands) ? commands : []
  const row = list.find((item: any) => {
    const identify = String(item?.data_identifier || item?.identify || item?.identifier || '').trim()
    return Boolean(identify)
  }) as any
  if (!row) return null

  const identify = String(row.data_identifier || row.identify || row.identifier || '').trim()
  const value = normalizeRecommendedCommandValue(row.params || row.instruct || row.command_value || row.value)
  const label = String(row.data_name || row.name || row.title || row.label || identify).trim()
  return { identify, value, label }
}

export function buildReadyCheckCommandCenterQuery(input: {
  deviceId: string
  draft?: RecommendedCommandDraft | null
  timeoutSeconds?: number
}): LocationQueryRaw {
  const draft = input.draft || null
  return {
    device_ids: input.deviceId,
    fleet_source: 'device_details',
    fleet_scope: 'single_device',
    fleet_selected_count: 1,
    first_device_id: input.deviceId,
    command_source: draft ? 'ready_check' : undefined,
    command_identify: draft?.identify || undefined,
    command_value: draft?.value || undefined,
    timeout_seconds: draft ? input.timeoutSeconds || 60 : undefined
  }
}
