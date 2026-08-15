import { describe, expect, it } from 'vitest'

import {
  SCENE_DRY_RUN_QUICK_FIX_KEYS,
  buildSceneActionDryRunPayload,
  buildSceneDryRunQuickFixActions,
  getSceneActionLocalBlocker
} from '../scene-dry-run-preview'

const t = (key: string, options?: Record<string, any>) => {
  if (!options) return key
  return `${key}:${JSON.stringify(options)}`
}

describe('scene-dry-run-preview', () => {
  it('builds an action-only dry-run payload instead of inventing trigger conditions', () => {
    const payload = buildSceneActionDryRunPayload({
      form: {
        id: 'scene-1',
        name: 'Restart device',
        description: 'Manual scene'
      },
      buildSubmitPayload: () => ({
        actions: [
          {
            action_type: '10',
            action_target: 'device-1',
            action_param_type: 'command',
            action_param: 'reboot',
            action_value: '{"method":"reboot","params":"{}"}'
          }
        ]
      })
    })

    expect(payload).toMatchObject({
      id: 'scene-1',
      name: 'Restart device',
      actions: [
        {
          action_type: '10',
          action_target: 'device-1',
          action_param_type: 'command',
          action_param: 'reboot'
        }
      ]
    })
    expect(payload).not.toHaveProperty('trigger_condition_groups')
    expect(getSceneActionLocalBlocker(payload, t)).toBe('')
  })

  it('reports local payload build errors without calling the rule dry-run contract', () => {
    const payload = buildSceneActionDryRunPayload({
      form: { id: 'scene-1' },
      buildSubmitPayload: () => {
        throw new Error('invalid command JSON')
      }
    })

    expect(payload).not.toHaveProperty('trigger_condition_groups')
    expect(payload.actions).toEqual([])
    expect(getSceneActionLocalBlocker(payload, t)).toContain('invalid command JSON')
  })

  it('points beginners to the smallest safe next action setup step', () => {
    expect(
      buildSceneDryRunQuickFixActions({
        actionGroups: [],
        texts: {
          addActionGroupTitle: 'Add group',
          addActionGroupDesc: 'Add group desc',
          addActionGroupButton: 'Add group button',
          selectOperateDeviceTitle: 'Operate device',
          selectOperateDeviceDesc: 'Operate device desc',
          selectOperateDeviceButton: 'Operate device button',
          addDeviceInstructionTitle: 'Add instruction',
          addDeviceInstructionDesc: 'Add instruction desc',
          addDeviceInstructionButton: 'Add instruction button'
        }
      })[0].key
    ).toBe(SCENE_DRY_RUN_QUICK_FIX_KEYS.addActionGroup)

    expect(
      buildSceneDryRunQuickFixActions({
        actionGroups: [{ actionType: null, actionInstructList: [] }],
        texts: {
          addActionGroupTitle: 'Add group',
          addActionGroupDesc: 'Add group desc',
          addActionGroupButton: 'Add group button',
          selectOperateDeviceTitle: 'Operate device',
          selectOperateDeviceDesc: 'Operate device desc',
          selectOperateDeviceButton: 'Operate device button',
          addDeviceInstructionTitle: 'Add instruction',
          addDeviceInstructionDesc: 'Add instruction desc',
          addDeviceInstructionButton: 'Add instruction button'
        }
      })[0].key
    ).toBe(SCENE_DRY_RUN_QUICK_FIX_KEYS.selectOperateDevice)
  })
})
