import { describe, expect, it } from 'vitest'
import {
  aggregateNumericValues,
  filterRelationTargetIds,
  generateRelationFieldKey,
  isEntityRelationConfigured,
  resolveEntityRelationValue
} from '../resolver'
import type { EntityRelationEdge, EntityRelationSourceConfig } from '../types'

describe('Entity Relation Resolver (ROADMAP P1.1)', () => {
  const baseConfig: EntityRelationSourceConfig = {
    enabled: true,
    rootType: 'device',
    rootId: 'gw-001',
    direction: 'from',
    relationType: 'Contains',
    targetType: 'device',
    targetKey: 'temperature',
    aggregation: 'avg'
  }

  const sampleEdges: EntityRelationEdge[] = [
    {
      from_type: 'device',
      from_id: 'gw-001',
      relation_type: 'Contains',
      to_type: 'device',
      to_id: 'dev-001'
    },
    {
      from_type: 'device',
      from_id: 'gw-001',
      relation_type: 'Contains',
      to_type: 'device',
      to_id: 'dev-002'
    },
    {
      from_type: 'device',
      from_id: 'gw-001',
      relation_type: 'Contains',
      to_type: 'device',
      to_id: 'dev-001' // duplicate edge, should deduplicate
    },
    {
      from_type: 'device',
      from_id: 'gw-001',
      relation_type: 'Monitors', // different relation
      to_type: 'device',
      to_id: 'dev-003'
    },
    {
      from_type: 'asset', // different from_type
      from_id: 'gw-001',
      relation_type: 'Contains',
      to_type: 'device',
      to_id: 'dev-004'
    },
    {
      from_type: 'device',
      from_id: 'sensor-999',
      relation_type: 'BelongsTo',
      to_type: 'device',
      to_id: 'gw-001'
    }
  ]

  describe('isEntityRelationConfigured', () => {
    it('returns true for fully configured config', () => {
      expect(isEntityRelationConfigured(baseConfig)).toBe(true)
    })

    it('returns false when disabled', () => {
      expect(isEntityRelationConfigured({ ...baseConfig, enabled: false })).toBe(false)
    })

    it('returns false when required fields are empty or whitespace', () => {
      expect(isEntityRelationConfigured({ ...baseConfig, rootId: '' })).toBe(false)
      expect(isEntityRelationConfigured({ ...baseConfig, relationType: '  ' })).toBe(false)
      expect(isEntityRelationConfigured({ ...baseConfig, targetKey: '' })).toBe(false)
      expect(isEntityRelationConfigured(null)).toBe(false)
      expect(isEntityRelationConfigured(undefined)).toBe(false)
    })
  })

  describe('generateRelationFieldKey', () => {
    it('generates consistent deterministic key', () => {
      const key = generateRelationFieldKey(baseConfig)
      expect(key).toBe('__rel_device_gw-001_from_Contains_temperature')
    })

    it('sanitizes spaces in relationType and targetKey', () => {
      const key = generateRelationFieldKey({
        ...baseConfig,
        relationType: 'Custom Relation',
        targetKey: 'temp reading'
      })
      expect(key).toBe('__rel_device_gw-001_from_Custom_Relation_temp_reading')
    })
  })

  describe('filterRelationTargetIds', () => {
    it('filters target entities with direction from and deduplicates', () => {
      const targets = filterRelationTargetIds(baseConfig, sampleEdges)
      expect(targets).toEqual(['dev-001', 'dev-002'])
    })

    it('filters target entities with direction to', () => {
      const toConfig: EntityRelationSourceConfig = {
        enabled: true,
        rootType: 'device',
        rootId: 'gw-001',
        direction: 'to',
        relationType: 'BelongsTo',
        targetType: 'device',
        targetKey: 'temperature'
      }
      const targets = filterRelationTargetIds(toConfig, sampleEdges)
      expect(targets).toEqual(['sensor-999'])
    })

    it('returns empty array if no edges match or edges are empty', () => {
      const targets = filterRelationTargetIds(
        { ...baseConfig, relationType: 'NonExistent' },
        sampleEdges
      )
      expect(targets).toEqual([])
      expect(filterRelationTargetIds(baseConfig, [])).toEqual([])
    })
  })

  describe('aggregateNumericValues', () => {
    const numbers = [10, 20, 30]

    it('calculates average correctly', () => {
      expect(aggregateNumericValues(numbers, 'avg')).toBe(20)
    })

    it('calculates sum correctly', () => {
      expect(aggregateNumericValues(numbers, 'sum')).toBe(60)
    })

    it('calculates max and min correctly', () => {
      expect(aggregateNumericValues(numbers, 'max')).toBe(30)
      expect(aggregateNumericValues(numbers, 'min')).toBe(10)
    })

    it('calculates count correctly', () => {
      expect(aggregateNumericValues(numbers, 'count')).toBe(3)
      expect(aggregateNumericValues([], 'count')).toBe(0)
    })

    it('returns latest as first item', () => {
      expect(aggregateNumericValues(numbers, 'latest')).toBe(10)
    })

    it('handles arrays with null and undefined', () => {
      expect(aggregateNumericValues([null, 15, undefined, 25], 'avg')).toBe(20)
      expect(aggregateNumericValues([null, undefined], 'avg')).toBeNull()
    })
  })

  describe('resolveEntityRelationValue', () => {
    const telemetryMap = {
      'dev-001': { temperature: 24.5, status: 'ONLINE' },
      'dev-002': { temperature: 27.5, status: 'STANDBY' }
    }

    it('resolves average numeric telemetry value across matching targets', () => {
      const res = resolveEntityRelationValue(baseConfig, sampleEdges, telemetryMap)
      expect(res.matchedTargetIds).toEqual(['dev-001', 'dev-002'])
      expect(res.resolvedValue).toBe(26) // (24.5 + 27.5) / 2 = 26
      expect(res.targetValues).toHaveLength(2)
      expect(res.targetValues[0]).toEqual({
        targetId: 'dev-001',
        value: 24.5,
        numericValue: 24.5
      })
    })

    it('resolves string values using latest fallback', () => {
      const statusConfig: EntityRelationSourceConfig = {
        ...baseConfig,
        targetKey: 'status',
        aggregation: 'latest'
      }
      const res = resolveEntityRelationValue(statusConfig, sampleEdges, telemetryMap)
      expect(res.resolvedValue).toBe('ONLINE')
    })

    it('handles count aggregation correctly', () => {
      const countConfig: EntityRelationSourceConfig = {
        ...baseConfig,
        aggregation: 'count'
      }
      const res = resolveEntityRelationValue(countConfig, sampleEdges, telemetryMap)
      expect(res.resolvedValue).toBe(2)
    })

    it('gracefully handles missing telemetry', () => {
      const res = resolveEntityRelationValue(baseConfig, sampleEdges, {})
      expect(res.matchedTargetIds).toEqual(['dev-001', 'dev-002'])
      expect(res.resolvedValue).toBeNull()
    })
  })
})