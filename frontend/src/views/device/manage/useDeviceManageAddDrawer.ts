/*
 * Add-device drawer state: manual/by-number entry mode, the connect-form payload handed to the
 * drawer, and the config option list it renders.
 */
import { ref } from 'vue'
import type { DrawerPlacement, StepsProps } from 'naive-ui'
import { deviceConnectForm } from '@/service/api/device'
import { loadDeviceConfigOptions } from './device-manage-options'

interface AddDrawerOptions {
  /** Navigate away instead of opening the drawer (service-access add mode). */
  openServiceAccess: () => void
  /** Refresh the device table after the drawer closes. */
  onCompleted: () => void
}

export function useDeviceManageAddDrawer({ openServiceAccess, onCompleted }: AddDrawerOptions) {
  const active = ref(false)
  const addDrawerVisited = ref(false)
  const addKey = ref<string | number>()
  const placement = ref<DrawerPlacement>('right')
  const current = ref<number>(1)
  const currentStatus = ref<StepsProps['status']>('process')
  const isSuccess = ref(false)

  const configOptions = ref<any[]>()
  const deviceId = ref()
  const deviceObj = ref()
  const manualDeviceNumber = ref('')
  const configId = ref()
  const formData = ref()

  async function getFormJson(id: string) {
    const res = await deviceConnectForm({ device_id: id })
    formData.value = res.data
  }

  function setUpId(dId: string, cId: string, dobj: string, nextDeviceNumber = '') {
    deviceId.value = dId
    manualDeviceNumber.value = nextDeviceNumber
    configId.value = cId
    deviceObj.value = JSON.parse(dobj)
    getFormJson(dId)
  }

  function activate(place: DrawerPlacement, key: string | number) {
    if (key === 'server') {
      openServiceAccess()
      return
    }
    current.value = 1
    manualDeviceNumber.value = ''
    addDrawerVisited.value = true
    active.value = true
    addKey.value = key
    placement.value = place
  }

  async function loadConfigOptions() {
    configOptions.value = await loadDeviceConfigOptions()
    return configOptions.value
  }

  return {
    active,
    addDrawerVisited,
    addKey,
    placement,
    current,
    currentStatus,
    isSuccess,
    configOptions,
    deviceId,
    deviceObj,
    manualDeviceNumber,
    configId,
    formData,
    setUpId,
    activate,
    loadConfigOptions,
    setIsSuccess: (flag: boolean) => {
      isSuccess.value = flag
    },
    completeHandAdd: onCompleted
  }
}
