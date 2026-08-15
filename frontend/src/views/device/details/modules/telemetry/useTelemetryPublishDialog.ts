import { computed, reactive, ref } from 'vue'

import { closeTelemetryPublishDialog, formatSimulationJson, openTelemetryPublishDialog } from './telemetryDialogState'
import {
  buildDirectTelemetryPayload,
  buildExpectedTelemetryPayload,
  hasInvalidJsonInput
} from './telemetryPublishState'

type UseTelemetryPublishDialogOptions = {
  expectMessageAddRequest: (payload: Record<string, any>) => Promise<any>
  telemetryDataPubRequest: (payload: Record<string, any>) => Promise<any>
  getDeviceId: () => string
  isJSON: (value: string) => boolean
  onSubmitSuccess: () => void
  translate: (key: string) => string
}

export const useTelemetryPublishDialog = ({
  expectMessageAddRequest,
  telemetryDataPubRequest,
  getDeviceId,
  isJSON,
  onSubmitSuccess,
  translate
}: UseTelemetryPublishDialogOptions) => {
  const showDialog = ref(false)
  const formValue = ref('')
  const form = reactive({
    expected: false,
    time: null as number | null
  })

  const openDialog = () => {
    openTelemetryPublishDialog(showDialog, formValue, form)
  }

  const closePublishDialog = () => {
    closeTelemetryPublishDialog(showDialog)
  }

  const submitTelemetryMessage = () => {
    if (form.expected) {
      return expectMessageAddRequest(
        buildExpectedTelemetryPayload({
          deviceId: getDeviceId(),
          payload: formValue.value,
          expiryHours: form.time
        })
      )
    }

    return telemetryDataPubRequest(buildDirectTelemetryPayload(getDeviceId(), formValue.value))
  }

  const handlePositiveClick = async () => {
    if (!isJSON(formValue.value)) return

    const result = await submitTelemetryMessage()
    if (result && !result.error) {
      closePublishDialog()
      onSubmitSuccess()
    }
  }

  const validationJson = computed(() => {
    if (hasInvalidJsonInput(formValue.value, isJSON)) {
      return 'error'
    }
    return undefined
  })

  const inputFeedback = computed(() => {
    if (hasInvalidJsonInput(formValue.value, isJSON)) {
      return translate('generate.inputRightJson')
    }
    return ''
  })

  const formatPublishPayload = () => {
    if (!formValue.value) return
    const nextValue = formatSimulationJson(formValue.value, isJSON)
    if (!nextValue) {
      window.$message?.warning(translate('custom.device_details.notJsonNoFormat'))
      return
    }
    formValue.value = nextValue
  }

  const clearPublishPayload = () => {
    formValue.value = ''
  }

  return {
    clearPublishPayload,
    closePublishDialog,
    form,
    formValue,
    formatPublishPayload,
    handlePositiveClick,
    inputFeedback,
    openDialog,
    showDialog,
    validationJson
  }
}
