/**
 * 设备条件来源状态 helper。
 * - 负责单设备 / 单类设备的来源切换、设备分组与配置查询、输入聚焦和下拉交互状态。
 * - 这里只维护设备来源相关状态；触发参数 option 仍由 trigger-param 相关 helper 协调。
 */
import { computed, onBeforeUpdate, ref } from 'vue'

/** 触发条件行（编辑器表单数据，字段宽松） */
type PremiseIfItemLike = {
  trigger_source?: unknown
  trigger_conditions_type?: unknown
  group_id?: unknown
  device_name?: unknown
  [key: string]: unknown
}

/** 设备分组树响应行（后端返回，仅读取 group 字段） */
type DeviceGroupTreeRow = { group?: unknown }

type TriggerSelectionResetter = (ifItem: PremiseIfItemLike) => void

type CreatePremiseDeviceConditionStateOptions = {
  t: (key: string) => string
  resetTriggerSelection: TriggerSelectionResetter
  deviceGroupTreeRequest: (params: Record<string, unknown>) => Promise<{ data?: DeviceGroupTreeRow[] | null }>
  deviceListRequest: (params: Record<string, unknown>) => Promise<{ data?: unknown[] | null }>
  deviceConfigRequest: (params: Record<string, unknown>) => Promise<{ data?: unknown[] | null }>
  emitConditionChose: (value: unknown) => void
}

export const createPremiseDeviceConditionState = ({
  t,
  resetTriggerSelection,
  deviceGroupTreeRequest,
  deviceListRequest,
  deviceConfigRequest,
  emitConditionChose
}: CreatePremiseDeviceConditionStateOptions) => {
  const deviceConditionOptions = computed(() => [
    {
      label: t('common.singleDevice'),
      value: '10'
    },
    {
      label: t('common.singleClassDevice'),
      value: '11'
    }
  ])

  const deviceConfigDisabled = ref(false)

  const triggerConditionsTypeChange = (ifItem: PremiseIfItemLike, data: unknown) => {
    ifItem.trigger_source = null
    resetTriggerSelection(ifItem)
    deviceConfigDisabled.value = data === '11'
    emitConditionChose(data)
  }

  const deviceGroupOptions = ref<any[]>([])
  const groupLoaded = ref(false)
  const deviceLoaded = ref(false)
  const deviceConfigLoaded = ref(false)
  let groupLoadPromise: Promise<void> | null = null
  let deviceLoadPromise: Promise<void> | null = null
  let deviceConfigLoadPromise: Promise<void> | null = null

  const getGroup = async () => {
    deviceGroupOptions.value = []
    const res = await deviceGroupTreeRequest({})
    res.data?.forEach(item => {
      deviceGroupOptions.value.push(item.group)
    })
    groupLoaded.value = true
  }

  const deviceOptions = ref<any[]>([])
  const queryDevice = ref({
    group_id: null as string | null,
    device_name: null as string | null,
    bind_config: 0
  })
  const btnloading = ref(false)

  const selectInstRef = ref<Record<number, boolean>>({})
  const onKeydownEnter = (e: Event) => {
    e.preventDefault()
    return false
  }

  const onDeviceKeydownEnter = (e: Event, ifIndex: number) => {
    selectInstRef.value[ifIndex] = true
    e.preventDefault()
    return false
  }

  const getDevice = async (groupId: any, name: any) => {
    queryDevice.value.group_id = groupId || null
    queryDevice.value.device_name = name || null
    btnloading.value = false
    deviceOptions.value = []
    if (!groupLoaded.value) {
      await ensureGroupsLoaded()
    }
    const res = await deviceListRequest(queryDevice.value)
    btnloading.value = true
    deviceOptions.value = res.data || []
    deviceLoaded.value = true
  }

  const triggerSourceChange = (ifItem: PremiseIfItemLike, ifIndex: number) => {
    resetTriggerSelection(ifItem)
    selectInstRef.value[ifIndex] = false
  }

  const queryDeviceName = ref<Record<number, any>>({})

  onBeforeUpdate(() => {
    queryDeviceName.value = {}
  })

  const setQueryDeviceNameRef = (el: any, index: number) => {
    if (el) {
      queryDeviceName.value[index] = el
    }
  }

  const handleFocus = (ifIndex: number) => {
    if (queryDeviceName.value[ifIndex]) {
      queryDeviceName.value[ifIndex].focus()
    } else {
      console.error(`Missing queryDeviceName ref for index ${ifIndex}`)
    }
  }

  const onTapInput = (item: PremiseIfItemLike, ifIndex: number) => {
    if (item.group_id || item.device_name) {
      getDevice(item.group_id, item.device_name)
    } else {
      selectInstRef.value[ifIndex] = true
    }
  }

  const deviceConfigOption = ref<any[]>([])
  const queryDeviceConfig = ref({
    device_config_name: ''
  })

  const getDeviceConfig = async (name: string) => {
    queryDeviceConfig.value.device_config_name = name || ''
    const res = await deviceConfigRequest(queryDeviceConfig.value)
    deviceConfigOption.value = res.data || []
    deviceConfigLoaded.value = true
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

  return {
    btnloading,
    deviceConditionOptions,
    deviceConfigDisabled,
    deviceConfigOption,
    deviceGroupOptions,
    deviceOptions,
    ensureDeviceConfigsLoaded,
    ensureDevicesLoaded,
    ensureGroupsLoaded,
    getDevice,
    getDeviceConfig,
    getGroup,
    handleFocus,
    onDeviceKeydownEnter,
    onKeydownEnter,
    onTapInput,
    queryDevice,
    queryDeviceConfig,
    selectInstRef,
    setQueryDeviceNameRef,
    triggerConditionsTypeChange,
    triggerSourceChange
  }
}
