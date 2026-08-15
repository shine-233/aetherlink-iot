type NormalizeIfItem = (ifItem: any) => void

type LoadTriggerParamOptions = (ifItem: any) => void | Promise<void>

type InitialConditionProps = {
  deviceId?: string
  deviceConfigId?: string
}

type RefreshableIfGroup = any[] | { ifItems?: any[] }

export const cloneConditionGroups = (conditionData: any[]) => JSON.parse(JSON.stringify(conditionData))

export const prepareEchoedIfGroups = (conditionData: any, normalizeIfItemForEcho: NormalizeIfItem) => {
  if (!conditionData || !Array.isArray(conditionData)) {
    return null
  }

  const ifGroups = cloneConditionGroups(conditionData)
  ifGroups.forEach((ifGroup) => {
    if (Array.isArray(ifGroup)) {
      ifGroup.forEach(normalizeIfItemForEcho)
    }
  })

  return ifGroups
}

export const hydrateEchoedIfGroupsOptions = (
  ifGroups: any[] = [],
  loadTriggerParamOptions: LoadTriggerParamOptions
) => {
  ifGroups.forEach((ifGroup) => {
    if (Array.isArray(ifGroup)) {
      ifGroup.forEach((ifItem) => {
        loadTriggerParamOptions(ifItem)
      })
    }
  })
}

export const createInitialConditionFromProps = (
  judgeItemTemplate: any,
  { deviceId, deviceConfigId }: InitialConditionProps
) => {
  const judgeItemData = JSON.parse(JSON.stringify(judgeItemTemplate))
  let deviceConfigDisabled = false

  if (deviceId) {
    judgeItemData.ifType = '1'
    judgeItemData.trigger_conditions_type = '10'
    judgeItemData.trigger_source = deviceId
  } else if (deviceConfigId) {
    judgeItemData.ifType = '1'
    judgeItemData.trigger_conditions_type = '11'
    judgeItemData.trigger_source = deviceConfigId
    deviceConfigDisabled = true
  }

  return {
    judgeItemData,
    deviceConfigDisabled
  }
}

const getIfItemsForUiRefresh = (ifGroup: RefreshableIfGroup) =>
  Array.isArray(ifGroup) ? ifGroup : ifGroup.ifItems || []

const updateStatusOptionLabel = (ifItem: any, statusOption: any) => {
  if (ifItem.triggerParamOptions && Array.isArray(ifItem.triggerParamOptions)) {
    const statusIndex = ifItem.triggerParamOptions.findIndex((opt: any) => opt.value === 'status')
    if (statusIndex !== -1) {
      ifItem.triggerParamOptions[statusIndex] = statusOption
    }
  }
}

export const refreshStatusOptionsForLocale = (ifGroups: RefreshableIfGroup[] = [], statusOption: any) => {
  ifGroups.forEach((ifGroup) => {
    const ifItems = getIfItemsForUiRefresh(ifGroup)
    ifItems.forEach((ifItem) => {
      updateStatusOptionLabel(ifItem, statusOption)
    })
  })
}
