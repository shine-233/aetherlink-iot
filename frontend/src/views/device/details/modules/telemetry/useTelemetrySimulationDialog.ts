import { reactive, ref, watch } from 'vue'
import {
  applyTelemetrySimulationSubmitError,
  applyTelemetrySimulationSubmitSuccess,
  clearSimulationPayload,
  closeTelemetrySimulationDialog,
  formatSimulationJson,
  openTelemetrySimulationDialog,
  toggleTelemetryAdvanced
} from './telemetryDialogState'
import {
  applySimulationInitData,
  applySimulationPayloadDefaults,
  buildSimulationPayload,
  createTelemetrySimulationFormState,
  extractSimulationErrorMessage,
  rememberSimulationPayloadByTopic,
  syncSimulationPayloadByTopic
} from './telemetrySimulationState'

type UseTelemetrySimulationDialogOptions = {
  getDeviceId: () => string
  getSimulationInitRequest: (params: { device_id: string }) => Promise<any>
  sendSimulationDataRequest: (payload: Record<string, any>) => Promise<any>
  translate: (key: string) => string
  isJSON: (value: string) => boolean
}

export const useTelemetrySimulationDialog = ({
  getDeviceId,
  getSimulationInitRequest,
  sendSimulationDataRequest,
  translate,
  isJSON
}: UseTelemetrySimulationDialogOptions) => {
  const showLogDialog = ref(false)
  const showError = ref(false)
  const erroMessage = ref('')
  const showAdvanced = ref(false)
  const simulationLoading = ref(false)
  const simulationForm = reactive(createTelemetrySimulationFormState())

  const requestSimulationInit = async () => {
    const { data, error } = await getSimulationInitRequest({
      device_id: getDeviceId()
    })
    if (!error && data) {
      applySimulationInitData(simulationForm, data)
    } else {
      applySimulationPayloadDefaults(simulationForm)
    }
  }

  const openUpLog = () => {
    openTelemetrySimulationDialog(showError, showLogDialog, showAdvanced)
    void requestSimulationInit()
  }

  const handleSimulationSubmitSuccess = () => {
    applyTelemetrySimulationSubmitSuccess(showLogDialog, showError)
    window.$message?.success(translate('custom.devicePage.success'))
  }

  const handleSimulationSubmitError = (error: any) => {
    applyTelemetrySimulationSubmitError(showError, erroMessage, extractSimulationErrorMessage(error))
  }

  const sendSimulationDataByForm = async () => {
    if (!simulationForm.default_data) {
      window.$message?.error(translate('custom.device_details.sendInputData'))
      return
    }

    simulationLoading.value = true
    try {
      const { error } = await sendSimulationDataRequest(buildSimulationPayload(simulationForm, getDeviceId()))
      if (!error) {
        handleSimulationSubmitSuccess()
      } else {
        handleSimulationSubmitError(error)
      }
    } catch (error) {
      handleSimulationSubmitError(error)
    } finally {
      simulationLoading.value = false
    }
  }

  const copySimulationData = async () => {
    if (!simulationForm.default_data) return
    try {
      await navigator.clipboard.writeText(simulationForm.default_data)
      window.$message?.success(translate('theme.configOperation.copySuccess'))
    } catch {
      window.$message?.error(translate('common.copyFailed'))
    }
  }

  const formatSimulationData = () => {
    if (!simulationForm.default_data) return
    const nextValue = formatSimulationJson(simulationForm.default_data, isJSON)
    if (!nextValue) {
      window.$message?.warning(translate('custom.device_details.notJsonNoFormat'))
      return
    }
    simulationForm.default_data = nextValue
  }

  const clearSimulationData = () => {
    clearSimulationPayload(simulationForm)
  }

  const toggleAdvanced = () => {
    toggleTelemetryAdvanced(showAdvanced)
  }

  const closeSimulationDialog = () => {
    closeTelemetrySimulationDialog(showLogDialog)
  }

  watch(
    () => simulationForm.topic,
    (val) => {
      syncSimulationPayloadByTopic(simulationForm, val)
    }
  )

  watch(
    () => simulationForm.default_data,
    (value) => {
      rememberSimulationPayloadByTopic(simulationForm, simulationForm.topic, value)
    }
  )

  return {
    closeSimulationDialog,
    clearSimulationData,
    copySimulationData,
    erroMessage,
    formatSimulationData,
    openUpLog,
    requestSimulationInit,
    sendSimulationDataByForm,
    showAdvanced,
    showError,
    showLogDialog,
    simulationForm,
    simulationLoading,
    toggleAdvanced
  }
}
