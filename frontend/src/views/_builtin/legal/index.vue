<!--
  文件用途: 承接旧站 /terms 和 /privacy 外部入口。
  核心逻辑: 展示部署方可替换的条款/隐私占位说明，避免登录页协议链接落入 404。
  关键注意事项: 旧站只提供入口链接，没有可迁移的正式正文；不要在这里伪造法律条款。
  重构建议: 若后续有正式文档服务，可把内容来源改为系统设置或 CMS 接口。
-->
<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { $t } from '@/locales'

const props = withDefaults(
  defineProps<{
    type?: 'terms' | 'privacy'
  }>(),
  {
    type: 'terms'
  }
)

const router = useRouter()

const isPrivacy = computed(() => props.type === 'privacy')
const title = computed(() => (isPrivacy.value ? $t('legal.privacyTitle') : $t('legal.termsTitle')))
const description = computed(() =>
  isPrivacy.value ? $t('legal.privacyDescription') : $t('legal.termsDescription')
)

function goBack() {
  router.back()
}

function goLogin() {
  router.push('/login')
}
</script>

<template>
  <main class="legal-page" data-testid="legal-page">
    <NCard class="legal-card" :bordered="false">
      <NSpace vertical size="large">
        <NResult status="info" :title="title" :description="description">
          <template #footer>
            <NSpace justify="center">
              <NButton data-testid="legal-back" @click="goBack">{{ $t('common.back') }}</NButton>
              <NButton data-testid="legal-login" type="primary" @click="goLogin">{{ $t('route.login') }}</NButton>
            </NSpace>
          </template>
        </NResult>
      </NSpace>
    </NCard>
  </main>
</template>

<style scoped>
.legal-page {
  min-height: 100vh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: rgb(var(--base-color));
}

.legal-card {
  width: min(720px, 100%);
}
</style>
