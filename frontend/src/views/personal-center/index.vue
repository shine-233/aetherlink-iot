<!--
  文件用途：个人中心页面，承载资料编辑、邮箱变更、密码修改、头像上传和告警邮箱入口。
  核心逻辑：读取当前用户详情并回填表单，保存资料后同步登录态与本地缓存，安全设置和邮箱变更通过独立链路处理。
  关键注意事项：个人中心与系统设置页存在功能重叠，修改登录态同步、语言切换或邮箱变更逻辑时，要同时核对两个入口是否一致。
  重构建议：后续适合继续把“资料加载/提交/登录态回写”收敛到组合函数，并用 focused tests 锁住两个入口的一致性。
-->
<!--
 * @Descripttion:
 * @version:
 * @Author: zhaoqi
 * @Date: 2024-03-17 09:14:38
 * @LastEditors: zhaoqi
 * @LastEditTime: 2024-03-20 17:23:40
-->
<script setup lang="ts">
import { computed, onMounted, ref, toRefs, watch } from 'vue'
import { NButton } from 'naive-ui'
import type { FormItemRule, FormRules } from 'naive-ui'
import { $t } from '@/locales'
import { localStg } from '@/utils/storage'
import { useAppStore } from '@/store/modules/app'
import { useAuthStore } from '@/store/modules/auth'
import { getConfirmPwdRule } from '@/utils/form/rule'
import { useNaiveForm } from '@/hooks/common/form'
import { changeInformation, fetchUserInfo, passwordModification } from '@/service/api/personal-center'
import { generateRandomHexString, getPlatformApiBaseUrl, validName, validPasswordByExp } from '@/utils/common/tool'
import {
  mergeUserAvatarIntoAdditionalInfo,
  resolvePlatformAssetUrl,
  resolveUserAvatarPath
} from '@/utils/auth-user-avatar'
import { encryptDataByRsa } from '@/utils/security/rsa-encrypt'
import { createProxyPattern } from '~/env.config'
import ProvinceCityDistrictSelector from '@/components/common/ProvinceCityDistrictSelector.vue'
import WarningEmailSetting from '@/views/management/setting/components/warning-email-setting.vue'
import TwoFactorSetting from './components/two-factor-setting.vue'
import { usePersonalCenterEmailChange } from './usePersonalCenterEmailChange'

// 开发环境使用代理路径，生产环境使用完整上传地址。
const isHttpProxy = import.meta.env.VITE_HTTP_PROXY === 'Y'
const uploadApiBaseUrl = isHttpProxy ? createProxyPattern() : getPlatformApiBaseUrl()
const url = ref(uploadApiBaseUrl)
const appStore = useAppStore()
const authStore = useAuthStore()
const { formRef, validate } = useNaiveForm()
const editType = ref(false)
const header = ref(false)
const headUrl = ref('')
const defaultAvatarUrl = '/rdi/default_avatar.png'

// 时区选项
const timezoneOptions = [
  { label: 'Asia/Shanghai (Shanghai Time)', value: 'Asia/Shanghai' },
  { label: 'Asia/Tokyo (Tokyo Time)', value: 'Asia/Tokyo' },
  { label: 'Asia/Seoul (Seoul Time)', value: 'Asia/Seoul' },
  { label: 'Asia/Singapore (Singapore Time)', value: 'Asia/Singapore' },
  { label: 'Asia/Hong_Kong (Hong Kong Time)', value: 'Asia/Hong_Kong' },
  { label: 'Asia/Bangkok (Bangkok Time)', value: 'Asia/Bangkok' },
  { label: 'Asia/Dubai (Dubai Time)', value: 'Asia/Dubai' },
  { label: 'Asia/Kolkata (India Time)', value: 'Asia/Kolkata' },
  { label: 'Europe/London (London Time)', value: 'Europe/London' },
  { label: 'Europe/Paris (Paris Time)', value: 'Europe/Paris' },
  { label: 'Europe/Berlin (Berlin Time)', value: 'Europe/Berlin' },
  { label: 'Europe/Moscow (Moscow Time)', value: 'Europe/Moscow' },
  { label: 'America/New_York (New York Time)', value: 'America/New_York' },
  { label: 'America/Los_Angeles (Los Angeles Time)', value: 'America/Los_Angeles' },
  { label: 'America/Chicago (Chicago Time)', value: 'America/Chicago' },
  { label: 'America/Toronto (Toronto Time)', value: 'America/Toronto' },
  { label: 'Australia/Sydney (Sydney Time)', value: 'Australia/Sydney' },
  { label: 'Australia/Melbourne (Melbourne Time)', value: 'Australia/Melbourne' },
  { label: 'Pacific/Auckland (Auckland Time)', value: 'Pacific/Auckland' },
  { label: 'UTC (Coordinated Universal Time)', value: 'UTC' }
]

