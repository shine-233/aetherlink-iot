<!--
文件用途：个人中心基本信息的编辑表单（昵称、手机号、邮箱入口、组织、时区、语言、地址）。
核心逻辑：直接编辑父级传入的 userInfo 响应式对象；保存/取消/修改邮箱通过 emit 交回父级。
关键注意事项：邮箱字段只读，修改必须走验证码弹窗。
-->
<script setup lang="ts">
import { NButton, NDivider, NForm, NFormItem, NInput, NSelect, type FormRules } from 'naive-ui'
import { $t } from '@/locales'
import ProvinceCityDistrictSelector from '@/components/common/ProvinceCityDistrictSelector.vue'
import { COUNTRY_CODE_OPTIONS, TIMEZONE_OPTIONS, type PersonalCenterUserInfo } from '../personal-center-profile'

const props = defineProps<{
  info: PersonalCenterUserInfo
  rules: FormRules
  languageOptions: Array<{ label: string; value: string }>
}>()

defineEmits<{ save: []; cancel: []; changeEmail: [] }>()

const handleAddressChange = (value: { province: string; city: string; district: string }) => {
  Object.assign(props.info.address, value)
}
</script>

<template>
  <div class="mb-32px">
    <NForm
      class="bg-#f8fafc p-18px pb-0 dark:bg-[#1E293B]"
      label-placement="left"
      label-align="left"
      label-width="120px"
      :rules="rules"
      :model="info"
    >
      <NFormItem path="name" :label="$t('page.manage.user.nickName')">
        <NInput v-model:value="info.name" :placeholder="$t('page.manage.user.form.nickName')" />
      </NFormItem>
      <NFormItem path="phone_number" :label="$t('generate.phoneNumber')">
        <div class="flex gap-2 w-full">
          <NSelect
            v-model:value="info.country_code"
            class="w-24"
            :options="COUNTRY_CODE_OPTIONS"
            :placeholder="$t('custom.personalCenter.countryCodePlaceholder')"
          />
          <NInput
            v-model:value="info.phone_only"
            class="flex-1"
            :placeholder="$t('custom.personalCenter.phonePlaceholder')"
          />
        </div>
      </NFormItem>
      <NFormItem path="email" :label="$t('generate.email-address')">
        <div class="flex gap-8px w-full">
          <NInput
            v-model:value="info.email"
            disabled
            :placeholder="$t('custom.personalCenter.emailChangeRequiresVerification')"
          />
          <NButton @click="$emit('changeEmail')">{{ $t('custom.personalCenter.changeEmailButton') }}</NButton>
        </div>
      </NFormItem>
      <NFormItem path="organization" :label="$t('page.manage.user.organization')">
        <NInput v-model:value="info.organization" :placeholder="$t('page.manage.user.form.organization')" />
      </NFormItem>
      <NFormItem path="timezone" :label="$t('page.manage.user.timezone')">
        <NSelect
          v-model:value="info.timezone"
          :options="TIMEZONE_OPTIONS"
          :placeholder="$t('page.manage.user.form.timezone')"
        />
      </NFormItem>
      <NFormItem path="default_language" :label="$t('page.manage.user.defaultLanguage')">
        <NSelect
          v-model:value="info.default_language"
          :options="languageOptions"
          :placeholder="$t('page.manage.user.form.defaultLanguage')"
        />
      </NFormItem>
      <NFormItem path="address.province" :label="$t('page.manage.user.address')">
        <ProvinceCityDistrictSelector
          :province="info.address.province"
          :city="info.address.city"
          :district="info.address.district"
          @change="handleAddressChange"
        />
      </NFormItem>
      <NFormItem path="address.detailed_address" :label="$t('page.manage.user.detailedAddress')">
        <NInput
          v-model:value="info.address.detailed_address"
          :placeholder="$t('page.manage.user.form.detailedAddress')"
        />
      </NFormItem>
    </NForm>
    <NDivider style="margin: 12px 0" />
    <div class="flex gap-4">
      <NButton type="primary" @click="$emit('save')">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M9 16.2L4.8 12l-1.4 1.4L9 19 21 7l-1.4-1.4L9 16.2z"></path>
        </svg>
        <span class="ml-2">{{ $t('common.confirm') }}</span>
      </NButton>
      <NButton @click="$emit('cancel')">{{ $t('common.cancel') }}</NButton>
    </div>
  </div>
</template>
