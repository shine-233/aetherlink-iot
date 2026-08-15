import { describe, expect, it } from 'vitest'
import {
  AUTOMATION_DRY_RUN_QUICK_FIX_KEYS,
  buildFirstAutomationDryRunQuickFixActions,
  buildGeneralAutomationDryRunQuickFixActions,
  createAlarmActionSlot,
  hasAlarmActionMissingTarget,
  hasAlarmActionSlot
} from '../automationDryRunQuickFixActions'

const texts = {
  addConditionTitle: 'Add a condition',
  addConditionDesc: 'Choose when the rule should run.',
  addConditionButton: 'Add condition',
  addAlarmActionTitle: 'Add an alarm action',
  addAlarmActionDesc: 'Start with a safe alarm action.',
  addAlarmActionButton: 'Add alarm action',
  createAlarmTargetTitle: 'Create alarm target',
  createAlarmTargetDesc: 'Choose or create the alert target.',
  createAlarmTargetButton: 'Create target'
}

describe('automationDryRunQuickFixActions', () => {
  it('guides normal automation editors to add missing conditions and actions', () => {
    expect(
      buildGeneralAutomationDryRunQuickFixActions({
        conditionCount: 0,
        actionCount: 0,
        actions: [],
        texts
      }).map((item) => item.key)
    ).toEqual([
      AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.addConditionGroup,
      AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.addAlarmActionSlot
    ])
  })

  it('promotes missing alarm targets without inventing a backend result', () => {
    expect(hasAlarmActionSlot([{ actionType: '30', action_target: null }])).toBe(true)
    expect(hasAlarmActionMissingTarget([{ actionType: '30', action_target: null }])).toBe(true)
    expect(hasAlarmActionMissingTarget([{ action_type: '30', action_target: 'alarm-1' }])).toBe(false)

    expect(
      buildGeneralAutomationDryRunQuickFixActions({
        conditionCount: 1,
        actionCount: 1,
        actions: [{ actionType: '30', action_target: '' }],
        texts
      })
    ).toEqual([
      expect.objectContaining({
        key: AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.createAlarmTarget,
        buttonLabel: 'Create target',
        type: 'warning'
      })
    ])
  })

  it('builds first-device starter quick fixes with stable keys', () => {
    expect(
      buildFirstAutomationDryRunQuickFixActions({
        actions: [],
        texts: {
          applyActionTitle: 'Apply recommended action',
          applyActionDesc: 'Use the starter alarm action.',
          applyActionButton: 'Apply',
          createAlarmTargetTitle: 'Create target',
          createAlarmTargetDesc: 'Choose the target.',
          createAlarmTargetButton: 'Create target'
        }
      }).map((item) => item.key)
    ).toEqual([AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.applyFirstAlarmAction])

    expect(
      buildFirstAutomationDryRunQuickFixActions({
        actions: [{ actionType: '30', action_target: null }],
        texts: {
          applyActionTitle: 'Apply recommended action',
          applyActionDesc: 'Use the starter alarm action.',
          applyActionButton: 'Apply',
          createAlarmTargetTitle: 'Create target',
          createAlarmTargetDesc: 'Choose the target.',
          createAlarmTargetButton: 'Create target'
        }
      }).map((item) => item.key)
    ).toEqual([AUTOMATION_DRY_RUN_QUICK_FIX_KEYS.createFirstAlarmTarget])
  })
  it('creates a safe unsaved alarm-action slot', () => {
    expect(createAlarmActionSlot()).toEqual({
      actionType: '30',
      action_type: '30',
      action_target: null
    })
  })
})
