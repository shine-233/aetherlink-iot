<!--
文件用途：渲染头部用户头像和用户菜单。
核心逻辑：根据登录状态生成下拉选项，并处理个人中心、登录注册和退出。
关键注意事项：退出和登录跳转会影响全局认证状态，改动需要验证 token 清理。
重构建议：可把下拉选项生成和动作执行拆开测试。
-->
<script setup lang="ts">
import { computed } from 'vue'
import type { VNode } from 'vue'
import { useRouter } from 'vue-router'
import { useSvgIconRender } from '@aetherlink/hooks'
import { useAuthStore } from '@/store/modules/auth'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
import { resolvePlatformAssetUrl, resolveUserAvatarPath } from '@/utils/auth-user-avatar'
import { useMarketAuth } from '@/views/device/config/composables/use-market-auth'
import SvgIcon from '@/components/custom/svg-icon.vue'
defineOptions({
  name: 'UserAvatar'
})

const authStore = useAuthStore()
const { routerPushByKey, toLogin } = useRouterPush()
const { SvgIconVNode } = useSvgIconRender(SvgIcon)
const { clearToken: clearMarketToken } = useMarketAuth()
const router = useRouter()
const defaultRdiAvatar = '/rdi/default_avatar.png'
const displayAvatarUrl = computed(() => {
  const avatarPath = resolveUserAvatarPath(authStore.userInfo)
  return avatarPath ? resolvePlatformAssetUrl(avatarPath) : defaultRdiAvatar
})
const displayUserName = computed(() => authStore.userInfo.name || authStore.userInfo.userName || '')

function loginOrRegister() {
  toLogin()
}

type DropdownKey = 'personal-center' | 'logout'

type DropdownOption =
  | {
      key: DropdownKey
      label: string
      icon?: () => VNode
    }
  | {
      type: 'divider'
      key: string
    }

const options = computed(() => {
  const opts: DropdownOption[] = [
    {
      label: $t('common.userCenter'),
      key: 'personal-center',
      icon: SvgIconVNode({ icon: 'ph:user-circle', fontSize: 18 })
    },
    {
      type: 'divider',
      key: 'divider'
    },
    {
      label: $t('common.logout'),
      key: 'logout',
      icon: SvgIconVNode({ icon: 'ph:sign-out', fontSize: 18 })
    }
  ]

  return opts
})

function logout() {
  window.$dialog?.info({
    title: $t('common.tip'),
    content: $t('common.logoutConfirm'),
    positiveText: $t('common.confirm'),
    negativeText: $t('common.cancel'),
    onPositiveClick: () => {
      clearMarketToken()
      authStore.requestLogout()
    }
  })
}

function handleDropdown(key: DropdownKey) {
  if (key === 'logout') {
    logout()
  } else if (key === 'personal-center') {
    router.push('/personal-center')
  } else {
    routerPushByKey(key)
  }
}
</script>

<template>
  <NButton v-if="!authStore.isLogin" quaternary @click="loginOrRegister">
    {{ $t('page.login.common.loginOrRegister') }}
  </NButton>
  <NDropdown v-else placement="bottom" trigger="click" :options="options" @select="handleDropdown">
    <div>
      <ButtonIcon>
        <img :src="displayAvatarUrl" alt="User" class="rdi-default-avatar" />
        <span class="text-16px font-medium">{{ displayUserName }}</span>
      </ButtonIcon>
    </div>
  </NDropdown>
</template>

<style scoped>
.rdi-default-avatar {
  width: 28px;
  height: 28px;
  border-radius: 9999px;
  object-fit: cover;
}
</style>
