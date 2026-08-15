import { OPERATE_DEVICE_ACTION_TYPE, type SceneActionGroupLike } from './scene-action-mappers'

export const SCENE_DRY_RUN_QUICK_FIX_KEYS = {
  addActionGroup: 'scene-add-action-group',
  selectOperateDevice: 'scene-select-operate-device',
  addDeviceInstruction: 'scene-add-device-instruction'
} as const

export interface SceneDryRunPayload {
  id?: string
  name?: string | null
  description?: string | null
  actions: any[]
  __localBuildError?: string
}

export interface SceneDryRunQuickFixTexts {
  addActionGroupTitle: string
  addActionGroupDesc: string
  addActionGroupButton: string
  selectOperateDeviceTitle: string
  selectOperateDeviceDesc: string
  selectOperateDeviceButton: string
  addDeviceInstructionTitle: string
  addDeviceInstructionDesc: string
  addDeviceInstructionButton: string
}

const getErrorMessage = (error: unknown) => {
  return error instanceof Error ? error.message : String(error || 'Unknown form error')
}

export const buildSceneActionDryRunPayload = (options: {
  form: { id?: string; name?: string | null; description?: string | null }
  buildSubmitPayload: () => { actions?: any[] }
}): SceneDryRunPayload => {
  try {
    const submitPayload = options.buildSubmitPayload()

    return {
      id: options.form.id,
      name: options.form.name,
      description: options.form.description,
      actions: submitPayload.actions || []
    }
  } catch (error) {
    return {
      id: options.form.id,
      name: options.form.name,
      description: options.form.description,
      actions: [],
      __localBuildError: getErrorMessage(error)
    }
  }
}

const isBlank = (value: unknown) => value === null || value === undefined || value === ''

const isOperateDeviceAction = (action: any) => action?.action_type === '10' || action?.action_type === '11'

const isSceneOrAlarmAction = (action: any) => action?.action_type === '20' || action?.action_type === '30'

export const getSceneActionLocalBlocker = (
  payload: SceneDryRunPayload,
  t: (key: string, options?: Record<string, any>) => string
) => {
  if (payload.__localBuildError) {
    return `${t('generate.sceneDryRunBuildError')}: ${payload.__localBuildError}`
  }

  if (!payload.actions.length) {
    return t('generate.sceneDryRunNoActionBlocker')
  }

  const incompleteIndex = payload.actions.findIndex((action: any) => {
    if (isSceneOrAlarmAction(action)) return isBlank(action.action_target)
    if (!isOperateDeviceAction(action)) return isBlank(action.action_type) || isBlank(action.action_target)

    return (
      isBlank(action.action_target) ||
      isBlank(action.action_param_type) ||
      isBlank(action.action_param) ||
      isBlank(action.action_value)
    )
  })

  if (incompleteIndex >= 0) {
    return t('generate.sceneDryRunIncompleteActionBlocker', { index: incompleteIndex + 1 })
  }

  return ''
}

export const buildSceneDryRunQuickFixActions = (options: {
  actionGroups: SceneActionGroupLike[]
  texts: SceneDryRunQuickFixTexts
}) => {
  const firstGroup = options.actionGroups[0]

  if (!firstGroup) {
    return [
      {
        key: SCENE_DRY_RUN_QUICK_FIX_KEYS.addActionGroup,
        title: options.texts.addActionGroupTitle,
        desc: options.texts.addActionGroupDesc,
        buttonLabel: options.texts.addActionGroupButton,
        type: 'primary' as const
      }
    ]
  }

  if (!firstGroup.actionType) {
    return [
      {
        key: SCENE_DRY_RUN_QUICK_FIX_KEYS.selectOperateDevice,
        title: options.texts.selectOperateDeviceTitle,
        desc: options.texts.selectOperateDeviceDesc,
        buttonLabel: options.texts.selectOperateDeviceButton,
        type: 'primary' as const
      }
    ]
  }

  if (firstGroup.actionType === OPERATE_DEVICE_ACTION_TYPE && firstGroup.actionInstructList.length === 0) {
    return [
      {
        key: SCENE_DRY_RUN_QUICK_FIX_KEYS.addDeviceInstruction,
        title: options.texts.addDeviceInstructionTitle,
        desc: options.texts.addDeviceInstructionDesc,
        buttonLabel: options.texts.addDeviceInstructionButton,
        type: 'primary' as const
      }
    ]
  }

  return []
}
