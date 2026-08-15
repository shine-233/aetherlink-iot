import { describe, expect, it } from 'vitest'

import {
  buildActionsPayload,
  buildActionValuePayload,
  formatActionGroupsForEcho,
  OPERATE_DEVICE_ACTION_TYPE,
  parseActionValueForEcho
} from '../scene-action-mappers'

describe('scene-action-mappers', () => {
  it('serializes telemetry-style action values into keyed JSON payloads', () => {
    expect(
      buildActionValuePayload({
        action_param_type: 'telemetry',
        action_param: 'switch',
        actionValue: 1
      })
    ).toBe('{"switch":1}')
  })

  it('serializes command-style action values into method plus params payloads', () => {
    expect(
      buildActionValuePayload({
        action_param_type: 'command',
        action_param: 'reboot',
        actionValue: '{"delay":5}'
      })
    ).toBe('{"method":"reboot","params":"{\\"delay\\":5}"}')
  })

  it('keeps inline JSON payloads unchanged for compatibility action param types', () => {
    expect(
      buildActionValuePayload({
        action_param_type: 'c_telemetry',
        actionValue: '{"switch":1}'
      })
    ).toBe('{"switch":1}')
  })

  it('deserializes telemetry and command payloads back into editor values', () => {
    expect(
      parseActionValueForEcho({
        action_param_type: 'telemetry',
        action_param: 'switch',
        action_value: '{"switch":1}'
      })
    ).toBe(1)

    expect(
      parseActionValueForEcho({
        action_param_type: 'command',
        action_param: 'reboot',
        action_value: '{"method":"reboot","params":"{\\"delay\\":5}"}'
      })
    ).toBe('{"delay":5}')
  })

  it('groups operate-device instructions for echo while preserving non-device action groups', () => {
    const grouped = formatActionGroupsForEcho([
      {
        id: 'scene-1',
        action_type: '20',
        scene_id: 'target-scene'
      },
      {
        id: 'instruction-1',
        action_type: '10',
        action_param_type: 'attributes',
        action_param: 'mode',
        action_value: '{"mode":"auto"}'
      },
      {
        id: 'instruction-2',
        action_type: '11',
        action_param_type: 'command',
        action_param: 'sync',
        action_value: '{"method":"sync","params":"{\\"full\\":true}"}'
      }
    ])

    expect(grouped[0]).toMatchObject({
      id: 'scene-1',
      actionType: '20'
    })
    expect(grouped[1]).toMatchObject({
      actionType: OPERATE_DEVICE_ACTION_TYPE
    })
    expect(grouped[1].actionInstructList).toEqual([
      expect.objectContaining({
        id: 'instruction-1',
        actionValue: 'auto'
      }),
      expect.objectContaining({
        id: 'instruction-2',
        actionValue: '{"full":true}'
      })
    ])
  })

  it('flattens operate-device groups back into API actions payloads', () => {
    const payload = buildActionsPayload([
      {
        actionType: OPERATE_DEVICE_ACTION_TYPE,
        actionInstructList: [
          {
            action_type: '10',
            action_param_type: 'telemetry',
            action_param: 'speed',
            actionValue: 12
          }
        ]
      },
      {
        actionType: '20',
        scene_id: 'scene-2'
      }
    ])

    expect(payload).toEqual([
      expect.objectContaining({
        action_type: '10',
        action_value: '{"speed":12}'
      }),
      expect.objectContaining({
        action_type: '20',
        scene_id: 'scene-2'
      })
    ])
  })
})
