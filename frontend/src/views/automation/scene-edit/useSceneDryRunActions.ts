/**
 * 文件用途：场景编辑页的试运行快速修复与保存门禁。
 * 核心逻辑：
 *   - 快速修复：新增动作组 / 首组切到「操作设备」/ 首组追加设备指令，执行后在下一 tick 刷新本地执行解释；
 *   - 保存门禁：先本地阻断检查，再跑后端试运行（后端不可用或结果阻断则禁止保存）。
 */
import { computed, nextTick, ref, type Ref } from 'vue'
import { $t } from '@/locales'
import type { AutomationDryRunQuickFixAction } from '../linkage-edit/modules/automationDryRunPreview'
import { runAutomationDryRunSaveGate } from '../linkage-edit/modules/automationSaveFlow'
import { OPERATE_DEVICE_ACTION_TYPE, type SceneActionGroupLike } from './scene-action-mappers'
import { createEmptySceneActionGroup, createEmptySceneInstruction } from './scene-action-form-factories'
import {
  SCENE_DRY_RUN_QUICK_FIX_KEYS,
  buildSceneDryRunQuickFixActions,
  getSceneActionLocalBlocker
} from './scene-dry-run-preview'

export interface UseSceneDryRunActionsOptions {
  actions: Ref<SceneActionGroupLike[]>
  message: { success: (text: string) => void; error: (text: string) => void }
  buildPayload: () => any
  refreshLocalExecutionExplanation: () => unknown
  runBackendDryRunForPayload: (payload: any) => Promise<any>
  onActionTypeChange: (group: SceneActionGroupLike, index: number, actionType: string | null) => void
}

export function useSceneDryRunActions(options: UseSceneDryRunActionsOptions) {
  const { actions, message } = options
  const isSaveDryRunLoading = ref(false)

  const sceneDryRunQuickFixActions = computed<AutomationDryRunQuickFixAction[]>(() =>
    buildSceneDryRunQuickFixActions({
      actionGroups: actions.value,
      texts: {
        addActionGroupTitle: $t('generate.sceneDryRunQuickFixAddActionGroupTitle'),
        addActionGroupDesc: $t('generate.sceneDryRunQuickFixAddActionGroupDesc'),
        addActionGroupButton: $t('generate.sceneDryRunQuickFixAddActionGroupButton'),
        selectOperateDeviceTitle: $t('generate.sceneDryRunQuickFixSelectOperateDeviceTitle'),
        selectOperateDeviceDesc: $t('generate.sceneDryRunQuickFixSelectOperateDeviceDesc'),
        selectOperateDeviceButton: $t('generate.sceneDryRunQuickFixSelectOperateDeviceButton'),
        addDeviceInstructionTitle: $t('generate.sceneDryRunQuickFixAddDeviceInstructionTitle'),
        addDeviceInstructionDesc: $t('generate.sceneDryRunQuickFixAddDeviceInstructionDesc'),
        addDeviceInstructionButton: $t('generate.sceneDryRunQuickFixAddDeviceInstructionButton')
      }
    })
  )

  const applied = (messageKey: string) => {
    message.success($t(messageKey as any))
    void nextTick(() => options.refreshLocalExecutionExplanation())
  }

  const firstGroupOrCreate = () => {
    if (!actions.value[0]) actions.value.push(createEmptySceneActionGroup() as SceneActionGroupLike)
    return actions.value[0]
  }

  const quickFixes: Record<string, () => void> = {
    [SCENE_DRY_RUN_QUICK_FIX_KEYS.addActionGroup]: () => {
      actions.value.push(createEmptySceneActionGroup() as SceneActionGroupLike)
      applied('generate.sceneDryRunQuickFixAddActionGroupAdded')
    },
    [SCENE_DRY_RUN_QUICK_FIX_KEYS.selectOperateDevice]: () => {
      const group = firstGroupOrCreate()
      group.actionType = OPERATE_DEVICE_ACTION_TYPE
      options.onActionTypeChange(group, 0, OPERATE_DEVICE_ACTION_TYPE)
      applied('generate.sceneDryRunQuickFixSelectOperateDeviceApplied')
    },
    [SCENE_DRY_RUN_QUICK_FIX_KEYS.addDeviceInstruction]: () => {
      const group = actions.value[0]
      if (!group) {
        quickFixes[SCENE_DRY_RUN_QUICK_FIX_KEYS.addActionGroup]()
        return
      }
      group.actionInstructList.push(createEmptySceneInstruction())
      applied('generate.sceneDryRunQuickFixAddDeviceInstructionAdded')
    }
  }

  const handleSceneDryRunQuickFix = (key: string) => quickFixes[key]?.()

  const ensureSceneDryRunCanSave = async () => {
    const payload = options.buildPayload()
    const localBlocker = getSceneActionLocalBlocker(payload, $t)
    if (localBlocker) {
      message.error(localBlocker)
      return false
    }
    isSaveDryRunLoading.value = true
    try {
      const result = await runAutomationDryRunSaveGate({
        payload,
        runBackendDryRunForPayload: options.runBackendDryRunForPayload,
        backendUnavailableMessage: $t('generate.automationDryRunBackendUnavailable'),
        saveBlockedMessage: $t('generate.automationDryRunSaveBlocked')
      })
      if (!result.canSave) message.error(result.message)
      return result.canSave
    } finally {
      isSaveDryRunLoading.value = false
    }
  }

  return { isSaveDryRunLoading, sceneDryRunQuickFixActions, handleSceneDryRunQuickFix, ensureSceneDryRunCanSave }
}
