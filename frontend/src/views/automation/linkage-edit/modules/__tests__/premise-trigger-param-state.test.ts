import { describe, expect, it } from 'vitest'

import {
  applyTriggerParamSelectionState,
  commitSelectedTriggerParam,
  createEventParamCondition,
  createEventParamUiState,
  createSelectedTriggerParamState,
  createTriggerParamKey,
  ensureTriggerParamOptionsState,
  isTriggerParamPathSelected,
  normalizeIfItemForEcho,
  parseEventParamOptions,
  validateEventTriggerJsonValue
} from '../premise-trigger-param-state'

describe('premise-trigger-param-state', () => {
  it('creates trigger param keys only when both type and key exist', () => {
    expect(createTriggerParamKey('telemetry', 'temp')).toBe('telemetry/temp')
    expect(createTriggerParamKey('telemetry', null)).toBeNull()
  })

  it('parses event param options from string payloads', () => {
    expect(
      parseEventParamOptions('[{"data_identifier":"switch","data_name":"Switch","data_type":"Bool"}]')
    ).toEqual([
      {
        label: 'switch(Switch)',
        value: 'switch',
        dataType: 'Bool'
      }
    ])
  })

  it('creates empty event state for non-event trigger params', () => {
    expect(createEventParamUiState('telemetry', null)).toEqual({
      eventParamsRaw: null,
      eventParamOptions: [],
      eventParamConditions: []
    })
  })

  it('applies trigger param selection state and resets comparator fields when requested', () => {
    const ifItem = {
      trigger_operator: '=',
      trigger_value: '1',
      minValue: '2',
      maxValue: '3'
    }

    applyTriggerParamSelectionState(
      ifItem,
      {
        triggerParamType: 'event',
        triggerParam: 'alarm',
        params: '[{"data_identifier":"switch"}]',
        eventParamConditions: [createEventParamCondition()]
      },
      { resetComparatorState: true }
    )

    expect(ifItem.trigger_param_type).toBe('event')
    expect(ifItem.trigger_param).toBe('alarm')
    expect(ifItem.trigger_param_key).toBe('event/alarm')
    expect(ifItem.trigger_operator).toBeNull()
    expect(ifItem.eventParamOptions).toHaveLength(1)
    expect(ifItem.eventParamConditions).toHaveLength(1)
  })

  it('creates selected trigger param state from cascader path data', () => {
    expect(
      createSelectedTriggerParamState([{ value: 'event' }, { key: 'alarm', params: [{ data_identifier: 'switch' }] }])
    ).toEqual({
      triggerParamType: 'event',
      triggerParam: 'alarm',
      params: [{ data_identifier: 'switch' }],
      eventParamConditions: [createEventParamCondition()]
    })
  })

  it('commits selected trigger param state through a single entry point', () => {
    const ifItem = {
      trigger_operator: '=',
      trigger_value: '1',
      minValue: '2',
      maxValue: '3'
    }

    commitSelectedTriggerParam(ifItem, {
      triggerParamType: 'telemetry',
      triggerParam: 'temp'
    })

    expect(ifItem.trigger_param_type).toBe('telemetry')
    expect(ifItem.trigger_param).toBe('temp')
    expect(ifItem.trigger_param_key).toBe('telemetry/temp')
    expect(ifItem.trigger_operator).toBeNull()
  })

  it('normalizes echoed ifItems with missing triggerParamOptions and event fields', () => {
    const ifItem = {
      trigger_conditions_type: '10',
      trigger_param_type: 'event',
      trigger_param: 'alarm',
      eventParamsRaw: '[{"data_identifier":"switch"}]'
    }

    normalizeIfItemForEcho(ifItem)

    expect(ifItem.triggerParamOptions).toEqual([])
    expect(ifItem.trigger_param_key).toBe('event/alarm')
    expect(ifItem.eventParamOptions).toEqual([
      {
        label: 'switch',
        value: 'switch',
        dataType: 'String'
      }
    ])
    expect(ifItem.eventParamConditions).toEqual([])
  })

  it('ensures triggerParamOptions is always an array', () => {
    const ifItem: Record<string, any> = {}
    ensureTriggerParamOptionsState(ifItem)
    expect(ifItem.triggerParamOptions).toEqual([])
  })

  it('distinguishes selected and cleared trigger-param cascader paths', () => {
    expect(isTriggerParamPathSelected([{ value: 'telemetry' }, { key: 'temp' }])).toBe(true)
    expect(isTriggerParamPathSelected([])).toBe(false)
  })

  it('validates only object-like event JSON payloads', () => {
    expect(validateEventTriggerJsonValue('{"switch":1}')).toBe(true)
    expect(validateEventTriggerJsonValue('123')).toBe(false)
    expect(validateEventTriggerJsonValue('oops')).toBe(false)
  })
})
