import { describe, expect, it, vi } from 'vitest'
import {
  decodeEventParamConditions,
  echoActionGroups,
  echoConditionGroups,
  parseActionValue
} from '../automationEchoPayload'

vi.mock('dayjs', () => {
  const dayjs = (value?: any) => ({
    format: (pattern?: string) => {
      if (pattern === 'YYYY-MM-DD') return '2026-07-05'
      if (pattern === 'YYYY-MM-DD HH') return '2026-07-05 08'
      return String(value || '2026-07-05T08:00:00')
    },
    valueOf: () => `ts:${value || 'now'}`
  })
  return { default: dayjs }
})

describe('automationEchoPayload', () => {
  it('decodes event field conditions into editable rows', () => {
    expect(
      decodeEventParamConditions(
        JSON.stringify({
          match_mode: 'field',
          conditions: [
            { field: 'code', operator: 'in', value: ['A', 'B'] },
            { field: 'temperature', operator: 'between', value: [10, 20] },
            { field: 'ok', operator: 'exists', value: true }
          ]
        })
      )
    ).toEqual([
      { field: 'code', operator: 'in', value: 'A,B', minValue: null, maxValue: null },
      { field: 'temperature', operator: 'between', value: [10, 20], minValue: 10, maxValue: 20 },
      { field: 'ok', operator: 'exists', value: true, minValue: null, maxValue: null }
    ])

    expect(decodeEventParamConditions('{bad json')).toEqual([])
    expect(decodeEventParamConditions(JSON.stringify({ match_mode: 'legacy' }))).toEqual([])
  })

  it('echoes backend condition groups into editable condition form state', () => {
    const result = echoConditionGroups([
      [
        {
          trigger_conditions_type: '10',
          trigger_param_type: 'telemetry',
          trigger_operator: 'between',
          trigger_value: '10-20',
          trigger_param: 'temperature'
        },
        {
          trigger_conditions_type: '22',
          trigger_value: '123|08:00:00+00:00|18:00:00+00:00'
        },
        {
          trigger_conditions_type: '21',
          task_type: 'WEEK',
          params: '15|08:30:00Z'
        }
      ]
    ])

    expect(result[0][0]).toEqual(
      expect.objectContaining({
        ifType: '1',
        minValue: '10',
        maxValue: '20',
        trigger_param_key: 'telemetry/temperature'
      })
    )
    expect(result[0][1]).toEqual(
      expect.objectContaining({
        ifType: '2',
        weekChoseValue: ['1', '2', '3'],
        startTimeValue: 'ts:2026-07-05 08:00:00+00:00',
        endTimeValue: 'ts:2026-07-05 18:00:00+00:00'
      })
    )
    expect(result[0][2]).toEqual(
      expect.objectContaining({
        ifType: '2',
        weekChoseValue: ['1', '5'],
        weekTimeValue: 'ts:2026-07-05 08:30:00Z'
      })
    )
  })

  it('echoes backend actions into grouped action editor state', () => {
    expect(parseActionValue('{bad json')).toEqual({})
    expect(
      echoActionGroups([
        {
          action_type: '10',
          action_param_type: 'telemetry',
          action_param: 'temperature',
          action_value: '{"temperature":25}'
        },
        {
          action_type: '10',
          action_param_type: 'command',
          action_param: 'reboot',
          action_value: '{"method":"reboot","params":{"delay":1}}'
        },
        {
          action_type: '11',
          action_param_type: 'c_command',
          action_value: '{"raw":true}'
        },
        {
          action_type: '30',
          action_target: 'alarm-1'
        }
      ])
    ).toEqual([
      {
        action_type: '30',
        action_target: 'alarm-1',
        actionType: '30'
      },
      {
        actionType: '1',
        actionInstructList: [
          {
            action_type: '10',
            action_param_type: 'telemetry',
            action_param: 'temperature',
            action_value: '{"temperature":25}',
            actionParamOptions: [],
            actionValue: 25
          },
          {
            action_type: '10',
            action_param_type: 'command',
            action_param: 'reboot',
            action_value: '{"method":"reboot","params":{"delay":1}}',
            actionParamOptions: [],
            actionValue: { delay: 1 }
          },
          {
            action_type: '11',
            action_param_type: 'c_command',
            action_value: '{"raw":true}',
            actionParamOptions: [],
            actionValue: '{"raw":true}'
          }
        ]
      }
    ])
  })
})
