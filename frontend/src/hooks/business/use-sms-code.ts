/*
 * 文件用途：提供邮箱短信验证码发送 Hook，统一获取验证码按钮文案和发送流程。
 * 核心逻辑：根据倒计时状态计算文案，校验邮箱后调用验证码接口并启动冷却。
 * 关键注意事项：接口失败、邮箱格式和国际化文案会直接影响登录体验。
 * 重构建议：建议补充失败分支测试，并把邮箱校验规则与表单规则保持一致。
 */
import { computed } from 'vue'
import { useLoading } from '@aetherlink/hooks'
import { fetchEmailCodeByEmail } from '@/service/api/auth'
import { $t } from '@/locales'
import useCountDown from './use-count-down'

export default function useSmsCode() {
  const { loading, startLoading, endLoading } = useLoading()
  const { counts, start, isCounting } = useCountDown(60)
  const initLabel = computed(() => $t('page.login.common.getCode'))
  const countingLabel = (second: number) => $t('page.login.common.countingLabel', { second })
  const label = computed(() => {
    let text = initLabel.value
    if (loading.value) {
      text = ''
    }
    if (isCounting.value) {
      text = countingLabel(counts.value)
    }
    return text
  })

  /** 判断邮箱格式是否正确 */
  async function isValidEmail(email) {
    // 正则表达式来匹配邮箱格式
    const emailRegex = /^[a-zA-Z0-9._-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,6}$/
    const valid = emailRegex.test(email)
    if (!valid) {
      window.$message?.error($t('page.login.common.emailInvalid'))
    } else if (email.trim() === '') {
      window.$message?.error($t('page.login.common.emailRequired'))
    }
    return valid
  }

  /**
   * 根据邮箱获取短信验证码
   *
   * @param email - 邮箱
   */
  async function getSmsCodeByEmail(email: string) {
    const valid = await isValidEmail(email)
    if (!valid || loading.value) return

    startLoading()
    try {
      const { error, data } = await fetchEmailCodeByEmail(email)

      if (!error && data) {
        // Success case
        start() // Start countdown on success
        window.$message?.success($t('page.login.common.codeSent'))
      } else if (error) {
        // Error case: Try to access potential custom properties on the error object
        const errorCode = (error as any)?.code // Safely access potential code
        const errorMessage = (error as any)?.message // Safely access potential message

        if (errorCode === 200008) {
          // Specific error code for email registered
          window.$message?.error(errorMessage || $t('page.login.common.emailRegistered'))
        } else {
          // Other errors reported by the API
          window.$message?.error(errorMessage || $t('page.login.common.codeError'))
        }
      } else {
        // Fallback for unexpected scenarios (e.g., no error, no data)
        window.$message?.error($t('page.login.common.codeError'))
      }
    } catch (err) {
      // Catch exceptions during the API call itself (e.g., network error)
      window.$message?.error($t('page.login.common.codeError'))
    } finally {
      endLoading()
    }
  }

  return {
    label,
    start,
    isCounting,
    loading,
    isValidEmail,
    getSmsCodeByEmail
  }
}
