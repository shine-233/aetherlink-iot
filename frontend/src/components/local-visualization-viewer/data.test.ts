import { describe, expect, it } from 'vitest'
import { buildChartOption, resolveMetric, resolveText } from './data'

describe('local viewer data binding', () => {
  it('renders static text and safely interpolates an AetherLink field', () => {
    expect(resolveText({ text: 'Static' }, {})).toEqual({ available: true, text: 'Static' })
    expect(resolveText({ text: 'Status: {{value}}', field: 'device.status' }, { 'device.status': 'online' })).toEqual({
      available: true,
      text: 'Status: online'
    })
    expect(resolveText({ text: '{{value}}', field: 'missing' }, {})).toEqual({ available: false, text: 'Unavailable' })
  })

  it('formats metric values and fails closed for missing or array fields', () => {
    expect(resolveMetric({ label: 'Temperature', field: 'temperature', unit: ' °C', decimals: 1 }, { temperature: 21.26 })).toEqual({
      available: true,
      label: 'Temperature',
      value: '21.3',
      unit: ' °C'
    })
    expect(resolveMetric({ label: 'Temperature', field: 'temperature' }, { temperature: [21] })).toMatchObject({
      available: false,
      value: 'Unavailable'
    })
  })

  it.each([
    ['line-chart' as const, 'line'],
    ['bar-chart' as const, 'bar']
  ])('builds only a controlled %s ECharts option', (type, seriesType) => {
    const built = buildChartOption(type, { title: 'Trend', categories: ['A', 'B'], values: [1, 2] }, {})
    expect(built.available).toBe(true)
    expect(built.option).toMatchObject({
      title: { text: 'Trend' },
      xAxis: { type: 'category', data: ['A', 'B'] },
      yAxis: { type: 'value' },
      series: [{ type: seriesType, data: [1, 2] }]
    })
    expect(JSON.stringify(built.option)).not.toMatch(/formatter|url|script/i)
  })

  it('maps chart categories and values from local fields and rejects malformed input', () => {
    const config = { categoryField: 'telemetry.labels', valueField: 'telemetry.values' }
    expect(
      buildChartOption('line-chart', config, {
        'telemetry.labels': ['10:00', '10:01'],
        'telemetry.values': [4, 5]
      })
    ).toMatchObject({ available: true, option: { series: [{ data: [4, 5] }] } })
    expect(buildChartOption('line-chart', config, { 'telemetry.labels': ['10:00'], 'telemetry.values': [4, 5] }).available).toBe(false)
  })
})
