/*
 * 文件用途：提供表单规则和 Naive UI 表单实例辅助 Hook。
 * 核心逻辑：生成必填规则，并封装 validate、restoreValidation 等表单实例调用。
 * 关键注意事项：规则文案和 FormInst 引用需要与调用组件生命周期同步。
 * 重构建议：建议把常见规则和表单实例控制拆分，便于按需复用。
 */
import { ref } from 'vue'
import type { FormInst } from 'naive-ui'
import { REG_CODE_SIX, REG_DEFAULT, REG_EMAIL, REG_PHONE, REG_PWD, REG_PHONE_WITH_COUNTRY_CODE } from '@/constants/reg'
import { $t } from '@/locales'

export function useFormRules(paramObj?) {
  const patternRules = {
    userName: {
      pattern: REG_DEFAULT,
      message: $t('form.userName.invalid'),
      trigger: 'change'
    },
    phone: {
      pattern: REG_PHONE,
      message: $t('form.phone.invalid'),
      trigger: 'change'
    },
    phoneWithCountryCode: {
      pattern: REG_PHONE_WITH_COUNTRY_CODE,
      message: $t('form.phone.invalid'),
      trigger: 'change'
    },
    pwd: {
      pattern: REG_PWD,
      message: $t('form.pwd.invalid'),
      trigger: 'change'
    },
    code: {
      pattern: REG_CODE_SIX,
      message: $t('form.code.invalid'),
      trigger: 'change'
    },
    email: {
      pattern: REG_EMAIL,
      message: $t('form.email.invalid'),
      trigger: 'change'
    }
  } satisfies Record<string, App.Global.FormRule>

  const formRules = {
    userName: [createRequiredRule($t('form.userName.required')), patternRules.userName],
    phone: [createRequiredRule($t('form.phone.required')), patternRules.phone],
    phoneWithCountryCode: [createRequiredRule($t('form.phone.required')), patternRules.phoneWithCountryCode],
    pwd: [createRequiredRule($t('form.pwd.required')), paramObj && paramObj.pwd ? paramObj.pwd : patternRules.pwd],
    code: [createRequiredRule($t('form.code.required')), patternRules.code],
    email: [createRequiredRule($t('form.email.required')), patternRules.email]
  } satisfies Record<string, App.Global.FormRule[]>

  /** the default required rule */
  const defaultRequiredRule = createRequiredRule($t('form.required'))

  function createRequiredRule(message: string): App.Global.FormRule {
    return {
      required: true,
      message
    }
  }

  return {
    patternRules,
    formRules,
    defaultRequiredRule,
    createRequiredRule
  }
}

export function useNaiveForm() {
  const formRef = ref<FormInst | null>(null)

  async function validate() {
    await formRef.value?.validate()
  }

  async function restoreValidation() {
    formRef.value?.restoreValidation()
  }

  return {
    formRef,
    validate,
    restoreValidation
  }
}
