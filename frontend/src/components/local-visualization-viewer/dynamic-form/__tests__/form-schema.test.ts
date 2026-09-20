import { describe, expect, it } from 'vitest'
import { convertFormToWidgetConfig, convertWidgetConfigToForm, validateWidgetForm } from '../form-schema'
import type { DynamicWidgetFormData } from '../types'

describe('Dynamic Form Schema & Validation', () => {
  describe('validateWidgetForm', () => {
    it('validates text widget requiring either title or field', () => {
      expect(validateWidgetForm('text', {}).valid).toBe(false)
      expect(validateWidgetForm('text', { title: 'Hello' }).valid).toBe(true)
      expect(validateWidgetForm('text', { field: 'status' }).valid).toBe(true)
    })

    it('validates metric widget requiring label and field', () => {
      expect(validateWidgetForm('metric', {}).valid).toBe(false)
      expect(validateWidgetForm('metric', { label: 'Temp' }).valid).toBe(false)
      expect(validateWidgetForm('metric', { label: 'Temp', field: 'temperature' }).valid).toBe(true)
      expect(validateWidgetForm('metric', { label: 'Temp', field: 'temperature', decimals: -1 }).valid).toBe(false)
    })

    it('validates chart min/max limits', () => {
      expect(validateWidgetForm('line-chart', { yMin: 100, yMax: 50 }).valid).toBe(false)
      expect(validateWidgetForm('line-chart', { yMin: 0, yMax: 100 }).valid).toBe(true)
    })
  })

  describe('converters', () => {
    it('converts form data to text widget config and back', () => {
      const form: DynamicWidgetFormData = {
        title: 'Server Status',
        field: 'system.status',
        fallback: 'Unknown'
      }
      const config = convertFormToWidgetConfig('text', form)
      const restored = convertWidgetConfigToForm('text', config)
      expect(restored.title).toBe('Server Status')
      expect(restored.field).toBe('system.status')
      expect(restored.fallback).toBe('Unknown')
    })

    it('converts chart widget config with thresholds and series', () => {
      const form: DynamicWidgetFormData = {
        title: 'CPU History',
        seriesName: 'CPU Usage',
        categoriesText: '10:00, 10:05, 10:10',
        valuesText: '25, 45, 80',
        chartStyle: 'area',
        threshold: { enabled: true, value: 75, label: 'Warning', color: '#ff4d4f' }
      }
      const config = convertFormToWidgetConfig('line-chart', form)
      expect(config.title).toBe('CPU History')
      expect(config.chartStyle).toBe('area')
      expect(config.threshold?.value).toBe(75)

      const restored = convertWidgetConfigToForm('line-chart', config)
      expect(restored.title).toBe('CPU History')
      expect(restored.chartStyle).toBe('area')
      expect(restored.threshold?.enabled).toBe(true)
    })

    it('converts and validates entityRelation data source correctly', () => {
      const invalidForm: DynamicWidgetFormData = {
        label: 'Dynamic Metric',
        entityRelation: {
          enabled: true,
          rootType: 'device',
          rootId: '', // missing rootId
          direction: 'from',
          relationType: '', // missing relationType
          targetType: 'device',
          targetKey: '' // missing targetKey
        }
      }
      const invalidRes = validateWidgetForm('metric', invalidForm)
      expect(invalidRes.valid).toBe(false)
      expect(invalidRes.errors.length).toBe(3)

      const validForm: DynamicWidgetFormData = {
        label: 'Substation Voltage',
        entityRelation: {
          enabled: true,
          rootType: 'device',
          rootId: 'sub-01',
          direction: 'from',
          relationType: 'Monitors',
          targetType: 'device',
          targetKey: 'voltage',
          aggregation: 'max'
        }
      }
      const validRes = validateWidgetForm('metric', validForm)
      expect(validRes.valid).toBe(true)

      const config = convertFormToWidgetConfig('metric', validForm)
      expect(config.field).toBe('__rel_device_sub-01_from_Monitors_voltage')
      expect(config.entityRelation?.enabled).toBe(true)
      expect(config.entityRelation?.aggregation).toBe('max')

      const restored = convertWidgetConfigToForm('metric', config)
      expect(restored.label).toBe('Substation Voltage')
      expect(restored.entityRelation?.enabled).toBe(true)
      expect(restored.entityRelation?.rootId).toBe('sub-01')
      expect(restored.entityRelation?.relationType).toBe('Monitors')
    })

    it('converts and validates unitConversion configuration for metric and chart widgets', () => {
      // Validation failure when enabled but neither system nor targetUnit provided
      const invalidForm: DynamicWidgetFormData = {
        label: 'Temp',
        field: 'temp',
        unitConversion: {
          enabled: true
        }
      }
      const invalidRes = validateWidgetForm('metric', invalidForm)
      expect(invalidRes.valid).toBe(false)
      expect(invalidRes.errors[0]).toContain('单位换算配置')

      // Valid metric with imperial unit system
      const validMetricForm: DynamicWidgetFormData = {
        label: 'Pressure',
        field: 'pres',
        unit: 'kPa',
        unitConversion: {
          enabled: true,
          unitSystem: 'imperial'
        }
      }
      expect(validateWidgetForm('metric', validMetricForm).valid).toBe(true)
      const metricCfg = convertFormToWidgetConfig('metric', validMetricForm)
      expect(metricCfg.unitConversion).toEqual({ enabled: true, unitSystem: 'imperial' })
      const restoredMetric = convertWidgetConfigToForm('metric', metricCfg)
      expect(restoredMetric.unitConversion).toEqual({ enabled: true, unitSystem: 'imperial' })

      // Valid chart with target unit
      const validChartForm: DynamicWidgetFormData = {
        title: 'Speed History',
        unit: 'km/h',
        categoriesText: '0, 1',
        valuesText: '60, 80',
        unitConversion: {
          enabled: true,
          targetUnit: 'mph'
        }
      }
      expect(validateWidgetForm('line-chart', validChartForm).valid).toBe(true)
      const chartCfg = convertFormToWidgetConfig('line-chart', validChartForm)
      expect(chartCfg.unit).toBe('km/h')
      expect(chartCfg.unitConversion).toEqual({ enabled: true, targetUnit: 'mph' })
      const restoredChart = convertWidgetConfigToForm('line-chart', chartCfg)
      expect(restoredChart.unit).toBe('km/h')
      expect(restoredChart.unitConversion).toEqual({ enabled: true, targetUnit: 'mph' })
    })

    it('validates and converts html widget form data', () => {
      expect(validateWidgetForm('html', {}).valid).toBe(false)
      expect(validateWidgetForm('html', { html: '<div>Valid</div>' }).valid).toBe(true)

      const form: DynamicWidgetFormData = {
        html: '<div class="banner">Status: {{state}}</div>',
        css: '.banner { font-size: 16px; }',
        field: 'state',
        fallback: 'No data'
      }
      const config = convertFormToWidgetConfig('html', form)
      expect(config).toEqual({
        html: '<div class="banner">Status: {{state}}</div>',
        css: '.banner { font-size: 16px; }',
        field: 'state',
        fallback: 'No data'
      })

      const restored = convertWidgetConfigToForm('html', config)
      expect(restored.html).toBe('<div class="banner">Status: {{state}}</div>')
      expect(restored.css).toBe('.banner { font-size: 16px; }')
      expect(restored.field).toBe('state')
      expect(restored.fallback).toBe('No data')
    })
  })
})