// 语言选项
type LocaleValue = App.I18n.LangType

const languageOptions = computed(() =>
  appStore.localeOptions.map((option) => ({
    label: option.label,
    value: option.key
  }))
)

// 国家区号选项
const countryCodeOptions = [
  { label: '+86', value: '+86' },
  { label: '+1', value: '+1' },
  { label: '+44', value: '+44' },
  { label: '+33', value: '+33' },
  { label: '+49', value: '+49' },
  { label: '+39', value: '+39' },
  { label: '+34', value: '+34' },
  { label: '+7', value: '+7' },
  { label: '+81', value: '+81' },
  { label: '+82', value: '+82' },
  { label: '+65', value: '+65' },
  { label: '+60', value: '+60' },
  { label: '+66', value: '+66' },
  { label: '+84', value: '+84' },
  { label: '+62', value: '+62' },
  { label: '+63', value: '+63' },
  { label: '+91', value: '+91' },
  { label: '+61', value: '+61' },
  { label: '+64', value: '+64' },
  { label: '+55', value: '+55' },
  { label: '+52', value: '+52' },
  { label: '+54', value: '+54' },
  { label: '+27', value: '+27' },
  { label: '+20', value: '+20' },
  { label: '+971', value: '+971' },
  { label: '+966', value: '+966' },
  { label: '+90', value: '+90' },
  { label: '+31', value: '+31' },
  { label: '+46', value: '+46' },
  { label: '+47', value: '+47' },
  { label: '+45', value: '+45' },
  { label: '+41', value: '+41' },
  { label: '+43', value: '+43' },
  { label: '+32', value: '+32' },
  { label: '+351', value: '+351' },
  { label: '+30', value: '+30' },
  { label: '+48', value: '+48' },
  { label: '+420', value: '+420' },
  { label: '+36', value: '+36' },
  { label: '+385', value: '+385' },
  { label: '+852', value: '+852' },
  { label: '+853', value: '+853' },
  { label: '+886', value: '+886' }
]

// 处理省市区选择变化。
const handleAddressChange = (value: { province: string; city: string; district: string }) => {
  userInfoData.value.address.province = value.province
  userInfoData.value.address.city = value.city
  userInfoData.value.address.district = value.district
}

const userInfoData = ref({
  additional_info: '',
  name: '',
  email: '',
  phone_number: '', // 完整手机号
  country_code: '+86',
  phone_only: '',
  authority: '',
  organization: '', // 组织
  timezone: '', // 时区
  default_language: '', // 默认语言
  avatar_url: '', // 头像
  address: {
    province: '', // 省份
    city: '', // 城市
    district: '', // 区县
    detailed_address: '' // 详细地址
  }
})

const authorityLocaleKeyByValue: Record<
  string,
  'generate.SYS_ADMIN' | 'generate.TENANT_ADMIN' | 'generate.TENANT_USER'
