import type {
  ActionParamOption,
  ActionParamOptionGroup,
  ActionParamTypeOption,
  SceneActionGroupLike,
  SceneInstructionLike
} from './scene-action-mappers'

export const createEmptySceneInstruction = (): SceneInstructionLike => ({
  action_target: null,
  action_type: null,
  action_param_type: null,
  action_param: null,
  action_param_key: null,
  action_value: null,
  deviceGroupId: null,
  actionParamOptions: [] as ActionParamOption[],
  actionParamOptionsData: [] as ActionParamOptionGroup[],
  actionParamTypeOptions: [] as ActionParamTypeOption[]
})

export const createEmptySceneActionGroup = (): SceneActionGroupLike => ({
  actionType: null,
  action_type: null,
  action_target: null,
  actionInstructList: []
})
