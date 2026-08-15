import { computed, ref } from 'vue'
import useCountDown from '@/hooks/business/use-count-down'
import { $t } from '@/locales'
import { fetchEmailCodeByEmail } from '@/service/api/auth'
import { changeAccountEmail } from '@/service/api/personal-center'

type EmailChangeForm = {
  new_email: string
  verify_code: string
}

type UsePersonalCenterEmailChangeOptions = {
  getCurrentEmail: () => string
  applyChangedEmail: (email: string) => void
}

const emptyEmailChangeForm = (): EmailChangeForm => ({
  new_email: '',
  verify_code: ''
})

export const usePersonalCenterEmailChange = ({
  getCurrentEmail,
  applyChangedEmail
}: UsePersonalCenterEmailChangeOptions) => {
  const emailModalVisible = ref(false)
  const emailCodeLoading = ref(false)
  const emailChangeLoading = ref(false)
  const emailChangeForm = ref<EmailChangeForm>(emptyEmailChangeForm())
  const { counts: emailCodeCountdown, isCounting: emailCodeCounting, start: startEmailCodeCountdown } = useCountDown(60)
  const emailCodeLabel = computed(() =>
    emailCodeCounting.value ? `${emailCodeCountdown.value}s` : $t('custom.personalCenter.sendCode')
  )

  const openEmailChangeModal = () => {
    emailChangeForm.value = emptyEmailChangeForm()
    emailModalVisible.value = true
  }

  const sendEmailChangeCode = async () => {
    const newEmail = emailChangeForm.value.new_email.trim()
    if (!newEmail) {
      window.$message?.error($t('custom.personalCenter.newEmailRequired'))
      return
    }

    const email = getCurrentEmail().trim()
    if (email && newEmail.toLowerCase() === email.toLowerCase()) {
      window.$message?.error($t('custom.management.accountEmail.sameEmailNotAllowed'))
      return
    }
    if (!email) {
      window.$message?.error($t('custom.management.accountEmail.currentEmail'))
      return
    }

    emailCodeLoading.value = true
    try {
      const { error } = await fetchEmailCodeByEmail(email)
      if (!error) {
        startEmailCodeCountdown()
        window.$message?.success($t('page.login.common.codeSent'))
      }
    } finally {
      emailCodeLoading.value = false
    }
  }

  const submitEmailChange = async () => {
    const newEmail = emailChangeForm.value.new_email.trim()
    const verifyCode = emailChangeForm.value.verify_code.trim()
    const email = getCurrentEmail().trim()
    if (!newEmail || !verifyCode) {
      window.$message?.error($t('custom.personalCenter.emailAndCodeRequired'))
      return
    }
    if (email && newEmail.toLowerCase() === email.toLowerCase()) {
      window.$message?.error($t('custom.management.accountEmail.sameEmailNotAllowed'))
      return
    }

    emailChangeLoading.value = true
    try {
      const { error, data } = await changeAccountEmail({
        new_email: newEmail,
        verify_code: verifyCode
      })
      if (!error) {
        const changedEmail = data?.new_email || newEmail
        applyChangedEmail(changedEmail)
        emailModalVisible.value = false
        const migratedCount = typeof data?.devices_migrated === 'number' ? data.devices_migrated : null
        window.$message?.success(
          migratedCount === null
            ? $t('custom.personalCenter.emailChangeSuccess')
            : $t('custom.personalCenter.emailChangeSuccessWithCount', { count: migratedCount })
        )
      }
    } finally {
      emailChangeLoading.value = false
    }
  }

  return {
    emailModalVisible,
    emailCodeLoading,
    emailChangeLoading,
    emailChangeForm,
    emailCodeCounting,
    emailCodeLabel,
    openEmailChangeModal,
    sendEmailChangeCode,
    submitEmailChange
  }
}
