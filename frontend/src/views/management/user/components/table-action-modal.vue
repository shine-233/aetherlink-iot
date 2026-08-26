<!--
后台用户新增/编辑弹窗，负责用户基础资料、联系方式、地址、语言与状态的维护。
核心链路：父页面传入新增或编辑模式 -> 回填表单默认值/编辑值 -> 校验密码、手机号和地址 -> 调用新增或编辑接口 -> 成功后通知父页面刷新列表。
静态维护重点：
1. 弹窗同时承载账号资料和区域化设置，字段较多，后续若再扩展用户档案，建议先抽离表单模型与规则生成逻辑。
2. `country_code + phone_only` 在页面内拼成完整手机号，修改接口契约时要同步检查新增和编辑两条链路。
3. 地址选择、时区、语言和状态都是高频后台配置字段，后续若多个弹窗复用，建议抽公共选项源与组合函数。
-->
<script setup lang="ts">
import { computed, reactive, ref, toRefs, watch } from 'vue'
import type { FormInst, FormItemRule } from 'naive-ui'
import {
  NButton,
  NForm,
  NFormItemGridItem,
  NGrid,
  NInput,
  NModal,
  NSpace,
  NSelect,
  NRadioGroup,
  NRadio
} from 'naive-ui'
import { addUser, editUser } from '@/service/api/auth'
import { createRequiredFormRule, formRules, getConfirmPwdRule } from '@/utils/form/rule'
import { userStatusOptions } from '@/constants/business'
import { $t } from '@/locales'
import ProvinceCityDistrictSelector from '@/components/common/ProvinceCityDistrictSelector.vue'

export interface Props {
  /** 弹窗可见性 */
  visible: boolean
  /** 弹窗类型 add: 新增 edit: 编辑 */
  type?: 'add' | 'edit'
  titleOverride?: string
  setupTenantAdminMode?: boolean
  /** 编辑的表格行数据 */
  editData?: UserManagement.User | null
}

export type ModalType = NonNullable<Props['type']>

defineOptions({ name: 'TableActionModal' })

const props = withDefaults(defineProps<Props>(), {
  type: 'add',
  setupTenantAdminMode: false,
  editData: null
})

interface Emits {
  (e: 'update:visible', visible: boolean): void

  /** 点击协议 */
  (e: 'success'): void
}

const emit = defineEmits<Emits>()

const modalVisible = computed({
  get() {
    return props.visible
  },
  set(visible) {
    emit('update:visible', visible)
  }
})

const customUserStatusOptions = computed(() => {
  return userStatusOptions.map(item => {
    const key = item.value === 'N' ? 'page.manage.user.status.normal' : 'page.manage.user.status.freeze'
    return {
      label: $t(key),
      value: item.value
    }
  })
})

// 后端最终使用完整手机号字段，页面通过区号和号码主体拆分编辑体验。
const fullPhoneNumber = computed(() => {
  return `${formModel.country_code}${formModel.phone_only}`
})

