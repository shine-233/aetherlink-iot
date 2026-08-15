/**
 * 文件用途: Pinia 认证 store，管理 token、登录登出、用户信息和 session 清理。
 * 核心逻辑: 登录后刷新用户信息并初始化授权路由，退出时清理 auth、tab、route 和 ThingsVis token 状态。
 * 关键注意事项: RSA 密码处理、密码策略提示、token storage 和 route 初始化顺序会影响登录后导航。
 * 重构建议: 将密码策略、storage 读写和用户信息 normalize 拆成纯 helper，并覆盖登录失败/退出清理测试。
 */
import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import dayjs from 'dayjs'
import { SetupStoreId } from '@/enum'
import { useLoading } from '@aetherlink/hooks'
import { useRouterPush } from '@/hooks/common/router'
import { fetchGetUserInfo, fetchLogin, logout } from '@/service/api'
import { transformUser } from '@/service/api/auth'
import { localStg } from '@/utils/storage'
import { $t } from '@/locales'
import { createLogger } from '@/utils/logger'
import { generateRandomHexString, validPassword } from '@/utils/common/tool'
import { encryptDataByRsa } from '@/utils/security/rsa-encrypt'
import { useRouteStore } from '../route'
import { useTabStore } from '../tab'
import { clearAuthStorage, getToken, getUserInfo } from './shared'
import { clearThingsVisToken } from '@/utils/thingsvis'

const logger = createLogger('AuthStore')

function isFrontendEncryptionEnabled(rawConfig: string | null) {
  if (!rawConfig) return false

  try {
    const config: unknown = JSON.parse(rawConfig)
    if (
      !Array.isArray(config) ||
      !config.every(
        item =>
          item !== null &&
          typeof item === 'object' &&
          typeof (item as { name?: unknown }).name === 'string' &&
          (typeof (item as { enable_flag?: unknown }).enable_flag === 'string' ||
            (item as { enable_flag?: unknown }).enable_flag === undefined)
      )
    ) {
      return false
    }

    return config.some(item => item.name === 'frontend_res' && item.enable_flag === 'enable')
  } catch {
    return false
  }
}

export const useAuthStore = defineStore(SetupStoreId.Auth, () => {
  const routeStore = useRouteStore()
  const { route, toLogin, redirectFromLogin } = useRouterPush(false)
  const { loading: loginLoading, startLoading, endLoading } = useLoading()

  const token = ref(getToken())

  /** Is login */
  const isLogin = computed(() => Boolean(token.value))

  const userInfo: Api.Auth.UserInfo = reactive(getUserInfo())
  /** Reset auth store */
  async function resetStore() {
    clearAuthStorage()
    clearThingsVisToken()
    token.value = ''
    Object.assign(userInfo, {
      authority: '',
      id: '',
      userId: '',
      userName: '',
      roles: []
    })

    if (!route.value.meta.constant) {
      await toLogin()
    }

    await routeStore.resetStore()
  }

  /**
   * Login
   *
   * @param userName User name
   * @param password Password
   */
  async function login(userName: string, password: string) {
    startLoading()
    try {
      let newP = password
      // 注意: enableZcAndYzm 由多个模块通过原生 localStorage 直接读写，未纳入 localStg 类型管理。
      // 后续可统一迁移到 localStg，并在 StorageType.Local 中声明该字段。
      const enableZcAndYzmRaw = localStorage.getItem('enableZcAndYzm')
      let salt: string | null = null
      if (isFrontendEncryptionEnabled(enableZcAndYzmRaw)) {
        salt = generateRandomHexString(16)
        newP = encryptDataByRsa(password + salt)
      }

      const { data: loginToken } = await fetchLogin(userName, newP, salt ?? null)
      if (!loginToken) {
        await resetStore()
        return
      }

      const { loop, info } = await loginByToken(loginToken)
      if (loop && info) {
        const password_last_updated = info.password_last_updated
        const now = new Date()
        const daysSinceUpdate = dayjs(now).diff(password_last_updated, 'day')
        // 密码不合规或超过 90 天未更新时，仍完成登录流程并跳转，由个人中心引导修改密码
        const tipFunc = async () => {
          await routeStore.initAuthRoute()
          await redirectFromLogin()
        }

        if (!validPassword(password)) {
          tipFunc()
        } else if (!info.password_last_updated || daysSinceUpdate > 90) {
          tipFunc()
        } else {
          await routeStore.initAuthRoute()
          await redirectFromLogin()
        }
      }
    } catch (error) {
      logger.error('[AuthStore] 登录失败:', error instanceof Error ? error.message : error)
      await resetStore()
    } finally {
      endLoading()
    }
  }

  /**
   * enter
   *
   * @param userId userId
   */
  async function enter(userId: string) {
    startLoading()
    const { clearTabs } = useTabStore()
    const { data: loginToken, error } = await transformUser({
      become_user_id: userId
    })

    if (!error) {
      const { info, loop } = await loginByToken(loginToken)

      clearTabs()
      if (loop) {
        await routeStore.initAuthRoute()
        await redirectFromLogin()
        if (routeStore.isInitAuthRoute) {
          window.$notification?.success({
            title: $t('page.login.common.loginSuccess'),
            content: $t('page.login.common.welcomeBack', {
              userName: info?.name
            }),
            duration: 4500
          })
        }
      }
    } else {
      await resetStore()
    }

    endLoading()
  }

  async function loginByToken(loginToken: Api.Auth.LoginToken) {
    // 1. stored in the localStorage, the later requests need it in headers
    localStg.set('token', loginToken.token)
    const expires_in = Date.now() + loginToken.expires_in * 1000
    localStg.set('token_expires_in', expires_in.toString())

    const { data: info, error } = await fetchGetUserInfo()

    if (!error) {
      // 2. store user info
      info.roles = [info.authority]
      localStg.set('userInfo', info)
      // 3. update auth route
      token.value = loginToken.token
      Object.assign(userInfo, info)

      // 4. 清除 ThingsVis token 缓存，确保使用新用户身份重新交换 SSO token
      clearThingsVisToken()

      return { loop: true, info }
    }

    return { loop: false, info }
  }
  async function requestLogout() {
    await logout()
    await resetStore()
  }

  return {
    token,
    userInfo,
    isLogin,
    loginLoading,
    resetStore,
    login,
    enter,
    requestLogout,
    loginByToken
  }
})
