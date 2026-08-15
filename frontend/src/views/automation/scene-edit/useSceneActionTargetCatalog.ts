import { ref } from 'vue'
import { deviceGroupTree } from '@/service/api'
import {
  deviceConfigAll,
  deviceConfigMetricsMenu,
  deviceListAll,
  deviceMetricsMenu
} from '@/service/api/automation'
import {
  applyActionParamOptionsData,
  normalizeActionParamOptionsData,
  resetInstructionSelection,
  resetInstructionTargetDependentState
} from './scene-action-form-state'
import {
  type ActionParamOptionGroup,
  type SceneInstructionLike,
  SINGLE_CLASS_DEVICE_ACTION_TARGET_TYPE,
  SINGLE_DEVICE_ACTION_TARGET_TYPE
} from './scene-action-mappers'

export type SelectOption = {
  id?: string
  name?: string
  label?: string
  value?: string
  disabled?: boolean
  [key: string]: any
}

type DeviceQuery = {
  group_id: string | null
  device_name: string | null
  bind_config: number
}

type DeviceConfigQuery = {
  device_config_name: string
}

export const useSceneActionTargetCatalog = () => {
  const loadingSelect = ref(false)
  const pendingCatalogRequests = ref(0)

  const deviceGroupOptions = ref<SelectOption[]>([])
  const deviceOptions = ref<SelectOption[]>([])
  const deviceConfigOption = ref<SelectOption[]>([])
  const hasLoadedDeviceGroups = ref(false)
  const hasLoadedDevices = ref(false)
  const hasLoadedDeviceConfigs = ref(false)
  let deviceGroupsLoadPromise: Promise<void> | null = null
  let deviceOptionsLoadPromise: Promise<void> | null = null
  let deviceConfigOptionsLoadPromise: Promise<void> | null = null

  const queryDevice = ref<DeviceQuery>({
    group_id: null,
    device_name: null,
    bind_config: 0
  })

  const queryDeviceConfig = ref<DeviceConfigQuery>({
    device_config_name: ''
  })

  const runWithLoading = async <T>(task: () => Promise<T>) => {
    pendingCatalogRequests.value += 1
    loadingSelect.value = true

    try {
      return await task()
    } finally {
      pendingCatalogRequests.value -= 1
      loadingSelect.value = pendingCatalogRequests.value > 0
    }
  }

  const getGroup = async () => {
    await runWithLoading(async () => {
      deviceGroupOptions.value = []
      const res = await deviceGroupTree({})
      deviceGroupOptions.value = (res.data || []).map((item: any) => item.group as SelectOption)
      hasLoadedDeviceGroups.value = true
      return res
    })
  }

  const getDevice = async (groupId: string | null, name: string | null) => {
    await runWithLoading(async () => {
      queryDevice.value.group_id = groupId || null
      queryDevice.value.device_name = name || null
      const res = await deviceListAll(queryDevice.value)
      deviceOptions.value = res.data || []
      hasLoadedDevices.value = true
      return res
    })
  }

  const getDeviceConfig = async (name: string | null) => {
    await runWithLoading(async () => {
      queryDeviceConfig.value.device_config_name = name || ''
      const res = await deviceConfigAll(queryDeviceConfig.value)
      deviceConfigOption.value = res.data || []
      hasLoadedDeviceConfigs.value = true
      return res
    })
  }

  const ensureDeviceGroupsLoaded = () => {
    if (hasLoadedDeviceGroups.value) return
    if (!deviceGroupsLoadPromise) {
      deviceGroupsLoadPromise = getGroup().finally(() => {
        deviceGroupsLoadPromise = null
      })
    }
    void deviceGroupsLoadPromise
  }

  const ensureDeviceOptionsLoaded = () => {
    if (hasLoadedDevices.value) return
    if (!deviceOptionsLoadPromise) {
      deviceOptionsLoadPromise = getDevice(null, null).finally(() => {
        deviceOptionsLoadPromise = null
      })
    }
    void deviceOptionsLoadPromise
  }

  const ensureDeviceConfigOptionsLoaded = () => {
    if (hasLoadedDeviceConfigs.value) return
    if (!deviceConfigOptionsLoadPromise) {
      deviceConfigOptionsLoadPromise = getDeviceConfig('').finally(() => {
        deviceConfigOptionsLoadPromise = null
      })
    }
    void deviceConfigOptionsLoadPromise
  }

  const ensureDeviceTargetCatalogsLoaded = () => {
    ensureDeviceGroupsLoaded()
    ensureDeviceOptionsLoaded()
  }

  const getActionParamMenuResponse = async (instructItem: SceneInstructionLike) => {
    if (instructItem.action_type === SINGLE_DEVICE_ACTION_TARGET_TYPE) {
      return deviceMetricsMenu({ device_id: instructItem.action_target })
    }

    if (instructItem.action_type === SINGLE_CLASS_DEVICE_ACTION_TARGET_TYPE) {
      return deviceConfigMetricsMenu({
        device_config_id: instructItem.action_target
      })
    }

    return null
  }

  const actionParamShow = async (instructItem: SceneInstructionLike) => {
    await runWithLoading(async () => {
      const res = await getActionParamMenuResponse(instructItem)
      if (res?.data) {
        const actionParamOptionsData = normalizeActionParamOptionsData(res.data as ActionParamOptionGroup[])
        applyActionParamOptionsData(instructItem, actionParamOptionsData)
      }
      return res
    })
  }

  const loadActionTargetCatalog = (actionType: string | null) => {
    if (actionType === SINGLE_DEVICE_ACTION_TARGET_TYPE) {
      void Promise.allSettled([getGroup(), getDevice(null, null)])
      return
    }

    if (actionType === SINGLE_CLASS_DEVICE_ACTION_TARGET_TYPE) {
      void getDeviceConfig('')
    }
  }

  const loadActionTargetCatalogsForEcho = (instructions: SceneInstructionLike[]) => {
    const tasks: Array<Promise<void>> = []

    if (instructions.some(item => item.action_type === SINGLE_DEVICE_ACTION_TARGET_TYPE)) {
      tasks.push(getGroup(), getDevice(null, null))
    }

    if (instructions.some(item => item.action_type === SINGLE_CLASS_DEVICE_ACTION_TARGET_TYPE)) {
      tasks.push(getDeviceConfig(''))
    }

    if (tasks.length > 0) {
      void Promise.allSettled(tasks)
    }
  }

  const actionTypeChange = (instructItem: SceneInstructionLike, actionType: string | null) => {
    resetInstructionSelection(instructItem)
    loadActionTargetCatalog(actionType)
  }

  const actionTargetChange = (instructItem: SceneInstructionLike) => {
    resetInstructionTargetDependentState(instructItem)
    void actionParamShow(instructItem)
  }

  const loadInitialSelectData = () => {
    void Promise.allSettled([getGroup(), getDevice(null, null), getDeviceConfig('')])
  }

  return {
    actionParamShow,
    actionTargetChange,
    actionTypeChange,
    deviceConfigOption,
    deviceGroupOptions,
    deviceOptions,
    ensureDeviceConfigOptionsLoaded,
    ensureDeviceGroupsLoaded,
    ensureDeviceOptionsLoaded,
    ensureDeviceTargetCatalogsLoaded,
    getDevice,
    getDeviceConfig,
    getGroup,
    loadActionTargetCatalogsForEcho,
    loadInitialSelectData,
    loadingSelect,
    queryDevice
  }
}
