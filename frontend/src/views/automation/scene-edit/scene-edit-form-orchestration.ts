import {
  buildActionsPayload,
  formatActionGroupsForEcho,
  type SceneActionGroupLike,
  type SceneInstructionLike
} from './scene-action-mappers'

export type SceneConfigFormLike<TActionGroup extends SceneActionGroupLike = SceneActionGroupLike> = {
  id: string
  name: string
  description: string
  actions: TActionGroup[]
}

const deepClone = <T>(value: T): T => JSON.parse(JSON.stringify(value)) as T

export const cloneSceneConfigForm = <TActionGroup extends SceneActionGroupLike>(
  form: SceneConfigFormLike<TActionGroup>
): SceneConfigFormLike<TActionGroup> => deepClone(form)

export const buildSceneSubmitPayload = <TActionGroup extends SceneActionGroupLike>(
  form: SceneConfigFormLike<TActionGroup>
): SceneConfigFormLike<TActionGroup> => {
  const clonedForm = cloneSceneConfigForm(form)
  clonedForm.actions = buildActionsPayload(clonedForm.actions) as TActionGroup[]
  return clonedForm
}

export const duplicateSceneActionGroup = <TActionGroup extends SceneActionGroupLike>(
  actionGroup: TActionGroup
): TActionGroup => {
  return deepClone(actionGroup)
}

export const formatSceneActionsForEdit = <TActionGroup extends SceneActionGroupLike>(
  actions: SceneInstructionLike[]
): TActionGroup[] => {
  return formatActionGroupsForEcho(actions) as TActionGroup[]
}
