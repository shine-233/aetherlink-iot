import { describe, expect, it } from 'vitest'
import {
  buildCycleOptions,
  buildExpirationTimeOptions,
  buildMonthRangeOptions,
  buildTimeConditionOptions,
  buildWeekOptions,
  createScheduleConditionFields,
  resetRepeatScheduleFields
} from '../premise-schedule-condition-state'

const t = (key: string) => key

describe('premise schedule condition state', () => {
  it('creates the schedule fields expected by edit echo and submit mapping', () => {
    expect(createScheduleConditionFields()).toMatchObject({
      task_type: null,
      params: null,
      execution_time: null,
      expiration_time: null,
      timeValue: null,
      onceTimeValue: null,
      hourTimeValue: null,
      dayTimeValue: null,
      weekTimeValue: null,
      monthTimeValue: null,
      weekChoseValue: [],
      monthChoseValue: null,
      startTimeValue: null,
      endTimeValue: null
    })
  })

  it('disables single and repeat time conditions when device conditions exist in the same group', () => {
    const options = buildTimeConditionOptions([{ ifType: '1' }], t)

    expect(options).toEqual([
      { label: 'common.single', value: '20', disabled: true },
      { label: 'common.repeat', value: '21', disabled: true },
      { label: 'common.timeFrame', value: '22' }
    ])
  })

  it('keeps all time conditions selectable when there is no device condition in the group', () => {
    const options = buildTimeConditionOptions([{ ifType: '2' }], t)

    expect(options).toEqual([
      { label: 'common.single', value: '20', disabled: false },
      { label: 'common.repeat', value: '21', disabled: false },
      { label: 'common.timeFrame', value: '22' }
    ])
  })

  it('builds the repeat cycle, week, expiration, and month-range option contracts', () => {
    expect(buildCycleOptions(t).map(option => option.value)).toEqual(['HOUR', 'DAY', 'WEEK', 'MONTH'])
    expect(buildWeekOptions(t).map(option => option.value)).toEqual(['1', '2', '3', '4', '5', '6', '7'])
    expect(buildExpirationTimeOptions(t).map(option => option.value)).toEqual([5, 10, 30, 60, 1440])
    expect(buildMonthRangeOptions()).toHaveLength(31)
    expect(buildMonthRangeOptions()[30]).toEqual({ label: '31', value: 31 })
  })

  it('resets repeat-only schedule fields without clearing one-time or range fields', () => {
    const ifItem = {
      hourTimeValue: '10:00',
      expiration_time: 10,
      dayTimeValue: '11:00',
      weekTimeValue: '12:00',
      monthChoseValue: 15,
      weekChoseValue: ['1'],
      monthTimeValue: '13:00',
      onceTimeValue: '2026-07-04',
      startTimeValue: '08:00',
      endTimeValue: '18:00'
    }

    resetRepeatScheduleFields(ifItem)

    expect(ifItem).toMatchObject({
      hourTimeValue: null,
      expiration_time: null,
      dayTimeValue: null,
      weekTimeValue: null,
      monthChoseValue: null,
      weekChoseValue: null,
      monthTimeValue: null,
      onceTimeValue: '2026-07-04',
      startTimeValue: '08:00',
      endTimeValue: '18:00'
    })
  })
})
