/**
 * 文件用途：生成请求层默认选项、Axios 配置和重试配置。
 * 核心逻辑：合并调用方选项，统一参数序列化、超时和 axios-retry 的成功/失败判断。
 * 关键注意事项：默认业务成功判定当前返回 true，实际项目应由调用方按接口协议覆盖。
 * 重构建议：后续可拆分默认值、序列化策略和重试策略，并为边界配置补测试。
 */
import type { AxiosError, CreateAxiosDefaults } from 'axios'
import type { IAxiosRetryConfig } from 'axios-retry'
import { isNetworkOrIdempotentRequestError } from 'axios-retry'
import { stringify } from 'qs'
import { isHttpSuccess } from './shared'
import type { RequestOption } from './type'

/**
 * 主动取消（AbortController / CancelToken）不是失败，不能被重试。
 *
 * axios-retry 默认的 isNetworkOrIdempotentRequestError 会把"无响应 + 有 code"的
 * ERR_CANCELED 当成网络错误重试 3 次，等于把用户取消的列表查询又发了三遍。
 */
export function isRetryableError(error: AxiosError): boolean {
  if (!error || typeof error !== 'object') return false

  if (error.code === 'ERR_CANCELED' || error.name === 'CanceledError' || error.name === 'AbortError') return false

  return isNetworkOrIdempotentRequestError(error)
}

export function createDefaultOptions<ResponseData = any>(options?: Partial<RequestOption<ResponseData>>) {
  const opts: RequestOption<ResponseData> = {
    onRequest: async (config) => config,
    isBackendSuccess: (_response) => true,
    onBackendFail: async () => {},
    transformBackendResponse: async (response) => response.data,
    onError: async () => {}
  }

  Object.assign(opts, options)

  return opts
}

export function createRetryOptions(config?: Partial<CreateAxiosDefaults>) {
  const retryConfig: IAxiosRetryConfig = {
    retries: 3,
    retryCondition: isRetryableError
  }

  Object.assign(retryConfig, config)

  return retryConfig
}

export function createAxiosConfig(config?: Partial<CreateAxiosDefaults>) {
  const TEN_SECONDS = 25 * 1000

  const axiosConfig: CreateAxiosDefaults = {
    timeout: TEN_SECONDS,
    headers: {
      'Content-Type': 'application/json'
    },
    validateStatus: isHttpSuccess,
    paramsSerializer: (params) => {
      return stringify(params)
    }
  }

  Object.assign(axiosConfig, config)

  return axiosConfig
}
