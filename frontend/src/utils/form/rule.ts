/*
 * 文件用途：提供表单校验规则集合，覆盖必填、密码确认、验证码、邮箱等常用规则。
 * 核心逻辑：组合 Naive UI FormItemRule、国际化文案和响应式字段生成校验器。
 * 关键注意事项：规则变更会影响登录和账号表单，需要验证空值、格式和确认字段同步。
 * 重构建议：可把业务规则与纯格式规则分文件，并补充单元测试。
 */
import type { Ref } from 'vue'
import type { FormItemRule } from 'naive-ui'
import { $t } from '@/locales'
import { REG_CODE_SIX, REG_EMAIL, REG_PHONE, REG_PWD } from '@/constants/reg'
/** 创建自定义错误信息的必填表单规则 */
export const createRequiredFormRule = (message = $t('form.required')): FormItemRule => ({ required: true, message })

export const requiredFormRule = createRequiredFormRule()

/** 表单规则 */
interface CustomFormRules {
  /** 手机号码 */
  phone: FormItemRule[]
  /** 密码 */
  pwd: FormItemRule[]
  /** 验证码 */
  code: FormItemRule[]
  /** 邮箱 */
  email: FormItemRule[]
}

/** 表单规则 */
export const formRules: CustomFormRules = {
  phone: [
    createRequiredFormRule($t('form.phone.required')),
    { pattern: REG_PHONE, message: $t('form.phone.invalid'), trigger: 'input' }
  ],
  pwd: [
    // createRequiredFormRule($t('form.pwd.required'))
    { pattern: REG_PWD, message: $t('form.pwd.invalid'), trigger: 'input' }
  ],
  code: [
    createRequiredFormRule($t('form.code.required')),
    {
      pattern: REG_CODE_SIX,
      message: $t('form.code.invalid'),
      trigger: 'input'
    }
  ],
  email: [
    {
      required: true,
      pattern: REG_EMAIL,
      message: $t('form.email.required'),
      trigger: 'blur'
    }
  ]
}

/** 是否为空字符串 */
function isBlankString(str: string) {
  return str.trim() === ''
}

/** 获取确认密码的表单规则 */
export function getConfirmPwdRule(pwd: Ref<string>) {
  const confirmPwdRule: FormItemRule[] = [
    { required: true, message: $t('form.pwd.tip') },
    {
      validator: (rule, value) => {
        if (!isBlankString(value) && value !== pwd.value) {
          return Promise.reject(rule.message)
        }
        return Promise.resolve()
      },
      message: $t('form.manycheck.invalid'),
      trigger: 'input'
    }
  ]
  return confirmPwdRule
}

/** 获取图片验证码的表单规则 */
export function getImgCodeRule(imgCode: Ref<string>) {
  const imgCodeRule: FormItemRule[] = [
    { required: true, message: $t('form.code.required') },
    {
      validator: (rule, value) => {
        if (!isBlankString(value) && value !== imgCode.value) {
          return Promise.reject(rule.message)
        }
        return Promise.resolve()
      },
      message: $t('form.code.invalid'),
      trigger: 'blur'
    }
  ]
  return imgCodeRule
}
