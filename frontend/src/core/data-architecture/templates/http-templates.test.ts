import { describe, expect, it } from 'vitest'

import { HTTP_CONFIG_TEMPLATES, getHttpConfigTemplates } from './http-templates'

describe('HTTP config templates', () => {
  it('provides the local request templates and expected internal paths', () => {
    expect(HTTP_CONFIG_TEMPLATES).toHaveLength(10)
    expect(HTTP_CONFIG_TEMPLATES.find(template => template.name === '设备遥测趋势')?.config.url).toBe(
      '/telemetry/datas/statistic'
    )
    expect(HTTP_CONFIG_TEMPLATES.some(template => template.config.url.startsWith('http'))).toBe(false)
  })

  it('returns isolated snapshots without changing the compatibility export', () => {
    const firstSnapshot = getHttpConfigTemplates()
    const secondSnapshot = getHttpConfigTemplates()

    firstSnapshot[0].name = 'mutated'
    firstSnapshot[0].config.headers[0].value = 'mutated'
    firstSnapshot[0].config.params.push({
      key: 'injected',
      value: 'value',
      enabled: true,
      isDynamic: false,
      dataType: 'string',
      variableName: ''
    })

    expect(secondSnapshot[0].name).toBe('设备列表查询')
    expect(secondSnapshot[0].config.headers[0].value).toBe('application/json')
    expect(secondSnapshot[0].config.params).toEqual([])
    expect(HTTP_CONFIG_TEMPLATES[0].name).toBe('设备列表查询')
  })
})