> = {
  SYS_ADMIN: 'generate.SYS_ADMIN',
  TENANT_ADMIN: 'generate.TENANT_ADMIN',
  TENANT_USER: 'generate.TENANT_USER'
}

const authorityLabel = computed(() => {
  const authority = String(userInfoData.value.authority || '').trim()
  if (!authority) return ''
  return $t(authorityLocaleKeyByValue[authority] || 'generate.user')
})
const {
  emailModalVisible,
  emailCodeLoading,
  emailChangeLoading,
  emailChangeForm,
  emailCodeCounting,
  emailCodeLabel,
  openEmailChangeModal,
  sendEmailChangeCode,
  submitEmailChange
} = usePersonalCenterEmailChange({
  getCurrentEmail: () =>
    String(userInfoData.value.email || authStore.userInfo.email || authStore.userInfo.userEmail || ''),
  applyChangedEmail: (changedEmail) => {
    userInfoData.value.email = changedEmail
    authStore.userInfo.email = changedEmail
    authStore.userInfo.userEmail = changedEmail
    localStg.set('userInfo', { ...authStore.userInfo })
  }
})

const parsePhoneNumber = (phoneNumber: string) => {
  if (!phoneNumber) return { country_code: '+86', phone_only: '' }
  const cleanPhone = phoneNumber.replace(/[^\d+]/g, '')
  const sortedCountryCodes = countryCodeOptions.map((option) => option.value).sort((a, b) => b.length - a.length)
  for (const code of sortedCountryCodes) {
    if (cleanPhone.startsWith(code)) {
      return {
        country_code: code,
        phone_only: cleanPhone.substring(code.length)
      }
    }
  }
  return {
    country_code: '+86',
    phone_only: cleanPhone
  }
}

const fullPhoneNumber = computed(() => `${userInfoData.value.country_code}${userInfoData.value.phone_only}`)
const displayPhoneNumber = computed(() => {
  const code = userInfoData.value.country_code?.trim()
  const phone = userInfoData.value.phone_only?.trim()
  if (code && phone) {
    return `${code} ${phone}`
  }
  return userInfoData.value.phone_number || ''
})

function normalizeLocale(value: unknown): LocaleValue | '' {
  const key = String(value || '')
    .trim()
    .toLowerCase()
    .replace(/_/g, '-')
  if (!key) return ''

  const localeMap: Record<string, LocaleValue> = {
    'zh-cn': 'zh-CN',
    'en-us': 'en-US',
    'fr-fr': 'fr-FR',
    'es-es': 'es-ES'
  }

  return localeMap[key] || ''
}

function syncAuthUserInfo(patch: Partial<Api.Auth.UserInfo>) {
  Object.assign(authStore.userInfo, {
    ...authStore.userInfo,
    ...patch
  })
  localStg.set('userInfo', { ...authStore.userInfo })
}

function applyAvatarPreview(source: Record<string, unknown>) {
  const avatarPath = resolveUserAvatarPath(source)
  header.value = Boolean(avatarPath)
  headUrl.value = avatarPath ? resolvePlatformAssetUrl(avatarPath) : ''
}

// 返回类型显式对齐 userInfoData 的形状：展开 data 会把 name/email 变成 any，
// 若不补齐这两个必需字段，赋值给 userInfoData 时会因缺少属性而类型不兼容。
function normalizeFetchedUserInfo(data: Record<string, any>): typeof userInfoData.value {
  const basePhone = data.phone_num || data.phone_number || ''
  const { country_code, phone_only } = parsePhoneNumber(basePhone)

  return {
    ...data,
    name: data.name || '',
    email: data.email || '',
    phone_number: basePhone,
    country_code,
    phone_only,
    authority: data.authority || '',
    additional_info: data.additional_info || data.additionalInfo || '{}',
    organization: data.organization || '',
    timezone: data.timezone || '',
    default_language: normalizeLocale(data.default_language),
    avatar_url: resolveUserAvatarPath(data),
    address: {
      province: data.address?.province || '',
      city: data.address?.city || '',
      district: data.address?.district || '',
      detailed_address: data.address?.detailed_address || ''
    }
  }
}

