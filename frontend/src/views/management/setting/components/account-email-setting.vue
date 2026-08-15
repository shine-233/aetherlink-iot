<!--
账号邮箱修改组件，负责展示当前登录邮箱、发送验证码并提交新邮箱变更。
核心链路：读取 authStore 当前邮箱 -> 向当前邮箱发送验证码 -> 输入新邮箱与验证码 -> 提交修改 -> 回写本地登录态并展示迁移设备数。
静态维护重点：
1. 修改成功后会同步更新 `authStore.userInfo` 和本地 `userInfo` 缓存，后续若登录态结构变化，这里必须同步调整。
2. 当前发送验证码使用的是“当前邮箱”，不是“新邮箱”，这是账户安全策略的一部分，改交互时不要误改成新邮箱验证。
3. `devices_migrated` 会影响用户对变更影响面的理解，后续若后端继续扩展迁移反馈，应在这里保留清晰展示。
-->
<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import useCountDown from '@/hooks/business/use-count-down'
import { useAuthStore } from '@/store/modules/auth'
import { fetchEmailCodeByEmail } from '@/service/api/auth'
import { changeAccountEmail } from '@/service/api/personal-center'
import { localStg } from '@/utils/storage'
import { $t } from '@/locales'

type LabelKey =
  | 'title'
  | 'currentEmail'
  | 'newEmail'
  | 'verifyCode'
  | 'sendCode'
  | 'change'
  | 'reset'
  | 'note'
  | 'newEmailRequired'
  | 'verifyCodeRequired'
  | 'sameEmailNotAllowed'
  | 'sent'
  | 'changed'
  | 'changedWithCount'
  | 'devicesMigrated'
  | 'devicesRetainedDetail'

const authStore = useAuthStore()
const t = (key: LabelKey) => $t(`custom.management.accountEmail.${key}`)

const form = reactive({
  new_email: '',
  verify_code: ''
})

const codeLoading = ref(false)
const submitLoading = ref(false)
const migratedDeviceCount = ref<number | null>(null)
const { counts: codeCountdown, isCounting: codeCounting, start: startCodeCountdown } = useCountDown(60)

const currentEmail = computed(() => String(authStore.userInfo.email || authStore.userInfo.userEmail || ''))
const sendCodeLabel = computed(() => (codeCounting.value ? `${codeCountdown.value}s` : t('sendCode')))

// 重置只清空待修改字段，不覆盖当前登录邮箱快照。
function resetForm() {
  form.new_email = ''
  form.verify_code = ''
  migratedDeviceCount.value = null
}

// 验证码发送给当前绑定邮箱，用于证明这次邮箱变更由现有账号持有人发起。
async function sendCode() {
  const newEmail = form.new_email.trim()
  if (!newEmail) {
    window.$message?.error(t('newEmailRequired'))
    return
  }
  const email = currentEmail.value.trim()
  if (email && newEmail.toLowerCase() === email.toLowerCase()) {
    window.$message?.error(t('sameEmailNotAllowed'))
    return
  }
  if (!email) {
    window.$message?.error(t('currentEmail'))
    return
  }

  codeLoading.value = true
  try {
    const { error } = await fetchEmailCodeByEmail(email)
    if (!error) {
      startCodeCountdown()
      window.$message?.success(t('sent'))
    }
  } finally {
    codeLoading.value = false
  }
}

// 提交成功后同时刷新本地登录态邮箱，避免页面其余位置继续展示旧邮箱。
async function submitChange() {
  const newEmail = form.new_email.trim()
  const verifyCode = form.verify_code.trim()
  const email = currentEmail.value.trim()
  if (!newEmail) {
    window.$message?.error(t('newEmailRequired'))
    return
  }
  if (!verifyCode) {
    window.$message?.error(t('verifyCodeRequired'))
    return
  }
  if (email && newEmail.toLowerCase() === email.toLowerCase()) {
    window.$message?.error(t('sameEmailNotAllowed'))
    return
  }

  submitLoading.value = true
  try {
    const { error, data } = await changeAccountEmail({
      new_email: newEmail,
      verify_code: verifyCode
    })
    if (!error) {
      const changedEmail = data?.new_email || newEmail
      migratedDeviceCount.value = typeof data?.devices_migrated === 'number' ? data.devices_migrated : null
      authStore.userInfo.email = changedEmail
      authStore.userInfo.userEmail = changedEmail
      localStg.set('userInfo', { ...authStore.userInfo })
      form.new_email = ''
      form.verify_code = ''
      window.$message?.success(
        migratedDeviceCount.value === null
          ? t('changed')
          : $t('custom.management.accountEmail.changedWithCount', { count: migratedDeviceCount.value })
      )
    }
  } finally {
    submitLoading.value = false
  }
}
</script>

<template>
  <NFlex vertical :size="16" class="account-email-setting">
    <NAlert type="info" :show-icon="false">
      {{ t('note') }}
    </NAlert>

    <NForm label-placement="left" label-width="140px">
      <NFormItem :label="t('currentEmail')">
        <NInput :value="currentEmail" readonly />
      </NFormItem>
      <NFormItem :label="t('newEmail')">
        <NInput v-model:value="form.new_email" placeholder="name@example.com" />
      </NFormItem>
      <NFormItem :label="t('verifyCode')">
        <div class="account-email-code">
          <NInput v-model:value="form.verify_code" />
          <NButton :loading="codeLoading" :disabled="codeCounting" @click="sendCode">{{ sendCodeLabel }}</NButton>
        </div>
      </NFormItem>
      <NFormItem>
        <NFlex align="center" :size="12">
          <NButton type="primary" :loading="submitLoading" @click="submitChange">{{ t('change') }}</NButton>
          <NButton @click="resetForm">{{ t('reset') }}</NButton>
          <NText v-if="migratedDeviceCount !== null" depth="2">
            {{ t('devicesMigrated') }}: {{ migratedDeviceCount }}
          </NText>
          <NText v-if="migratedDeviceCount !== null" depth="3">
            {{ t('devicesRetainedDetail') }}
          </NText>
        </NFlex>
      </NFormItem>
    </NForm>
  </NFlex>
</template>

<style scoped>
.account-email-setting {
  max-width: 760px;
}

.account-email-code {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 8px;
  width: 100%;
}
</style>
