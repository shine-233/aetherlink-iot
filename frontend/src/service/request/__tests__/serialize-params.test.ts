import { createRequire } from 'node:module'
import path from 'node:path'
import process from 'node:process'
import { describe, expect, it } from 'vitest'
import { serializeParams } from '../../../../packages/axios/src/serialize-params'

// qs 仍是 @aetherlink/axios 的已声明依赖，这里只作为行为基准（oracle）使用。
const requireFromAxiosPkg = createRequire(path.resolve(process.cwd(), 'packages/axios/package.json'))
const qs = requireFromAxiosPkg('qs') as { stringify: (value: unknown) => string }

const cases: Array<[string, unknown]> = [
  ['flat paging', { page: 1, page_size: 20, name: 'sensor 01' }],
  ['arrays use indices', { ids: ['a', 'b', 'c'], status: [1, 2] }],
  ['nested objects and arrays', { b: { c: 'x y', d: [{ e: 1 }, { f: [true, false] }] } }],
  ['null becomes empty, undefined and functions skipped', { n: null, u: undefined, fn() {}, keep: 'v' }],
  ['sparse/null array items', { x: [null, undefined, 'a'] }],
  ['dates use ISO strings', { dt: new Date(0), nested: { at: new Date(1700000000000) } }],
  ['RFC3986 reserved and unreserved chars', { s: "!*()'~-_.", sym: 'a&b=c#d/e?f+g%', sp: ' ' }],
  ['unicode and emoji', { z: '中文', e: '😀', k中: 'v' }],
  ['falsy scalars', { em: '', f: false, num: 0, neg: -1.5 }],
  ['empty containers emit nothing', { emptyArr: [], emptyObj: {}, deep: { a: { b: [] } }, ok: 1 }],
  ['bracketed keys are encoded as-is', { 'k[1]': 'v', 'a.b': 1 }],
  ['bigint and symbol', { big: BigInt(10), s: Symbol('s') }],
  ['nested arrays', { c: [[1, [2]]] }],
  ['top-level array', ['a', 'b']],
  ['empty object', {}],
  ['non-object input', 'abc'],
  ['null input', null],
  ['undefined input', undefined]
]

describe('serializeParams matches qs.stringify defaults', () => {
  it.each(cases)('%s', (_label, input) => {
    expect(serializeParams(input)).toBe(qs.stringify(input))
  })

  it('throws on cyclic input like qs', () => {
    const cyclic: Record<string, unknown> = { a: 1 }
    cyclic.self = cyclic
    expect(() => serializeParams(cyclic)).toThrow(RangeError)
  })

  it('allows the same object to appear twice when not cyclic', () => {
    const shared = { id: 1 }
    const input = { a: shared, b: shared }
    expect(serializeParams(input)).toBe(qs.stringify(input))
  })

  it('replaces lone surrogates instead of throwing', () => {
    expect(serializeParams({ d: '\uD800x' })).toBe('d=%EF%BF%BDx')
  })
})
