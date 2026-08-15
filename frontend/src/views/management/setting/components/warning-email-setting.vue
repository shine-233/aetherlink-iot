<!--
告警邮箱配置组件，负责维护系统告警通知邮件接收名单。
核心链路：加载当前告警邮箱列表 -> 在文本域中按逗号/分号/换行编辑邮箱 -> 本地去重与格式校验 -> 保存后以服务端返回结果为准重新同步。
静态维护重点：
1. 这里维护的是全局告警接收名单，不是一次性输入框，修改后会直接影响告警通知触达范围。
2. 当前解析逻辑把邮箱统一转成小写并去重，后续如果要支持备注、姓名或分组，必须先重做数据结构而不是继续复用纯文本方案。
3. 邮箱格式只做基础正则校验，后续若要增强交互，可优先补逐项标签化输入和更明确的错误定位。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { fetchWarningEmails, updateWarningEmails } from '@/service/api/personal-center'
import { $t } from '@/locales'
import { useAuthStore } from '@/store/modules/auth'

type LabelKey =
  | 'emails'
  | 'placeholder'
  | 'note'
  | 'help'
  | 'scope'
  | 'deviceAlerts'
  | 'readOnly'
  | 'reload'
  | 'reset'
  | 'save'
  | 'saved'
  | 'invalidEmail'

const emailPattern = /^[^\s@,;]+@[^\s@,;]+\.[^\s@,;]+$/

const t = (key: LabelKey) => $t(`custom.management.warningEmail.${key}`)
const authStore = useAuthStore()

const loading = ref(false)
const saving = ref(false)
const emailText = ref('')
const savedEmails = ref<string[]>([])
const notificationScopeChecked = true
const canEditWarningEmails = computed(() => {
  const roles = Array.isArray(authStore.userInfo?.roles) ? authStore.userInfo.roles : []
  const authority = String(authStore.userInfo?.authority || '').trim()
  return roles.includes('SYS_ADMIN') || roles.includes('TENANT_ADMIN') || authority === 'SYS_ADMIN' || authority === 'TENANT_ADMIN'
})

// 支持按换行、逗号、分号混合输入，并统一做去重与小写归一化。
function parseEmails() {
  const result: string[] = []
  const seen = new Set<string>()
  for (const item of emailText.value.split(/[\n,;]+/)) {
    const email = item.trim().toLowerCase()
    if (!email || seen.has(email)) continue
    seen.add(email)
    result.push(email)
  }
  return result
}

// 以服务端保存结果为准回写文本域，避免本地格式化与后端最终存储不一致。
function syncEmails(emails: string[]) {
  savedEmails.value = emails
  emailText.value = emails.join(', ')
}

// 页面初始化与用户主动刷新都复用同一加载入口。
async function loadWarningEmails() {
  loading.value = true
  try {
    const { error, data } = await fetchWarningEmails()
    if (!error) syncEmails(Array.isArray(data) ? data : [])
  } finally {
    loading.value = false
  }
}

// 重置只回退到最近一次成功保存或加载的邮箱名单。
function resetWarningEmails() {
  emailText.value = savedEmails.value.join(', ')
}

// 保存前先完成本地格式校验，避免把明显无效的邮箱名单提交给后端。
async function saveWarningEmails() {
  if (!canEditWarningEmails.value) {
    return
  }
  const emails = parseEmails()
  const invalid = emails.find(email => !emailPattern.test(email))
  if (invalid) {
    window.$message?.error(`${t('invalidEmail')}: ${invalid}`)
    return
  }

  saving.value = true
  try {
    const { error, data } = await updateWarningEmails({ emails })
    if (!error) {
      syncEmails(Array.isArray(data) ? data : emails)
      window.$message?.success(t('saved'))
    }
  } finally {
    saving.value = false
  }
}

onMounted(loadWarningEmails)
</script>

<template>
  <NSpin :show="loading">
    <NFlex vertical :size="16" class="warning-email-setting">
      <NAlert type="info" :show-icon="false">
        {{ t('note') }}
      </NAlert>
      <NText v-if="!canEditWarningEmails" :depth="3">{{ t('readOnly') }}</NText>
      <NForm label-placement="left" :label-width="140">
        <NFormItem :label="t('emails')">
          <NInput
            v-model:value="emailText"
            type="textarea"
            :placeholder="t('placeholder')"
            :autosize="{ minRows: 4, maxRows: 8 }"
            clearable
            :disabled="!canEditWarningEmails"
          />
        </NFormItem>
        <NFormItem :label="t('scope')">
          <NCheckbox :checked="notificationScopeChecked" disabled>
            {{ t('deviceAlerts') }}
          </NCheckbox>
        </NFormItem>
        <NFormItem>
          <NFlex vertical :size="12" class="warning-email-actions">
            <NText :depth="3">{{ t('help') }}</NText>
            <NSpace>
              <NButton :loading="loading" @click="loadWarningEmails">{{ t('reload') }}</NButton>
              <NButton :disabled="!canEditWarningEmails" @click="resetWarningEmails">{{ t('reset') }}</NButton>
              <NButton type="primary" :loading="saving" :disabled="!canEditWarningEmails" @click="saveWarningEmails">
                {{ t('save') }}
              </NButton>
            </NSpace>
          </NFlex>
        </NFormItem>
      </NForm>
    </NFlex>
  </NSpin>
</template>

<style scoped>
.warning-email-setting {
  max-width: 760px;
  padding-top: 12px;
}

.warning-email-actions {
  width: 100%;
}
</style>
