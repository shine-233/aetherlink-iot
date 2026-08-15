import { describe, expect, it } from 'vitest'

import {
  ACTION_PARAM_PLACEHOLDERS,
  applyActionParamOptionsData,
  applyActionParamSelection,
  applyActionParamTypeChange,
  buildActionParamTypeOptions,
  clearActionValueValidationState,
  markInvalidJsonActionValue,
  normalizeActionParamOptionsData,
  resetInstructionSelection,
  resetInstructionTargetDependentState,
  validateJsonActionValue,
  validateSceneActionJsonValues
} from '../scene-action-form-state'

describe('scene-action-form-state', () => {
  it('normalizes grouped action param options into select-ready shape', () => {
    const normalized = normalizeActionParamOptionsData([
      {
        data_source_type: 'telemetry',
        label: 'Telemetry',
        options: [{ key: 'speed', label: 'Speed', data_type: 'INT' }]
      }
    ])

    expect(normalized).toEqual([
      {
        data_source_type: 'telemetry',
        label: '(Telemetry)telemetry',
        value: 'telemetry',
        options: [
          {
            key: 'speed',
            label: 'speed(Speed)',
            value: 'speed',
            data_type: 'INT'
          }
        ]
      }
    ])
    expect(buildActionParamTypeOptions(normalized)).toEqual([{ label: '(Telemetry)telemetry', value: 'telemetry' }])
  })

  it('resets target-dependent instruction state without touching action target', () => {
    const instructItem = {
      action_target: 'device-1',
      action_param_type: 'telemetry',
      action_param: 'speed',
      actionValue: 10,
      actionParamOptionsData: [{}],
      actionParamTypeOptions: [{}],
      actionParamOptions: [{}]
    }

    resetInstructionTargetDependentState(instructItem as any)

    expect(instructItem).toMatchObject({
      action_target: 'device-1',
      action_param_type: null,
      action_param: null,
      actionValue: null,
      actionParamOptionsData: [],
      actionParamTypeOptions: [],
      actionParamOptions: []
    })
  })

  it('resets full instruction selection when action type changes', () => {
    const instructItem = {
      action_target: 'device-1',
      action_param_type: 'telemetry',
      action_param: 'speed',
      action_param_key: 'speed',
      action_value: '{"speed":1}'
    }

    resetInstructionSelection(instructItem as any)

    expect(instructItem).toMatchObject({
      action_target: null,
      action_param_type: null,
      action_param: null,
      action_param_key: null,
      action_value: null
    })
  })

  it('updates action param type selection and placeholder together', () => {
    const instructItem = {
      action_param_type: null,
      action_param: 'old',
      actionParamData: { key: 'old' },
      actionParamOptionsData: normalizeActionParamOptionsData([
        {
          data_source_type: 'command',
          options: [{ key: 'reboot', label: 'Reboot' }]
        }
      ]),
      actionParamOptions: [],
      actionValue: 'old-value',
      showSubSelect: true,
      placeholder: ''
    }

    applyActionParamTypeChange(instructItem as any, 'command')

    expect(instructItem.action_param).toBeNull()
    expect(instructItem.actionParamData).toBeNull()
    expect(instructItem.actionParamOptions).toEqual([
      { key: 'reboot', label: 'reboot(Reboot)', value: 'reboot' }
    ])
    expect(instructItem.placeholder).toBe(ACTION_PARAM_PLACEHOLDERS.command)
    expect(instructItem.actionValue).toBeNull()
    expect(instructItem.showSubSelect).toBe(true)
  })

  it('hydrates dependent options and selected data after menu load', () => {
    const instructItem = {
      action_param_type: 'telemetry',
      action_param: 'speed',
      actionParamOptionsData: [],
      actionParamTypeOptions: [],
      actionParamOptions: [],
      actionParamData: null,
      showSubSelect: false
    }

    applyActionParamOptionsData(
      instructItem as any,
      normalizeActionParamOptionsData([
        {
          data_source_type: 'telemetry',
          options: [{ key: 'speed', data_type: 'FLOAT' }]
        }
      ])
    )

    expect(instructItem.actionParamTypeOptions).toEqual([{ label: 'telemetry', value: 'telemetry' }])
    expect(instructItem.actionParamOptions).toEqual([{ key: 'speed', value: 'speed', label: 'speed', data_type: 'float' }])
    expect(instructItem.actionParamData).toEqual({ key: 'speed', value: 'speed', label: 'speed', data_type: 'float' })
    expect(instructItem.showSubSelect).toBe(true)
  })

  it('updates selected action param data and clears stale action value', () => {
    const instructItem = {
      actionValue: '{"old":1}',
      actionParamOptions: [
        { key: 'speed', data_type: 'INT' },
        { key: 'mode', data_type: 'STRING' }
      ],
      actionParamData: null
    }

    applyActionParamSelection(instructItem as any, 'mode')

    expect(instructItem.actionValue).toBeNull()
    expect(instructItem.actionParamData).toEqual({ key: 'mode', data_type: 'string' })
  })

  it('validates only JSON-requiring action values', () => {
    expect(validateJsonActionValue('command', '{"delay":1}')).toBe(true)
    expect(validateJsonActionValue('command', '123')).toBe(false)
    expect(validateJsonActionValue('c_telemetry', '{"switch":1}')).toBe(true)
    expect(validateJsonActionValue('telemetry', 'plain-text')).toBe(true)
  })

  it('marks and reports invalid JSON action values before submit', () => {
    const invalidInstruction = {
      action_param_type: 'command',
      actionValue: 'not-json',
      inputFeedback: '',
      inputValidationStatus: undefined
    }

    const issues = validateSceneActionJsonValues([
      {
        actionType: '1',
        actionInstructList: [invalidInstruction as any]
      }
    ])

    expect(issues).toHaveLength(1)
    expect(issues[0]).toMatchObject({
      actionGroupIndex: 0,
      instructIndex: 0,
      actionParamType: 'command',
      message: 'common.enterJson'
    })
    expect(invalidInstruction.inputValidationStatus).toBe('error')
    expect(invalidInstruction.inputFeedback).toBe('common.enterJson')
  })

  it('clears stale JSON errors when a save-time scan finds valid JSON', () => {
    const validInstruction = {
      action_param_type: 'c_attribute',
      actionValue: '{"addr":1}',
      inputFeedback: 'common.enterJson',
      inputValidationStatus: 'error'
    }

    const issues = validateSceneActionJsonValues([
      {
        actionType: '1',
        actionInstructList: [validInstruction as any]
      }
    ])

    expect(issues).toEqual([])
    expect(validInstruction.inputFeedback).toBe('')
    expect(validInstruction.inputValidationStatus).toBeUndefined()
  })

  it('keeps non-json action values out of the save-time JSON scan', () => {
    const telemetryInstruction = {
      action_param_type: 'telemetry',
      actionValue: 'plain-text',
      inputFeedback: '',
      inputValidationStatus: undefined
    }

    expect(validateSceneActionJsonValues([
      {
        actionType: '1',
        actionInstructList: [telemetryInstruction as any]
      }
    ])).toEqual([])
    expect(telemetryInstruction.inputValidationStatus).toBeUndefined()
  })

  it('shares explicit action value validation state helpers', () => {
    const instructItem = {
      inputFeedback: '',
      inputValidationStatus: undefined
    }

    markInvalidJsonActionValue(instructItem as any, 'Invalid JSON')
    expect(instructItem).toMatchObject({
      inputFeedback: 'Invalid JSON',
      inputValidationStatus: 'error'
    })

    clearActionValueValidationState(instructItem as any)
    expect(instructItem.inputFeedback).toBe('')
    expect(instructItem.inputValidationStatus).toBeUndefined()
  })
})
