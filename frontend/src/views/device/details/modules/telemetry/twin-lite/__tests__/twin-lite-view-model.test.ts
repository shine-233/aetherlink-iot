import { describe, expect, it } from 'vitest'
import type { TwinLiteRow, TwinLiteState } from '../twin-lite-normalizer'
import {
  buildTwinGuidanceItems,
  buildTwinRepairSummary,
  desiredObservationType,
  formatTwinValue,
  groupTwinRows,
  isTwinStatePayload,
  parseTwinDesiredInput,
  serializeTwinValue,
  twinConfirmationBoundaryKey,
  twinConvergenceAlertType,
  twinMetadataLines,
  twinRepairAlertType,
  twinRepairHeadlineKey
} from '../twin-lite-view-model'

const t = (key: string) => key

const row = (overrides: Partial<TwinLiteRow>): TwinLiteRow =>
  ({
    key: 'k',
    label: 'k',
    source: 'telemetry',
    desired: 1,
    reported: 1,
    comparable: true,
    matched: true,
    status: 'pending',
    ...overrides
  }) as TwinLiteRow

const state = (rows: TwinLiteRow[], summary: Partial<TwinLiteState['summary']> = {}): TwinLiteState =>
  ({
    rows,
    summary: {
      desiredCount: rows.length,
      reportedCount: 0,
      matchedCount: 0,
      deltaCount: 0,
      unavailableCount: 0,
      ...summary
    }
  }) as TwinLiteState

describe('twin-lite-view-model', () => {
  it('formats, serializes and parses desired values', () => {
    expect(formatTwinValue('')).toBe('--')
    expect(formatTwinValue({ a: 1 })).toBe('{"a":1}')
    expect(formatTwinValue(3)).toBe('3')
    expect(serializeTwinValue(null)).toBe('')
    expect(serializeTwinValue('x')).toBe('x')
    expect(serializeTwinValue({ a: 1 })).toBe('{\n  "a": 1\n}')
    expect(parseTwinDesiredInput(' {"a":1} ', 'empty')).toEqual({ a: 1 })
    expect(parseTwinDesiredInput('auto', 'empty')).toBe('auto')
    expect(() => parseTwinDesiredInput('  ', 'empty')).toThrow('empty')
  })

  it('detects backend twin payloads', () => {
    expect(isTwinStatePayload(null)).toBe(false)
    expect(isTwinStatePayload({ rows: [] })).toBe(false)
    expect(isTwinStatePayload({ rows: [], summary: {} })).toBe(true)
  })

  it('groups rows and derives repair status', () => {
    const rows = [
      row({ key: 'ok' }),
      row({ key: 'drift', matched: false, reported: 2 }),
      row({ key: 'missing', matched: false, reported: null }),
      row({ key: 'cmd', source: 'command', matched: false, reported: 5 })
    ]
    const groups = groupTwinRows(rows)
    expect(groups.drift.map((item) => item.key)).toEqual(['drift', 'missing', 'cmd'])
    expect(groups.unavailable.map((item) => item.key)).toEqual(['missing'])
    expect(groups.repairable.map((item) => item.key)).toEqual(['drift'])
    expect(groups.command.map((item) => item.key)).toEqual(['cmd'])
    expect(twinRepairAlertType(groups)).toBe('warning')
    expect(twinRepairHeadlineKey(groups)).toBe('custom.device_details.twinRepairNeedsAction')

    const empty = groupTwinRows([row({})])
    expect(twinRepairAlertType(empty)).toBe('success')
    expect(buildTwinGuidanceItems(empty, 1, t).map((item) => item.type)).toEqual(['success'])
    expect(buildTwinGuidanceItems(empty, 0, t)).toEqual([])
    expect(buildTwinGuidanceItems(groups, rows.length, t).map((item) => item.type)).toEqual(['warning', 'info', 'info'])
  })

  it('builds a copyable repair summary', () => {
    const drift = [row({ key: 'temp', label: 'Temperature', matched: false, desired: 26, reported: 25 })]
    const text = buildTwinRepairSummary('dev-1', state(drift, { deltaCount: 1 }), drift, { telemetry: 'Telemetry' }, t)
    expect(text).toContain('Device: dev-1')
    expect(text).toContain('1. [Telemetry] Temperature: desired=26; reported=25')
    expect(buildTwinRepairSummary('dev-1', state([]), [], {}, t).endsWith('--')).toBe(true)
  })

  it('maps convergence status and boundary keys', () => {
    expect(twinConvergenceAlertType('ready')).toBe('success')
    expect(twinConvergenceAlertType('no_desired')).toBe('info')
    expect(twinConvergenceAlertType('waiting_reported')).toBe('info')
    expect(twinConfirmationBoundaryKey(state([], { evidenceBoundary: 'platform_visible_evidence_only' } as any))).toBe(
      'custom.device_details.twinConfirmationBoundaryPlatform'
    )
    expect(twinConfirmationBoundaryKey(state([]))).toBe('custom.device_details.twinConfirmationBoundaryDefault')
  })

  it('renders metadata lines and desired observation types', () => {
    const lines = twinMetadataLines(row({ desired_revision: 3, last_write_source: 'api' } as any), t)
    expect(lines).toEqual([
      'custom.device_details.twinDesiredRevision: 3',
      'custom.device_details.twinLastWriteSource: custom.device_details.twinLastWriteSource.api'
    ])
    expect(desiredObservationType(null)).toBe('info')
    expect(desiredObservationType(row({ reported: null }))).toBe('info')
    expect(desiredObservationType(row({ matched: true }))).toBe('success')
    expect(desiredObservationType(row({ matched: false }))).toBe('warning')
  })
})
