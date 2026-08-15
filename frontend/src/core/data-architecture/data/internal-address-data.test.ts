import { describe, expect, it } from 'vitest'

import {
  getAllApis,
  getApiByValue,
  getApisByModule,
  internalAddressOptions,
  searchApis
} from './internal-address-data'

describe('internal-address-data', () => {
  it('provides grouped and searchable internal API metadata', () => {
    expect(internalAddressOptions.map(group => group.key)).toEqual([
      'telemetry',
      'device',
      'attribute',
      'event',
      'alarm'
    ])
    expect(getApisByModule('telemetry').length).toBeGreaterThan(0)
    expect(getApisByModule('missing')).toEqual([])
    expect(getApiByValue('/telemetry/datas/current/{id}')).toMatchObject({
      method: 'GET',
      module: 'telemetry',
      hasPathParams: true
    })
    expect(searchApis('telemetryDataCurrent').map(api => api.value)).toContain('/telemetry/datas/current/{id}')
  })

  it('uses an ordered and bounded history time seed', () => {
    const historyApi = getApiByValue('/telemetry/datas/history/pagination')
    const startTime = historyApi?.commonParams?.find(param => param.name === 'start_time')?.example
    const endTime = historyApi?.commonParams?.find(param => param.name === 'end_time')?.example

    expect(startTime).toBe(1711656000000)
    expect(endTime).toBe(1711659600000)
    expect(startTime).toBeLessThan(endTime)
  })

  it('isolates the public option tree and every query result from the private registry', () => {
    internalAddressOptions[0].label = 'mutated group'
    internalAddressOptions[0].children[0].label = 'mutated public API'

    const byModule = getApisByModule('telemetry')
    expect(byModule[0].label).toBe('设备遥测当前值查询')
    byModule[0].label = 'mutated module result'
    byModule[0].commonParams?.push({ name: 'injected', type: 'string', required: false })

    const byValue = getApiByValue('/telemetry/datas/current/{id}')
    expect(byValue?.label).toBe('设备遥测当前值查询')
    expect(byValue?.commonParams?.some(param => param.name === 'injected')).toBe(false)
    if (byValue) byValue.label = 'mutated value result'

    const allApis = getAllApis()
    const currentApi = allApis.find(api => api.value === '/telemetry/datas/current/{id}')
    expect(currentApi?.label).toBe('设备遥测当前值查询')
    if (currentApi) currentApi.label = 'mutated all result'

    expect(searchApis('设备遥测当前值查询')[0]?.label).toBe('设备遥测当前值查询')
    expect(getApiByValue('/telemetry/datas/current/{id}')?.label).toBe('设备遥测当前值查询')
  })
})
