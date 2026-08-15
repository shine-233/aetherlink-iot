import { describe, expect, it } from 'vitest'

import {
  addEventParamCondition,
  buildEventExistsOptions,
  deleteEventParamCondition,
  eventConditionOperatorChange,
  getEventOperatorOptions,
  resolveSelectedEventParams,
  syncSelectedEventParams
} from '../premise-event-param-conditions'

describe('premise-event-param-conditions', () => {
  const t = (key: string) =>
    ({
      'custom.automation.eventParam.exists': 'Exists',
      'custom.automation.eventParam.notExists': 'Does not exist',
      'custom.automation.eventParam.existsOperator': 'Exists check'
    })[key] || key

  const determineOptions = [
    { label: '=', value: '=' },
    { label: '!=', value: '!=' },
    { label: '>', value: '>' },
    { label: '<', value: '<' },
    { label: 'between', value: 'between' },
    { label: 'in', value: 'in' }
  ]

  it('builds event exists select options without relying on component state', () => {
    expect(buildEventExistsOptions(t)).toEqual([
      { label: 'Exists', value: true },
      { label: 'Does not exist', value: false }
    ])
  })

  it('resolves selected event params from trigger options', () => {
    expect(
      resolveSelectedEventParams(
        [
          {
            value: 'event',
            options: [{ key: 'alarm', params: [{ data_identifier: 'level' }] }]
          }
        ],
        'alarm'
      )
    ).toEqual([{ data_identifier: 'level' }])
  })

  it('syncs selected event params into event UI state', () => {
    const ifItem: Record<string, any> = {
      trigger_param_type: 'event',
      trigger_param: 'alarm',
      triggerParamOptions: [
        {
          value: 'event',
          options: [{ key: 'alarm', params: [{ data_identifier: 'level', data_type: 'Number' }] }]
        }
      ],
      eventParamConditions: [{ field: 'level', operator: '>', value: 80, minValue: null, maxValue: null }]
    }

    syncSelectedEventParams(ifItem)

    expect(ifItem.eventParamsRaw).toEqual([{ data_identifier: 'level', data_type: 'Number' }])
    expect(ifItem.eventParamOptions).toEqual([{ label: 'level', value: 'level', dataType: 'Number' }])
    expect(ifItem.eventParamConditions).toHaveLength(1)
  })

  it('limits event operators by event param data type', () => {
    expect(
      getEventOperatorOptions(
        {
          eventParamOptions: [{ value: 'flag', dataType: 'Boolean' }]
        },
        { field: 'flag' },
        determineOptions,
        t
      ).map((item) => item.value)
    ).toEqual(['=', '!=', 'exists'])

    expect(
      getEventOperatorOptions(
        {
          eventParamOptions: [{ value: 'level', dataType: 'Number' }]
        },
        { field: 'level' },
        determineOptions,
        t
      ).map((item) => item.value)
    ).toEqual(['=', '!=', '>', '<', 'between', 'in', 'exists'])
  })

  it('adds, deletes, and resets event conditions through helper entry points', () => {
    const ifItem: Record<string, any> = {}

    addEventParamCondition(ifItem)
    expect(ifItem.eventParamConditions).toEqual([
      {
        field: null,
        operator: '=',
        value: null,
        minValue: null,
        maxValue: null
      }
    ])

    const condition = { operator: 'exists', value: 'old', minValue: 1, maxValue: 2 }
    eventConditionOperatorChange(condition)
    expect(condition).toEqual({ operator: 'exists', value: true, minValue: null, maxValue: null })

    deleteEventParamCondition(ifItem, 0)
    expect(ifItem.eventParamConditions).toEqual([])
  })
})
