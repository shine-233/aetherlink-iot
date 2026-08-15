/**
 * 文件用途：验证脚本沙箱的安全加固规则和执行器的危险代码拦截行为。
 * 核心逻辑：构造默认沙箱与执行器，覆盖计时器暴露、构造器逃逸、原型访问等攻击路径。
 * 关键注意事项：这些用例是安全回归防线，放宽断言前需要确认不会恢复宿主逃逸风险。
 * 重构建议：可按攻击类型拆分测试分组，并补充新规则的负向与正向样例。
 */
import { describe, expect, it, vi } from 'vitest'

import { ScriptExecutor } from './executor'
import { ScriptSandbox, defaultSandboxConfig } from './sandbox'

const createSandbox = () => new ScriptSandbox(defaultSandboxConfig)

describe('ScriptSandbox security hardening', () => {
  it('does not expose host timers by default', () => {
    const sandbox = createSandbox().createSandbox(defaultSandboxConfig)

    expect(defaultSandboxConfig.allowedGlobals).not.toContain('setTimeout')
    expect(defaultSandboxConfig.allowedGlobals).not.toContain('setInterval')
    expect(sandbox.setTimeout).toBeUndefined()
    expect(sandbox.setInterval).toBeUndefined()
  })

  it('blocks ambient host fetch instead of falling through the sandbox scope', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    const sandbox = createSandbox()
    const env = sandbox.createSandbox(defaultSandboxConfig)

    await expect(sandbox.executeInSandbox("return fetch('https://example.test/data')", env)).rejects.toThrow(
      'SCRIPT_NETWORK_EXTERNAL_BLOCKED'
    )
    expect(fetchSpy).not.toHaveBeenCalled()

    fetchSpy.mockRestore()
  })

  it('keeps an explicitly requested fetch global externally blocked', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    const config = {
      ...defaultSandboxConfig,
      allowedGlobals: [...defaultSandboxConfig.allowedGlobals, 'fetch']
    }
    const sandbox = new ScriptSandbox(config)
    const env = sandbox.createSandbox(config)

    await expect(env.fetch('https://example.test/data')).rejects.toThrow('SCRIPT_NETWORK_EXTERNAL_BLOCKED')
    expect(fetchSpy).not.toHaveBeenCalled()

    fetchSpy.mockRestore()
  })

  it('prevents custom globals from restoring blocked network constructors', async () => {
    const constructorNames = [
      'WebSocket',
      'XMLHttpRequest',
      'EventSource',
      'Worker',
      'SharedWorker',
      'BroadcastChannel'
    ] as const

    for (const constructorName of constructorNames) {
      const injectedConstructor = vi.fn()
      const executor = new ScriptExecutor()
      const result = await executor.execute({
        code: `return new ${constructorName}('https://example.test/socket')`,
        globals: { [constructorName]: injectedConstructor }
      })

      expect(result.success).toBe(false)
      expect(result.error?.message).toContain('SCRIPT_NETWORK_EXTERNAL_BLOCKED')
      expect(injectedConstructor).not.toHaveBeenCalled()
    }
  })

  it('prevents custom globals from restoring host storage access', async () => {
    const injectedStorage = { getItem: vi.fn().mockReturnValue('secret') }
    const executor = new ScriptExecutor()
    const result = await executor.execute({
      code: "return localStorage.getItem('token')",
      globals: { localStorage: injectedStorage }
    })

    expect(result.success).toBe(false)
    expect(result.error?.message).toContain('SCRIPT_STORAGE_EXTERNAL_BLOCKED')
    expect(injectedStorage.getItem).not.toHaveBeenCalled()
  })

  it('preserves network utility contracts while explicitly blocking external access', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch')
    const env = createSandbox().createSandbox(defaultSandboxConfig)
    const { networkUtils } = env._utils

    await expect(networkUtils.httpGet('https://example.test/data')).rejects.toThrow(
      'SCRIPT_NETWORK_EXTERNAL_BLOCKED'
    )
    await expect(networkUtils.httpPost('https://example.test/data', { value: 1 })).rejects.toThrow(
      'SCRIPT_NETWORK_EXTERNAL_BLOCKED'
    )
    await expect(networkUtils.httpPut('https://example.test/data', { value: 2 })).rejects.toThrow(
      'SCRIPT_NETWORK_EXTERNAL_BLOCKED'
    )
    await expect(networkUtils.httpDelete('https://example.test/data')).rejects.toThrow(
      'SCRIPT_NETWORK_EXTERNAL_BLOCKED'
    )
    expect(fetchSpy).not.toHaveBeenCalled()

    fetchSpy.mockRestore()
  })

  it('rejects constructor.constructor bracket escapes', async () => {
    const sandbox = createSandbox()
    const env = sandbox.createSandbox(defaultSandboxConfig)

    await expect(
      sandbox.executeInSandbox('return ({})["constructor"]["constructor"]("return globalThis")()', env)
    ).rejects.toThrow()
  })

  it('rejects prototype bracket access', () => {
    const result = createSandbox().checkCodeSecurity('Object["prototype"]["polluted"] = true')

    expect(result.safe).toBe(false)
    expect(result.issues.length).toBeGreaterThan(0)
  })

  it('does not let compatibility flags disable hard security checks', () => {
    const permissiveConfig = {
      ...defaultSandboxConfig,
      enabled: false,
      blockedGlobals: [],
      allowEval: true,
      allowFunction: true,
      allowPrototypePollution: true
    }
    const sandbox = new ScriptSandbox(permissiveConfig)

    expect(sandbox.checkCodeSecurity('return eval("1 + 1")').safe).toBe(false)
    expect(sandbox.checkCodeSecurity('return new Function("return 1")()').safe).toBe(false)
    expect(sandbox.checkCodeSecurity('Object["prototype"].polluted = true').safe).toBe(false)
  })

  it('rejects direct host global references', () => {
    const sandbox = createSandbox()

    expect(sandbox.checkCodeSecurity('return globalThis.localStorage').safe).toBe(false)
    expect(sandbox.checkCodeSecurity('return this["constructor"]').safe).toBe(false)
  })

  it('rejects obvious non-preemptible infinite loops before execution', () => {
    const sandbox = createSandbox()

    expect(sandbox.checkCodeSecurity('while (true) {}').safe).toBe(false)
    expect(sandbox.checkCodeSecurity('for (;;) {}').safe).toBe(false)
  })
})

