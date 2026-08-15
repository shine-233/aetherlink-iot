import { describe, expect, it } from 'vitest'

import { buildTelemetryHistoryDownloadUrl } from '../history-data-state'

describe('history-data-state', () => {
  it('builds telemetry history download URLs from backend file paths', () => {
    expect(buildTelemetryHistoryDownloadUrl('http://localhost:9999/api/v1', 'downloads/history.csv')).toBe(
      'http://localhost:9999/downloads/history.csv'
    )
    expect(buildTelemetryHistoryDownloadUrl('http://localhost:9999/api/v1/', '/downloads/history.csv')).toBe(
      'http://localhost:9999/downloads/history.csv'
    )
  })

  it('rejects empty, absolute, and traversal download paths', () => {
    expect(buildTelemetryHistoryDownloadUrl('http://localhost:9999/api/v1', '')).toBe('')
    expect(buildTelemetryHistoryDownloadUrl('http://localhost:9999/api/v1', 'https://example.com/file.csv')).toBe('')
    expect(buildTelemetryHistoryDownloadUrl('http://localhost:9999/api/v1', '../secret.csv')).toBe('')
    expect(buildTelemetryHistoryDownloadUrl('http://localhost:9999/api/v1', 'downloads/../secret.csv')).toBe('')
  })
})
