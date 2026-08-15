import { describe, expect, it, vi } from 'vitest'
import {
  buildEventParamConditionValue,
  buildSubmitActions,
  buildSubmitConditionGroups,
  hasEmptyEventParamMatchCondition,
  hasOnlyTimeRangeConditionGroup,
  hasScheduleConditionWithAlarmAction
} from '../automationSubmitPayload'

vi.mock('dayjs', () => {
  const dayjs = (value?: any) => ({
    format: (pattern?: string) => {
      if (pattern === 'HH:mm:ssZ') return value === 'end' ? '18:00:00+00:00' : '08:00:00+00:00'
      if (pattern === 'HH:mm:00Z') return '08:30:00Z'
      if (pattern === 'mm:00Z') return '30:00Z'
      return '2026-07-05T08:00:00+00:00'
    }
  })
  return { default: dayjs }
})

describe('automationSubmitPayload', () => {
  it('builds event-field match values without mutating the original editor condition group', () => {
    const source = [
      [
        {
          trigger_conditions_type: '10',
          trigger_param_type: 'event',
          eventParamConditions: [
            { field: 'code', operator: 'in', value: 'A, B,,C' },
            { field: 'ok', operator: 'exists', value: false }
          ]
        }
      ]
    ]

    const result = buildSubmitConditionGroups(source)
    expect(result[0][0].trigger_operator).toBe('=')
    expect(JSON.parse(result[0][0].trigger_value)).toEqual({
      match_mode: 'field',
      conditions: [
        { field: 'code', operator: 'in', value: ['A', 'B', 'C'] },
        { field: 'ok', operator: 'exists', value: false }
      ]
    })
    expect(source[0][0]).not.toHaveProperty('trigger_value')
  })

  it('keeps invalid event-field rows visible to submit blockers instead of silently dropping them', () => {
    const result = buildSubmitConditionGroups([
      [
        {
          trigger_conditions_type: '10',
          trigger_param_type: 'event',
          eventParamConditions: [{ field: ' ', operator: '=', value: 'ignored' }]
        }
      ]
    ])

    expect(JSON.parse(result[0][0].trigger_value)).toEqual({
      match_mode: 'field',
      conditions: [{ field: '', operator: '=', value: 'ignored' }]
    })
    expect(hasEmptyEventParamMatchCondition(result)).toBe(true)
  })

  it('normalizes between and schedule conditions for submit payloads', () => {
    expect(
      buildEventParamConditionValue({
        field: 'temperature',
        operator: 'between',
        minValue: 10,
        maxValue: 20
      })
    ).toEqual({ field: 'temperature', operator: 'between', value: [10, 20] })

    const result = buildSubmitConditionGroups([
      [
        {
          trigger_conditions_type: '10',
          trigger_param_type: 'telemetry',
          trigger_operator: 'between',
          minValue: 10,
          maxValue: 20
        },
        {
          trigger_conditions_type: '22',
          weekChoseValue: ['1', '2'],
          startTimeValue: 'start',
          endTimeValue: 'end'
        }
      ]
    ])

    expect(result[0][0].trigger_value).toBe('10-20')
    expect(result[0][1].trigger_value).toBe('12|08:00:00+00:00|18:00:00+00:00')
  })

  it('flattens grouped actions while preserving backend action_value contracts', () => {
    const result = buildSubmitActions([
      {
        actionType: '1',
        actionInstructList: [
          {
            action_type: '10',
            action_param_type: 'telemetry',
            action_param: 'temperature',
            actionValue: 25
          },
          {
            action_type: '10',
            action_param_type: 'command',
            action_param: 'reboot',
            actionValue: { delay: 1 }
          },
          {
            action_type: '10',
            action_param_type: 'c_command',
            actionValue: '{"raw":true}'
          }
        ]
      },
      {
        actionType: '30',
        action_target: 'alarm-1'
      }
    ])

    expect(result).toEqual([
      {
        action_type: '10',
        action_param_type: 'telemetry',
        action_param: 'temperature',
        actionValue: 25,
        action_value: '{"temperature":25}'
      },
      {
        action_type: '10',
        action_param_type: 'command',
        action_param: 'reboot',
        actionValue: { delay: 1 },
        action_value: '{"method":"reboot","params":{"delay":1}}'
      },
      {
        action_type: '10',
        action_param_type: 'c_command',
        actionValue: '{"raw":true}',
        action_value: '{"raw":true}'
      },
      {
        actionType: '30',
        action_target: 'alarm-1',
        action_type: '30'
      }
    ])
  })

  it('keeps submit blockers readable and reusable', () => {
    expect(hasOnlyTimeRangeConditionGroup([[{ trigger_conditions_type: '22' }]])).toBe(true)
    expect(
      hasOnlyTimeRangeConditionGroup([[{ trigger_conditions_type: '22' }, { trigger_conditions_type: '10' }]])
    ).toBe(false)
    expect(
      hasScheduleConditionWithAlarmAction([[{ ifType: '2' }]], [{ actionType: '30' }])
    ).toBe(true)
    expect(
      hasScheduleConditionWithAlarmAction([[{ ifType: '1' }]], [{ actionType: '30' }])
    ).toBe(false)
  })
})
