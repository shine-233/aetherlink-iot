export type FirstAutomationTelemetryCardKey = 'key' | 'value' | 'time' | 'source'

export interface FirstAutomationTelemetryStarter {
  telemetryKey?: string
  telemetryValue?: string
  telemetryAt?: string
  deviceId?: string
  deviceConfigId?: string
}

export interface FirstAutomationTelemetryRecommendationTexts {
  keyTitle: string
  valueTitle: string
  timeTitle: string
  keyFallback: string
  valueFallback: string
  timeFallback: string
  sourceTitle: string
  sourceDevice: string
  sourceTemplate: string
  sourceFallback: string
  conditionHint: string
  nextActionTitle: string
  nextActionWithKey: string
  nextActionWithoutKey: string
  conditionDraftTitle: string
  conditionDraftWithValue: string
  conditionDraftWithoutValue: string
  conditionDraftMissing: string
  actionDraftTitle: string
  actionDraftDesc: string
}

export interface FirstAutomationTelemetryRecommendationCard {
  key: FirstAutomationTelemetryCardKey
  title: string
  value: string
  status: 'ready' | 'missing'
}

export interface FirstAutomationTelemetryRecommendation {
  hasTelemetryKey: boolean
  conditionHint: string
  nextAction: {
    title: string
    desc: string
    status: 'ready' | 'missing'
  }
  conditionDraft: FirstAutomationRecommendedConditionDraft
  actionDraft: FirstAutomationRecommendedActionDraft
  cards: FirstAutomationTelemetryRecommendationCard[]
}

export interface FirstAutomationRecommendedCondition {
  ifType: '1'
  trigger_conditions_type: '10' | '11'
  trigger_source: string
  trigger_param_type: 'telemetry'
  trigger_param: string
  trigger_param_key: string
  trigger_operator: '>' | '='
  trigger_value: string
  minValue: null
  maxValue: null
  triggerParamOptions: any[]
}

export interface FirstAutomationRecommendedConditionDraft {
  available: boolean
  title: string
  desc: string
  status: 'ready' | 'missing'
  condition: FirstAutomationRecommendedCondition | null
}

export interface FirstAutomationRecommendedAction {
  actionType: '30'
  action_type: '30'
  action_target: null
}

export interface FirstAutomationRecommendedActionDraft {
  title: string
  desc: string
  status: 'needs-target'
  action: FirstAutomationRecommendedAction
}

const isNumericTelemetryValue = (value: string) => value.trim() !== '' && Number.isFinite(Number(value))

export const buildFirstAutomationRecommendedActionDraft = (
  texts: Pick<FirstAutomationTelemetryRecommendationTexts, 'actionDraftTitle' | 'actionDraftDesc'>
): FirstAutomationRecommendedActionDraft => ({
  title: texts.actionDraftTitle,
  desc: texts.actionDraftDesc,
  status: 'needs-target',
  action: {
    actionType: '30',
    action_type: '30',
    action_target: null
  }
})

export const buildFirstAutomationRecommendedConditionDraft = (
  starter: FirstAutomationTelemetryStarter,
  texts: Pick<
    FirstAutomationTelemetryRecommendationTexts,
    | 'conditionDraftTitle'
    | 'conditionDraftWithValue'
    | 'conditionDraftWithoutValue'
    | 'conditionDraftMissing'
    | 'sourceDevice'
    | 'sourceTemplate'
  >
): FirstAutomationRecommendedConditionDraft => {
  const telemetryKey = starter.telemetryKey?.trim() || ''
  const telemetryValue = starter.telemetryValue?.trim() || ''
  const deviceId = starter.deviceId?.trim() || ''
  const deviceConfigId = starter.deviceConfigId?.trim() || ''
  const sourceType = deviceId ? '10' : deviceConfigId ? '11' : ''
  const sourceId = deviceId || deviceConfigId

  if (!telemetryKey || !sourceType || !sourceId) {
    return {
      available: false,
      title: texts.conditionDraftTitle,
      desc: texts.conditionDraftMissing,
      status: 'missing',
      condition: null
    }
  }

  const operator = isNumericTelemetryValue(telemetryValue) ? '>' : '='
  const sourceLabel =
    sourceType === '10'
      ? texts.sourceDevice.replace('{id}', sourceId)
      : texts.sourceTemplate.replace('{id}', sourceId)
  const desc = telemetryValue
    ? texts.conditionDraftWithValue
        .replace('{source}', sourceLabel)
        .replace('{telemetry}', telemetryKey)
        .replace('{operator}', operator)
        .replace('{value}', telemetryValue)
    : texts.conditionDraftWithoutValue.replace('{source}', sourceLabel).replace('{telemetry}', telemetryKey)

  return {
    available: true,
    title: texts.conditionDraftTitle,
    desc,
    status: 'ready',
    condition: {
      ifType: '1',
      trigger_conditions_type: sourceType,
      trigger_source: sourceId,
      trigger_param_type: 'telemetry',
      trigger_param: telemetryKey,
      trigger_param_key: `telemetry/${telemetryKey}`,
      trigger_operator: operator,
      trigger_value: telemetryValue,
      minValue: null,
      maxValue: null,
      triggerParamOptions: []
    }
  }
}

export const buildFirstAutomationTelemetryRecommendation = (
  starter: FirstAutomationTelemetryStarter,
  texts: FirstAutomationTelemetryRecommendationTexts
): FirstAutomationTelemetryRecommendation => {
  const telemetryKey = starter.telemetryKey?.trim() || ''
  const telemetryValue = starter.telemetryValue?.trim() || ''
  const telemetryAt = starter.telemetryAt?.trim() || ''
  const deviceId = starter.deviceId?.trim() || ''
  const deviceConfigId = starter.deviceConfigId?.trim() || ''
  const sourceValue = deviceId
    ? texts.sourceDevice.replace('{id}', deviceId)
    : deviceConfigId
      ? texts.sourceTemplate.replace('{id}', deviceConfigId)
      : texts.sourceFallback

  return {
    hasTelemetryKey: Boolean(telemetryKey),
    conditionHint: texts.conditionHint,
    nextAction: {
      title: texts.nextActionTitle,
      desc: telemetryKey ? texts.nextActionWithKey.replace('{telemetry}', telemetryKey) : texts.nextActionWithoutKey,
      status: telemetryKey ? 'ready' : 'missing'
    },
    conditionDraft: buildFirstAutomationRecommendedConditionDraft(starter, texts),
    actionDraft: buildFirstAutomationRecommendedActionDraft(texts),
    cards: [
      {
        key: 'key',
        title: texts.keyTitle,
        value: telemetryKey || texts.keyFallback,
        status: telemetryKey ? 'ready' : 'missing'
      },
      {
        key: 'value',
        title: texts.valueTitle,
        value: telemetryValue || texts.valueFallback,
        status: telemetryValue ? 'ready' : 'missing'
      },
      {
        key: 'time',
        title: texts.timeTitle,
        value: telemetryAt || texts.timeFallback,
        status: telemetryAt ? 'ready' : 'missing'
      },
      {
        key: 'source',
        title: texts.sourceTitle,
        value: sourceValue,
        status: deviceId || deviceConfigId ? 'ready' : 'missing'
      }
    ]
  }
}
