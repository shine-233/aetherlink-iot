import {
  getInteractionActionType,
  isExternalInteractionUrl,
  isInternalInteractionUrl
} from './interactionResponseProtocol'

type Translate = (key: string) => string

function operatorLabel(operator: string, t: Translate) {
  const operatorMap: Record<string, string> = {
    equals: t('interaction.operators.equals'),
    notEquals: t('interaction.operators.notEquals'),
    greaterThan: t('interaction.operators.greaterThan'),
    greaterThanOrEqual: t('interaction.operators.greaterThanOrEqual'),
    lessThan: t('interaction.operators.lessThan'),
    lessThanOrEqual: t('interaction.operators.lessThanOrEqual'),
    contains: t('interaction.operators.contains'),
    startsWith: t('interaction.operators.startsWith'),
    endsWith: t('interaction.operators.endsWith')
  }
  return operatorMap[operator] || operator
}

function eventLabel(event: string, t: Translate) {
  const labelMap: Record<string, string> = {
    click: t('interaction.events.click'),
    hover: t('interaction.events.hover'),
    dataChange: t('interaction.events.dataChange')
  }
  return labelMap[event] || event
}

function conditionSummary(condition: any, t: Translate) {
  if (!condition) return t('interaction.empty.noCondition')

  const conditionType = condition.type
  const value = condition.value
  if (conditionType === 'comparison') {
    return `${operatorLabel(condition.operator, t)} ${value}`
  }
  if (conditionType === 'range') {
    return `${t('interaction.summary.range')} ${value}`
  }
  if (conditionType === 'expression') {
    return `${t('interaction.summary.expression')} ${value}`
  }
  return String(value || '')
}

function jumpResponseSummary(url: string, t: Translate) {
  if (isExternalInteractionUrl(url)) return t('interaction.summary.jumpToExternal')
  if (isInternalInteractionUrl(url)) return t('interaction.summary.jumpToInternal')
  return `${t('interaction.summary.jumpTo')} ${url}`
}

export function interactionSummaryTitle(interaction: any, t: Translate) {
  const actionType = getInteractionActionType(interaction)
  if (actionType === 'jump') return t('interaction.summary.pageJump')
  if (actionType === 'modify') return t('interaction.summary.modifyProperty')
  return t('interaction.summary.customAction')
}

export function interactionSummaryDesc(interaction: any, t: Translate) {
  const event = eventLabel(interaction.event, t)
  const actionType = getInteractionActionType(interaction)
  const response = interaction.responses?.[0] || {}

  if (interaction.event === 'dataChange') {
    const watchedProperty = interaction.watchedProperty || t('interaction.empty.notSpecified')
    let baseDesc = `${t('interaction.summary.listening')} ${watchedProperty} (${conditionSummary(
      interaction.condition,
      t
    )})`

    if (actionType === 'jump') {
      baseDesc += ` -> ${jumpResponseSummary(response.value || '', t)}`
    } else if (actionType === 'modify') {
      const target = response.targetComponentId || t('interaction.empty.component')
      const property = response.targetProperty || t('interaction.empty.property')
      baseDesc += ` -> ${t('interaction.summary.modify')}${target}.${property}`
    }

    return baseDesc
  }

  if (actionType === 'jump') {
    const url = response.value || ''
    if (isExternalInteractionUrl(url)) return `${event}${t('interaction.summary.whenClick')}: ${url}`
    if (isInternalInteractionUrl(url)) return `${event}${t('interaction.summary.whenHover')}: ${url}`
    return `${event}${t('interaction.summary.whenEvent')} ${url}`
  }

  if (actionType === 'modify') {
    const target = response.targetComponentId || t('interaction.empty.component')
    const property = response.targetProperty || t('interaction.empty.property')
    return `${event}${t('interaction.summary.whenEventModify')}${target}.${property}`
  }

  return `${event}${t('interaction.summary.whenEventCustom')}`
}
