import { describe, expect, it } from 'vitest'
import {
  buildFirstAutomationRecommendedActionDraft,
  buildFirstAutomationRecommendedConditionDraft,
  buildFirstAutomationTelemetryRecommendation
} from '../automationStarterRecommendation'

const texts = {
  keyTitle: 'field',
  valueTitle: 'value',
  timeTitle: 'time',
  keyFallback: 'latest field',
  valueFallback: 'no value',
  timeFallback: 'no time',
  sourceTitle: 'source',
  sourceDevice: 'device {id}',
  sourceTemplate: 'template {id}',
  sourceFallback: 'choose source',
  conditionHint: 'choose temperature',
  nextActionTitle: 'next action',
  nextActionWithKey: 'set threshold for {telemetry}',
  nextActionWithoutKey: 'confirm telemetry first',
  conditionDraftTitle: 'condition draft',
  conditionDraftWithValue: 'draft {source} {telemetry} {operator} {value}',
  conditionDraftWithoutValue: 'draft {source} {telemetry}',
  conditionDraftMissing: 'missing draft',
  actionDraftTitle: 'action draft',
  actionDraftDesc: 'trigger an alarm after choosing a target'
}

describe('automationStarterRecommendation', () => {
  it('marks the telemetry proof as ready when the first-device route includes latest telemetry', () => {
    const recommendation = buildFirstAutomationTelemetryRecommendation(
      {
        telemetryKey: 'temperature',
        telemetryValue: '36.5',
        telemetryAt: '2026-07-06T12:00:00.000Z',
        deviceId: 'device-1',
        deviceConfigId: 'config-1'
      },
      texts
    )

    expect(recommendation.hasTelemetryKey).toBe(true)
    expect(recommendation.conditionHint).toBe('choose temperature')
    expect(recommendation.nextAction).toEqual({
      title: 'next action',
      desc: 'set threshold for temperature',
      status: 'ready'
    })
    expect(recommendation.cards).toEqual([
      { key: 'key', title: 'field', value: 'temperature', status: 'ready' },
      { key: 'value', title: 'value', value: '36.5', status: 'ready' },
      { key: 'time', title: 'time', value: '2026-07-06T12:00:00.000Z', status: 'ready' },
      { key: 'source', title: 'source', value: 'device device-1', status: 'ready' }
    ])
    expect(recommendation.conditionDraft).toMatchObject({
      available: true,
      title: 'condition draft',
      desc: 'draft device device-1 temperature > 36.5',
      status: 'ready',
      condition: {
        ifType: '1',
        trigger_conditions_type: '10',
        trigger_source: 'device-1',
        trigger_param_type: 'telemetry',
        trigger_param: 'temperature',
        trigger_param_key: 'telemetry/temperature',
        trigger_operator: '>',
        trigger_value: '36.5'
      }
    })
    expect(recommendation.actionDraft).toEqual({
      title: 'action draft',
      desc: 'trigger an alarm after choosing a target',
      status: 'needs-target',
      action: {
        actionType: '30',
        action_type: '30',
        action_target: null
      }
    })
  })

  it('keeps the beginner guide useful when telemetry proof is still missing', () => {
    const recommendation = buildFirstAutomationTelemetryRecommendation({}, texts)

    expect(recommendation.hasTelemetryKey).toBe(false)
    expect(recommendation.nextAction).toEqual({
      title: 'next action',
      desc: 'confirm telemetry first',
      status: 'missing'
    })
    expect(recommendation.cards).toEqual([
      { key: 'key', title: 'field', value: 'latest field', status: 'missing' },
      { key: 'value', title: 'value', value: 'no value', status: 'missing' },
      { key: 'time', title: 'time', value: 'no time', status: 'missing' },
      { key: 'source', title: 'source', value: 'choose source', status: 'missing' }
    ])
    expect(recommendation.conditionDraft).toEqual({
      available: false,
      title: 'condition draft',
      desc: 'missing draft',
      status: 'missing',
      condition: null
    })
  })

  it('builds a device-scoped telemetry condition draft when a first device is known', () => {
    const draft = buildFirstAutomationRecommendedConditionDraft(
      {
        telemetryKey: 'temperature',
        telemetryValue: '36.5',
        deviceId: 'device-1',
        deviceConfigId: 'config-1'
      },
      texts
    )

    expect(draft.condition).toMatchObject({
      ifType: '1',
      trigger_conditions_type: '10',
      trigger_source: 'device-1',
      trigger_param_type: 'telemetry',
      trigger_param: 'temperature',
      trigger_param_key: 'telemetry/temperature',
      trigger_operator: '>',
      trigger_value: '36.5',
      minValue: null,
      maxValue: null,
      triggerParamOptions: []
    })
  })

  it('falls back to a template-scoped draft when only the template is known', () => {
    const draft = buildFirstAutomationRecommendedConditionDraft(
      {
        telemetryKey: 'status',
        telemetryValue: 'online',
        deviceConfigId: 'config-1'
      },
      texts
    )

    expect(draft.condition).toMatchObject({
      trigger_conditions_type: '11',
      trigger_source: 'config-1',
      trigger_operator: '=',
      trigger_value: 'online'
    })
  })

  it('builds a safe alarm action slot that still needs a target before save', () => {
    const draft = buildFirstAutomationRecommendedActionDraft(texts)

    expect(draft).toEqual({
      title: 'action draft',
      desc: 'trigger an alarm after choosing a target',
      status: 'needs-target',
      action: {
        actionType: '30',
        action_type: '30',
        action_target: null
      }
    })
  })
})
