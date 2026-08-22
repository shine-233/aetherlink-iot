<!--
文件用途：系统设置页中的账号资料与密码维护组件，负责当前登录用户的资料回显、语言切换和密码修改。
核心逻辑：页面挂载后读取用户详情并回填资料表单；保存资料时同步远端、Pinia 登录态与本地缓存；修改密码时按系统功能开关决定是否追加盐值并进行 RSA 加密。
状态流说明：本组件维护“资料加载中 / 资料保存中 / 密码保存中”三组状态，分别控制首屏回显、资料提交按钮和密码提交按钮，避免两块表单互相污染。
使用注意事项：资料保存依赖 fetchUserInfo 返回的原始对象快照，提交时会把快照中的未编辑字段一起带回；如果后端字段结构调整，这里的快照合并策略需要同步校对。
静态审查建议：可以把“资料加载 + 资料保存 + 密码保存”抽成统一的个人中心组合函数；`enableZcAndYzm` 的 localStorage 读取协议较脆弱，后续更适合改成由系统设置 store 提供稳定能力标记。
-->
<script setup lang="ts">
import { computed, onMounted, reactive, ref, toRef } from 'vue'
import type { FormInst, FormItemRule, FormRules } from 'naive-ui'
import { $t } from '@/locales'
import { useAppStore } from '@/store/modules/app'
import { useAuthStore } from '@/store/modules/auth'
import { changeInformation, fetchUserInfo, passwordModification } from '@/service/api/personal-center'
import { localStg } from '@/utils/storage'
import { getConfirmPwdRule } from '@/utils/form/rule'
import { generateRandomHexString, validName, validPasswordByExp } from '@/utils/common/tool'
import { resolveUserAvatarPath } from '@/utils/auth-user-avatar'
import { encryptDataByRsa } from '@/utils/security/rsa-encrypt'

type LocaleValue = App.I18n.LangType

const appStore = useAppStore()
const authStore = useAuthStore()

// 三组加载态分别覆盖首屏回显、资料保存和密码保存，避免一个按钮的等待状态阻塞另一块表单。
const profileFormRef = ref<FormInst | null>(null)
const passwordFormRef = ref<FormInst | null>(null)
const loading = ref(false)
const profileSaving = ref(false)
const passwordSaving = ref(false)
// 保留接口原始快照，资料提交时把未在页面展示的用户字段一并带回，减少误清空风险。
const userInfoSnapshot = ref<Record<string, any>>({})

// 语言下拉直接映射 appStore 的语言配置，保证资料页与全局国际化选项来源一致。
const languageOptions = computed(() =>
  appStore.localeOptions.map(option => ({
    label: option.label,
    value: option.key
  }))
)

const profileForm = reactive({
  name: '',
  default_language: appStore.locale as LocaleValue
})

const passwordForm = reactive({
  old_password: '',
  password: '',
  passwords: ''
})

const profileRules: FormRules = {
  name: [
    {
      required: true,
      validator(_rule: FormItemRule, value: string) {
        if (!validName(value?.trim())) {
          return new Error($t('custom.personalCenter.nameFieldNotEmpty'))
        }
        return true
      },
      trigger: ['input', 'blur']
    }
  ],
  default_language: {
    required: true,
    message: $t('page.manage.user.form.defaultLanguage'),
    trigger: ['change', 'blur']
  }
}

const passwordRules: FormRules = {
  old_password: {
    required: true,
    message: $t('generate.old-password'),
    trigger: ['input', 'blur']
  },
  password: [
    {
      required: true,
      validator(rule: FormItemRule, value: string) {
        if (!value || value.length < 8 || value.length > 20 || !validPasswordByExp(value)) {
          return Promise.reject(rule.message)
        }
        return Promise.resolve()
      },
      message: $t('form.pwd.tip'),
      trigger: ['input', 'blur']
    }
  ],
  passwords: getConfirmPwdRule(toRef(passwordForm, 'password'))
}

