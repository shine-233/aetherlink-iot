import { describe, expect, it, vi } from 'vitest'

import type { HttpParameter } from '@/core/data-architecture/types/http-config'
import { resolveHttpParameterValue } from './DataItemFetcherParameterResolution'

function parameter(overrides: Partial<HttpParameter> = {}): HttpParameter {
  return {
    key: 'limit',
    value: '10',
    enabled: true,
    isDynamic: false,
    dataType: 'number',
    variableName: 'var_limit',
    description: '',
    paramType: 'query',
    ...overrides
  }
}

describe('DataItemFetcherParameterResolution', () => {
  it('keeps a manual parameter static when its generated variable name contains an underscore', async () => {
    const readComponentProperty = vi.fn()

    await expect(resolveHttpParameterValue(parameter(), readComponentProperty)).resolves.toBe(10)
    expect(readComponentProperty).not.toHaveBeenCalled()
  })

  it('resolves an explicit component binding and converts the resolved value', async () => {
    const readComponentProperty = vi.fn().mockResolvedValue('26')
    const param = parameter({
      value: 'card-1.base.temperature',
      valueMode: 'component',
      selectedTemplate: 'component-property-binding'
    })

    await expect(resolveHttpParameterValue(param, readComponentProperty)).resolves.toBe(26)
    expect(readComponentProperty).toHaveBeenCalledWith('card-1.base.temperature')
  })

  it('recovers a damaged legacy binding only when its value and variable name form a recoverable pair', async () => {
    const readComponentProperty = vi.fn().mockResolvedValue('42')
    const param = parameter({
      value: '123',
      variableName: 'cardA_limit'
    })

    await expect(resolveHttpParameterValue(param, readComponentProperty)).resolves.toBe(42)
    expect(readComponentProperty).toHaveBeenCalledWith('cardA.base.limit')
  })
})
