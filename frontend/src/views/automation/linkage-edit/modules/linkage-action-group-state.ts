import { ref } from 'vue'
import { $t } from '@/locales'

/** 动作指令行（编辑器数据，字段宽松） */
type ActionInstructItem = {
  action_target?: unknown
  action_type?: unknown
  action_param_type?: unknown
  action_param?: unknown
  actionValue?: unknown
  deviceGroupId?: unknown
  actionParamOptions?: unknown[]
  actionParamOptionsData?: unknown[]
  actionParamTypeOptions?: unknown[]
  [key: string]: unknown
}

/** 动作组行（编辑器数据，字段宽松） */
type ActionGroupItem = {
  actionType?: unknown
  action_type?: unknown
  action_target?: unknown
  actionInstructList?: ActionInstructItem[]
  [key: string]: unknown
}

interface LinkageActionGroupStateOptions {
  configFormRef: any
  getConditionsType: () => unknown
  hydrateActionParam: (instructItem: ActionInstructItem) => void | Promise<void>
  preloadDevices: () => void | Promise<void>
  preloadDeviceConfigs: () => void | Promise<void>
}

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value))

const createActionInstructItem = (): ActionInstructItem => ({
  action_target: null,
  action_type: null,
  action_param_type: null,
  action_param: null,
  actionValue: null,
  deviceGroupId: null,
  actionParamOptions: [],
  actionParamOptionsData: [],
  actionParamTypeOptions: []
})

const createActionItem = (): ActionGroupItem => ({
  actionType: '30',
  action_type: null,
  action_target: null,
  actionInstructList: []
})

export const useLinkageActionGroupState = ({
  configFormRef,
  getConditionsType,
  hydrateActionParam,
  preloadDevices,
  preloadDeviceConfigs
}: LinkageActionGroupStateOptions) => {
  const actionForm = ref<any>({
    actionGroups: []
  })

  const actionOptions = ref<any[]>([
    {
      label: $t('common.operateDevice'),
      value: '1',
      disabled: false
    },
    {
      label: $t('common.activateScene'),
      value: '20'
    },
    {
      label: $t('common.triggerAlarm'),
      value: '30'
    }
    // {
    //   label: $t('common.triggerService'),
    //   value: '40'
    // }
  ])

  const actionTypeOptions = ref<any[]>([
    {
      label: $t('common.singleDevice'),
      value: '10'
    },
    {
      label: $t('common.singleClassDevice'),
      value: '11'
    }
  ])

  const resetActionData = () => {
    actionForm.value.actionGroups.forEach(item => {
      if (item.actionInstructList && item.actionInstructList.length > 0) {
        const instructItem = createActionInstructItem()
        instructItem.action_type = getConditionsType()
        item.actionInstructList = [instructItem]
      }
    })
  }

  const applyActionData = (actionData: unknown) => {
    if (!Array.isArray(actionData)) {
      return
    }
    actionForm.value.actionGroups = clone(actionData)
    actionForm.value.actionGroups.forEach(item => {
      if (item.actionType === '1' && Array.isArray(item.actionInstructList)) {
        item.actionInstructList.forEach((instructItem) => {
          // Keep echoed options when the catalog request is unavailable; a later
          // successful hydration still replaces them with the current catalog.
          if (!Array.isArray(instructItem.actionParamOptions)) {
            instructItem.actionParamOptions = []
          }
          if (!Array.isArray(instructItem.actionParamOptionsData)) {
            instructItem.actionParamOptionsData = []
          }
          if (!Array.isArray(instructItem.actionParamTypeOptions)) {
            instructItem.actionParamTypeOptions = []
          }
          hydrateActionParam(instructItem)
        })
      }
    })
  }

  const addIfGroupsSubItem = async (actionGroupIndex: number) => {
    await configFormRef.value?.validate()
    const data = createActionInstructItem()
    if (getConditionsType() === '11') {
      data.action_type = '11'
    }
    actionForm.value.actionGroups[actionGroupIndex].actionInstructList.push(data)
  }

  const actionChange = (actionGroupItem: { actionInstructList?: unknown; action_type?: unknown; action_target?: unknown }, actionGroupIndex: number, data: unknown) => {
    actionOptions.value.forEach((item) => {
      item.disabled = false
    })
    actionGroupItem.actionInstructList = [createActionInstructItem()]
    actionGroupItem.action_type = null
    actionGroupItem.action_target = null

    if (data === '1') {
      void addIfGroupsSubItem(actionGroupIndex)
    }
  }

  const actionTypeChange = (instructItem: ActionInstructItem, data: unknown) => {
    instructItem.action_target = null
    instructItem.action_param_type = null
    instructItem.action_param = null
    instructItem.actionValue = null

    if (data === '10') {
      preloadDevices()
    } else if (data === '11') {
      preloadDeviceConfigs()
    }
  }

  const addActionGroupItem = async () => {
    await configFormRef.value?.validate()
    actionForm.value.actionGroups.push(createActionItem())
  }

  const addAlarmActionSlot = () => {
    actionForm.value.actionGroups.push(createActionItem())
  }

  const deleteActionGroupItem = (actionGroupIndex: number) => {
    actionForm.value.actionGroups.splice(actionGroupIndex, 1)
  }

  const deleteIfGroupsSubItem = (actionGroupIndex: number, ifIndex: number) => {
    actionForm.value.actionGroups[actionGroupIndex].actionInstructList.splice(ifIndex, 1)
  }

  return {
    actionForm,
    actionOptions,
    actionTypeOptions,
    resetActionData,
    applyActionData,
    actionChange,
    actionTypeChange,
    addActionGroupItem,
    addAlarmActionSlot,
    deleteActionGroupItem,
    addIfGroupsSubItem,
    deleteIfGroupsSubItem
  }
}