function normalizeLocale(value: unknown): LocaleValue {
  const key = String(value || '')
    .trim()
    .toLowerCase()
  const localeMap: Record<string, LocaleValue> = {
    'zh-cn': 'zh-CN',
    'en-us': 'en-US',
    'fr-fr': 'fr-FR',
    'es-es': 'es-ES'
  }
  return localeMap[key] || appStore.locale
}

// 后端历史字段存在 phone_number / phone_num 两种形态，统一在这里兜底读取。
function getPhoneNumber(data: Record<string, any>) {
  return data.phone_number || data.phone_num || ''
}

// 把接口返回的账号详情标准化为页面表单与本地快照，隔离字段命名差异和缺省结构。
function syncProfile(data: Record<string, any>) {
  userInfoSnapshot.value = {
    ...data,
    phone_number: getPhoneNumber(data),
    additional_info: data.additional_info || data.additionalInfo || '{}',
    address: data.address || {
      province: '',
      city: '',
      district: '',
      detailed_address: ''
    }
  }
  profileForm.name = String(data.name || authStore.userInfo.name || authStore.userInfo.userName || '')
  profileForm.default_language = normalizeLocale(data.default_language)
}

function syncAuthUserInfo(data: Record<string, any>) {
  const nextName = String(data.name || authStore.userInfo.name || authStore.userInfo.userName || '')
  const nextEmail = String(data.email || authStore.userInfo.email || authStore.userInfo.userEmail || '')
  const nextLocale = normalizeLocale(data.default_language)
  Object.assign(authStore.userInfo, {
    ...authStore.userInfo,
    name: nextName,
    userName: nextName,
    email: nextEmail,
    userEmail: nextEmail,
    default_language: nextLocale,
    additional_info: data.additional_info || data.additionalInfo || authStore.userInfo.additional_info || '{}',
    additionalInfo: data.additional_info || data.additionalInfo || authStore.userInfo.additionalInfo || '{}',
    avatar_url: resolveUserAvatarPath(data) || authStore.userInfo.avatar_url || ''
  })
  localStg.set('userInfo', { ...authStore.userInfo })
}

// 首屏与手动刷新共用同一条回显链路，静态上已经具备较清晰的收口点。
async function loadUserInfo() {
  loading.value = true
  try {
    const { error, data } = await fetchUserInfo()
    if (!error && data) {
      syncProfile(data)
      syncAuthUserInfo(data)
    }
  } finally {
    loading.value = false
  }
}

// 资料保存会复用用户快照补齐隐藏字段，并在成功后同步 Pinia 与 localStorage，保证刷新前后展示一致。
// 静态审查建议：这里默认远端接受“整对象更新”；如果后端后续改成局部 patch，建议拆分 payload 构造逻辑并显式列出允许修改的字段。
async function saveProfile() {
  await profileFormRef.value?.validate()

  profileSaving.value = true
  try {
    const nextLocale = profileForm.default_language
    const payload = {
      ...userInfoSnapshot.value,
      name: profileForm.name.trim(),
      default_language: nextLocale,
      phone_number: getPhoneNumber(userInfoSnapshot.value)
    }
    const { error } = await changeInformation(payload)
    if (!error) {
      syncProfile(payload)
      syncAuthUserInfo(payload)
      window.$message?.success($t('custom.management.accountProfile.profileSaved'))
      if (nextLocale !== appStore.locale) {
        appStore.changeLocale(nextLocale, { persistRemote: false })
      }
    }
  } finally {
    profileSaving.value = false
  }
}

// 密码表单与资料表单相互独立，保存成功或手动重置时都应清空输入并重置校验状态。
function resetPasswordForm() {
  passwordForm.old_password = ''
  passwordForm.password = ''
  passwordForm.passwords = ''
  passwordFormRef.value?.restoreValidation()
}