async function refreshUserInfo() {
  const { data } = await fetchUserInfo()
  // 后端可能返回空 data；空对象兜底避免 null/undefined 进入字段归一化。
  userInfoData.value = normalizeFetchedUserInfo(data ?? {})
  applyAvatarPreview(userInfoData.value)
  syncAuthUserInfo({
    name: userInfoData.value.name || authStore.userInfo.name,
    userName: userInfoData.value.name || authStore.userInfo.userName,
    email: userInfoData.value.email || authStore.userInfo.email,
    userEmail: userInfoData.value.email || authStore.userInfo.userEmail,
    default_language: userInfoData.value.default_language || authStore.userInfo.default_language,
    additional_info: userInfoData.value.additional_info,
    additionalInfo: userInfoData.value.additional_info,
    avatar_url: userInfoData.value.avatar_url || authStore.userInfo.avatar_url
  })
}

watch(
  () => [userInfoData.value.country_code, userInfoData.value.phone_only],
  () => {
    userInfoData.value.phone_number = fullPhoneNumber.value
  },
  { immediate: true }
)

const getSubmitUserInfoData = () => {
  const rest = Object.fromEntries(
    Object.entries(userInfoData.value).filter(([key]) => !['country_code', 'phone_only'].includes(key))
  )
  return {
    ...rest,
    phone_number: fullPhoneNumber.value
  }
}
/** 初始化密码表单数据 */
const formData = ref({
  name: '',
  old_password: '',
  password: '',
  passwords: ''
})

const rules: FormRules = {
  email: {
    required: true,
    trigger: ['blur', 'input'],
    message: $t('generate.email-address')
  },
  name: {
    required: true,
    trigger: ['blur', 'input'],
    message: $t('page.manage.user.nickName')
  },
  phone_number: {
    required: true,
    trigger: ['blur', 'input'],
    message: $t('generate.phoneNumber')
  },
  organization: {
    required: false,
    trigger: ['blur', 'input'],
    message: $t('page.manage.user.form.organization')
  },
  timezone: {
    required: false,
    trigger: ['blur', 'change'],
    message: $t('page.manage.user.form.timezone')
  },
  default_language: {
    required: false,
    trigger: ['blur', 'change'],
    message: $t('page.manage.user.form.defaultLanguage')
  },
  'address.province': {
    required: false,
    trigger: ['blur', 'change'],
    message: $t('page.manage.user.form.address')
  },
  'address.detailed_address': {
    required: false,
    trigger: ['blur', 'input'],
    message: $t('page.manage.user.form.detailedAddress')
  }
}
const passRules: FormRules = {
  name: [
    {
      required: true,
      validator(rule: FormItemRule, value: string) {
        if (rule && !validName(value)) {
          return new Error($t('custom.personalCenter.nameFieldNotEmpty'))
        }
        return true
      },
      trigger: ['input', 'blur']
    }
  ],
  password: [
    {
      required: true,
      validator(rule: FormItemRule, value: string) {
        if (value.length < 8 || value.length > 20) {
          return Promise.reject(rule.message)
        }
        if (!validPasswordByExp(value)) {
          return Promise.reject(rule.message)
        }
        return Promise.resolve()
      },
      message: $t('form.pwd.tip'),
      trigger: ['input', 'blur']
    }
  ],
  passwords: getConfirmPwdRule(toRefs(formData.value).password)
}

function editName() {
  editType.value = true
  // openModal();
  // setModalType('amend');
}

/** 取消编辑模式 */
function closeEdit() {
  editType.value = false
}

