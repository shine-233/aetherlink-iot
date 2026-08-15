<!--
市场登录弹窗，负责物模型市场相关页面在安装/发布前补充市场账号登录。
核心链路：打开弹窗 -> 输入市场用户名密码 -> 调用市场登录接口 -> 将 token 写入 useMarketAuth -> 通知上层继续挂起动作。
静态维护重点：
1. 登录成功后只保存 token，不保存用户资料；后续若市场页面需要展示当前登录人，建议补用户信息获取链路。
2. 错误提示依赖 axios 拦截器统一处理，组件内部不重复弹窗；调整请求层时要同步检查这里的静默假设。
3. 注册入口通过当前站点端口推导 `:18083/register`，若部署拓扑变化，这里要和部署文档一起更新。
4. 当前弹窗只处理“获取登录 token”这一步，不承担登录后市场上下文预热或权限校验。
-->
<script setup lang="ts">
import { ref, reactive } from 'vue'
import { NModal, NForm, NFormItem, NInput, NButton } from 'naive-ui'
import { $t } from '@/locales'
import { marketLogin } from '@/service/api/market'
import { useMarketAuth } from '../composables/use-market-auth'

const { setToken } = useMarketAuth()

const emit = defineEmits(['login-success'])

const visible = ref(false)
const loading = ref(false)
const formRef = ref<any>(null)

const loginForm = reactive({
  username: '',
  password: ''
})

const loginRules = {
  username: { required: true, message: () => $t('market.usernamePlaceholder'), trigger: 'blur' },
  password: { required: true, message: $t('market.password'), trigger: 'blur' }
}

// 每次打开都清空输入，避免多个安装/发布动作之间复用旧账号密码。
// 这也意味着“记住上次输入”的体验并不在本组件职责范围内。
const open = () => {
  loginForm.username = ''
  loginForm.password = ''
  visible.value = true
}

// 登录成功后把 token 交给 useMarketAuth，由上层根据 `login-success` 事件继续后续动作。
// 静态审查提示: token 既可能位于 `res.token`，也可能位于 `res.data.token`，
// 说明接口返回形状存在兼容分支，后续若统一 API 封装可考虑把兼容逻辑下沉到服务层。
const handleLogin = async () => {
  await formRef.value?.validate()
  loading.value = true
  try {
    const res: any = await marketLogin({ username: loginForm.username, password: loginForm.password })
    const token = res?.token || res?.data?.token
    if (token) {
      setToken(token)
      window.$message?.success($t('market.loginSuccess'))
      visible.value = false
      emit('login-success', token)
    }
  } catch (e: any) {
    // error toast 已由 axios 拦截器 onError 统一处理，无需重复弹窗
  } finally {
    loading.value = false
  }
}

// 注册页地址目前根据部署约定从当前 origin 推导，适合单一 Portal 端口部署场景。
// 若后续接入网关、反向代理或独立市场域名，这里的地址拼装建议集中收口到配置层。
const handleGoToRegister = () => {
  const marketUrl = window.location.origin.replace(/:\d+$/, ':18083') + '/register'
  window.open(marketUrl, '_blank')
}

defineExpose({ open })
</script>

<template>
  <NModal v-model:show="visible" preset="dialog" :title="$t('market.loginTitle')" style="width: 420px">
    <NForm ref="formRef" :model="loginForm" :rules="loginRules" label-placement="left" label-width="80">
      <NFormItem :label="$t('market.username')" path="username">
        <NInput v-model:value="loginForm.username" :placeholder="$t('market.usernamePlaceholder')" />
      </NFormItem>
      <NFormItem :label="$t('market.password')" path="password">
        <NInput
          v-model:value="loginForm.password"
          type="password"
          :placeholder="$t('market.password')"
          show-password-on="click"
        />
      </NFormItem>
    </NForm>
    <div style="margin-top: 10px; text-align: right">
      <span>{{ $t('market.noAccount') }}</span>
      <NButton text type="primary" style="margin-left: 4px" @click="handleGoToRegister">
        {{ $t('market.goToRegister') }}
      </NButton>
    </div>
    <template #action>
      <NButton @click="visible = false">{{ $t('common.cancel') }}</NButton>
      <NButton type="primary" :loading="loading" @click="handleLogin">{{ $t('market.login') }}</NButton>
    </template>
  </NModal>
</template>
