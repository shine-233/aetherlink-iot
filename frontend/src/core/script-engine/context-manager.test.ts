import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ScriptContextManager } from './context-manager'

vi.mock('nanoid', () => ({
  nanoid: vi.fn()
}))

import { nanoid } from 'nanoid'

describe('ScriptContextManager', () => {
  beforeEach(() => {
    vi.mocked(nanoid).mockReset()
    vi.mocked(nanoid).mockReturnValueOnce('context-1').mockReturnValueOnce('context-2')
  })

  it('starts empty and creates only explicitly requested contexts', () => {
    const manager = new ScriptContextManager()
    expect(manager.getAllContexts()).toEqual([])
    expect(manager.getContextByName('默认上下文')).toBeNull()

    const context = manager.createContext('request-script', { deviceId: 'device-1' })
    expect(context).toMatchObject({
      id: 'context-1',
      name: 'request-script',
      variables: { deviceId: 'device-1' }
    })
    expect(manager.getContext('context-1')).toBe(context)
  })

  it('returns honest failure values for missing contexts', () => {
    const manager = new ScriptContextManager()

    expect(manager.updateContext('missing', { name: 'updated' })).toBe(false)
    expect(manager.deleteContext('missing')).toBe(false)
    expect(manager.cloneContext('missing', 'copy')).toBeNull()
    expect(manager.mergeContexts('missing-source', 'missing-target')).toBe(false)
    expect(manager.addVariable('missing', 'value', 1)).toBe(false)
    expect(manager.removeVariable('missing', 'value')).toBe(false)
  })

  it('deep-clones variables and keeps function references when cloning a context', () => {
    const manager = new ScriptContextManager()
    const source = manager.createContext('source', {
      nested: { values: [1, 2] }
    })
    const customFunction = () => 'ok'
    manager.addFunction(source.id, 'customFunction', customFunction)

    const clone = manager.cloneContext(source.id, 'clone')
    expect(clone).toMatchObject({
      id: 'context-2',
      name: 'clone',
      variables: { nested: { values: [1, 2] } }
    })
    expect(clone?.functions.customFunction).toBe(customFunction)

    ;(clone?.variables.nested.values as number[]).push(3)
    expect(source.variables.nested.values).toEqual([1, 2])
  })
})
