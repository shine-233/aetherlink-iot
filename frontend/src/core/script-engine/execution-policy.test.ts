/**
 * 文件用途：验证脚本执行来源信任策略与引擎入口强制。
 * 核心逻辑：默认拒绝导入配置来源脚本；显式放行后放行；编辑器脚本始终允许。
 * 关键注意事项：该策略是"导入看板→任意脚本执行"攻击链的门禁，禁止默认放开。
 * 重构建议：随 trustOrigin 落地补齐按看板粒度的授权用例。
 */
import { afterEach, describe, expect, it } from 'vitest'
import {
  SCRIPT_IMPORTED_CONFIG_BLOCKED,
  importedConfigScriptsAllowed,
  setImportedConfigScriptsAllowed
} from './execution-policy'
import { ScriptEngine } from './script-engine'

afterEach(() => {
  setImportedConfigScriptsAllowed(false)
})

describe('script execution policy', () => {
  it('denies imported-config scripts by default', () => {
    expect(importedConfigScriptsAllowed()).toBe(false)
    expect(() => setImportedConfigScriptsAllowed(false)).not.toThrow()
  })

  it('keeps editor scripts executable while imported scripts are blocked', async () => {
    const engine = new ScriptEngine({ enablePerformanceMonitoring: false })
    await expect(engine.execute('return 1 + 1')).resolves.toMatchObject({ success: true })
    await expect(engine.execute('return 40 + 2', undefined, 'editor')).resolves.toMatchObject({
      success: true
    })
    await expect(engine.execute('return true', undefined, 'imported-config')).rejects.toThrow(
      SCRIPT_IMPORTED_CONFIG_BLOCKED
    )
  })

  it('allows imported-config scripts only after explicit opt-in', async () => {
    const engine = new ScriptEngine({ enablePerformanceMonitoring: false })
    setImportedConfigScriptsAllowed(true)
    await expect(engine.execute('return "ok"', undefined, 'imported-config')).resolves.toMatchObject({
      success: true
    })
    setImportedConfigScriptsAllowed(false)
    await expect(engine.execute('return "ok"', undefined, 'imported-config')).rejects.toThrow(
      SCRIPT_IMPORTED_CONFIG_BLOCKED
    )
  })
})
