import { ref } from 'vue'
import { runAutomationDryRunSaveGate } from './automationSaveFlow'

type DryRunService = (payload: any) => Promise<any>
type Translate = (key: string) => string

export function useAutomationSaveGate(options: {
  runBackendDryRunForPayload: DryRunService
  t: Translate
}) {
  const isSaveDryRunLoading = ref(false)

  const ensureBackendDryRunCanSave = async (payload: any) => {
    isSaveDryRunLoading.value = true
    try {
      const result = await runAutomationDryRunSaveGate({
        payload,
        runBackendDryRunForPayload: options.runBackendDryRunForPayload,
        backendUnavailableMessage: options.t('generate.automationDryRunBackendUnavailable'),
        saveBlockedMessage: options.t('generate.automationDryRunSaveBlocked')
      })
      if (!result.canSave) window.$message?.error(result.message)

      return result.canSave
    } finally {
      isSaveDryRunLoading.value = false
    }
  }

  return {
    ensureBackendDryRunCanSave,
    isSaveDryRunLoading
  }
}