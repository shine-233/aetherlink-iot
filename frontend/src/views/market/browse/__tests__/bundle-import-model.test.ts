/**
 * 文件用途：P1.6 打包导入闸门的纯逻辑契约测试。
 *
 * 覆盖重点（对应路线图 §4 门禁的四类路径）：
 *  1. 成功：ready 直接可提交；needs-confirm 勾选后可提交。
 *  2. 失败：非法 JSON / 非对象 / 空文件 → invalid。
 *  3. 越权式绕过：blocking 与 unsigned 下**勾选确认也不可提交**——
 *     给一个注定被拒的按钮是骗用户。
 *  4. 幂等：同包重复导入的结果汇总（idempotent 独立成态，不与 created 混算）。
 */
import { describe, expect, it } from 'vitest'

import {
  buildBundleImportPayload,
  canSubmitBundleImport,
  decideBundleImport,
  hasBlocking,
  hasBundleSignature,
  hasOverwrite,
  nextConfirmOverwrite,
  parseBundleText,
  previewNameLists,
  summarizeBundleImportResults,
  type BundleImportDecision,
  type MarketBundlePayload,
  type MarketBundlePreview
} from '../bundle-import-model'

const SIGNED: MarketBundlePayload = {
  type_key: 'hvac',
  exported_at: 1_700_000_000_000,
  count: 2,
  templates: [{ name: 't1' }, { name: 't2' }],
  digest: 'a'.repeat(64),
  signature: 'b'.repeat(64),
  signed_key_id: 'k1'
}

const CLEAN: MarketBundlePreview = { total: 2, create: ['t1', 't2'], overwrite: [], blocking: [] }
const WITH_OVERWRITE: MarketBundlePreview = { total: 2, create: ['t1'], overwrite: ['t2'], blocking: [] }
const WITH_BLOCKING: MarketBundlePreview = { total: 1, create: [], overwrite: ['t2'], blocking: ['duplicate:t2'] }

describe('parseBundleText', () => {
  it('parses a JSON object payload', () => {
    const result = parseBundleText(JSON.stringify(SIGNED))
    expect(result.ok).toBe(true)
    if (result.ok) expect(result.bundle.type_key).toBe('hvac')
  })

  it('rejects empty and whitespace-only input', () => {
    expect(parseBundleText('')).toEqual({ ok: false, errorKey: 'page.marketBrowse.importErrorEmpty' })
    expect(parseBundleText('   \n ')).toEqual({ ok: false, errorKey: 'page.marketBrowse.importErrorEmpty' })
  })

  it('rejects malformed JSON', () => {
    expect(parseBundleText('{"templates": [')).toEqual({ ok: false, errorKey: 'page.marketBrowse.importErrorJson' })
  })

  it('rejects arrays and scalars (a bundle must be an object)', () => {
    expect(parseBundleText('[]')).toEqual({ ok: false, errorKey: 'page.marketBrowse.importErrorShape' })
    expect(parseBundleText('"hello"')).toEqual({ ok: false, errorKey: 'page.marketBrowse.importErrorShape' })
    expect(parseBundleText('null')).toEqual({ ok: false, errorKey: 'page.marketBrowse.importErrorShape' })
  })
})

describe('hasBundleSignature', () => {
  it('requires all three signature fields', () => {
    expect(hasBundleSignature(SIGNED)).toBe(true)
  })

  it('is false when any field is missing', () => {
    const partials = [
      { ...SIGNED, digest: undefined },
      { ...SIGNED, signature: undefined },
      { ...SIGNED, signed_key_id: undefined }
    ]
    for (const bundle of partials) expect(hasBundleSignature(bundle)).toBe(false)
  })

  it('is false for null and for blank strings', () => {
    expect(hasBundleSignature(null)).toBe(false)
    expect(hasBundleSignature({ digest: '', signature: 'x', signed_key_id: 'y' })).toBe(false)
  })
})

describe('hasBlocking / hasOverwrite null safety', () => {
  it('treats missing arrays as empty rather than throwing', () => {
    expect(hasBlocking(null)).toBe(false)
    expect(hasBlocking(undefined)).toBe(false)
    expect(hasBlocking({})).toBe(false)
    expect(hasOverwrite(null)).toBe(false)
    expect(hasOverwrite({})).toBe(false)
  })

  it('detects non-empty arrays', () => {
    expect(hasBlocking(WITH_BLOCKING)).toBe(true)
    expect(hasOverwrite(WITH_OVERWRITE)).toBe(true)
  })
})

