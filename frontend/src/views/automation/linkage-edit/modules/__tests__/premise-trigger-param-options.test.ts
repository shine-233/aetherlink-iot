import { describe, expect, it } from 'vitest'

import {
  buildTriggerParamOptions,
  canLoadTriggerParamOptions,
  clearTriggerParamOptions,
  formatTriggerParamOptions,
  hasTriggerParamOptions,
  loadTriggerParamOptionsForIfItem,
  setTriggerParamOptions
} from '../premise-trigger-param-options'

describe('premise-trigger-param-options', () => {
  const statusOption = {
    value: 'status',
    label: 'status',
    options: [{ value: 'status/On-line', key: 'On-line', label: 'On-line' }]
  }

  it('formats trigger param groups into cascader-compatible options', () => {
    expect(
      formatTriggerParamOptions([
        {
          data_source_type: 'telemetry',
          label: 'Telemetry',
          options: [{ key: 'temp', label: 'Temperature' }]
        }
      ])
    ).toEqual([
      {
        data_source_type: 'telemetry',
        value: 'telemetry',
        label: 'telemetry(Telemetry)',
        options: [
          {
            key: 'temp',
            label: 'temp(Temperature)',
            value: 'telemetry/temp'
          }
        ]
      }
    ])
  })

  it('always appends the status option when it is missing', () => {
    const result = buildTriggerParamOptions(
      [{ data_source_type: 'telemetry', options: [{ key: 'temp' }] }],
      statusOption
    )

    expect(result.map(item => item.value)).toEqual(['telemetry', 'status'])
  })

  it('recognizes when trigger-param options should load and when they are already present', () => {
    const ifItem = {
      trigger_source: 'device-1',
      trigger_conditions_type: '10',
      triggerParamOptions: []
    }

    expect(canLoadTriggerParamOptions(ifItem)).toBe(true)
    expect(hasTriggerParamOptions(ifItem)).toBe(false)

    setTriggerParamOptions(ifItem, [{ data_source_type: 'telemetry', options: [{ key: 'temp' }] }], statusOption)

    expect(hasTriggerParamOptions(ifItem)).toBe(true)
  })

  it('clears trigger-param options through a single helper', () => {
    const ifItem = { triggerParamOptions: [{ value: 'telemetry' }] }
    clearTriggerParamOptions(ifItem)
    expect(ifItem.triggerParamOptions).toEqual([])
  })

  it('loads trigger-param options through one orchestration helper and always syncs once', async () => {
    const ifItem: Record<string, any> = {
      trigger_source: 'device-1',
      trigger_conditions_type: '10',
      triggerParamOptions: []
    }
    let syncCount = 0

    await loadTriggerParamOptionsForIfItem(ifItem, {
      deviceMetricsConditionMenu: async () => ({
        data: [{ data_source_type: 'telemetry', options: [{ key: 'temp', label: 'Temperature' }] }]
      }),
      configMetricsConditionMenu: async () => null,
      statusOption,
      syncSelectedEventParams: () => {
        syncCount += 1
      }
    })

    expect(ifItem.triggerParamOptions.map((item: any) => item.value)).toEqual(['telemetry', 'status'])
    expect(syncCount).toBe(1)
  })
})
