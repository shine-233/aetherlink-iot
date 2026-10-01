import { describe, expect, it, vi } from 'vitest'

vi.mock('@/locales', () => ({ $t: (key: string, fallback?: string) => fallback ?? key }))

import {
  isReportRevisionConflict,
  normalizeReportFormat,
  reportErrorCode,
  reportFormFromSchedule,
  splitReportLines
} from '../report-helpers'

describe('report-helpers', () => {
  it('normalizes unknown formats to csv', () => {
    expect(normalizeReportFormat('pdf')).toBe('pdf')
    expect(normalizeReportFormat('html')).toBe('html')
    expect(normalizeReportFormat('xlsx')).toBe('csv')
    expect(normalizeReportFormat(null)).toBe('csv')
  })

  it('splits, trims and deduplicates multi-line targets', () => {
    expect(splitReportLines(' a\nb, a ,\n\nc ')).toEqual(['a', 'b', 'c'])
  })

  it('builds an edit form with revision and defensive copies', () => {
    const schedule = {
      id: 's1',
      name: 'n',
      cron_expr: '* * * * *',
      timezone: '',
      recipients: 'r',
      device_ids: ['d'],
      keys: ['k'],
      lookback_hours: 2,
      format: 'weird',
      enabled: false,
      revision: 5
    } as any
    const form = reportFormFromSchedule(schedule)
    expect(form).toMatchObject({ timezone: 'UTC', format: 'csv', revision: 5, enabled: false })
    form.device_ids.push('x')
    expect(schedule.device_ids).toEqual(['d'])
    expect(reportFormFromSchedule()).not.toHaveProperty('revision')
  })

  it('reads structured error codes from every supported envelope', () => {
    expect(reportErrorCode({ response: { data: { code: 201002 } } })).toBe(201002)
    expect(reportErrorCode({ data: { code: 7 } })).toBe(7)
    expect(reportErrorCode({ code: 9 })).toBe(9)
    expect(reportErrorCode('x')).toBeUndefined()
    expect(isReportRevisionConflict({ code: 201002 })).toBe(true)
    expect(isReportRevisionConflict({ message: 'revision conflict' })).toBe(false)
  })
})
