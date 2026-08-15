/**
 * Shared frontend API request client.
 *
 * It selects the current platform API base URL, injects auth/language headers,
 * removes empty query params, and normalizes the backend response envelope.
 */
import { BACKEND_ERROR_CODE, createFlatRequest } from '@aetherlink/axios'
import type { CustomAxiosRequestConfig, FlatRequestInstance } from '@aetherlink/axios'
import { localStg } from '@/utils/storage'
import { $t } from '@/locales'
import { createProxyPattern, createServiceConfig } from '~/env.config'

const { otherBaseURL } = createServiceConfig(import.meta.env)
const isHttpProxy = import.meta.env.VITE_HTTP_PROXY === 'Y'
const platformApiBaseUrl = otherBaseURL.platform || `${window.location.origin}/api/v1`

export const request: FlatRequestInstance = createFlatRequest<App.Service.BackendResponse>(
  {
    baseURL: isHttpProxy ? createProxyPattern() : platformApiBaseUrl
  },
  {
    async onRequest(config) {
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
    async onError(error) {
      if (error?.response?.status === 401) {
        window.$message?.destroyAll()
        window.$message?.error($t('common.loginExpired'))

        setTimeout(() => {
          localStg.remove('token')
          localStg.remove('userInfo')
          localStg.remove('token_expires_in')
          window.location.reload()
        }, 1000)
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
