import { computed, ref, watch } from 'vue'
import { debounce } from 'lodash-es'

type CheckDevice = (pid: string) => Promise<{ data?: { is_available?: boolean }; error?: unknown }>
type ActivateDevice = (payload: { pid_number: string }) => Promise<{ error?: unknown }>

type Logger = {
  error: (message: string, context?: Record<string, unknown>) => void
}

type UseDeviceManageActivationFlowOptions = {
  checkDevice: CheckDevice
  activateDevice: ActivateDevice
  logger: Logger
  onActivated?: () => void
}

const RDI_PID_PATTERN = /^[A-Z0-9]{12}$/
const PID_AVAILABLE_COLOR = 'rgb(2,153,52)'
const PID_UNAVAILABLE_COLOR = 'rgb(255, 26, 26)'

export const normalizeDevicePid = (value: unknown) =>
  String(value || '')
    .trim()
    .toUpperCase()

export const isValidDevicePid = (pid: string) => RDI_PID_PATTERN.test(pid)

export function useDeviceManageActivationFlow(options: UseDeviceManageActivationFlowOptions) {
  const deviceNumber = ref('')
  const buttonDisabled = ref(true)
  const showMessage = ref(false)
  const messageColor = ref('')

  const messageStyle = computed(() => ({
    color: messageColor.value,
    marginLeft: '10px',
    marginTop: '5px'
  }))

  const resetActivationState = () => {
    deviceNumber.value = ''
    showMessage.value = false
    buttonDisabled.value = true
  }

  const setPidAvailabilityState = (available: boolean, visible = true) => {
    buttonDisabled.value = !available
    showMessage.value = visible
    messageColor.value = available ? PID_AVAILABLE_COLOR : PID_UNAVAILABLE_COLOR
  }

  const completeAdd = async () => {
    const pid = normalizeDevicePid(deviceNumber.value)
    if (!isValidDevicePid(pid)) {
      setPidAvailabilityState(false)
      return
    }

    const { error } = await options.activateDevice({
      pid_number: pid
    })
    if (!error) {
      resetActivationState()
      options.onActivated?.()
    }
  }

  watch(
    deviceNumber,
    debounce(async (newDeviceNumber) => {
      let pid = ''
      try {
        pid = normalizeDevicePid(newDeviceNumber)
        if (pid !== newDeviceNumber) {
          deviceNumber.value = pid
          return
        }
        if (!pid) {
          showMessage.value = false
          buttonDisabled.value = true
          return
        }
        if (!isValidDevicePid(pid)) {
          setPidAvailabilityState(false)
          return
        }
        const { data, error } = await options.checkDevice(pid)
        setPidAvailabilityState(!error && Boolean(data?.is_available))
      } catch (error) {
        options.logger.error('[DeviceManage] 检查设备PID失败:', {
          pid,
          error: error instanceof Error ? error.message : error
        })
      }
    }, 500)
  )

  return {
    deviceNumber,
    buttonDisabled,
    showMessage,
    messageStyle,
    completeAdd,
    resetActivationState
  }
}
