import type { TelemetryCardFreshness, TelemetryCardRecord } from './telemetryCardViewState'

type Translate = (key: string) => string
type TelemetryFreshnessResolver = (telemetry: TelemetryCardRecord) => TelemetryCardFreshness

const csvCell = (value: unknown) => {
  const text = value === null || value === undefined ? '' : String(value)
  return `"${text.replace(/"/g, '""')}"`
}

const telemetryValue = (telemetry: TelemetryCardRecord) => {
  if ('value' in telemetry) return telemetry.value
  return ''
}

export function buildTelemetryCsv(
  telemetryRows: TelemetryCardRecord[],
  options: {
    getFreshness: TelemetryFreshnessResolver
    translate: Translate
  }
) {
  const header = ['key', 'label', 'value', 'unit', 'timestamp', 'freshness']
  const rows = telemetryRows.map((telemetry) => {
    const freshness = options.getFreshness(telemetry)
    return [
      telemetry.key,
      telemetry.label || telemetry.name,
      telemetryValue(telemetry),
      telemetry.unit,
      telemetry.ts,
      options.translate(freshness.i18nKey)
    ].map(csvCell)
  })

  return [header.map(csvCell), ...rows].map((row) => row.join(',')).join('\n')
}

export function downloadTelemetryCsv(filename: string, csv: string) {
  if (typeof document === 'undefined') return false

  const blob = new Blob([`\ufeff${csv}`], { type: 'text/csv;charset=utf-8' })
  const href = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = href
  link.download = filename
  link.style.display = 'none'

  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(href)

  return true
}
