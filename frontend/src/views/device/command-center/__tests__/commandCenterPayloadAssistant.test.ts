import { describe, expect, it } from 'vitest'
import { buildCommandPayloadInsight } from '../commandCenterPayloadAssistant'

describe('buildCommandPayloadInsight', () => {
  it('marks empty payloads as identifier-only commands', () => {
    const insight = buildCommandPayloadInsight('   ')

    expect(insight.type).toBe('default')
    expect(insight.canFormat).toBe(false)
    expect(insight.fieldCount).toBe(0)
  })

  it('formats valid object payloads and counts top-level fields', () => {
    const insight = buildCommandPayloadInsight('{"action":"sync","force":true}')

    expect(insight.type).toBe('success')
    expect(insight.canFormat).toBe(true)
    expect(insight.fieldCount).toBe(2)
    expect(insight.formatted).toContain('"action": "sync"')
  })

  it('treats both malformed JSON and raw plain text as invalid payloads', () => {
    expect(buildCommandPayloadInsight('{"action":').type).toBe('error')
    expect(buildCommandPayloadInsight('restart now').type).toBe('error')
  })

  it('accepts JSON string payloads for text-based commands', () => {
    const insight = buildCommandPayloadInsight('"restart now"')

    expect(insight.type).toBe('success')
    expect(insight.canFormat).toBe(true)
    expect(insight.fieldCount).toBe(1)
  })
})
