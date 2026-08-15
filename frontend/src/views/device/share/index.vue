<!--
  文件用途: 设备分享链接接受页。
  核心逻辑: 从 route query 读取 share_token，调用 RDI 分享接受接口，并展示成功、已拥有、已分享或错误状态。
  关键注意事项: share_token 为空、重复接受和权限边界是外部入口的关键失败分支。
  重构建议: 将结果状态 normalize 成纯函数，并用稳定 data-testid 覆盖成功和失败路径。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { acceptRdiSharedDevice } from '@/service/api/rdi'
import { $t } from '@/locales'

type ShareStatus = 'loading' | 'success' | 'error'

const route = useRoute()
const router = useRouter()
const status = ref<ShareStatus>('loading')
const errorMessage = ref('')
const deviceId = ref('')
const alreadyAccepted = ref(false)
const sharedWithMe = ref(false)

const shareToken = computed(() => {
  const raw = route.query.share_token
  return Array.isArray(raw) ? raw[0] || '' : raw || ''
})

const resultTitle = computed(() => {
  if (alreadyAccepted.value) {
    return sharedWithMe.value ? $t('rdi.share.alreadyShared') : $t('rdi.share.alreadyOwned')
  }
  return $t('rdi.share.success')
})

const resultDescription = computed(() => {
  if (sharedWithMe.value) {
    return $t('rdi.share.successDescription')
  }
  return $t('rdi.share.alreadyOwnedDescription')
})

const showSharedWithMeAction = computed(() => sharedWithMe.value)

async function acceptShare() {
  status.value = 'loading'
  errorMessage.value = ''
  deviceId.value = ''
  alreadyAccepted.value = false
  sharedWithMe.value = false

  if (!shareToken.value) {
    status.value = 'error'
    errorMessage.value = $t('rdi.share.missingToken')
    return
  }

  try {
    const { data } = await acceptRdiSharedDevice(shareToken.value)
    if (!data) {
      throw new Error($t('rdi.share.failedDescription'))
    }
    deviceId.value = data.device?.device_id || ''
    alreadyAccepted.value = Boolean(data.already_accepted)
    sharedWithMe.value = Boolean(data.shared_with_me)
    status.value = 'success'
  } catch (error: any) {
    status.value = 'error'
    errorMessage.value = error?.error?.message || error?.message || $t('rdi.share.failedDescription')
  }
}

function goBack() {
  router.back()
}

function goDeviceDetails() {
  if (!deviceId.value) return
  router.push({ name: 'device_details', query: { d_id: deviceId.value } })
}

function goSharedWithMe() {
  router.push({
    path: '/device/shared-with-me',
    query: deviceId.value ? { device_id: deviceId.value } : {}
  })
}

onMounted(() => {
  acceptShare()
})
</script>

<template>
  <div class="share-page" data-testid="share-page">
    <NCard class="share-card" :bordered="false">
      <NSpace vertical size="large" align="center">
        <div class="share-header">
          <NButton data-testid="share-back" @click="goBack">{{ $t('rdi.share.back') }}</NButton>
          <NButton data-testid="share-refresh" :loading="status === 'loading'" @click="acceptShare">
            {{ $t('rdi.share.refresh') }}
          </NButton>
        </div>

        <NSpin v-if="status === 'loading'" data-testid="share-loading" size="large" />

        <div v-else-if="status === 'success'" data-testid="share-success" :data-already-accepted="alreadyAccepted">
          <NResult
            status="success"
            :title="resultTitle"
            :description="resultDescription"
          >
            <template #footer>
              <NSpace justify="center">
                <NButton data-testid="share-open-device" type="primary" :disabled="!deviceId" @click="goDeviceDetails">
                  {{ $t('rdi.share.openDevice') }}
                </NButton>
                <NButton v-if="showSharedWithMeAction" data-testid="share-open-shared-with-me" @click="goSharedWithMe">
                  {{ $t('rdi.share.shareToMe') }}
                </NButton>
              </NSpace>
            </template>
          </NResult>
        </div>

        <div v-else data-testid="share-error">
          <NResult status="error" :title="$t('rdi.share.failed')" :description="errorMessage">
            <template #footer>
              <NSpace justify="center">
                <NButton data-testid="share-retry" type="primary" @click="acceptShare">{{ $t('rdi.share.retry') }}</NButton>
                <NButton data-testid="share-error-open-shared-with-me" @click="goSharedWithMe">
                  {{ $t('rdi.share.shareToMe') }}
                </NButton>
              </NSpace>
            </template>
          </NResult>
        </div>
      </NSpace>
    </NCard>
  </div>
</template>

<style scoped>
.share-page {
  display: flex;
  min-height: 60vh;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.share-card {
  width: min(560px, 100%);
}

.share-header {
  display: flex;
  width: 100%;
  justify-content: space-between;
  gap: 12px;
}
</style>
