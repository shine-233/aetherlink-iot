import { describe, expect, it } from 'vitest'

import { defaultSandboxConfig } from './sandbox'
import { ScriptEngine } from './script-engine'

describe('ScriptEngine sandbox configuration', () => {
  it('uses the same custom security policy for checks and execution', async () => {
    const engine = new ScriptEngine({
      sandboxConfig: {
        ...defaultSandboxConfig,
        customSecurityPolicy: code => !code.includes('blockedCall')
      }
    })
    const blockedCode = 'return blockedCall()'

    expect(engine.checkScriptSecurity(blockedCode)).toEqual({
      safe: false,
      issues: ['自定义安全策略检查失败']
    })

    const result = await engine.execute(blockedCode)
    expect(result.success).toBe(false)
    expect(result.error?.message).toContain('代码安全检查失败: 自定义安全策略检查失败')
    expect(result.error?.message).not.toContain('blockedCall is not defined')
  })

  it('still executes code accepted by the custom security policy', async () => {
    const engine = new ScriptEngine({
      sandboxConfig: {
        ...defaultSandboxConfig,
        customSecurityPolicy: code => !code.includes('blockedCall')
      }
    })

    const result = await engine.execute<number>('return 42')

    expect(result.success).toBe(true)
    expect(result.data).toBe(42)
  })

  it('applies an updated sandbox policy to checks and subsequent executions', async () => {
    const engine = new ScriptEngine()
    const code = 'return blockedAfterUpdate()'

    expect(engine.checkScriptSecurity(code).safe).toBe(true)

    engine.updateConfig({
      sandboxConfig: {
        ...defaultSandboxConfig,
        customSecurityPolicy: candidate => !candidate.includes('blockedAfterUpdate')
      }
    })

    expect(engine.checkScriptSecurity(code)).toEqual({
      safe: false,
      issues: ['自定义安全策略检查失败']
    })
    const result = await engine.execute(code)
    expect(result.success).toBe(false)
    expect(result.error?.message).toContain('代码安全检查失败: 自定义安全策略检查失败')
  })
})
