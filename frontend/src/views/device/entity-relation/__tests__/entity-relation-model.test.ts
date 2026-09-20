/**
 * 文件用途: 通用实体关系前端模型的定向用例（ROADMAP P1.1）。
 * 核心逻辑: 覆盖白名单、自环判定、反向关系、元数据解析、删除策略语义。
 * 关键注意事项:
 *  1. 这些规则是**后端契约的镜像**，不是前端自立的标准。改动前先对齐
 *     `internal/model/entity_relation.go`。
 *  2. 自环用例里"不同类型但同 ID"必须**通过**——把它判成自环会误伤真实数据。
 */
import { describe, expect, it } from 'vitest'
import {
  ENTITY_TYPES,
  METADATA_MAX_BYTES,
  RELATION_TYPE_MAX_LENGTH,
  describeDeletionPolicy,
  findReverse,
  isAllowedEntityType,
  isReverseOf,
  metadataByteLength,
  parseMetadata,
  relationKey,
  validateDraft,
  type EntityRelation,
  type EntityRelationDraft
} from '../entity-relation-model'

const draft = (overrides: Partial<EntityRelationDraft> = {}): EntityRelationDraft => ({
  from_type: 'device',
  from_id: 'dev-1',
  relation_type: 'belongs_to',
  to_type: 'asset',
  to_id: 'asset-1',
  metadata: '',
  ...overrides
})

const relation = (overrides: Partial<EntityRelation> = {}): EntityRelation => ({
  id: 'rel-1',
  tenant_id: 't1',
  from_type: 'device',
  from_id: 'dev-1',
  relation_type: 'belongs_to',
  to_type: 'asset',
  to_id: 'asset-1',
  metadata: null,
  created_at: '2026-09-12T00:00:00Z',
  updated_at: '2026-09-12T00:00:00Z',
  ...overrides
})

describe('entity type whitelist', () => {
  it('accepts exactly the four controlled types', () => {
    expect(ENTITY_TYPES).toEqual(['device', 'asset', 'customer', 'gateway'])
  })

  it.each([
    ['device', true],
    ['asset', true],
    ['customer', true],
    ['gateway', true],
    ['Device', false],
    ['server', false],
    ['', false]
  ])('isAllowedEntityType(%s) === %s', (value, expected) => {
    expect(isAllowedEntityType(value)).toBe(expected)
  })
})

describe('validateDraft', () => {
  it('accepts a well-formed draft', () => {
    expect(validateDraft(draft()).ok).toBe(true)
  })

  it.each([
    ['from_id', { from_id: '   ' }],
    ['to_id', { to_id: '' }],
    ['relation_type', { relation_type: '  ' }]
  ])('rejects missing %s', (field, override) => {
    const result = validateDraft(draft(override))
    expect(result.ok).toBe(false)
    expect(result.errors[field as string]).toBeTruthy()
  })

  it.each([['server'], ['DEVICE'], ['']])('rejects unknown entity type %s', (value) => {
    const result = validateDraft(draft({ from_type: value as never }))
    expect(result.ok).toBe(false)
    expect(result.errors.from_type).toBeTruthy()
  })

  it('rejects self loop with same type and same id', () => {
    const result = validateDraft(draft({ to_type: 'device', to_id: 'dev-1' }))
    expect(result.ok).toBe(false)
    expect(result.errors.to_id).toContain('自己')
  })

  // 关键：不同类型 + 同 ID 不是自环，ID 空间按类型隔离。
  it('allows same id across different types', () => {
    const result = validateDraft(draft({ from_id: 'x', to_type: 'asset', to_id: 'x' }))
    expect(result.ok).toBe(true)
  })

  it('rejects over-long relation type', () => {
    const result = validateDraft(draft({ relation_type: 'r'.repeat(RELATION_TYPE_MAX_LENGTH + 1) }))
    expect(result.ok).toBe(false)
    expect(result.errors.relation_type).toBeTruthy()
  })

  it('accepts relation type at the length limit', () => {
    const result = validateDraft(draft({ relation_type: 'r'.repeat(RELATION_TYPE_MAX_LENGTH) }))
    expect(result.ok).toBe(true)
  })

  it('rejects oversized metadata', () => {
    const result = validateDraft(draft({ metadata: JSON.stringify({ pad: 'x'.repeat(METADATA_MAX_BYTES) }) }))
    expect(result.ok).toBe(false)
    expect(result.errors.metadata).toBeTruthy()
  })
})

describe('metadata handling', () => {
  it('treats blank metadata as absent', () => {
    expect(metadataByteLength('')).toBe(0)
    expect(metadataByteLength('   ')).toBe(0)
    expect(parseMetadata('')).toBeUndefined()
    expect(parseMetadata('  ')).toBeUndefined()
  })

  it('counts utf-8 bytes rather than characters', () => {
    // 一个中文字符 3 字节：按字符数算会低估，进而放行超限的元数据。
    expect(metadataByteLength('中文')).toBe(6)
  })

  it('accepts a json object', () => {
    expect(parseMetadata('{"a":1}')).toBe('{"a":1}')
  })

  it.each([['not json'], ['[1,2]'], ['"str"'], ['123']])('rejects %s', (value) => {
    expect(() => parseMetadata(value)).toThrow()
  })
})

describe('reverse relations', () => {
  const a = relation()
  const b = relation({ id: 'rel-2', from_type: 'asset', from_id: 'asset-1', to_type: 'device', to_id: 'dev-1' })

  it('detects a reverse pair', () => {
    expect(isReverseOf(a, b)).toBe(true)
  })

  it('is symmetric', () => {
    expect(isReverseOf(b, a)).toBe(true)
  })

  it('does not match when relation type differs', () => {
    expect(isReverseOf(a, { ...b, relation_type: 'other' })).toBe(false)
  })

  it('does not match a self relation', () => {
    expect(isReverseOf(a, a)).toBe(false)
  })

  it('finds an existing reverse in a list', () => {
    expect(findReverse([a, b], draft())?.id).toBe('rel-2')
  })

  it('returns undefined when no reverse exists', () => {
    expect(findReverse([a], draft())).toBeUndefined()
  })

  it('returns undefined for an invalid draft instead of guessing', () => {
    expect(findReverse([a, b], draft({ from_type: '' }))).toBeUndefined()
  })
})

describe('relationKey', () => {
  const keyed = relation()

  it('ignores id and tenant when identifying a relation', () => {
    expect(relationKey(keyed)).toBe(relationKey(relation({ id: 'other', tenant_id: 't2' })))
  })

  it('differs when an endpoint differs', () => {
    expect(relationKey(keyed)).not.toBe(relationKey(relation({ to_id: 'asset-2' })))
  })
})

describe('deletion policy', () => {
  it('defaults semantics are spelled out, not silent', () => {
    expect(describeDeletionPolicy('protect')).toContain('拒绝')
    expect(describeDeletionPolicy('cascade')).toContain('不可恢复')
  })
})