// 新设计不再保留旧版标签页切换逻辑，个人中心资料直接在单页内完成编辑。
/** 更新用户资料 */
async function updataUserInfo() {
  const nextLocale = normalizeLocale(userInfoData.value.default_language)
  const previousLocale = appStore.locale
  const nextName = String(userInfoData.value.name || '').trim()
  userInfoData.value.name = nextName
  userInfoData.value.default_language = nextLocale
  const { error } = await changeInformation(getSubmitUserInfoData())
  if (!error) {
    syncAuthUserInfo({
      name: nextName,
      userName: nextName,
      default_language: nextLocale
    })
    if (nextLocale && nextLocale !== previousLocale) {
      appStore.changeLocale(nextLocale, { persistRemote: false })
    }
    window.$message?.success($t('custom.grouping_details.operationSuccess'))
    closeEdit() // 保存成功后退出编辑模式
  }
}
/** 重置密码表单 */
const resetPass = async () => {
  formData.value.old_password = ''
  formData.value.passwords = ''
  formData.value.password = ''
}
/** 提交密码修改 */
const submitPass = async () => {
  await validate()
  const cacheStr = localStorage.getItem('enableZcAndYzm')
  const data = cacheStr ? JSON.parse(cacheStr) : []
  let salt: any = null
  let password1 = formData.value.password
  if (data.find((v) => v.name === 'frontend_res')?.enable_flag === 'enable') {
    salt = generateRandomHexString(16)
    // RSA helper 已迁移为 WebCrypto 异步实现（RSA-OAEP/SHA-256）
    password1 = await encryptDataByRsa(password1 + salt)
  }
  const param = {
    old_password: formData.value.old_password,
    password: password1,
    salt
  }
  const res = await passwordModification(param)
  if (!res.error) {
    window.$message?.success($t('custom.grouping_details.operationSuccess'))
  }
}

async function handleFinish({ event }: { event?: ProgressEvent }) {
  const response = JSON.parse((event?.target as XMLHttpRequest).response)
  const info = mergeUserAvatarIntoAdditionalInfo(userInfoData.value.additional_info, response.data.path)
  userInfoData.value.additional_info = info
  userInfoData.value.avatar_url = response.data.path
  applyAvatarPreview(userInfoData.value)
  syncAuthUserInfo({
    additional_info: info,
    additionalInfo: info,
    avatar_url: response.data.path
  })

  const { error } = await changeInformation(getSubmitUserInfoData())
  if (!error) {
    await refreshUserInfo()
    if (!resolveUserAvatarPath(userInfoData.value)) {
      userInfoData.value.additional_info = info
      userInfoData.value.avatar_url = response.data.path
      applyAvatarPreview(userInfoData.value)
    }
    syncAuthUserInfo({
      additional_info: userInfoData.value.additional_info,
      additionalInfo: userInfoData.value.additional_info,
      avatar_url: userInfoData.value.avatar_url || response.data.path
    })
    window.$message?.success($t('custom.grouping_details.operationSuccess'))
  }
}

function handleUploadFinish(payload: { event?: ProgressEvent }) {
  void handleFinish(payload)
}
onMounted(async () => {
  await refreshUserInfo()
})
</script>