// 时区选项目前内嵌在弹窗内，和列表页查询保持一致。
// 城市描述走 i18n，避免在选项标签里硬编码中文。
const timezoneDefs: { value: string; cityKey: string }[] = [
  { value: 'Asia/Shanghai', cityKey: 'page.manage.user.tz.shanghai' },
  { value: 'Asia/Tokyo', cityKey: 'page.manage.user.tz.tokyo' },
  { value: 'Asia/Seoul', cityKey: 'page.manage.user.tz.seoul' },
  { value: 'Asia/Singapore', cityKey: 'page.manage.user.tz.singapore' },
  { value: 'Asia/Hong_Kong', cityKey: 'page.manage.user.tz.hongKong' },
  { value: 'Asia/Bangkok', cityKey: 'page.manage.user.tz.bangkok' },
  { value: 'Asia/Dubai', cityKey: 'page.manage.user.tz.dubai' },
  { value: 'Asia/Kolkata', cityKey: 'page.manage.user.tz.kolkata' },
  { value: 'Europe/London', cityKey: 'page.manage.user.tz.london' },
  { value: 'Europe/Paris', cityKey: 'page.manage.user.tz.paris' },
  { value: 'Europe/Berlin', cityKey: 'page.manage.user.tz.berlin' },
  { value: 'Europe/Moscow', cityKey: 'page.manage.user.tz.moscow' },
  { value: 'America/New_York', cityKey: 'page.manage.user.tz.newYork' },
  { value: 'America/Los_Angeles', cityKey: 'page.manage.user.tz.losAngeles' },
  { value: 'America/Chicago', cityKey: 'page.manage.user.tz.chicago' },
  { value: 'America/Toronto', cityKey: 'page.manage.user.tz.toronto' },
  { value: 'Australia/Sydney', cityKey: 'page.manage.user.tz.sydney' },
  { value: 'Australia/Melbourne', cityKey: 'page.manage.user.tz.melbourne' },
  { value: 'Pacific/Auckland', cityKey: 'page.manage.user.tz.auckland' },
  { value: 'UTC', cityKey: 'page.manage.user.tz.utc' }
]
const timezoneOptions = computed(() =>
  timezoneDefs.map(item => ({ label: `${item.value} (${$t(item.cityKey)})`, value: item.value }))
)

// 默认语言选择会影响用户后台显示语言，应与平台支持语言列表保持同步。
const languageOptions = [
  { label: 'Chinese', value: 'zh-CN' },
  { label: 'English', value: 'en-US' },
  { label: 'Francais', value: 'fr-FR' },
  { label: 'Espanol', value: 'es-ES' }
]

// 国家区号决定手机号拼装规则，后续若支持更多国家，优先改常量源而不是在模板里硬编码。
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

// 省市区选择器只负责结构化地址字段，详细地址仍由独立输入框维护。
const handleAddressChange = (value: { province: string; city: string; district: string }) => {
  // 更新表单模型中的地址数据
  formModel.address.province = value.province
  formModel.address.city = value.city
  formModel.address.district = value.district
}

const closeModal = () => {
  modalVisible.value = false
}

const title = computed(() => {
  if (props.titleOverride) return props.titleOverride
  const titles: Record<ModalType, string> = {
    add: $t('common.add'),
    edit: $t('common.edit')
  }
  return titles[props.type]
})

const formRef = ref<HTMLElement & FormInst>()
// 提交进行中标记：防止重复提交，并驱动确认按钮 loading 态。
const submitLoading = ref(false)

type FormModel = Pick<UserManagement.User, 'email' | 'name' | 'phone_number' | 'gender' | 'remark' | 'status'> & {
  password: string
  confirmPwd: string
  organization: string
  timezone: string
  default_language: string
  country_code: string
  phone_only: string
  address: {
    province: string
    city: string
    district: string
    detailed_address: string
  }
}

const formModel = reactive<FormModel>(createDefaultFormModel())

const rules = ref({})
// 手机号主体目前只做数字校验，不按区号做更细的长度规则；这是后续值得继续补强的静态热点。
const phoneNumberRule: FormItemRule = {
  validator: (rule, value) => {
    if (!value) return true
    // 只允许数字
    if (!/^\d+$/.test(value)) {
      return new Error($t('form.phone.invalid'))
    }
    return true
  },
  trigger: ['input', 'blur']
}
watch(
  () => props.type,
  () => {
    if (props.type == 'add') {
      rules.value = {
        name: createRequiredFormRule($t('common.pleaseCheckValue')),
        gender: createRequiredFormRule($t('common.pleaseCheckValue')),
        phone_only: [createRequiredFormRule($t('form.phone.required')), phoneNumberRule] as FormItemRule[],
        email: formRules.email,
        password: [{ required: true, message: $t('form.pwd.tip') } as FormItemRule, ...formRules.pwd],
        confirmPwd: getConfirmPwdRule(toRefs(formModel).password),
        status: createRequiredFormRule($t('common.pleaseCheckValue')),
        remark: createRequiredFormRule($t('common.pleaseCheckValue')),
        // organization: createRequiredFormRule($t('common.pleaseCheckValue')),
        timezone: createRequiredFormRule($t('common.pleaseCheckValue')),
        default_language: createRequiredFormRule($t('common.pleaseCheckValue'))
        // address: {
        //   province: createRequiredFormRule($t('page.manage.user.form.address')),
        //   city: createRequiredFormRule($t('page.manage.user.form.address')),
        //   district: createRequiredFormRule($t('page.manage.user.form.address')),
        //   detailed_address: createRequiredFormRule($t('page.manage.user.form.detailedAddress'))
        // }
      }
    } else {
      rules.value = {}
    }
  },
  {
    immediate: true
  }
)

