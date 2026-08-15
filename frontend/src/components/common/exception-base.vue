<!--
  文件用途：渲染通用异常、空状态或错误提示页面。
  核心逻辑：按传入状态展示图标、标题、描述和操作插槽，为 403/404/500 等场景提供统一结构。
  关键注意事项：异常文案通常面向最终用户，调整默认值时需同步本地化和路由兜底场景。
  重构建议：可把状态码到默认文案和图标的映射抽成常量，方便扩展。
-->
<script lang="ts" setup>
import { computed } from 'vue'
import { $t } from '@/locales'
import { useRouterPush } from '@/hooks/common/router'
import { useAuthStore } from '@/store/modules/auth'
import { useMarketAuth } from '@/views/device/config/composables/use-market-auth'
// vite-svg-loader treats a bare SVG import as a Vue component.  These values
// are rendered by a native <img>, so request the URL form explicitly; without
// the query Vite serializes the component object as "[object Object]" and the
// built exception pages lose their illustration at runtime.
import noPermissionIllustration from '@/assets/illustrations/no-permission.svg?url'
import notFoundIllustration from '@/assets/illustrations/not-found.svg?url'
import serviceErrorIllustration from '@/assets/illustrations/service-error.svg?url'

defineOptions({ name: 'ExceptionBase' })

type ExceptionType = '403' | '404' | '500'

interface Props {
  /**
   * Exception type
   *
   * - 403: no permission
   * - 404: not found
   * - 500: service error
   */
  type: ExceptionType
}

const props = defineProps<Props>()

const illustrationMap: Record<ExceptionType, string> = {
  '403': noPermissionIllustration,
  '404': notFoundIllustration,
  '500': serviceErrorIllustration
}

const illustration = computed(() => illustrationMap[props.type])

const { toLogin } = useRouterPush()
const authStore = useAuthStore()
const { clearToken: clearMarketToken } = useMarketAuth()

function logout() {
  window.$dialog?.info({
    title: $t('common.tip'),
    content: $t('common.logoutConfirm'),
    positiveText: $t('common.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: () => {
      clearMarketToken()
      authStore.resetStore()
      toLogin()
    }
  })
}
</script>

<template>
  <div class="min-h-520px wh-full flex-vertical-center gap-24px overflow-hidden">
    <ButtonIcon class="position-absolute position-right-2xl position-top-2xl" @click="logout">
      <SvgIcon icon="ph:sign-out" class="text-icon-medium" />
      <span class="text-14px font-medium">{{ $t('common.logout') }}</span>
    </ButtonIcon>
    <div class="flex w-400px max-w-full">
      <img :src="illustration" alt="" class="h-auto w-full object-contain" />
    </div>
    <RouterLink to="/">
      <NButton type="primary">{{ $t('common.backToHome') }}</NButton>
    </RouterLink>
  </div>
</template>

<style scoped></style>
