import { describe, expect, it, vi } from 'vitest'
import type { ReportRun } from '@/service/api/report'
import {
  canRetryReportRun,
  createReportIdempotencyKey,
  isUncertainReportTransportError,
  reportStatusTagType
} from '../report-model'

const run = (overrides: Partial<ReportRun> = {}): ReportRun => ({
  run_id: 'run-1',
  schedule_id: 'schedule-1',
  trigger: 'manual',
  overall_status: 'processing',
  generation_status: 'processing',
  delivery_status: 'pending',
  window_start_at: '2026-09-08T00:00:00Z',
  window_end_at: '2026-09-09T00:00:00Z',
  generation_attempts: 1,
  delivery_attempts: 0,
  duplicate_delivery_risk: false,
  created_at: '2026-09-09T00:00:01Z',
  ...overrides
})

describe('report view model', () => {
  it('allows explicit retry only for failed or ambiguous overall outcomes', () => {
    expect(
      canRetryReportRun(run({ overall_status: 'failed', generation_status: 'failed', delivery_status: 'pending' }))
    ).toBe(true)
    expect(canRetryReportRun(run({ overall_status: 'processing', delivery_status: 'failed' }))).toBe(false)
    expect(canRetryReportRun(run({ overall_status: 'ambiguous', delivery_status: 'ambiguous' }))).toBe(true)
    expect(canRetryReportRun(run({ overall_status: 'succeeded', delivery_status: 'accepted' }))).toBe(false)
    expect(canRetryReportRun(run({ overall_status: 'retrying', delivery_status: 'retrying' }))).toBe(false)
  })

  it('renders projected active states as info, accepted as success, and ambiguous as warning', () => {
    expect(reportStatusTagType('queued')).toBe('info')
    expect(reportStatusTagType('running')).toBe('info')
    expect(reportStatusTagType('accepted')).toBe('success')
    expect(reportStatusTagType('ambiguous')).toBe('warning')
  })

  it('blocks retry when the owning schedule is disabled', () => {
    const failed = run({ overall_status: 'failed', generation_status: 'failed' })
    expect(canRetryReportRun(failed, true)).toBe(true)
    expect(canRetryReportRun(failed, false)).toBe(false)
  })

  it('distinguishes uncertain no-response transport errors from definite responses', () => {
    expect(isUncertainReportTransportError({ code: 'ERR_NETWORK', request: {} })).toBe(true)
    expect(isUncertainReportTransportError({ code: 'ECONNABORTED', isAxiosError: true })).toBe(true)
    expect(isUncertainReportTransportError({ code: 'ERR_NETWORK', response: { status: 503 } })).toBe(false)
    expect(isUncertainReportTransportError({ response: { status: 409 } })).toBe(false)
    expect(isUncertainReportTransportError(new Error('local rejection'))).toBe(false)
  })

  it('creates a new key for every logical action', () => {
    const randomUUID = vi
      .spyOn(crypto, 'randomUUID')
      .mockReturnValueOnce('00000000-0000-4000-8000-000000000001')
      .mockReturnValueOnce('00000000-0000-4000-8000-000000000002')

    expect(createReportIdempotencyKey()).not.toBe(createReportIdempotencyKey())
    randomUUID.mockRestore()
  })
})
