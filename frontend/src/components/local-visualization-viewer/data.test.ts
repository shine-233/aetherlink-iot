import { describe, expect, it } from 'vitest'
import { buildChartOption, resolveHtml, resolveMetric, resolveText } from './data'

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

  it('supports chartStyle, colorTheme, yMin/yMax, and threshold configuration', () => {
    const config = {
      categories: ['10:00', '10:01'],
      values: [15, 85],
      chartStyle: 'area' as const,
      colorTheme: '#10b981',
      yMin: 0,
      yMax: 100,
      threshold: {
        enabled: true,
        value: 80,
        label: 'High Alarm',
        color: '#f43f5e'
      }
    }
    const built = buildChartOption('line-chart', config, {})
    expect(built.available).toBe(true)
    const series = built.option.series?.[0] as any
    expect(series.areaStyle).toEqual({ opacity: 0.25 })
    expect(series.itemStyle).toEqual({ color: '#10b981' })
    expect(built.option.yAxis).toMatchObject({ min: 0, max: 100 })
    expect(series.markLine?.data).toEqual([
      {
        yAxis: 80,
        name: 'High Alarm',
        lineStyle: {
          color: '#f43f5e',
          type: 'dashed'
        }
      }
    ])
  })

  it('performs unit conversion for metric widgets (targetUnit & unitSystem)', () => {
    // 1. Metric to Imperial temperature by targetUnit
    const metricConverted = resolveMetric(
      {
        label: 'Temperature',
        field: 'temp',
        unit: '°C',
        decimals: 1,
        unitConversion: { enabled: true, targetUnit: '°F' }
      },
      { temp: 100 }
    )
    expect(metricConverted).toEqual({
      available: true,
      label: 'Temperature',
      value: '212.0',
      unit: '°F'
    })

    // 2. Unit system conversion (Metric -> Imperial canonical)
    const systemConverted = resolveMetric(
      {
        label: 'Pressure',
        field: 'pressure',
        unit: 'kPa',
        decimals: 2,
        unitConversion: { enabled: true, unitSystem: 'imperial' }
      },
      { pressure: 100 }
    )
    expect(systemConverted.unit).toBe('psi')
    expect(Number(systemConverted.value)).toBeCloseTo(14.5, 1)

    // 3. Fail-closed on dimension mismatch
    const fallback = resolveMetric(
      {
        label: 'Fail Safe',
        field: 'temp',
        unit: '°C',
        unitConversion: { enabled: true, targetUnit: 'psi' }
      },
      { temp: 25 }
    )
    expect(fallback).toEqual({
      available: true,
      label: 'Fail Safe',
      value: '25',
      unit: '°C'
    })
  })

  it('performs unit conversion for chart widgets across series, thresholds, and axes', () => {
    const chartConfig = {
      title: 'Temp Trend',
      unit: '°C',
      categories: ['00:00', '01:00'],
      values: [0, 100],
      yMin: 0,
      yMax: 100,
      threshold: { enabled: true, value: 50, label: 'High' },
      unitConversion: { enabled: true, unitSystem: 'imperial' as const }
    }
    const built = buildChartOption('line-chart', chartConfig, {})
    expect(built.available).toBe(true)
    // Values converted from [0, 100] °C to [32, 212] °F
    expect(built.option.series?.[0].data).toEqual([32, 212])
    // yAxis name and limits converted
    expect(built.option.yAxis).toMatchObject({
      name: '°F',
      min: 32,
      max: 212
    })
    // Threshold converted from 50 °C to 122 °F
    const markLineData = (built.option.series?.[0] as any).markLine?.data
    expect(markLineData?.[0].yAxis).toBe(122)
  })

  it('renders and sanitizes HTML widgets with placeholder interpolation and scoped CSS', () => {
    const config = {
      html: '<div class="card"><h3>Device: {{device_name}}</h3><p>Temp: ${temp} °C</p></div>',
      css: '.card { padding: 10px; }',
      field: 'temp'
    }
    const resolved = resolveHtml(config, { device_name: 'Motor-01', temp: 42.5 }, 'w-html-1')
    expect(resolved.available).toBe(true)
    expect(resolved.html).toContain('Device: Motor-01')
    expect(resolved.html).toContain('Temp: 42.5 °C')
    expect(resolved.scopedCss).toContain('[data-widget-id="w-html-1"] .card {')
  })

  it('sanitizes malicious payload injected via telemetry fields', () => {
    const config = {
      html: '<div class="alert">Notice: {{msg}}</div>',
      field: 'msg'
    }
    const resolved = resolveHtml(config, { msg: '<script>alert("PWNED")</script><img src=x onerror=alert(1)>' })
    expect(resolved.available).toBe(true)
    expect(resolved.html).not.toContain('script')
    expect(resolved.html).not.toContain('onerror')
    expect(resolved.html).toContain('<div class="alert">Notice: <img src="x"></div>')
  })

  it('falls back when bound primary field is missing', () => {
    const config = {
      html: '<div>Status: {{status}}</div>',
      field: 'status',
      fallback: '<div class="offline">Device is Offline</div>'
    }
    const resolved = resolveHtml(config, {})
    expect(resolved.available).toBe(false)
    expect(resolved.html).toBe('<div class="offline">Device is Offline</div>')
  })
})

