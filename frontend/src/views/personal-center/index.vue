<!--
  文件用途：个人中心页面，承载资料编辑、邮箱变更、密码修改、头像上传和告警邮箱入口。
  核心逻辑：资料数据规则在 personal-center-profile（纯函数），密码表单在 usePersonalCenterPassword，
    邮箱变更在 usePersonalCenterEmailChange；只读资料与邮箱弹窗拆为子组件，本文件负责加载/提交/登录态回写。
  关键注意事项：个人中心与系统设置页存在功能重叠，修改登录态同步、语言切换或邮箱变更逻辑时，要同时核对两个入口是否一致。
-->
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { NButton } from 'naive-ui'
import type { FormRules } from 'naive-ui'
import { $t } from '@/locales'
import { localStg } from '@/utils/storage'
import { useAppStore } from '@/store/modules/app'
import { useAuthStore } from '@/store/modules/auth'
import { changeInformation, fetchUserInfo } from '@/service/api/personal-center'
import { getPlatformApiBaseUrl } from '@/utils/common/tool'
import {
  mergeUserAvatarIntoAdditionalInfo,
  resolvePlatformAssetUrl,
  resolveUserAvatarPath
} from '@/utils/auth-user-avatar'
import { createProxyPattern } from '~/env.config'
import WarningEmailSetting from '@/views/management/setting/components/warning-email-setting.vue'
import TwoFactorSetting from './components/two-factor-setting.vue'
import ProfileSummary from './components/profile-summary.vue'
import EmailChangeModal from './components/email-change-modal.vue'
import ProfileEditForm from './components/profile-edit-form.vue'
import { usePersonalCenterEmailChange } from './usePersonalCenterEmailChange'
import { usePersonalCenterPassword } from './usePersonalCenterPassword'
import {
  authorityLocaleKey,
  buildSubmitUserInfo,
  emptyPersonalCenterUserInfo,
  formatDisplayPhone,
  normalizeFetchedUserInfo,
  normalizeLocale,
  type PersonalCenterUserInfo
} from './personal-center-profile'

// 开发环境使用代理路径，生产环境使用完整上传地址。
const url = ref(import.meta.env.VITE_HTTP_PROXY === 'Y' ? createProxyPattern() : getPlatformApiBaseUrl())
const appStore = useAppStore()
const authStore = useAuthStore()
const editType = ref(false)
const header = ref(false)
const headUrl = ref('')
const defaultAvatarUrl = '/rdi/default_avatar.png'

const languageOptions = computed(() =>
  appStore.localeOptions.map((option) => ({ label: option.label, value: option.key }))
)

const userInfoData = ref<PersonalCenterUserInfo>(emptyPersonalCenterUserInfo())

const handleAddressChange = (value: { province: string; city: string; district: string }) => {
  Object.assign(userInfoData.value.address, value)
}

const authorityLabel = computed(() => {
  const key = authorityLocaleKey(userInfoData.value.authority)
  return key ? $t(key) : ''
})

function syncAuthUserInfo(patch: Partial<Api.Auth.UserInfo>) {
  Object.assign(authStore.userInfo, patch)
  localStg.set('userInfo', { ...authStore.userInfo })
}

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
    syncAuthUserInfo({ email: changedEmail, userEmail: changedEmail })
  }
})

const { formRef, formData, passRules, resetPass, submitPass } = usePersonalCenterPassword()

const fullPhoneNumber = computed(() => `${userInfoData.value.country_code}${userInfoData.value.phone_only}`)
const displayPhoneNumber = computed(() => formatDisplayPhone(userInfoData.value))
const getSubmitUserInfoData = () => buildSubmitUserInfo(userInfoData.value)

watch(
  fullPhoneNumber,
  (phone) => {
    userInfoData.value.phone_number = phone
  },
  { immediate: true }
)

function applyAvatarPreview(source: Record<string, unknown>) {
  const avatarPath = resolveUserAvatarPath(source)
  header.value = Boolean(avatarPath)
  headUrl.value = avatarPath ? resolvePlatformAssetUrl(avatarPath) : ''
}

