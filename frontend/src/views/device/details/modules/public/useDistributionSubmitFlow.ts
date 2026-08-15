import type { ComputedRef, Ref } from 'vue'
import { ref } from 'vue'
import type { FormInst } from 'naive-ui'
import type { FlatResponseFailData, FlatResponseSuccessData } from '@aetherlink/axios'
import dayjs from 'dayjs'
import { commandDataPub, type DirectMethodResult } from '@/service/api'
import { $t } from '@/locales'
import { isJSON } from '@/utils/common/tool'
import { buildAttributePayload } from './distributionAttributePayload'
import { buildCommandPayload } from './distributionCommandPayload'
import {
  buildDistributionSubmitPayload,
  buildExpectedMessagePayload,
  buildQuickCommandPayload,
  isApiError,
  quickCommandKey
} from './distributionSubmitPayload'

type SubmitApi = (params: any) => Promise<FlatResponseSuccessData | FlatResponseFailData>

export type CommandSubmitTracking = {
  logRecorded?: boolean
  messageId?: string
  status?: string | number
}

type DistributionFormModel = {
  commandValue: string
  textValue: string
  expected: boolean
  time: number | null
  waitForResponse: boolean
  timeoutSeconds: number | null
}

type DistributionSubmitFlowOptions = {
  activeTab: Ref<string>
  attributeList: Ref<any[]>
  closeDialog: () => void
  deviceId: () => string
  directMethodApi?: () => SubmitApi | undefined
  expectApi: () => SubmitApi | undefined
  fetchData: () => Promise<void>
  formModel: DistributionFormModel
  formRef: Ref<FormInst | null>
  hasAttributeSelection: ComputedRef<boolean>
  isCommand: () => boolean | undefined
  logger: { error: (...args: any[]) => void }
  onDirectMethodResult?: (result: DirectMethodResult) => void | Promise<void>
  onSubmitTracking?: (tracking: CommandSubmitTracking) => void | Promise<void>
  paramsData: Ref<any[]>
  submitApi: () => SubmitApi | undefined
}

function extractDirectMethodResult(result: unknown): DirectMethodResult | null {
  const payload = (result as any)?.data || result
  if (!payload?.message_id || !payload?.outcome) return null
  return payload as DirectMethodResult
}

function notifyDirectMethodResult(result: DirectMethodResult) {
  switch (result.outcome) {
    case 'device_succeeded':
      window.$message?.success($t('generate.directMethodSucceeded'))
      return
    case 'timeout':
      window.$message?.warning($t('generate.directMethodTimedOut'))
      return
    case 'device_failed':
      window.$message?.error($t('generate.directMethodDeviceFailed'))
      return
    default:
      window.$message?.error($t('generate.directMethodDeliveryFailed'))
  }
}

function extractCommandSubmitTracking(result: unknown): CommandSubmitTracking {
  const payload = (result as any)?.data || result
  return {
    logRecorded: payload?.log_recorded ?? payload?.logRecorded,
    messageId: payload?.message_id || payload?.messageId,
    status: payload?.status
  }
}

function buildSubmitSuccessMessage(result: unknown, isCommand?: boolean, isExpected?: boolean) {
  if (!isCommand || isExpected) return $t('generate.sendingSuccess')

  const tracking = extractCommandSubmitTracking(result)
  if (tracking.messageId) {
    if (tracking.logRecorded === false) {
      return `${$t('generate.commandSubmittedLogUnavailable')} ${tracking.messageId}`
    }
    return `${$t('generate.commandSubmittedWithMessageId')} ${tracking.messageId}`
  }
  return $t('generate.commandSubmittedAwaitingLog')
}

