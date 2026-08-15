import { describe, expect, it } from 'vitest'
import {
  buildTwinLiteEvidenceBundle,
  buildTwinLiteEvidenceFileName,
  buildTwinLiteState,
  normalizeTwinLiteConvergenceStatus,
  normalizeTwinLiteNextAction
} from '../twin-lite-normalizer'

describe('twin-lite-normalizer', () => {
  it('maps telemetry and attribute desired payloads into comparable rows', () => {
    const result = buildTwinLiteState(
      [
        {
          id: 'desired-revision-1',
          send_type: 'telemetry',
          payload: '{"temperature":22,"switch":true}',
          status: 'pending',
          created_at: '2026-07-19T00:00:00.000Z',
          expiry_time: '2026-07-20T00:00:00.000Z'
        },
        { send_type: 'attribute', payload: '{"device_name":"Aether"}', status: 'pending' }
      ],
      [
        { key: 'temperature', value: 21, ts: '2026-07-19T00:05:00.000Z' },
        { key: 'switch', value: true }
      ],
      [{ key: 'device_name', value: 'Aether' }]
    )

    expect(result.summary.desiredCount).toBe(3)
    expect(result.summary.matchedCount).toBe(2)
    expect(result.summary.deltaCount).toBe(1)
    expect(result.rows.find((row) => row.key === 'temperature')).toMatchObject({
      source: 'telemetry',
      desired: 22,
      reported: 21,
      matched: false,
      desired_updated_at: '2026-07-19T00:00:00.000Z',
      desired_expires_at: '2026-07-20T00:00:00.000Z',
      reported_at: '2026-07-19T00:05:00.000Z',
      desired_revision: 'desired-revision-1',
      last_write_source: 'reported'
    })
  })

  it('does not invent last-write evidence when a reported timestamp is missing', () => {
    const result = buildTwinLiteState(
      [{ id: 'desired-revision-2', send_type: 'telemetry', payload: '{"temperature":22}' }],
      [{ key: 'temperature', value: 21 }],
      []
    )

    expect(result.rows[0]).toMatchObject({
      desired_revision: 'desired-revision-2',
      reported: 21
    })
    expect(result.rows[0].reported_at).toBeUndefined()
    expect(result.rows[0].last_write_source).toBeUndefined()
  })

  it('marks command rows as not comparable', () => {
    const result = buildTwinLiteState(
      [{ id: 'cmd-1', send_type: 'command', label: 'Reboot', payload: '{"reboot":true}' }],
      [],
      []
    )

    expect(result.summary.desiredCount).toBe(1)
    expect(result.summary.deltaCount).toBe(0)
    expect(result.rows[0]).toMatchObject({
      source: 'command',
      comparable: false,
      matched: false
    })
  })

  it('normalizes untrusted backend status and action values to known locale keys', () => {
    expect(normalizeTwinLiteConvergenceStatus('ready', 'needs_review')).toBe('ready')
    expect(normalizeTwinLiteConvergenceStatus('backend_custom_status', 'needs_review')).toBe('needs_review')
    expect(normalizeTwinLiteNextAction('wait_for_reported_state', 'ready')).toBe('wait_for_reported_state')
    expect(normalizeTwinLiteNextAction('Compare delta rows before closing.', 'waiting_reported')).toBe(
      'wait_for_reported_state'
    )
  })

  it('builds a downloadable platform-visible evidence bundle', () => {
    const state = buildTwinLiteState(
      [{ send_type: 'telemetry', payload: '{"temperature":22}', status: 'pending' }],
      [{ key: 'temperature', value: 21 }],
      []
    )
    const bundle = buildTwinLiteEvidenceBundle({
      deviceId: 'device/alpha 1',
      exportedAt: '2026-07-07T00:00:00.000Z',
      state,
      status: 'needs_review',
      nextAction: 'Compare delta rows before closing the customer case.',
      evidenceBoundary: 'Platform-visible evidence only.'
    })

    expect(bundle).toMatchObject({
      schema_version: 'twin-lite-evidence-v1',
      device_id: 'device/alpha 1',
      exported_at: '2026-07-07T00:00:00.000Z',
      status: 'needs_review',
      next_action: 'Compare delta rows before closing the customer case.',
      evidence_boundary: 'Platform-visible evidence only.',
      scope: {
        source: 'device-twin-workbench',
        rows: 1,
        platform_visible_evidence_only: true
      },
      summary: {
        desiredCount: 1,
        deltaCount: 1
      }
    })
    expect(bundle.rows[0]).toMatchObject({
      key: 'temperature',
      desired: 22,
      reported: 21,
      matched: false
    })
    expect(buildTwinLiteEvidenceFileName('device/alpha 1', bundle.exported_at)).toBe(
      'device-twin-evidence-device-alpha-1-2026-07-07T00-00-00-000Z.json'
    )
  })
})
