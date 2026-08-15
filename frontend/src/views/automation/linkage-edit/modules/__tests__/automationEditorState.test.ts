/**
 * 文件用途: 覆盖联动编辑页的默认表单、表单规则和路由上下文读取。
 * 核心逻辑: 验证页面初始化规则不依赖组件实例，避免散落在 SFC 中重复维护。
 * 关键注意事项: 这里只证明编辑器初始化状态，不代表保存、dry-run 或真实自动化执行。
 * 重构建议: 后续若 route query 扩展，先补这里的纯函数用例再改页面。
 */
import { describe, expect, it } from 'vitest'
import {
  createAutomationFormRules,
  createDefaultAutomationForm,
  readAutomationRouteContext
} from '../automationEditorState'

const t = (key: string) => key

describe('automationEditorState', () => {
  it('creates the default scene automation form state', () => {
    expect(createDefaultAutomationForm()).toEqual({
      id: '',
      name: null,
      description: null,
      enabled: 'Y',
      trigger_condition_groups: [],
      actions: []
    })
  })

  it('creates localized form rules for required editor fields', () => {
    expect(createAutomationFormRules(t)).toMatchObject({
      name: {
        required: true,
        message: 'generate.enter-scene-linkage-name',
        trigger: 'blur'
      },
      description: {
        required: false,
        message: 'generate.sceneLinkDesc',
        trigger: 'blur'
      },
      trigger_condition_groups: {
        required: true,
        message: 'generate.addExecutionConditions'
      },
      actions: {
        required: true,
        message: 'generate.addExecutionAction'
      }
    })
  })

  it('reads route query values used by edit mode and post-save navigation', () => {
    expect(
      readAutomationRouteContext({
        id: 'scene-1',
        backType: 'device',
        device_id: 'device-1',
        device_config_id: 'config-1',
        onboarding: 'first-device',
        starter: 'first-telemetry-rule',
        first_device_name: 'Pump',
        first_device_number: 'P-001',
        telemetry_key: 'temperature',
        telemetry_value: '36.5',
        telemetry_at: '2026-07-06T12:00:00.000Z'
      })
    ).toEqual({
      configId: 'scene-1',
      backType: 'device',
      onboarding: 'first-device',
      propsData: {
        device_id: 'device-1',
        device_config_id: 'config-1'
      },
      starter: {
        type: 'first-telemetry-rule',
        deviceName: 'Pump',
        deviceNumber: 'P-001',
        telemetryKey: 'temperature',
        telemetryValue: '36.5',
        telemetryAt: '2026-07-06T12:00:00.000Z'
      }
    })

    expect(readAutomationRouteContext({})).toEqual({
      configId: '',
      backType: '',
      onboarding: '',
      propsData: {
        device_id: '',
        device_config_id: ''
      },
      starter: {
        type: '',
        deviceName: '',
        deviceNumber: '',
        telemetryKey: '',
        telemetryValue: '',
        telemetryAt: ''
      }
    })
  })

  it('normalizes multi-value route query fields to the first value', () => {
    expect(
      readAutomationRouteContext({
        id: ['scene-1', 'scene-2'],
        backType: ['config'],
        device_config_id: ['config-1'],
        starter: ['first-telemetry-rule', 'other-starter'],
        first_device_name: ['Pump', 'Fallback Pump'],
        first_device_number: ['P-001', 'P-002'],
        telemetry_key: ['temperature', 'humidity'],
        telemetry_value: ['36.5', '80'],
        telemetry_at: ['2026-07-06T12:00:00.000Z', '2026-07-06T13:00:00.000Z']
      })
    ).toEqual({
      configId: 'scene-1',
      backType: 'config',
      onboarding: '',
      propsData: {
        device_id: '',
        device_config_id: 'config-1'
      },
      starter: {
        type: 'first-telemetry-rule',
        deviceName: 'Pump',
        deviceNumber: 'P-001',
        telemetryKey: 'temperature',
        telemetryValue: '36.5',
        telemetryAt: '2026-07-06T12:00:00.000Z'
      }
    })
  })
})
