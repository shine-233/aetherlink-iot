/**
 * Shared frontend API request client.
 *
 * It selects the current platform API base URL, injects auth/language headers,
 * removes empty query params, and normalizes the backend response envelope.
 */
import { BACKEND_ERROR_CODE, createFlatRequest } from '@aetherlink/axios'
import type { CustomAxiosRequestConfig, FlatRequestInstance } from '@aetherlink/axios'
import { $t } from '@/locales'
import { clearAuthStorage } from '@/store/modules/auth/shared'
import { localStg } from '@/utils/storage'
import { createProxyPattern, createServiceConfig } from '~/env.config'
import { refreshAuthToken, scheduleProactiveTokenRefresh } from './auth-refresh'

const { otherBaseURL } = createServiceConfig(import.meta.env)
const isHttpProxy = import.meta.env.VITE_HTTP_PROXY === 'Y'
const platformApiBaseUrl = otherBaseURL.platform || `${window.location.origin}/api/v1`

// 后端业务码约定：HTTP 401 + 40102 表示 token 过期（可静默续签），其余 401 视为无效凭证直接登出
const TOKEN_EXPIRED_BUSINESS_CODE = 40102

export const request: FlatRequestInstance = createFlatRequest<App.Service.BackendResponse>(
  {
    baseURL: isHttpProxy ? createProxyPattern() : platformApiBaseUrl
  },
  {
    async onRequest(config) {
      // fire-and-forget：惰性主动续签检查，节流后异步执行，不阻塞当前请求
      scheduleProactiveTokenRefresh()

      const { params } = config
      const headers: Record<string, string> = {}
      const token = localStg.get('token')
      const userLanguage = localStg.get('lang')

      if (token) {
        headers['x-token'] = token
      }

      if (userLanguage) {
        headers['Accept-Language'] = userLanguage
      }

      Object.assign(config.headers, headers)

      if (
        params &&
        typeof params === 'object' &&
        !Array.isArray(params) &&
        (Object.getPrototypeOf(params) === Object.prototype || Object.getPrototypeOf(params) === null)
      ) {
        const normalizedParams = { ...params }
        Object.keys(normalizedParams).forEach(key => {
          if (normalizedParams[key] === '') {
            normalizedParams[key] = undefined
          }
        })
        config.params = normalizedParams
      }

      return config
    },
    isBackendSuccess(response) {
      return response.data.code === 200
    },
    async onBackendFail(_response) {
      // Non-200 backend business codes are surfaced through onError.
    },
    transformBackendResponse(response) {
      if (response.config.method !== 'get') {
        window.$message?.destroyAll()
      }

      if ((response.config as CustomAxiosRequestConfig).needMessage) {
        return response.data
      }

      return response.data.data
    },
    async onError(error, instance) {
      if (error?.response?.status === 401) {
        const config = error.config as CustomAxiosRequestConfig | undefined
        const bizCode = error.response?.data?.code

        // 仅当业务码为 40102(token 过期) 且请求未被重试过时，先做一次静默续签并重放
        if (bizCode === TOKEN_EXPIRED_BUSINESS_CODE && config && !config._retry) {
          config._retry = true
          const refreshed = await refreshAuthToken()

          if (refreshed) {
            // 用原始实例重放一次：拦截器会重新注入最新 token；
            // 若重放仍返回 40102，_retry 已置位将直接进入登出兜底，不会循环
            return instance.request(config)
          }
        }

        // 续签失败、无效凭证(40101 等)或已重试过：登出兜底（替代旧版 toast + 整页 reload）
        window.$message?.destroyAll()
        window.$message?.error($t('common.loginExpired'))
        clearAuthStorage()
        redirectToLogin()
        return
      }

      if ((error.config as CustomAxiosRequestConfig | undefined)?.silentError) {
        return
      }

      let message = error.message
      if (error.response?.status === 404) {
        window.$message?.destroyAll()
        window.$message?.error($t('common.resourceNotFound'))
        return
      }

      // navigator.onLine cannot prove API reachability; use Axios' no-response network contract instead.
      if (error.code === 'ERR_NETWORK' && !error.response) {
        message = $t('common.networkError')
      }

      if (error.code === BACKEND_ERROR_CODE) {
        message = error.response?.data?.message || message
      }

      window.$message?.destroyAll()
      window.$message?.error(message)
    }
  }
)

/**
 * 401 登出兜底的跳转：携带当前路径作为 redirect 参数，与 useRouterPush 的
 * redirectFromLogin（route.query.redirect）约定保持一致。
 */
function redirectToLogin() {
  const currentFullPath = `${window.location.pathname}${window.location.search}`
  window.location.href = `/login?redirect=${encodeURIComponent(currentFullPath)}`
}
