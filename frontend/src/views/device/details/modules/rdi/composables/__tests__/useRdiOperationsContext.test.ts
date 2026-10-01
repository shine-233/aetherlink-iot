/**
 * 文件用途: useRdiOperationsContext 纯派生函数与 inject 守卫的单元测试。
 * 核心逻辑: 覆盖下拉选项构造、干接点延时折叠判断与统一写入，以及脱离 provider 调用时的错误提示。
 */
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import {
  buildLevelOptions,
  buildSwitchModeOptions,
  buildTemperatureUnitOptions,
  hasDistinctDryContactDelays,
  useRdiOperationsContext,
  writeUnifiedDryContactDelay
} from '../useRdiOperationsContext'

const t = (key: string) => `#${key}`

describe('useRdiOperationsContext helpers', () => {
  it('builds localized select options with stable backend values', () => {
    expect(buildSwitchModeOptions(t as any).map((item) => item.value)).toEqual([
      'powered_on',
      'powered_off',
      'disabled'
    ])
    expect(buildTemperatureUnitOptions(t as any)).toEqual([
      { label: '#celsius (C)', value: 'C' },
      { label: '#fahrenheit (F)', value: 'F' }
    ])
    expect(buildLevelOptions(t as any)).toEqual([
      { label: '#high', value: 'high' },
      { label: '#low', value: 'low' }
    ])
  })

  it('detects distinct dry contact delays and writes unified values to both fields', () => {
    const config = { dry_contact_alarm_delay: 60, dry_contact_normal_delay: 60 }
    expect(hasDistinctDryContactDelays(config)).toBe(false)

    config.dry_contact_normal_delay = 120
    expect(hasDistinctDryContactDelays(config)).toBe(true)

    writeUnifiedDryContactDelay(config, 300)
    expect(config).toEqual({ dry_contact_alarm_delay: 300, dry_contact_normal_delay: 300 })
    expect(hasDistinctDryContactDelays(config)).toBe(false)
  })

  it('throws a descriptive error when a section is mounted without the provider', () => {
    const Orphan = defineComponent({
      setup() {
        useRdiOperationsContext()
        return () => h('div')
      }
    })
    expect(() => mount(Orphan)).toThrow(/RdiDeviceOperationsView/)
  })
})
