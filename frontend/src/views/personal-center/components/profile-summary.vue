<!--
文件用途：个人中心基本信息的只读展示（昵称、账号类型、邮箱、手机、组织、时区、语言、地址）。
核心逻辑：纯展示；「修改邮箱」通过 emit 交回父级打开验证弹窗。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { NButton, NDivider } from 'naive-ui'
import { $t } from '@/locales'
import { formatDisplayPhone, type PersonalCenterUserInfo } from '../personal-center-profile'

const props = defineProps<{
  info: PersonalCenterUserInfo
  authorityLabel: string
}>()

defineEmits<{ changeEmail: [] }>()

const notSet = (value?: string) => value || $t('common.notSet')
const rows = computed(() => {
  const { info } = props
  const region = [info.address.province, info.address.city, info.address.district].filter(Boolean).join(' / ')
  return [
    { key: 'name', label: $t('page.manage.user.nickName'), value: info.name },
    { key: 'authority', label: $t('generate.account-type'), value: props.authorityLabel },
    { key: 'email', label: $t('generate.email-address'), value: info.email },
    { key: 'phone', label: $t('generate.phoneNumber'), value: formatDisplayPhone(info) },
    { key: 'organization', label: $t('page.manage.user.organization'), value: notSet(info.organization) },
    { key: 'timezone', label: $t('page.manage.user.timezone'), value: notSet(info.timezone) },
    { key: 'language', label: $t('page.manage.user.defaultLanguage'), value: notSet(info.default_language) },
    { key: 'address', label: $t('page.manage.user.address'), value: notSet(region) },
    {
      key: 'detailedAddress',
      label: $t('page.manage.user.detailedAddress'),
      value: notSet(info.address.detailed_address)
    }
  ]
})
</script>

<template>
  <div class="mb-32px">
    <template v-for="row in rows" :key="row.key">
      <div class="flex justify-start">
        <div class="w-120px text-14px text-#666 dark:text-gray-600">{{ row.label }}</div>
        <div v-if="row.key === 'email'" class="flex items-center gap-8px">
          <span>{{ row.value }}</span>
          <NButton size="tiny" @click="$emit('changeEmail')">
            {{ $t('custom.personalCenter.changeEmailButton') }}
          </NButton>
        </div>
        <div v-else>{{ row.value }}</div>
      </div>
      <NDivider style="margin: 12px 0" />
    </template>
  </div>
</template>