// 是否加密密码由本地系统功能开关决定，说明账号设置与系统功能配置存在隐式耦合。
// 静态审查建议：后续更适合由 setting store 暴露只读能力标记，避免这里直接解析 localStorage 协议。
function shouldEncryptPassword() {
  try {
    const data = JSON.parse(localStorage.getItem('enableZcAndYzm') || '[]')
    return Array.isArray(data) && data.some(item => item?.name === 'frontend_res' && item?.enable_flag === 'enable')
  } catch {
    return false
  }
}

// 密码保存链路会在功能开关开启时追加随机盐值并走 RSA 加密，成功后仅清空表单，不会主动刷新用户资料。
async function savePassword() {
  await passwordFormRef.value?.validate()

  passwordSaving.value = true
  try {
    let salt: string | null = null
    let password = passwordForm.password
    if (shouldEncryptPassword()) {
      salt = generateRandomHexString(16)
      // RSA helper 已迁移为 WebCrypto 异步实现（RSA-OAEP/SHA-256）
      password = await encryptDataByRsa(password + salt)
    }
    const { error } = await passwordModification({
      old_password: passwordForm.old_password,
      password,
      salt
    })
    if (!error) {
      resetPasswordForm()
      window.$message?.success($t('custom.management.accountProfile.passwordSaved'))
    }
  } finally {
    passwordSaving.value = false
  }
}

// 组件挂载即触发一次资料回显，保持系统设置页切入时的首屏数据完整。
onMounted(loadUserInfo)
</script>

<template>
  <NSpin :show="loading">
    <NFlex vertical :size="20" class="account-profile-setting">
      <NForm ref="profileFormRef" :model="profileForm" :rules="profileRules" label-placement="left" label-width="140px">
        <NFormItem path="name" :label="$t('page.manage.user.nickName')">
          <NInput v-model:value="profileForm.name" :placeholder="$t('page.manage.user.form.nickName')" />
        </NFormItem>
        <NFormItem path="default_language" :label="$t('page.manage.user.defaultLanguage')">
          <NSelect
            v-model:value="profileForm.default_language"
            :options="languageOptions"
            :placeholder="$t('page.manage.user.form.defaultLanguage')"
          />
        </NFormItem>
        <NFormItem>
          <NSpace>
            <NButton type="primary" :loading="profileSaving" @click="saveProfile">
              {{ $t('common.save') }}
            </NButton>
            <NButton :loading="loading" @click="loadUserInfo">
              {{ $t('common.refresh') }}
            </NButton>
          </NSpace>
        </NFormItem>
      </NForm>

      <NDivider />

      <NForm
        ref="passwordFormRef"
        :model="passwordForm"
        :rules="passwordRules"
        label-placement="left"
        label-width="140px"
      >
        <NFormItem path="old_password" :label="$t('generate.old-password')">
          <NInput
            v-model:value="passwordForm.old_password"
            type="password"
            show-password-on="click"
            :placeholder="$t('generate.old-password')"
          />
        </NFormItem>
        <NFormItem path="password" :label="$t('generate.new-password')">
          <NInput
            v-model:value="passwordForm.password"
            type="password"
            show-password-on="click"
            :placeholder="$t('generate.new-password')"
          />
        </NFormItem>
        <NFormItem path="passwords" :label="$t('generate.repeat-new-password')">
          <NInput
            v-model:value="passwordForm.passwords"
            type="password"
            show-password-on="click"
            :placeholder="$t('generate.repeat-new-password')"
          />
        </NFormItem>
        <NFormItem>
          <NSpace>
            <NButton type="primary" :loading="passwordSaving" @click="savePassword">
              {{ $t('common.save') }}
            </NButton>
            <NButton @click="resetPasswordForm">
              {{ $t('generate.reset') }}
            </NButton>
          </NSpace>
        </NFormItem>
      </NForm>
    </NFlex>
  </NSpin>
</template>

<style scoped>
.account-profile-setting {
  max-width: 760px;
  padding-top: 12px;
}
</style>
