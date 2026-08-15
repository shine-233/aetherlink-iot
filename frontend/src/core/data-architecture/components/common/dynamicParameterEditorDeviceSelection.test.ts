import { describe, expect, it, vi } from 'vitest'

vi.mock('@/core/data-architecture/types/http-config', () => ({
  generateVariableName: (key: string) => `var_${key}`
}))

import {
  buildDeviceParameterFromSelection,
  buildDeviceParametersForAvailableSlots,
  getAvailableDeviceParameterSlots
} from './dynamicParameterEditorDeviceSelection'

describe('dynamicParameterEditorDeviceSelection', () => {
  it('treats an explicit zero limit as having no available slots', () => {
    expect(getAvailableDeviceParameterSlots(0, 0)).toBe(0)
    expect(buildDeviceParametersForAvailableSlots([{ key: 'temperature' }], 0, 0)).toBeNull()
  })

  it('falls back to the original value when source metadata is missing', () => {
    expect(buildDeviceParameterFromSelection({ key: 'status', value: 'online' })).toMatchObject({
      key: 'status',
      value: 'online',
      variableName: '',
      description: ''
    })
  })

  it('uses metricsId as the key when key is missing', () => {
    expect(
      buildDeviceParameterFromSelection({
        metricsId: 'temperature',
        source: { deviceName: 'Pump A', metricsName: 'Temperature' }
      })
    ).toMatchObject({
      key: 'temperature',
      value: 'Pump A.Temperature',
      variableName: 'var_temperature',
      description: '设备: Pump A, 指标: Temperature'
    })
  })

  it('truncates selected parameters to the remaining capacity', () => {
    const result = buildDeviceParametersForAvailableSlots(
      [{ key: 'temperature' }, { key: 'humidity' }, { key: 'pressure' }],
      2,
      4
    )

    expect(result?.map(parameter => parameter.key)).toEqual(['temperature', 'humidity'])
  })
})