describe('decideBundleImport', () => {
  const cases: Array<[string, MarketBundlePayload | null, MarketBundlePreview | null, BundleImportDecision]> = [
    ['null bundle', null, null, 'invalid'],
    ['unsigned bundle', { ...SIGNED, signature: undefined }, CLEAN, 'unsigned'],
    ['empty bundle', { ...SIGNED, templates: [], count: 0 }, CLEAN, 'empty'],
    ['no preview yet', SIGNED, null, 'blocked'],
    ['blocking items', SIGNED, WITH_BLOCKING, 'blocked'],
    ['overwrite items', SIGNED, WITH_OVERWRITE, 'needs-confirm'],
    ['clean preview', SIGNED, CLEAN, 'ready']
  ]

  for (const [name, bundle, preview, expected] of cases) {
    it(`decides ${expected} for ${name}`, () => {
      expect(decideBundleImport(bundle, preview)).toBe(expected)
    })
  }

  it('checks signature before emptiness (an unsigned empty bundle is still unsigned)', () => {
    expect(decideBundleImport({ templates: [] }, CLEAN)).toBe('unsigned')
  })
})

describe('canSubmitBundleImport', () => {
  it('allows a ready bundle without confirmation', () => {
    expect(canSubmitBundleImport('ready', false)).toBe(true)
  })

  it('requires explicit confirmation when overwriting', () => {
    expect(canSubmitBundleImport('needs-confirm', false)).toBe(false)
    expect(canSubmitBundleImport('needs-confirm', true)).toBe(true)
  })

  it('never allows submission for blocked / unsigned / empty / invalid, confirmed or not', () => {
    for (const decision of ['blocked', 'unsigned', 'empty', 'invalid'] as BundleImportDecision[]) {
      expect(canSubmitBundleImport(decision, false)).toBe(false)
      // 勾选也无效：后端会拒，给按钮就是骗用户点一次再报错
      expect(canSubmitBundleImport(decision, true)).toBe(false)
    }
  })
})

describe('nextConfirmOverwrite', () => {
  it('resets when the file fingerprint changes', () => {
    expect(nextConfirmOverwrite('a.json|10|1', 'b.json|10|1')).toBe(false)
  })

  it('stays false for the first selection', () => {
    expect(nextConfirmOverwrite(null, 'a.json|10|1')).toBe(false)
  })
})

describe('buildBundleImportPayload', () => {
  it('sends only bundle for a plain import', () => {
    expect(buildBundleImportPayload(SIGNED)).toEqual({ bundle: SIGNED })
  })

  it('sends preview flag for dry runs', () => {
    expect(buildBundleImportPayload(SIGNED, { preview: true })).toEqual({ bundle: SIGNED, preview: true })
  })

  it('sends confirm_overwrite only when confirmed', () => {
    expect(buildBundleImportPayload(SIGNED, { confirmOverwrite: true })).toEqual({
      bundle: SIGNED,
      confirm_overwrite: true
    })
  })
})

describe('summarizeBundleImportResults', () => {
  it('counts each outcome separately', () => {
    const summary = summarizeBundleImportResults([
      { name: 'a', outcome: 'created' },
      { name: 'b', outcome: 'idempotent' },
      { name: 'c', outcome: 'rejected', reason: 'bad signature' }
    ])
    expect(summary).toEqual({ total: 3, created: 1, idempotent: 1, rejected: 1 })
  })

  it('counts unknown outcomes as rejected rather than dropping them', () => {
    const summary = summarizeBundleImportResults([{ name: 'a', outcome: 'weird' }, { name: 'b' }])
    expect(summary).toEqual({ total: 2, created: 0, idempotent: 0, rejected: 2 })
  })

  it('returns zeros for null / non-array input', () => {
    expect(summarizeBundleImportResults(null)).toEqual({ total: 0, created: 0, idempotent: 0, rejected: 0 })
  })
})

describe('previewNameLists', () => {
  it('returns empty arrays for a null preview', () => {
    expect(previewNameLists(null)).toEqual({ create: [], overwrite: [], blocking: [] })
  })

  it('keeps blocking separate from overwrite so the UI cannot merge them', () => {
    expect(previewNameLists(WITH_BLOCKING)).toEqual({
      create: [],
      overwrite: ['t2'],
      blocking: ['duplicate:t2']
    })
  })
})
