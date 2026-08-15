import { ref, type Ref } from 'vue'
import {
  buildInteractionDraft,
  createDefaultActionFormState,
  createDefaultInteractionState,
  createEmptyConditionFormState,
  createEmptyTargetBindingFormState,
  readInteractionEditFormState,
  type InteractionFormState,
  type TargetPropertyInfo
} from './interactionFormState'
import type { InteractionActionType, InteractionUrlType } from './interactionResponseProtocol'

type UseInteractionCardWizardDraftOptions = {
  interactions: Ref<any[]>
  emitUpdate: (value: any[]) => void
  loadMenuOptions: () => void | Promise<void>
}

export function useInteractionCardWizardDraft(options: UseInteractionCardWizardDraftOptions) {
  const editingIndex = ref(-1)
  const currentInteraction = ref<InteractionFormState>(createDefaultInteractionState())
  const currentActionType = ref<InteractionActionType>('')

  const urlType = ref<InteractionUrlType>('external')
  const selectedMenuPath = ref('')

  const currentWatchedProperty = ref('')
  const currentConditionType = ref('')
  const currentConditionOperator = ref('')
  const currentConditionValue = ref('')

  const currentTargetPropertyBinding = ref('')
  const currentTargetPropertyInfo = ref<TargetPropertyInfo | null>(null)

  function applyConditionState() {
    const conditionState = createEmptyConditionFormState()
    currentWatchedProperty.value = conditionState.watchedProperty
    currentConditionType.value = conditionState.type
    currentConditionOperator.value = conditionState.operator
    currentConditionValue.value = conditionState.value
  }

  function applyTargetBindingState() {
    const targetBinding = createEmptyTargetBindingFormState()
    currentTargetPropertyBinding.value = targetBinding.bindingPath
    currentTargetPropertyInfo.value = targetBinding.propertyInfo
  }

  function hydrateInteractionEditForm(interaction: any) {
    const editState = readInteractionEditFormState(interaction)
    currentInteraction.value = editState.interaction
    currentWatchedProperty.value = editState.condition.watchedProperty
    currentConditionType.value = editState.condition.type
    currentConditionOperator.value = editState.condition.operator
    currentConditionValue.value = editState.condition.value
    currentTargetPropertyBinding.value = editState.targetBinding.bindingPath
    currentTargetPropertyInfo.value = editState.targetBinding.propertyInfo
    currentActionType.value = editState.action.actionType
    urlType.value = editState.action.urlType
    selectedMenuPath.value = editState.action.selectedMenuPath

    if (editState.action.shouldLoadMenuOptions) {
      void options.loadMenuOptions()
    }
  }

  function beginInteractionEdit(index: number) {
    editingIndex.value = index
    hydrateInteractionEditForm(options.interactions.value[index])
  }

  function commitInteraction(interaction: any) {
    if (editingIndex.value >= 0) {
      options.interactions.value[editingIndex.value] = interaction
      editingIndex.value = -1
    } else {
      options.interactions.value.push(interaction)
    }

    options.emitUpdate(options.interactions.value)
  }

  function resetInteractionSaveForm() {
    currentInteraction.value = createDefaultInteractionState()
    const actionState = createDefaultActionFormState()
    currentActionType.value = actionState.actionType
    urlType.value = actionState.urlType
    selectedMenuPath.value = actionState.selectedMenuPath
    applyConditionState()
    applyTargetBindingState()
  }

  function saveCurrentInteraction() {
    const interaction = buildInteractionDraft({
      interaction: currentInteraction.value,
      condition: {
        watchedProperty: currentWatchedProperty.value,
        type: currentConditionType.value,
        operator: currentConditionOperator.value,
        value: currentConditionValue.value
      },
      targetBinding: {
        bindingPath: currentTargetPropertyBinding.value,
        propertyInfo: currentTargetPropertyInfo.value
      },
      action: {
        actionType: currentActionType.value,
        urlType: urlType.value,
        selectedMenuPath: selectedMenuPath.value,
        shouldLoadMenuOptions: false
      }
    })

    commitInteraction(interaction)
    resetInteractionSaveForm()
  }

  return {
    editingIndex,
    currentInteraction,
    currentActionType,
    urlType,
    selectedMenuPath,
    currentWatchedProperty,
    currentConditionType,
    currentConditionOperator,
    currentConditionValue,
    currentTargetPropertyBinding,
    currentTargetPropertyInfo,
    beginInteractionEdit,
    saveCurrentInteraction
  }
}
