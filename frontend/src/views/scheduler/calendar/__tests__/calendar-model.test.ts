import { describe, expect, it } from 'vitest'
import type { SchedulerEventItem } from '@/service/api'
import {
  bucketEventsByDay,
  buildCalendarCells,
  buildEventPayload,
  emptyFormModel,
  formModelFromRow,
  localDateKey,
  monthWindow,
  shiftMonthDate,
  sourceLabelKey,
  validateFormModel
} from '../calendar-model'

function event(overrides: Partial<SchedulerEventItem>): SchedulerEventItem {
  return {
    id: 'e1',
    name: 'n',
    source_type: 'scene',
    origin: 'scheduler_registry',
    ref_type: '',
    ref_id: 'r',
    cron: '0 * * * *',
    timezone: 'UTC',
    enabled: true,
    next_run_at: null,
    ...overrides
  } as SchedulerEventItem
}

describe('calendar grid', () => {
  it('builds a Monday-first 6x7 grid covering the month', () => {
    // 2026-09-01 is a Tuesday → grid starts Monday 2026-08-31.
    const cells = buildCalendarCells(new Date(2026, 8, 15), new Date(2026, 8, 10))
    expect(cells).toHaveLength(42)
    expect(cells[0].key).toBe('2026-08-31')
    expect(cells[0].inMonth).toBe(false)
    expect(cells[1].key).toBe('2026-09-01')
    expect(cells.filter((c) => c.inMonth)).toHaveLength(30)
    expect(cells.find((c) => c.isToday)?.key).toBe('2026-09-10')
  })

  it('computes month windows and shifts across years', () => {
    const { fromMs, toMs } = monthWindow(new Date(2026, 1, 10))
    expect(new Date(fromMs).getDate()).toBe(1)
    expect(new Date(toMs).getDate()).toBe(28)
    expect(localDateKey(shiftMonthDate(new Date(2026, 11, 5), 1))).toBe('2027-01-01')
  })

  it('buckets events by local day and skips unscheduled ones', () => {
    const at = new Date(2026, 8, 3, 10, 0).toISOString()
    const buckets = bucketEventsByDay([
      event({ id: 'a', next_run_at: at }),
      event({ id: 'b', next_run_at: at }),
      event({ id: 'c', next_run_at: null })
    ])
    expect(buckets.get('2026-09-03')?.map((e) => e.id)).toEqual(['a', 'b'])
    expect(buckets.size).toBe(1)
  })

  it('maps source types to i18n keys', () => {
    expect(sourceLabelKey('rpc')).toBe('page.schedulerCalendar.sourceRpc')
  })
})

describe('calendar form', () => {
  it('validates per event type', () => {
    const model = emptyFormModel()
    expect(validateFormModel(model)).toBe('page.schedulerCalendar.formNameRequired')
    model.name = 'x'
    expect(validateFormModel(model)).toBe('page.schedulerCalendar.formRefIdRequired')
    model.ref_id = 'scene-1'
    expect(validateFormModel(model)).toBe('page.schedulerCalendar.formCronRequired')
    model.cron = '0 * * * *'
    expect(validateFormModel(model)).toBeNull()
    expect(validateFormModel({ ...model, event_type: 'rpc', next_run_at: null })).toBe(
      'page.schedulerCalendar.formRunAtRequired'
    )
  })

  it('sends only cron for cron types and only next_run_at for rpc', () => {
    const base = { ...emptyFormModel(), name: ' n ', ref_id: ' ', cron: ' 0 * * * * ', next_run_at: 0 }
    expect(buildEventPayload(base)).toEqual({
      name: 'n',
      ref_id: undefined,
      cron: '0 * * * *',
      next_run_at: undefined,
      enabled: true
    })
    const rpc = buildEventPayload({ ...base, event_type: 'rpc', next_run_at: Date.UTC(2026, 0, 1) })
    expect(rpc.cron).toBeUndefined()
    expect(rpc.next_run_at).toBe('2026-01-01T00:00:00.000Z')
  })

  it('round-trips a row into the form model', () => {
    const model = formModelFromRow(event({ source_type: 'rpc', next_run_at: '2026-01-01T00:00:00Z', enabled: false }))
    expect(model.event_type).toBe('rpc')
    expect(model.next_run_at).toBe(Date.UTC(2026, 0, 1))
    expect(model.enabled).toBe(false)
  })
})
