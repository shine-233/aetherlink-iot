<!--
  文件用途：个人中心 2FA（TOTP）设置卡片——启用/激活（一次性恢复码）/停用。
  边界说明：secret 与恢复码仅在本组件会话内展示一次；接口见 service/api/two-factor.ts。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { NButton, NInput, NAlert } from 'naive-ui'
import { $t } from '@/locales'
import {
  fetchTotpActivate,
  fetchTotpDisable,
  fetchTotpSetup,
  fetchTotpStatus
} from '@/service/api/two-factor'

const enabled = ref(false)
const loading = ref(true)
const setup = ref<{ secret: string; uri: string } | null>(null)
const code = ref('')
const recoveryCodes = ref<string[]>([])
const busy = ref(false)
const errorMsg = ref('')

async function loadStatus() {
  loading.value = true
  const { data } = await fetchTotpStatus()
  enabled.value = Boolean(data?.enabled)
  loading.value = false
}

async function handleSetup() {
  errorMsg.value = ''
  const { data, error } = await fetchTotpSetup()
  if (error) {
    errorMsg.value = error instanceof Error ? error.message : String(error ?? '')
    return
  }
  setup.value = data
}

async function handleActivate() {
  if (!code.value.trim()) return
  busy.value = true
  errorMsg.value = ''
  const { data, error } = await fetchTotpActivate(code.value.trim())
  busy.value = false
  if (error) {
    errorMsg.value = error instanceof Error ? error.message : String(error ?? '')
    return
  }
  recoveryCodes.value = data?.codes ?? []
  enabled.value = true
  setup.value = null
  code.value = ''
}

async function handleDisable() {
  if (!code.value.trim()) return
  busy.value = true
  errorMsg.value = ''
  const { error } = await fetchTotpDisable(code.value.trim())
  busy.value = false
  if (error) {
    errorMsg.value = error instanceof Error ? error.message : String(error ?? '')
    return
  }
  enabled.value = false
  code.value = ''
}

onMounted(loadStatus)
</script>

<template>
  <div class="two-factor-setting">
    <NAlert v-if="errorMsg" type="error" :show-icon="false" class="mb-12px">
      {{ errorMsg }}
    </NAlert>

    <template v-if="!loading && !enabled && !setup">
      <div class="flex items-center gap-16px">
        <span>{{ $t('custom.twoFactor.totpStatus') }}: {{ $t('custom.twoFactor.disabled') }}</span>
        <NButton type="primary" size="small" @click="handleSetup">
          {{ $t('custom.twoFactor.enable') }}
        </NButton>
      </div>
    </template>

    <template v-if="!loading && setup">
      <div class="mb-8px">{{ $t('custom.twoFactor.scanHint') }}</div>
      <NInput :value="setup.uri" type="textarea" readonly class="mb-8px" />
      <div class="flex items-center gap-8px mb-8px">
        <NInput v-model:value="code" :placeholder="$t('custom.twoFactor.codePlaceholder')" style="width: 240px" />
        <NButton type="primary" size="small" :loading="busy" @click="handleActivate">
          {{ $t('custom.twoFactor.activate') }}
        </NButton>
      </div>
      <NAlert v-if="recoveryCodes.length" type="warning" :show-icon="false">
        {{ $t('custom.twoFactor.recoveryHint') }}
        <div class="font-mono text-13px">{{ recoveryCodes.join('  ') }}</div>
      </NAlert>
    </template>

    <template v-if="!loading && enabled">
      <div class="flex items-center gap-8px">
        <span>{{ $t('custom.twoFactor.totpStatus') }}: {{ $t('custom.twoFactor.enabled') }}</span>
        <NInput v-model:value="code" :placeholder="$t('custom.twoFactor.codePlaceholder')" style="width: 240px" />
        <NButton size="small" :loading="busy" @click="handleDisable">
          {{ $t('custom.twoFactor.disable') }}
        </NButton>
      </div>
    </template>
  </div>
</template>
