import { describe, expect, it } from 'vitest'
import {
  buildDistributionSubmitPayload,
  buildExpectedMessagePayload,
  buildQuickCommandPayload,
  isApiError,
  normalizedPayloadValue,
  quickCommandKey
} from '../distributionSubmitPayload'

describe('distributionSubmitPayload', () => {
  it('builds immediate command and attribute submit payloads without changing backend contracts', () => {
    expect(
      buildDistributionSubmitPayload({
        deviceId: 'device-1',
        isCommand: true,
        textValue: '{"power":true}',
        commandValue: 'set_power'
      })
    ).toEqual({
      device_id: 'device-1',
      value: '{"power":true}',
      identify: 'set_power'
    })

    expect(
      buildDistributionSubmitPayload({
        deviceId: 'device-1',
        isCommand: false,
        textValue: '{"temperature":20}'
      })
    ).toEqual({
      device_id: 'device-1',
      value: '{"temperature":20}'
    })
  })

  it('builds expected message payloads with the existing send_type and identify rules', () => {
    expect(
      buildExpectedMessagePayload({
        deviceId: 'device-1',
        isCommand: true,
        textValue: '{"mode":"eco"}',
        commandValue: 'set_mode',
        expiry: '2026-07-05T10:00:00+08:00'
      })
    ).toEqual({
      device_id: 'device-1',
      payload: '{"mode":"eco"}',
      send_type: 'command',
      expiry: '2026-07-05T10:00:00+08:00',
      identify: 'set_mode'
    })

    expect(
      buildExpectedMessagePayload({
        deviceId: 'device-1',
        isCommand: false,
        textValue: '{"target":10}',
        commandValue: 'ignored',
        expiry: null
      })
    ).toEqual({
      device_id: 'device-1',
      payload: '{"target":10}',
      send_type: 'attribute',
      expiry: null,
      identify: null
    })
  })

  it('keeps quick command payload and in-flight key rules stable', () => {
    const row = {
      id: 'button-1',
      buttom_name: 'Restart',
      instruct: '{"restart":true}',
      data_identifier: 'restart'
    }

    expect(buildQuickCommandPayload('device-1', row)).toEqual({
      device_id: 'device-1',
      value: '{"restart":true}',
      identify: 'restart'
    })
    expect(quickCommandKey(row)).toBe('button-1')
    expect(quickCommandKey({ data_identifier: 'restart' })).toBe('restart')
  })

  it('normalizes empty payloads and detects response error envelopes', () => {
    expect(normalizedPayloadValue('')).toBeNull()
    expect(normalizedPayloadValue(null)).toBeNull()
    expect(normalizedPayloadValue('{"ok":true}')).toBe('{"ok":true}')
    expect(isApiError(undefined)).toBe(false)
    expect(isApiError({ error: null })).toBe(false)
    expect(isApiError({ error: { message: 'failed' } })).toBe(true)
  })
})