function createDefaultFormModel(): FormModel {
  return {
    name: '',
    gender: null,
    phone_number: '',
    email: '',
    password: '',
    confirmPwd: '',
    remark: '',
    status: 'N',
    organization: '',
    timezone: 'Asia/Shanghai',
    default_language: 'en-US',
    country_code: '+86',
    phone_only: '',
    address: {
      province: '',
      city: '',
      district: '',
      detailed_address: ''
    }
  }
}

// 解析手机号，拆分为区号和手机号部分
function parsePhoneNumber(phoneNumber: string): { country_code: string; phone_only: string } {
  if (!phoneNumber) {
    return { country_code: '+86', phone_only: '' }
  }

  // 移除所有空格和特殊字符，只保留数字和+号
  const cleanPhone = phoneNumber.replace(/[^\d+]/g, '')

  // 按长度匹配区号（从长到短匹配，避免误匹配）
  const sortedCountryCodes = countryCodeOptions.map(option => option.value).sort((a, b) => b.length - a.length)

  for (const code of sortedCountryCodes) {
    if (cleanPhone.startsWith(code)) {
      return {
        country_code: code,
        phone_only: cleanPhone.substring(code.length)
      }
    }
  }

  // 如果没有匹配到区号，默认使用+86
  return {
    country_code: '+86',
    phone_only: cleanPhone
  }
}

function handleUpdateFormModel(model: Partial<FormModel>) {
  Object.assign(formModel, model)
}

function handleUpdateFormModelByModalType() {
  const handlers: Record<ModalType, () => void> = {
    add: () => {
      const defaultFormModel = createDefaultFormModel()
      if (props.setupTenantAdminMode) {
        defaultFormModel.remark = $t('custom.management.user.firstTenantAdminRemark')
      }

      handleUpdateFormModel(defaultFormModel)
    },
    edit: () => {
      if (props.editData) {
        // 从后端数据中提取地址字段（地址字段在 address 对象中）
        const editDataAny = props.editData as any
        const addressData = editDataAny.address || {
          province: '',
          city: '',
          district: '',
          detailed_address: ''
        }

        // 解析现有手机号，拆分为区号和手机号部分
        const phoneData = parsePhoneNumber(editDataAny.phone_number || '')

        // 编辑模式下不需要构建级联选择器的值，因为我们使用的是独立的省市区字段
        const editFormData = {
          ...editDataAny,
          password: '',
          confirmPwd: '',
          organization: editDataAny.organization || '',
          timezone: editDataAny.timezone || 'Asia/Shanghai',
          default_language: editDataAny.default_language || 'en-US',
          country_code: phoneData.country_code,
          phone_only: phoneData.phone_only,
          address: {
            ...addressData
          }
        }
        handleUpdateFormModel(editFormData)

        // 编辑模式下地址数据已经直接设置到表单模型中
      }
    }
  }

  handlers[props.type]()
}

async function handleSubmit() {
  await formRef.value?.validate()

  // 准备提交的数据，确保地址字段正确
  const submitData = {
    ...formModel
  }

  // 移除不需要提交的字段
  delete (submitData as any).confirmPwd

  submitLoading.value = true
  try {
    let data: any
    if (props.type === 'add') {
      data = await addUser(submitData)
    } else if (props.type === 'edit') {
      data = await editUser(submitData)
    }
    // 失败时保持弹窗打开，保留用户输入；错误提示由请求层全局 onError 统一弹出。
    if (!data.error) {
      emit('success')
      closeModal()
    }
  } finally {
    submitLoading.value = false
  }
}