export function useDistributionSubmitFlow(options: DistributionSubmitFlowOptions) {
  const submitting = ref(false)
  const quickCommandLoadingId = ref('')

  const submit = async () => {
    if (submitting.value) return

    try {
      await options.formRef.value?.validate()
      submitting.value = true

      if (options.activeTab.value === 'visual') {
        if (options.isCommand() && options.paramsData.value.length > 0) {
          options.formModel.textValue = JSON.stringify(buildCommandPayload(options.paramsData.value))
        }
        if (!options.isCommand()) {
          if (!options.hasAttributeSelection.value) {
            window.$message?.warning($t('generate.select-attribute-first'))
            return
          }
          options.formModel.textValue = JSON.stringify(buildAttributePayload(options.attributeList.value))
        }
      }

      if (options.formModel.textValue && !isJSON(options.formModel.textValue)) {
        window.$message?.error($t('generate.inputRightJson'))
        return
      }

      const submitPayload = buildDistributionSubmitPayload({
        deviceId: options.deviceId(),
        isCommand: options.isCommand(),
        textValue: options.formModel.textValue,
        commandValue: options.formModel.commandValue
      })

      const useDirectMethod = Boolean(
        options.isCommand() && !options.formModel.expected && options.formModel.waitForResponse
      )

      let result: FlatResponseSuccessData | FlatResponseFailData | undefined
      if (options.formModel.expected) {
        const expectApi = options.expectApi()
        if (expectApi) {
          const expiry = options.formModel.time ? new Date().getTime() + options.formModel.time * 60 * 60 * 1000 : null
          result = await expectApi(
            buildExpectedMessagePayload({
              deviceId: options.deviceId(),
              isCommand: options.isCommand(),
              textValue: options.formModel.textValue,
              commandValue: options.formModel.commandValue,
              expiry: expiry ? dayjs(expiry).format('YYYY-MM-DDTHH:mm:ssZ') : null
            })
          )
        }
      } else if (useDirectMethod) {
        const directMethodApi = options.directMethodApi?.()
        if (!directMethodApi) {
          window.$message?.error($t('generate.directMethodUnavailable'))
          return
        }
        result = await directMethodApi({
          ...submitPayload,
          timeout_seconds: options.formModel.timeoutSeconds || 10
        })
      } else {
        const submitApi = options.submitApi()
        if (submitApi) {
          result = await submitApi(submitPayload)
        }
      }

      if (isApiError(result)) {
        window.$message?.error($t('generate.sendingFail'))
        return
      }

      if (options.isCommand() && !options.formModel.expected) {
        const tracking = extractCommandSubmitTracking(result)
        if (tracking.messageId) {
          await options.onSubmitTracking?.(tracking)
        }
      }
      if (useDirectMethod) {
        const directMethodResult = extractDirectMethodResult(result)
        if (!directMethodResult) {
          window.$message?.error($t('generate.directMethodInvalidResponse'))
          return
        }
        await options.onDirectMethodResult?.(directMethodResult)
        notifyDirectMethodResult(directMethodResult)
      } else {
        window.$message?.success(
          buildSubmitSuccessMessage(result, Boolean(options.isCommand()), options.formModel.expected)
        )
      }
      await options.fetchData()
      options.closeDialog()
    } catch (errors) {
      window.$message?.error($t('common.validateFail') || 'Validation failed, please check your input.')
      options.logger.error('Form validation failed:', errors)
    } finally {
      submitting.value = false
    }
  }

  const onCommandChange = async (row: any) => {
    const commandKey = quickCommandKey(row)
    if (quickCommandLoadingId.value) return

    quickCommandLoadingId.value = commandKey
    try {
      const result = await commandDataPub(buildQuickCommandPayload(options.deviceId(), row))
      if (isApiError(result)) {
        window.$message?.error($t('generate.sendingFail'))
        return
      }
      const tracking = extractCommandSubmitTracking(result)
      if (tracking.messageId) {
        await options.onSubmitTracking?.(tracking)
      }
      window.$message?.success(buildSubmitSuccessMessage(result, true, false))
      await options.fetchData()
    } finally {
      quickCommandLoadingId.value = ''
    }
  }

  return {
    onCommandChange,
    quickCommandLoadingId,
    submit,
    submitting
  }
}
