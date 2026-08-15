import { ref } from 'vue'
import { deviceGroupTree } from '@/service/api'
import { warningMessageList } from '@/service/api/alarm'
import { deviceConfigAll, deviceListAll, sceneGet } from '@/service/api/automation'

const pushUniqueLoader = (
  loaders: Array<Promise<void> | void>,
  loaderKeys: Set<string>,
  key: string,
  loader: () => Promise<void> | void
) => {
  if (loaderKeys.has(key)) return
  loaderKeys.add(key)
  loaders.push(loader())
}

export const useLinkageActionTargetOptions = () => {
  const loadingSelect = ref(false)
  const groupLoaded = ref(false)
  const deviceLoaded = ref(false)
  const deviceConfigLoaded = ref(false)
  const sceneLoaded = ref(false)
  const alarmLoaded = ref(false)
  let groupLoadPromise: Promise<void> | null = null
  let deviceLoadPromise: Promise<void> | null = null
  let deviceConfigLoadPromise: Promise<void> | null = null
  let sceneLoadPromise: Promise<void> | null = null
  let alarmLoadPromise: Promise<void> | null = null

  const deviceGroupOptions = ref<any[]>([])
  const getGroup = async () => {
    deviceGroupOptions.value = []
    const res = await deviceGroupTree({})
    res.data.forEach((item: any) => {
      deviceGroupOptions.value.push(item.group)
    })
    groupLoaded.value = true
  }

  const deviceOptions = ref<any[]>([])
  const queryDevice = ref({
    group_id: null,
    device_name: null,
    bind_config: 0
  })

  const getDevice = async (groupId: any, name: any) => {
    queryDevice.value.group_id = groupId || null
    queryDevice.value.device_name = name || null
    if (!groupLoaded.value) {
      await ensureGroupsLoaded()
    }
    const res = await deviceListAll(queryDevice.value)
    deviceOptions.value = res.data
    deviceLoaded.value = true
  }

  const deviceConfigOption = ref<any[]>([])
  const queryDeviceConfig = ref({
    device_config_name: ''
  })

  const getDeviceConfig = async (name: any) => {
    queryDeviceConfig.value.device_config_name = name || ''
    const res = await deviceConfigAll(queryDeviceConfig.value)
    deviceConfigOption.value = res.data || []
    deviceConfigLoaded.value = true
  }

  const sceneList = ref<any[]>([])
  const queryScene = ref({
    page: 1,
    page_size: 10,
    name: ''
  })

  const getSceneList = async (name: string) => {
    queryScene.value.name = name || ''
    loadingSelect.value = true
    try {
      const res = await sceneGet(queryScene.value)
      sceneList.value = res.data.list
      sceneLoaded.value = true
    } finally {
      loadingSelect.value = false
    }
  }

  const alarmList = ref<any[]>([])
  const queryAlarm = ref({
    page: 1,
    page_size: 10,
    name: ''
  })

  const getAlarmList = async (name: string) => {
    queryAlarm.value.name = name || ''
    loadingSelect.value = true
    try {
      const res = await warningMessageList(queryAlarm.value)
      alarmList.value = res.data.list
      alarmLoaded.value = true
    } finally {
      loadingSelect.value = false
    }
  }

  const ensureGroupsLoaded = () => {
    if (groupLoaded.value) return Promise.resolve()
    if (!groupLoadPromise) {
      groupLoadPromise = getGroup().finally(() => {
        groupLoadPromise = null
      })
    }
    return groupLoadPromise
  }
  const ensureDevicesLoaded = () => {
    if (deviceLoaded.value) return Promise.resolve()
    if (!deviceLoadPromise) {
      deviceLoadPromise = getDevice(null, null).finally(() => {
        deviceLoadPromise = null
      })
    }
    return deviceLoadPromise
  }
  const ensureDeviceConfigsLoaded = () => {
    if (deviceConfigLoaded.value) return Promise.resolve()
    if (!deviceConfigLoadPromise) {
      deviceConfigLoadPromise = getDeviceConfig('').finally(() => {
        deviceConfigLoadPromise = null
      })
    }
    return deviceConfigLoadPromise
  }
  const ensureScenesLoaded = () => {
    if (sceneLoaded.value) return Promise.resolve()
    if (!sceneLoadPromise) {
      sceneLoadPromise = getSceneList('').finally(() => {
        sceneLoadPromise = null
      })
    }
    return sceneLoadPromise
  }
  const ensureAlarmsLoaded = () => {
    if (alarmLoaded.value) return Promise.resolve()
    if (!alarmLoadPromise) {
      alarmLoadPromise = getAlarmList('').finally(() => {
        alarmLoadPromise = null
      })
    }
    return alarmLoadPromise
  }

  const hydrateActionTargetCatalogsForEcho = async (actionData: any) => {
    if (!Array.isArray(actionData)) return
    const loaders: Array<Promise<void> | void> = []
    const loaderKeys = new Set<string>()

    actionData.forEach((actionGroup: any) => {
      if (actionGroup.actionType === '1' && Array.isArray(actionGroup.actionInstructList)) {
        actionGroup.actionInstructList.forEach((instructItem: any) => {
          if (instructItem.action_type === '10') {
            pushUniqueLoader(loaders, loaderKeys, 'devices', ensureDevicesLoaded)
          }
          if (instructItem.action_type === '11') {
            pushUniqueLoader(loaders, loaderKeys, 'device-configs', ensureDeviceConfigsLoaded)
          }
        })
      }
      if (actionGroup.actionType === '20') {
        pushUniqueLoader(loaders, loaderKeys, 'scenes', ensureScenesLoaded)
      }
      if (actionGroup.actionType === '30') {
        pushUniqueLoader(loaders, loaderKeys, 'alarms', ensureAlarmsLoaded)
      }
    })

    await Promise.all(loaders)
  }

  return {
    loadingSelect,
    deviceGroupOptions,
    getGroup,
    ensureGroupsLoaded,
    deviceOptions,
    queryDevice,
    getDevice,
    ensureDevicesLoaded,
    deviceConfigOption,
    getDeviceConfig,
    ensureDeviceConfigsLoaded,
    sceneList,
    getSceneList,
    ensureScenesLoaded,
    alarmList,
    getAlarmList,
    ensureAlarmsLoaded,
    hydrateActionTargetCatalogsForEcho
  }
}
