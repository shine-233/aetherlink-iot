type TelemetryItem = DeviceManagement.telemetryData
type TelemetryPatch = Partial<TelemetryItem> & Record<string, any>

export const hasIncomingTelemetryValue = (value: unknown) => value !== null && value !== undefined && value !== ''

const knownTelemetryKeySet = (items: TelemetryItem[]) =>
  new Set(items.flatMap((item) => (item.key === 'systime' ? [] : [item.key])))

const telemetryTemplateFields = (template?: TelemetryPatch) => {
  const { key: _originKey, label: _label, ...rest } = template || {}
  return rest
}

const buildTelemetryItem = (key: string, value: unknown, template?: TelemetryPatch, ts?: string): TelemetryItem =>
  ({
    ...telemetryTemplateFields(template),
    key,
    value,
    ts,
    unit: ''
  }) as TelemetryItem

export const mergeRealtimeTelemetry = (
  currentItems: TelemetryItem[],
  info: Record<string, any>,
  template?: TelemetryPatch
): TelemetryItem[] => {
  const knownKeys = knownTelemetryKeySet(currentItems)
  const updatedTelemetry = currentItems.map((item) => {
    const incomingValue = info[item.key]
    const hasValue = hasIncomingTelemetryValue(incomingValue)
    return {
      ...item,
      value: hasValue ? incomingValue : item.value,
      ts: hasValue && info.systime ? info.systime : item.ts || ''
    } as TelemetryItem
  })
  const newTelemetry = Object.keys(info).reduce<TelemetryItem[]>((result, key) => {
    if (key === 'systime' || knownKeys.has(key) || !hasIncomingTelemetryValue(info[key])) return result
    result.push(buildTelemetryItem(key, info[key], template, info.systime))
    return result
  }, [])

  return [...updatedTelemetry, ...newTelemetry]
}
