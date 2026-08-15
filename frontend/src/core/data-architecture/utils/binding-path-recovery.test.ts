/**
 * 文件用途: binding path recovery 的测试文件。
 * 核心逻辑: 构造局部 fixture、mock 依赖并断言公开契约和关键边界行为。
 * 关键注意事项: 测试应覆盖可观察行为，避免绑定无关实现细节导致重构成本过高。
 * 重构建议: 补充历史配置、异常输入和回归场景，提升重构保护力度。
 */

import { describe, expect, it } from 'vitest'

import {
  isDamagedComponentBindingPath,
  isValidComponentBindingPath,
  recoverComponentBindingPathFromVariableName,
  resolveRecoverableComponentBindingPath
} from './binding-path-recovery'

describe('binding-path-recovery', () => {
  it('recovers component binding paths from persisted variable names', () => {
    expect(recoverComponentBindingPathFromVariableName('chartA_deviceId')).toBe('chartA.base.deviceId')
    expect(recoverComponentBindingPathFromVariableName('target-card_styles.color')).toBe('target-card.base.styles.color')
    expect(recoverComponentBindingPathFromVariableName('missingSeparator')).toBeNull()
  })

  it('classifies damaged short binding-path fragments without accepting arbitrary invalid paths', () => {
    expect(isDamagedComponentBindingPath('123', 'chartA_deviceId')).toBe(true)
    expect(isDamagedComponentBindingPath('missing-dot-but-long', 'chartA_deviceId')).toBe(false)
    expect(isValidComponentBindingPath('chartA.base.deviceId')).toBe(true)
    expect(isValidComponentBindingPath('123')).toBe(false)
  })

  it('uses strict editor validation while preserving empty binding clears', () => {
    expect(isValidComponentBindingPath('', { allowEmpty: true, strict: true })).toBe(true)
    expect(isValidComponentBindingPath('card.base.undefined', { strict: true })).toBe(false)

    expect(resolveRecoverableComponentBindingPath('bad', 'targetCard_color', { strict: true })).toEqual({
      bindingPath: 'targetCard.base.color',
      isValid: true,
      recovered: true,
      wasDamaged: true
    })
  })
})
