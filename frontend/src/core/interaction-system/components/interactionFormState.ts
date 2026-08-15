import {
  buildJumpResponse,
  buildModifyResponse,
  readCompatibilityJumpEditState,
  readJumpEditState,
  readModifyEditState,
  type InteractionActionType,
  type InteractionUrlType
} from './interactionResponseProtocol'

export type TargetPropertyInfo = {
  componentId: string
  layer: string
  propertyName: string
}

export type InteractionFormState = {
  event: string
  enabled: boolean
  priority: number
  url: string
  target: string
  targetComponentId: string
  targetProperty: string
  updateValue: string
}

export type InteractionConditionFormState = {
  watchedProperty: string
  type: string
  operator: string
  value: string
}

export type InteractionTargetBindingFormState = {
  bindingPath: string
  propertyInfo: TargetPropertyInfo | null
}

export type InteractionActionFormState = {
  actionType: InteractionActionType
  urlType: InteractionUrlType
  selectedMenuPath: string
  shouldLoadMenuOptions: boolean
}

export type InteractionEditFormState = {
  interaction: InteractionFormState
  condition: InteractionConditionFormState
  targetBinding: InteractionTargetBindingFormState
  action: InteractionActionFormState
}

export type BuildInteractionDraftInput = {
  interaction: InteractionFormState
  condition: InteractionConditionFormState
  targetBinding: InteractionTargetBindingFormState
  action: InteractionActionFormState
}

export function createDefaultInteractionState(overrides: Partial<InteractionFormState> = {}): InteractionFormState {
  return {
    event: 'click',
    enabled: true,
    priority: 1,
    url: '',
    target: '_blank',
    targetComponentId: '',
    targetProperty: '',
    updateValue: '',
    ...overrides
  }
}

export function createEmptyConditionFormState(): InteractionConditionFormState {
  return {
    watchedProperty: '',
    type: '',
    operator: '',
    value: ''
  }
}

export function createEmptyTargetBindingFormState(): InteractionTargetBindingFormState {
  return {
    bindingPath: '',
    propertyInfo: null
  }
}

export function createDefaultActionFormState(
  overrides: Partial<InteractionActionFormState> = {}
): InteractionActionFormState {
  return {
    actionType: '',
    urlType: 'external',
    selectedMenuPath: '',
    shouldLoadMenuOptions: false,
    ...overrides
  }
}

export function createTargetBindingPath(componentId?: string, property?: string) {
  if (!componentId || !property) return ''
  return `${componentId}.${property}`
}

export function readInteractionEditFormState(interaction: any): InteractionEditFormState {
  const editState: InteractionEditFormState = {
    interaction: createDefaultInteractionState({
      event: interaction.event,
      enabled: interaction.enabled,
      priority: interaction.priority
    }),
    condition: readConditionFormState(interaction),
    targetBinding: createEmptyTargetBindingFormState(),
    action: createDefaultActionFormState()
  }

  applyResponseEditState(editState, interaction.responses?.[0])
  return editState
}

function readConditionFormState(interaction: any): InteractionConditionFormState {
  const conditionState = createEmptyConditionFormState()
  if (interaction.event !== 'dataChange') return conditionState

  conditionState.watchedProperty = interaction.watchedProperty || ''
  const condition = interaction.condition
  if (!condition) return conditionState

  conditionState.type = condition.type || ''
  if (condition.type === 'comparison') {
    conditionState.operator = condition.operator || ''
    conditionState.value = condition.value || ''
  } else if (condition.type === 'range' || condition.type === 'expression') {
    conditionState.value = condition.value || ''
  }

  return conditionState
}

function applyResponseEditState(editState: InteractionEditFormState, response: any) {
  if (!response) return

  if (response.action === 'jump') {
    const state = readJumpEditState(response)
    editState.action = createDefaultActionFormState({
      actionType: 'jump',
      urlType: state.urlType,
      selectedMenuPath: state.selectedMenuPath,
      shouldLoadMenuOptions: state.shouldLoadMenuOptions
    })
    editState.interaction.target = state.target
    editState.interaction.url = state.url
    return
  }

  if (response.action === 'navigateToUrl') {
    const state = readCompatibilityJumpEditState(response.value || '', response.target)
    editState.action = createDefaultActionFormState({
      actionType: 'jump',
      urlType: state.urlType,
      selectedMenuPath: state.selectedMenuPath,
      shouldLoadMenuOptions: state.shouldLoadMenuOptions
    })
    editState.interaction.target = state.target
    editState.interaction.url = state.url
    return
  }

  if (response.action === 'modify' || response.action === 'updateComponentData') {
    const state = readModifyEditState(response)
    editState.action = createDefaultActionFormState({
      actionType: 'modify'
    })
    editState.interaction.targetComponentId = state.targetComponentId
    editState.interaction.targetProperty = state.targetProperty
    editState.interaction.updateValue = state.updateValue
    editState.targetBinding.bindingPath = createTargetBindingPath(state.targetComponentId, state.targetProperty)
  }
}

export function buildInteractionDraft(input: BuildInteractionDraftInput) {
  const interaction: any = {
    event: input.interaction.event,
    enabled: input.interaction.enabled,
    priority: input.interaction.priority,
    responses: []
  }

  if (input.interaction.event === 'dataChange') {
    interaction.watchedProperty = input.condition.watchedProperty
    const condition = buildCondition(input.condition)
    if (condition) {
      interaction.condition = condition
    }
  }

  interaction.responses = buildInteractionResponses(input)
  return interaction
}

function buildCondition(conditionState: InteractionConditionFormState) {
  if (!conditionState.type) return null

  const condition: Record<string, any> = {
    type: conditionState.type
  }

  if (conditionState.type === 'comparison') {
    condition.operator = conditionState.operator
    condition.value = conditionState.value
  } else if (conditionState.type === 'range' || conditionState.type === 'expression') {
    condition.value = conditionState.value
  }

  return condition
}

function buildInteractionResponses(input: BuildInteractionDraftInput) {
  if (input.action.actionType === 'jump') {
    return [
      buildJumpResponse({
        urlType: input.action.urlType,
        url: input.interaction.url,
        selectedMenuPath: input.action.selectedMenuPath,
        target: input.interaction.target
      })
    ]
  }

  if (input.action.actionType === 'modify') {
    const { targetComponentId, targetProperty } = resolveModifyTarget(input)
    return [
      buildModifyResponse({
        targetComponentId,
        targetProperty,
        updateValue: input.interaction.updateValue,
        bindingPath: input.targetBinding.bindingPath
      })
    ]
  }

  return []
}

function resolveModifyTarget(input: BuildInteractionDraftInput) {
  const propertyInfo = input.targetBinding.propertyInfo
  if (input.targetBinding.bindingPath && propertyInfo) {
    return {
      targetComponentId: propertyInfo.componentId,
      targetProperty: `${propertyInfo.layer}.${propertyInfo.propertyName}`
    }
  }

  return {
    targetComponentId: input.interaction.targetComponentId,
    targetProperty: input.interaction.targetProperty
  }
}