async function refreshUserInfo() {
  const { data } = await fetchUserInfo()
  // 后端可能返回空 data；空对象兜底避免 null/undefined 进入字段归一化。
  userInfoData.value = normalizeFetchedUserInfo(data ?? {})
  const info = userInfoData.value
  const current = authStore.userInfo
  applyAvatarPreview(info)
  syncAuthUserInfo({
    name: info.name || current.name,
    userName: info.name || current.userName,
    email: info.email || current.email,
    userEmail: info.email || current.userEmail,
    default_language: info.default_language || current.default_language,
    additional_info: info.additional_info,
    additionalInfo: info.additional_info,
    avatar_url: info.avatar_url || current.avatar_url
  })
}

const required = (message: string, trigger: string[], isRequired = true) => ({
  required: isRequired,
  trigger,
  message
})
const rules: FormRules = {
  email: required($t('generate.email-address'), ['blur', 'input']),
  name: required($t('page.manage.user.nickName'), ['blur', 'input']),
  phone_number: required($t('generate.phoneNumber'), ['blur', 'input']),
  organization: required($t('page.manage.user.form.organization'), ['blur', 'input'], false),
  timezone: required($t('page.manage.user.form.timezone'), ['blur', 'change'], false),
  default_language: required($t('page.manage.user.form.defaultLanguage'), ['blur', 'change'], false),
  'address.province': required($t('page.manage.user.form.address'), ['blur', 'change'], false),
  'address.detailed_address': required($t('page.manage.user.form.detailedAddress'), ['blur', 'input'], false)
}

const editName = () => {
  editType.value = true
}
const closeEdit = () => {
  editType.value = false
}

async function updataUserInfo() {
  const nextLocale = normalizeLocale(userInfoData.value.default_language)
  const previousLocale = appStore.locale
  const nextName = String(userInfoData.value.name || '').trim()
  userInfoData.value.name = nextName
  userInfoData.value.default_language = nextLocale
  const { error } = await changeInformation(getSubmitUserInfoData())
  if (error) return
  syncAuthUserInfo({ name: nextName, userName: nextName, default_language: nextLocale })
  if (nextLocale && nextLocale !== previousLocale) appStore.changeLocale(nextLocale, { persistRemote: false })
  window.$message?.success($t('custom.grouping_details.operationSuccess'))
  closeEdit()
}

const setAvatar = (additionalInfo: string, avatarPath: string) => {
  userInfoData.value.additional_info = additionalInfo
  userInfoData.value.avatar_url = avatarPath
  applyAvatarPreview(userInfoData.value)
}

async function handleFinish({ event }: { event?: ProgressEvent }) {
  const response = JSON.parse((event?.target as XMLHttpRequest).response)
  const path: string = response.data.path
  const info = mergeUserAvatarIntoAdditionalInfo(userInfoData.value.additional_info, path)
  setAvatar(info, path)
  syncAuthUserInfo({ additional_info: info, additionalInfo: info, avatar_url: path })

  const { error } = await changeInformation(getSubmitUserInfoData())
  if (error) return
  await refreshUserInfo()
  // 后端回读若尚未包含新头像（缓存/延迟），保留本地已上传的头像。
  if (!resolveUserAvatarPath(userInfoData.value)) setAvatar(info, path)
  syncAuthUserInfo({
    additional_info: userInfoData.value.additional_info,
    additionalInfo: userInfoData.value.additional_info,
    avatar_url: userInfoData.value.avatar_url || path
  })
  window.$message?.success($t('custom.grouping_details.operationSuccess'))
}

const handleUploadFinish = (payload: { event?: ProgressEvent }) => void handleFinish(payload)

onMounted(refreshUserInfo)
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
            <ProfileSummary
              v-if="!editType"
              :info="userInfoData"
              :authority-label="authorityLabel"
              @change-email="openEmailChangeModal"
            />

            <ProfileEditForm
              v-if="editType"
              :info="userInfoData"
              :rules="rules"
              :language-options="languageOptions"
              @save="updataUserInfo"
              @cancel="closeEdit"
              @change-email="openEmailChangeModal"
            />
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
    <EmailChangeModal
      v-model:show="emailModalVisible"
      :form="emailChangeForm"
      :code-loading="emailCodeLoading"
      :code-counting="emailCodeCounting"
      :code-label="emailCodeLabel"
      :submitting="emailChangeLoading"
      @send-code="sendEmailChangeCode"
      @submit="submitEmailChange"
    />
  </div>
</template>
