/**
 * 文件用途：个人中心「修改密码」表单状态、校验规则与提交。
 * 核心逻辑：系统开启 frontend_res 时，新密码拼接随机盐后用 RSA-OAEP 加密再提交；否则明文 + salt=null。
 */
import { ref, toRefs } from 'vue'
import type { FormItemRule, FormRules } from 'naive-ui'
import { $t } from '@/locales'
import { useNaiveForm } from '@/hooks/common/form'
import { getConfirmPwdRule } from '@/utils/form/rule'
import { passwordModification } from '@/service/api/personal-center'
import { generateRandomHexString, validName, validPasswordByExp } from '@/utils/common/tool'
import { encryptDataByRsa } from '@/utils/security/rsa-encrypt'

const RSA_FLAG_STORAGE_KEY = 'enableZcAndYzm'

/** 读取登录页缓存的开关列表，判断是否要求前端 RSA 加密密码。 */
export function isFrontendRsaEnabled(storage: Pick<Storage, 'getItem'> = localStorage) {
  try {
    const flags = JSON.parse(storage.getItem(RSA_FLAG_STORAGE_KEY) || '[]')
    return Array.isArray(flags) && flags.find((flag) => flag?.name === 'frontend_res')?.enable_flag === 'enable'
  } catch {
    return false
  }
}

export function usePersonalCenterPassword() {
  const { formRef, validate } = useNaiveForm()
  const formData = ref({ name: '', old_password: '', password: '', passwords: '' })

  const passRules: FormRules = {
    name: [
      {
        required: true,
        validator(rule: FormItemRule, value: string) {
          if (rule && !validName(value)) return new Error($t('custom.personalCenter.nameFieldNotEmpty'))
          return true
        },
        trigger: ['input', 'blur']
      }
    ],
    password: [
      {
        required: true,
        validator(rule: FormItemRule, value: string) {
          if (value.length < 8 || value.length > 20 || !validPasswordByExp(value)) return Promise.reject(rule.message)
          return Promise.resolve()
        },
        message: $t('form.pwd.tip'),
        trigger: ['input', 'blur']
      }
    ],
    passwords: getConfirmPwdRule(toRefs(formData.value).password)
  }

  const resetPass = async () => {
    formData.value.old_password = ''
    formData.value.passwords = ''
    formData.value.password = ''
  }

  const submitPass = async () => {
    await validate()
    let salt: string | null = null
    let password = formData.value.password
    if (isFrontendRsaEnabled()) {
      salt = generateRandomHexString(16)
      password = await encryptDataByRsa(password + salt)
    }
    const res = await passwordModification({ old_password: formData.value.old_password, password, salt })
    if (!res.error) window.$message?.success($t('custom.grouping_details.operationSuccess'))
  }

  return { formRef, formData, passRules, resetPass, submitPass }
}
