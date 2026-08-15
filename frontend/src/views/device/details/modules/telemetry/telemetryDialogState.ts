type ValueRef<T> = { value: T }

type TelemetryPublishFormState = {
  expected: boolean
  time: number | null
}

type TelemetrySimulationPayloadState = {
  default_data: string
}

export const resetTelemetryPublishForm = (formValue: ValueRef<string>, form: TelemetryPublishFormState) => {
  formValue.value = ''
  form.expected = false
  form.time = null
}

export const openTelemetryPublishDialog = (
  showDialog: ValueRef<boolean>,
  formValue: ValueRef<string>,
  form: TelemetryPublishFormState
) => {
  showDialog.value = true
  resetTelemetryPublishForm(formValue, form)
}

export const openTelemetrySimulationDialog = (
  showError: ValueRef<boolean>,
  showLogDialog: ValueRef<boolean>,
  showAdvanced: ValueRef<boolean>
) => {
  showError.value = false
  showLogDialog.value = true
  showAdvanced.value = false
}

export const closeTelemetryPublishDialog = (showDialog: ValueRef<boolean>) => {
  showDialog.value = false
}

export const closeTelemetrySimulationDialog = (showLogDialog: ValueRef<boolean>) => {
  showLogDialog.value = false
}

export const toggleTelemetryAdvanced = (showAdvanced: ValueRef<boolean>) => {
  showAdvanced.value = !showAdvanced.value
}

export const applyTelemetrySimulationSubmitSuccess = (
  showLogDialog: ValueRef<boolean>,
  showError: ValueRef<boolean>
) => {
  showLogDialog.value = false
  showError.value = false
}

export const applyTelemetrySimulationSubmitError = (
  showError: ValueRef<boolean>,
  errorMessage: ValueRef<string>,
  message: string
) => {
  showError.value = true
  errorMessage.value = message
}

export const formatSimulationJson = (value: string, isJSON: (value: string) => boolean) => {
  if (!value || !isJSON(value)) return null
  return JSON.stringify(JSON.parse(value), null, 2)
}

export const clearSimulationPayload = (payloadState: TelemetrySimulationPayloadState) => {
  payloadState.default_data = ''
}