describe('ScriptExecutor sandbox hardening', () => {
  it('blocks direct interval escape through the host global', async () => {
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval')
    const executor = new ScriptExecutor()

    const result = await executor.execute({
      code: 'setInterval(() => {}, 1); return 1',
      timeout: 50
    })

    expect(result.success).toBe(false)
    expect(setIntervalSpy).not.toHaveBeenCalled()

    setIntervalSpy.mockRestore()
  })

  it('does not allow custom globals to replace the blocked network adapter', async () => {
    const injectedFetch = vi.fn().mockResolvedValue({ ok: true })
    const executor = new ScriptExecutor()

    const result = await executor.execute({
      code: "return fetch('https://example.test/data')",
      timeout: 50,
      globals: { fetch: injectedFetch },
      allowNetworkAccess: true
    })

    expect(result.success).toBe(false)
    expect(result.error?.message).toContain('SCRIPT_NETWORK_EXTERNAL_BLOCKED')
    expect(injectedFetch).not.toHaveBeenCalled()
  })

  it('blocks async interval escape through the host global', async () => {
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval')
    const executor = new ScriptExecutor()

    const result = await executor.execute({
      code: 'await Promise.resolve().then(() => setInterval(() => {}, 1)); return 1',
      timeout: 50
    })

    expect(result.success).toBe(false)
    expect(setIntervalSpy).not.toHaveBeenCalled()

    setIntervalSpy.mockRestore()
  })

  it('fails obvious dead-loop scripts without entering execution', async () => {
    const executor = new ScriptExecutor()

    const result = await executor.execute({
      code: 'while (true) {}',
      timeout: 50
    })

    expect(result.success).toBe(false)
    expect(result.error?.message).toContain('明显无限while循环')
  })
})
