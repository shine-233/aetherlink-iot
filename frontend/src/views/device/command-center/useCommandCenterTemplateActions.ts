import type { Ref } from 'vue'
import { writeClipboardText } from '@/utils/clipboard'
import type { FleetCommandJobListItem } from '@/service/api/device'
import {
  serializeCommandTemplatesForExport,
  type CommandCenterCommandTemplateImportResult,
  type CommandCenterSavedCommandTemplate
} from './useCommandCenterCommandTemplates'

type CommandTemplateDraft = {
  identify: string
  value: string
  timeoutSeconds: number
  name?: string
  syncName?: boolean
}

type BuiltInCommandTemplate = {
  identify: string
  value: string
  timeoutSeconds: number
}

type UseCommandCenterTemplateActionsOptions = {
  commandIdentify: Ref<string>
  commandValue: Ref<string>
  timeoutSeconds: Ref<number>
  commandTemplateName: Ref<string>
  saveCommandTemplate: (draft: CommandTemplateDraft) => boolean
  deleteCommandTemplate: (templateId: string) => void
  importCommandTemplates: (raw: string) => CommandCenterCommandTemplateImportResult
  resetCommandJobDraft: () => void
  clearReusedCommandJobDraft: () => void
  t: (key: string) => string
  copyText?: (text: string) => Promise<boolean>
  notifySuccess?: (message: string) => void
  notifyWarning?: (message: string) => void
}

export const useCommandCenterTemplateActions = ({
  commandIdentify,
  commandValue,
  timeoutSeconds,
  commandTemplateName,
  saveCommandTemplate,
  deleteCommandTemplate,
  importCommandTemplates,
  resetCommandJobDraft,
  clearReusedCommandJobDraft,
  t,
  copyText = writeClipboardText,
  notifySuccess = (message) => window.$message?.success(message),
  notifyWarning = (message) => window.$message?.warning(message)
}: UseCommandCenterTemplateActionsOptions) => {
  const saveCurrentCommandTemplate = () => {
    const saved = saveCommandTemplate({
      identify: commandIdentify.value,
      value: commandValue.value,
      timeoutSeconds: timeoutSeconds.value
    })
    if (saved) {
      notifySuccess(t('custom.commandCenter.saveCommandTemplateSuccess'))
    } else {
      notifyWarning(t('custom.commandCenter.saveCommandTemplateMissingIdentify'))
    }
  }

  const saveCommandJobTemplate = (job: FleetCommandJobListItem) => {
    const templateName = `${job.identify || t('custom.commandCenter.commandIdentifier')} ${job.job_id.slice(0, 8)}`
    const saved = saveCommandTemplate({
      name: templateName,
      identify: job.identify || '',
      value: job.command_value || '',
      timeoutSeconds: job.timeout_seconds || 60,
      syncName: false
    })
    if (saved) {
      notifySuccess(t('custom.commandCenter.saveJobAsTemplateSuccess'))
    } else {
      notifyWarning(t('custom.commandCenter.saveCommandTemplateMissingIdentify'))
    }
  }

  const copyCommandTemplateExport = async (templates: CommandCenterSavedCommandTemplate[]) => {
    if (!templates.length) return
    const ok = await copyText(serializeCommandTemplatesForExport(templates))
    if (ok) {
      notifySuccess(
        templates.length === 1
          ? t('custom.commandCenter.copyCommandTemplateSuccess')
          : t('custom.commandCenter.copyCommandTemplatesSuccess')
      )
    } else {
      notifyWarning(t('common.copyFailed'))
    }
  }

  const importSavedCommandTemplates = (raw: string) => {
    try {
      const result = importCommandTemplates(raw)
      if (result.imported > 0) {
        notifySuccess(
          t('custom.commandCenter.importCommandTemplatesSuccess').replace('{count}', String(result.imported))
        )
        return
      }
    } catch {
      // Fall through to the same customer-facing warning for malformed or unsupported JSON.
    }
    notifyWarning(t('custom.commandCenter.importCommandTemplatesInvalid'))
  }

  const applyBuiltInCommandTemplate = (template: BuiltInCommandTemplate) => {
    commandIdentify.value = template.identify
    commandValue.value = template.value
    timeoutSeconds.value = template.timeoutSeconds
    clearReusedCommandJobDraft()
    resetCommandJobDraft()
  }

  const applySavedCommandTemplate = (template: CommandCenterSavedCommandTemplate) => {
    commandIdentify.value = template.identify
    commandValue.value = template.value
    timeoutSeconds.value = template.timeoutSeconds
    commandTemplateName.value = template.name
    clearReusedCommandJobDraft()
    resetCommandJobDraft()
    notifySuccess(t('custom.commandCenter.applyCommandTemplateSuccess'))
  }

  const deleteSavedCommandTemplate = (templateId: string) => {
    deleteCommandTemplate(templateId)
    notifySuccess(t('custom.commandCenter.deleteCommandTemplateSuccess'))
  }

  return {
    applyBuiltInCommandTemplate,
    applySavedCommandTemplate,
    copyCommandTemplateExport,
    deleteSavedCommandTemplate,
    importSavedCommandTemplates,
    saveCommandJobTemplate,
    saveCurrentCommandTemplate
  }
}
