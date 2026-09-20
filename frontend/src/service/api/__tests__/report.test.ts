import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockGet, mockPost, mockPut, mockDelete } = vi.hoisted(() => ({
  mockGet: vi.fn(),
  mockPost: vi.fn(),
  mockPut: vi.fn(),
  mockDelete: vi.fn()
}))

vi.mock('@/service/request', () => ({
  request: { get: mockGet, post: mockPost, put: mockPut, delete: mockDelete }
}))

import {
  createReportSchedule,
  deleteReportSchedule,
  getReportRun,
  getReportSchedule,
  listReportRuns,
  listReportSchedules,
  retryReportRun,
  runReportSchedule,
  updateReportSchedule,
  type CreateReportSchedulePayload,
  type UpdateReportSchedulePayload
} from '../report'

const createPayload: CreateReportSchedulePayload = {
  name: 'Daily fleet',
  cron_expr: '0 8 * * *',
  timezone: 'UTC',
  recipients: 'ops@example.com',
  device_ids: ['device/1'],
  keys: ['temperature'],
  lookback_hours: 24,
  format: 'csv'
}
const updatePayload: UpdateReportSchedulePayload = {
  revision: 3,
  name: 'Updated fleet',
  enabled: false
}

describe('report API service', () => {
  beforeEach(() => vi.clearAllMocks())

  it('uses paginated schedule routes and operation-specific payloads', async () => {
    await listReportSchedules({ page: 2, page_size: 10, search: 'fleet', enabled: true })
    await getReportSchedule('schedule/1')
    await createReportSchedule(createPayload)
    await updateReportSchedule('schedule/1', updatePayload)
    await deleteReportSchedule('schedule/1', 3)

    expect(mockGet).toHaveBeenNthCalledWith(1, '/report/schedules', {
      params: { page: 2, page_size: 10, search: 'fleet', enabled: true },
      silentError: true
    })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/report/schedules/schedule%2F1', { silentError: true })
    expect(mockPost).toHaveBeenCalledWith('/report/schedules', createPayload, { silentError: true })
    expect(createPayload).not.toHaveProperty('enabled')
    expect(mockPut).toHaveBeenCalledWith('/report/schedules/schedule%2F1', updatePayload, { silentError: true })
    expect(updatePayload).toEqual({ revision: 3, name: 'Updated fleet', enabled: false })
    expect(updatePayload).not.toHaveProperty('id')
    expect(mockDelete).toHaveBeenCalledWith('/report/schedules/schedule%2F1', {
      params: { revision: 3 },
      silentError: true
    })
  })

  it('keeps one caller-provided idempotency key on run-now transport retries', async () => {
    const options = { idempotencyKey: 'run-action-1' }
    await runReportSchedule('schedule-1', options)
    await runReportSchedule('schedule-1', options)

    expect(mockPost).toHaveBeenNthCalledWith(
      1,
      '/report/schedules/schedule-1/run',
      {},
      {
        headers: { 'Idempotency-Key': 'run-action-1' },
        silentError: true
      }
    )
    expect(mockPost).toHaveBeenNthCalledWith(
      2,
      '/report/schedules/schedule-1/run',
      {},
      {
        headers: { 'Idempotency-Key': 'run-action-1' },
        silentError: true
      }
    )
  })

  it('uses nested history, detail, and retry routes with a new logical-action key', async () => {
    await listReportRuns('schedule-1', { page: 1, page_size: 10 })
    await getReportRun('schedule-1', 'run/1')
    await retryReportRun('schedule-1', 'run/1', { idempotencyKey: 'retry-action-1' })

    expect(mockGet).toHaveBeenNthCalledWith(1, '/report/schedules/schedule-1/runs', {
      params: { page: 1, page_size: 10 },
      silentError: true
    })
    expect(mockGet).toHaveBeenNthCalledWith(2, '/report/schedules/schedule-1/runs/run%2F1', { silentError: true })
    expect(mockPost).toHaveBeenCalledWith(
      '/report/schedules/schedule-1/runs/run%2F1/retry',
      {},
      {
        headers: { 'Idempotency-Key': 'retry-action-1' },
        silentError: true
      }
    )
    expect(mockPost).not.toHaveBeenCalledWith(
      expect.anything(),
      expect.anything(),
      expect.objectContaining({
        headers: { 'Idempotency-Key': 'run-action-1' }
      })
    )
  })
})