<template>
  <div>
    <n-card>
      <div class="flex-col justify-center items-center">
        <div>
          <n-upload
            :action="url + '/file/up'"
            :show-file-list="false"
            :headers="{
              'x-token': localStg.get('token') || ''
            }"
            :data="{
              type: 'user_icon'
            }"
            @finish="handleUploadFinish"
          >
            <div class="relative w-100px h-100px">
              <n-avatar v-if="!header" class="w-100px h-100px" round :src="defaultAvatarUrl" />
              <n-avatar v-else class="w-100px h-100px" round :src="headUrl" />
              <div
                class="absolute bottom-0 right-0 w-32px h-32px bg-#6366f1 rounded-50% z-9999 flex justify-center items-center"
              >
                <svg width="16" height="16" viewBox="0 0 24 24" fill="white">
                  <path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z"></path>
                </svg>
              </div>
            </div>
          </n-upload>
        </div>
        <div class="text-24px text-#1a1a1a font-600 mb-8px dark:text-#E0E0E0">{{ userInfoData.name }}</div>
        <div>
          <!-- 角色文案沿用 generate.* 多语言 key。 -->
          {{ authorityLabel }}
        </div>
      </div>
      <n-divider />
      <!-- 基本信息 -->
      <div>
        <div>
          <div class="flex justify-between mb-20px">
            <div class="flex text-16px font-600 mb-20px items-center gap-6px">
              <svg width="22" height="22" viewBox="0 0 24 24" fill="currentColor">
                <path
                  d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm0 3c1.66 0 3 1.34 3 3s-1.34 3-3 3-3-1.34-3-3 1.34-3 3-3zm0 14.2c-2.5 0-4.71-1.28-6-3.22.03-1.99 4-3.08 6-3.08 1.99 0 5.97 1.09 6 3.08-1.29 1.94-3.5 3.22-6 3.22z"
                ></path>
              </svg>
              <div>
                {{ $t('generate.baseInfo') }}
              </div>
            </div>
            <NButton :title="$t('common.edit')" size="small" @click="editName()">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                <path
                  d="M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25zM20.71 7.04c.39-.39.39-1.02 0-1.41l-2.34-2.34c-.39-.39-1.02-.39-1.41 0l-1.83 1.83 3.75 3.75 1.83-1.83z"
                ></path>
              </svg>
            </NButton>
          </div>
          <div class="mt--4">
            <!-- 显示模式 -->
            <div v-if="!editType" class="mb-32px">
              <div class="flex justify-start">
                <div class="w-120px text-14px text-#666 dark:text-gray-600">{{ $t('page.manage.user.nickName') }}</div>

                <div>{{ userInfoData.name }}</div>
              </div>
              <n-divider style="margin: 12px 0" />
              <div class="flex justify-start">
                <div class="w-120px text-14px text-#666 dark:text-gray-600">{{ $t('generate.account-type') }}</div>
                <div>
                  {{ authorityLabel }}
                </div>
              </div>
              <n-divider style="margin: 12px 0" />
              <div class="flex justify-start">
                <div class="w-120px text-14px text-#666 dark:text-gray-600">{{ $t('generate.email-address') }}</div>

                <div class="flex items-center gap-8px">
                  <span>{{ userInfoData.email }}</span>
                  <NButton size="tiny" @click="openEmailChangeModal">
                    {{ $t('custom.personalCenter.changeEmailButton') }}
                  </NButton>
                </div>
              </div>
              <n-divider style="margin: 12px 0" />
              <div class="flex justify-start">
                <div class="w-120px text-14px text-#666 dark:text-gray-600">{{ $t('generate.phoneNumber') }}</div>

                <div>{{ displayPhoneNumber }}</div>
              </div>
              <n-divider style="margin: 12px 0" />
              <div class="flex justify-start">
                <div class="w-120px text-14px text-#666 dark:text-gray-600">
                  {{ $t('page.manage.user.organization') }}
                </div>
                <div>{{ userInfoData.organization || $t('common.notSet') }}</div>
              </div>
              <n-divider style="margin: 12px 0" />
              <div class="flex justify-start">
                <div class="w-120px text-14px text-#666 dark:text-gray-600">{{ $t('page.manage.user.timezone') }}</div>
                <div>{{ userInfoData.timezone || $t('common.notSet') }}</div>
              </div>
              <n-divider style="margin: 12px 0" />
              <div class="flex justify-start">
                <div class="w-120px text-14px text-#666 dark:text-gray-600">
                  {{ $t('page.manage.user.defaultLanguage') }}
                </div>
                <div>{{ userInfoData.default_language || $t('common.notSet') }}</div>
              </div>
              <n-divider style="margin: 12px 0" />
              <div class="flex justify-start">
                <div class="w-120px text-14px text-#666 dark:text-gray-600">{{ $t('page.manage.user.address') }}</div>
                <div>
                  {{
                    [userInfoData.address.province, userInfoData.address.city, userInfoData.address.district]
                      .filter(Boolean)
                      .join(' / ') || $t('common.notSet')
                  }}
                </div>
              </div>
              <n-divider style="margin: 12px 0" />
              <div class="flex justify-start">
                <div class="w-120px text-14px text-#666 dark:text-gray-600">
                  {{ $t('page.manage.user.detailedAddress') }}
                </div>
                <div>{{ userInfoData.address.detailed_address || $t('common.notSet') }}</div>
              </div>
              <n-divider style="margin: 12px 0" />
            </div>

            <!-- 编辑模式 -->
            <div v-if="editType" class="mb-32px">
              <NForm
                class="bg-#f8fafc p-18px pb-0 dark:bg-[#1E293B]"
                label-placement="left"
                label-align="left"
                label-width="120px"
                :rules="rules"
                :model="userInfoData"
              >
                <NFormItem path="name" :label="$t('page.manage.user.nickName')">
                  <NInput v-model:value="userInfoData.name" :placeholder="$t('page.manage.user.form.nickName')" />
                </NFormItem>

                <NFormItem path="phone_number" :label="$t('generate.phoneNumber')">
                  <div class="flex gap-2 w-full">
                    <NSelect
                      v-model:value="userInfoData.country_code"
                      class="w-24"
                      :options="countryCodeOptions"
                      :placeholder="$t('custom.personalCenter.countryCodePlaceholder')"
                    />
                    <NInput
                      v-model:value="userInfoData.phone_only"
                      class="flex-1"
                      :placeholder="$t('custom.personalCenter.phonePlaceholder')"
                    />
                  </div>
                </NFormItem>

                <NFormItem path="email" :label="$t('generate.email-address')">
                  <div class="flex gap-8px w-full">
                    <NInput
                      v-model:value="userInfoData.email"
                      disabled
                      :placeholder="$t('custom.personalCenter.emailChangeRequiresVerification')"
                    />
                    <NButton @click="openEmailChangeModal">
                      {{ $t('custom.personalCenter.changeEmailButton') }}
                    </NButton>
                  </div>
                </NFormItem>

                <NFormItem path="organization" :label="$t('page.manage.user.organization')">
                  <NInput
                    v-model:value="userInfoData.organization"
                    :placeholder="$t('page.manage.user.form.organization')"
                  />
                </NFormItem>

                <NFormItem path="timezone" :label="$t('page.manage.user.timezone')">
                  <NSelect
                    v-model:value="userInfoData.timezone"
                    :options="timezoneOptions"
                    :placeholder="$t('page.manage.user.form.timezone')"
                  />
                </NFormItem>

                <NFormItem path="default_language" :label="$t('page.manage.user.defaultLanguage')">
                  <NSelect
                    v-model:value="userInfoData.default_language"
                    :options="languageOptions"
                    :placeholder="$t('page.manage.user.form.defaultLanguage')"
                  />
                </NFormItem>

                <NFormItem path="address.province" :label="$t('page.manage.user.address')">
                  <ProvinceCityDistrictSelector
                    :province="userInfoData.address.province"
                    :city="userInfoData.address.city"
                    :district="userInfoData.address.district"
                    @change="handleAddressChange"
                  />
                </NFormItem>

                <NFormItem path="address.detailed_address" :label="$t('page.manage.user.detailedAddress')">
                  <NInput
                    v-model:value="userInfoData.address.detailed_address"
                    :placeholder="$t('page.manage.user.form.detailedAddress')"
                  />
                </NFormItem>
              </NForm>
              <n-divider style="margin: 12px 0" />
              <div class="flex gap-4">
                <NButton type="primary" @click="updataUserInfo">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
                    <path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z"></path>
                  </svg>
                  <span class="ml-2">
                    {{ $t('common.confirm') }}
                  </span>
                </NButton>
                <NButton @click="closeEdit">
                  {{ $t('common.cancel') }}
                </NButton>
              </div>
            </div>
          </div>
        </div>

        <!-- 密码修改 -->
        <div>
          <div class="flex text-16px font-600 mb-20px items-center gap-6px">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
              <path
                d="M18 8h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zm-6 9c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zm3.1-9H8.9V6c0-1.71 1.39-3.1 3.1-3.1 1.71 0 3.1 1.39 3.1 3.1v2z"
              ></path>
            </svg>
            <div>
              {{ $t('generate.secureSet') }}
            </div>
          </div>

          <div class="bg-#f8fafc p-20px dark:bg-[#1E293B]">
            <NForm ref="formRef" label-placement="top" :model="formData" :rules="passRules">
              <NFormItem :label="$t('generate.old-password')" path="old_password">
                <NInput
                  v-model:value="formData.old_password"
                  type="password"
                  show-password-on="click"
                  :placeholder="$t('generate.old-password')"
                />
              </NFormItem>

              <NFormItem :label="$t('generate.new-password')" path="password">
                <NInput
                  v-model:value="formData.password"
                  type="password"
                  show-password-on="click"
                  :placeholder="$t('generate.new-password')"
                />
              </NFormItem>

              <NFormItem :label="$t('generate.repeat-new-password')" path="passwords">
                <NInput
                  v-model:value="formData.passwords"
                  type="password"
                  show-password-on="click"
                  :placeholder="$t('generate.repeat-new-password')"
                />
              </NFormItem>

              <div class="flex gap-4">
                <NButton type="primary" @click="submitPass">
                  {{ $t('common.save') }}
                </NButton>
                <NButton @click="resetPass">
                  {{ $t('generate.reset') }}
                </NButton>
              </div>
            </NForm>
          </div>
        </div>
      </div>
      <n-divider />
      <div class="mt-24px">
        <div class="flex text-16px font-600 mb-20px items-center gap-6px">
          <span>{{ $t('custom.management.warningEmail') }}</span>
        </div>
        <WarningEmailSetting />
      </div>
      <n-divider />
      <div class="mt-24px">
        <div class="flex text-16px font-600 mb-20px items-center gap-6px">
          <span>{{ $t('custom.twoFactor.totpStatus') }}</span>
        </div>
        <TwoFactorSetting />
      </div>
    </n-card>
    <NModal
      v-model:show="emailModalVisible"
      preset="card"
      :title="$t('custom.personalCenter.changeAccountEmail')"
      class="max-w-520px"
    >
      <NSpace vertical size="large">
        <NAlert type="info" :show-icon="false">
          {{ $t('custom.personalCenter.emailChangeNotice') }}
        </NAlert>
        <NForm label-placement="top">
          <NFormItem :label="$t('custom.personalCenter.newEmail')">
            <NInput v-model:value="emailChangeForm.new_email" placeholder="name@example.com" />
          </NFormItem>
          <NFormItem :label="$t('custom.personalCenter.verificationCode')">
            <div class="flex gap-8px w-full">
              <NInput
                v-model:value="emailChangeForm.verify_code"
                :placeholder="$t('custom.personalCenter.codePlaceholder')"
              />
              <NButton :loading="emailCodeLoading" :disabled="emailCodeCounting" @click="sendEmailChangeCode">
                {{ emailCodeLabel }}
              </NButton>
            </div>
          </NFormItem>
        </NForm>
        <div class="flex justify-end gap-8px">
          <NButton @click="emailModalVisible = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="emailChangeLoading" @click="submitEmailChange">
            {{ $t('common.confirm') }}
          </NButton>
        </div>
      </NSpace>
    </NModal>
  </div>
</template>

<style scoped>
.avatar {
  width: 100px;
  height: 100px;
  border-radius: 50%;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 36px;
  color: #fff;
  font-weight: 500;
}
</style>