watch(
  () => props.visible,
  newVal => {
    if (newVal) {
      handleUpdateFormModelByModalType()
    }
  }
)

// 监听区号和手机号变化，更新完整手机号
watch(
  () => [formModel.country_code, formModel.phone_only],
  () => {
    formModel.phone_number = fullPhoneNumber.value
  },
  { deep: true }
)
</script>

<template>
  <NModal v-model:show="modalVisible" preset="card" :title="title" :aria-label="title" class="w-700px">
    <NForm ref="formRef" label-placement="left" :label-width="80" :model="formModel" :rules="rules">
      <NGrid :cols="24" :x-gap="18">
        <NFormItemGridItem :span="12" :label="$t('page.manage.user.userName')" path="name">
          <NInput v-model:value="formModel.name" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.user.userEmail')" path="email">
          <NInput v-model:value="formModel.email" :disabled="type === 'edit'" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.user.userPhone')" path="phone_only">
          <div class="flex gap-2">
            <NSelect
              v-model:value="formModel.country_code"
              :options="countryCodeOptions"
              class="w-24"
              :placeholder="$t('custom.management.user.areaCodePlaceholder')"
            />
            <NInput
              v-model:value="formModel.phone_only"
              :placeholder="$t('page.manage.user.form.userPhone')"
              class="flex-1"
            />
          </div>
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.user.organization')" path="organization">
          <NInput v-model:value="formModel.organization" :placeholder="$t('page.manage.user.form.organization')" />
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.user.timezone')" path="timezone">
          <NSelect
            v-model:value="formModel.timezone"
            :options="timezoneOptions"
            :placeholder="$t('page.manage.user.form.timezone')"
          />
        </NFormItemGridItem>
        <NFormItemGridItem :span="12" :label="$t('page.manage.user.defaultLanguage')" path="default_language">
          <NSelect
            v-model:value="formModel.default_language"
            :options="languageOptions"
            :placeholder="$t('page.manage.user.form.defaultLanguage')"
          />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('page.manage.user.address')" path="address.province">
          <ProvinceCityDistrictSelector
            :province="formModel.address.province"
            :city="formModel.address.city"
            :district="formModel.address.district"
            @change="handleAddressChange"
          />
        </NFormItemGridItem>
        <NFormItemGridItem :span="24" :label="$t('page.manage.user.detailedAddress')" path="address.detailed_address">
          <NInput
            v-model:value="formModel.address.detailed_address"
            :placeholder="$t('page.manage.user.form.detailedAddress')"
          />
        </NFormItemGridItem>

        <template v-if="type === 'add'">
          <NFormItemGridItem :span="12" :label="$t('page.manage.user.password')" path="password">
            <NInput v-model:value="formModel.password" type="password" />
          </NFormItemGridItem>
          <NFormItemGridItem :span="12" :label="$t('page.manage.user.confirmPwd')" path="confirmPwd">
            <NInput v-model:value="formModel.confirmPwd" type="password" />
          </NFormItemGridItem>
        </template>
        <n-form-item-grid-item v-else :span="12" :label="$t('page.manage.user.accountStatus')">
          <n-radio-group v-model:value="formModel.status">
            <n-radio v-for="item in customUserStatusOptions" :key="item.value" :value="item.value">
              {{ item.label }}
            </n-radio>
          </n-radio-group>
        </n-form-item-grid-item>
        <NFormItemGridItem :span="24" :label="$t('common.remark')">
          <NInput v-model:value="formModel.remark" type="textarea" />
        </NFormItemGridItem>
      </NGrid>
      <NSpace class="w-full pt-16px" :size="24" justify="end">
        <NButton class="w-72px" @click="closeModal">{{ $t('common.cancel') }}</NButton>
        <NButton class="w-72px" type="primary" :loading="submitLoading" @click="handleSubmit">{{ $t('common.confirm') }}</NButton>
      </NSpace>
    </NForm>
  </NModal>
</template>

<style scoped></style>
