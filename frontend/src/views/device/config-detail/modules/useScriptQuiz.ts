/*
 * "Debug run" for a data-processing script.
 *
 * Shares the same form as the save flow: validate required fields, hand the simulated input and
 * the script body to the backend, then write the response text into the result box.
 */
import type { FormInst } from 'naive-ui'
import type { Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { dataScriptQuiz } from '@/service/api/device'

type ScriptQuizResponse = {
  code?: number | string
  message?: string
  data?: unknown
}

type QuizForm = Record<string, any>

export function useScriptQuiz(formRef: Ref<(HTMLElement & FormInst) | undefined>, form: Ref<QuizForm>) {
  const { t } = useI18n()

  async function doQuiz() {
    await formRef.value?.validate()

    try {
      const response = await dataScriptQuiz(form.value)
      // 兼容“请求层失败”和“业务层返回错误对象”两类响应结构。
      if (response.error && response.data === null) {
        const errorInfo = response.error as { message?: string; name?: string; code?: string | number }
        const errorMessage = errorInfo.message || t('page.dataForward.requestFailed')
        form.value.resolt_analog_input = `${t('page.dataForward.debugFailed')}\n${t('page.dataForward.errorType')}: ${errorInfo.name || 'Unknown'}\n${t('page.dataForward.errorCode')}: ${errorInfo.code || 'N/A'}\n${t('page.dataForward.errorMessage')}: ${errorMessage}`
        return
      }

      // 有些接口封装会把真正的 code/data/message 再包进 response.data，需要先解包再判定结果。
      let actualResponse = response as unknown as ScriptQuizResponse
      if (response.data && typeof response.data === 'object' && 'code' in response.data) {
        actualResponse = response.data as ScriptQuizResponse
      }

      // 结果区只接收字符串，因此这里把各种 data 形态统一转换成可直接展示的文本。
      // 使用宽松比较是因为后端 code 可能返回 number 也可能返回 string。
      if (actualResponse.code == 200 || actualResponse.code === '200') {
        if (typeof actualResponse.data === 'string') {
          form.value.resolt_analog_input =
            actualResponse.data === 'null' ? t('page.dataForward.debugSuccessWithNull') : actualResponse.data
        } else if (actualResponse.data === null || actualResponse.data === undefined) {
          form.value.resolt_analog_input = t('page.dataForward.debugSuccessWithNull')
        } else {
          form.value.resolt_analog_input = JSON.stringify(actualResponse.data, null, 2)
        }
      } else {
        const errorMessage = actualResponse.message || t('page.dataForward.noErrorMessage')
        form.value.resolt_analog_input = `${t('page.dataForward.debugFailed')}\ncode: ${actualResponse.code}\nmessage: ${errorMessage}`
      }
    } catch (error: unknown) {
      console.error('调试请求异常:', error)
      const errorMessage = error instanceof Error ? error.message : String(error)
      form.value.resolt_analog_input =
        t('page.dataForward.debugRequestFailed') + ': ' + (errorMessage || t('page.dataForward.unknownError'))
    }
  }

  return { doQuiz }
}
