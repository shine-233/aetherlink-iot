/**
 * 文件用途：创建并导出 AetherLink 的扁平 Axios 请求实例。
 * 核心逻辑：组装默认配置、请求/响应拦截、业务成功判定、错误映射、重试和取消能力。
 * 关键注意事项：这里是请求链路主入口，拦截器顺序和错误抛出语义会直接影响上层业务。
 * 重构建议：可逐步拆出错误归一化、取消控制和响应映射逻辑，降低入口文件复杂度。
 */
import axios, { AxiosError } from 'axios'
import type { AxiosResponse, CancelTokenSource, CreateAxiosDefaults, InternalAxiosRequestConfig } from 'axios'
import axiosRetry from 'axios-retry'
import { nanoid } from '@aetherlink/utils'
import { createAxiosConfig, createDefaultOptions, createRetryOptions } from './options'
import { BACKEND_ERROR_CODE, REQUEST_ID_KEY } from './constant'
import type {
  CustomAxiosRequestConfig,
  FlatRequestInstance,
  MappedType,
  RequestInstance,
  RequestOption,
  ResponseType
} from './type'

function createCommonRequest<ResponseData = any>(
  axiosConfig?: CreateAxiosDefaults,
  options?: Partial<RequestOption<ResponseData>>
) {
  const opts = createDefaultOptions<ResponseData>(options)
  const axiosConf = createAxiosConfig(axiosConfig)
  const instance = axios.create(axiosConf)
  const cancelTokenSourceMap = new Map<string, CancelTokenSource>()

  axiosRetry(instance, createRetryOptions(axiosConf))

  instance.interceptors.request.use(conf => {
    const config: InternalAxiosRequestConfig = { ...conf }
    const requestId = nanoid()

    config.headers.set(REQUEST_ID_KEY, requestId)

    const cancelTokenSource = axios.CancelToken.source()
    config.cancelToken = cancelTokenSource.token
    cancelTokenSourceMap.set(requestId, cancelTokenSource)

    return opts.onRequest?.(config) || config
  })

  instance.interceptors.response.use(
    async response => {
      if (opts.isBackendSuccess(response)) {
        return Promise.resolve(response)
      }

      const fail = await opts.onBackendFail(response, instance)
      if (fail) {
        return fail
      }

      const backendError = new AxiosError<ResponseData>(
        'the backend request error',
        BACKEND_ERROR_CODE,
        response.config,
        response.request,
        response
      )

      const recoveredResponse = await opts.onError(backendError, instance)
      if (recoveredResponse) {
        return recoveredResponse
      }

      return Promise.reject(backendError)
    },
    async (error: AxiosError<ResponseData>) => {
      const recoveredResponse = await opts.onError(error, instance)
      if (recoveredResponse) {
        return recoveredResponse
      }

      return Promise.reject(error)
    }
  )

  function cancelRequest(requestId: string) {
    const cancelTokenSource = cancelTokenSourceMap.get(requestId)
    if (cancelTokenSource) {
      cancelTokenSource.cancel()
      cancelTokenSourceMap.delete(requestId)
    }
  }

  function cancelAllRequest() {
    cancelTokenSourceMap.forEach(cancelTokenSource => {
      cancelTokenSource.cancel()
    })
    cancelTokenSourceMap.clear()
  }

  return {
    instance,
    opts,
    cancelRequest,
    cancelAllRequest
  }
}

export function createRequest<ResponseData = any>(
  axiosConfig?: CreateAxiosDefaults,
  options?: Partial<RequestOption<ResponseData>>
) {
  const { instance, opts, cancelRequest, cancelAllRequest } = createCommonRequest<ResponseData>(axiosConfig, options)

  const request: RequestInstance = async function request<T = any, R extends ResponseType = 'json'>(
    config: CustomAxiosRequestConfig
  ) {
    const response: AxiosResponse<ResponseData> = await instance(config)
    const responseType = response.config?.responseType || 'json'

    if (responseType === 'json') {
      return opts.transformBackendResponse(response)
    }

    return response.data as MappedType<R, T>
  } as RequestInstance

  Object.assign(request, {
    async get<T = any, R extends ResponseType = 'json'>(url: string, config?: CustomAxiosRequestConfig<R>) {
      return request<T, R>({ ...config, url, method: 'get' })
    },
    async post<T = any, R extends ResponseType = 'json'>(
      url: string,
      data?: any,
      config?: CustomAxiosRequestConfig<R>
    ) {
      return request<T, R>({ ...config, url, data, method: 'post' })
    },
    async put<T = any, R extends ResponseType = 'json'>(url: string, data?: any, config?: CustomAxiosRequestConfig<R>) {
      return request<T, R>({ ...config, url, data, method: 'put' })
    },
    async delete<T = any, R extends ResponseType = 'json'>(url: string, config?: CustomAxiosRequestConfig<R>) {
      return request<T, R>({ ...config, url, method: 'delete' })
    },
    async delete2<T = any, R extends ResponseType = 'json'>(
      url: string,
      data?: any,
      config?: CustomAxiosRequestConfig<R>
    ) {
      return request<T, R>({ ...config, url, data, method: 'delete' })
    },
    cancelRequest,
    cancelAllRequest
  })

  return request
}

export { BACKEND_ERROR_CODE, REQUEST_ID_KEY }
export type * from './type'
export type * from './options'
export type * from './constant'
export type * from './shared'

export function createFlatRequest<ResponseData = any>(
  axiosConfig?: CreateAxiosDefaults,
  options?: Partial<RequestOption<ResponseData>>
) {
  const { instance, opts, cancelRequest, cancelAllRequest } = createCommonRequest<ResponseData>(axiosConfig, options)

  instance.interceptors.request.use(config => {
    if (config.params) {
      config.params = Object.entries(config.params).reduce((acc: Record<string, any>, [key, value]) => {
        if (value !== null) {
          acc[key] = value
        }
        return acc
      }, {})
    }
    if (config.data) {
      config.data = Object.entries(config.data).reduce((acc: Record<string, any>, [key, value]) => {
        if (value !== null) {
          acc[key] = value
        }
        return acc
      }, {})
    }
    return config
  })

  const flatRequest: FlatRequestInstance = async function flatRequest<T = any, R extends ResponseType = 'json'>(
    config: CustomAxiosRequestConfig
  ) {
    try {
      const response: AxiosResponse<ResponseData> = await instance(config)
      const responseType = response.config?.responseType || 'json'

      if (responseType === 'json') {
        return { data: opts.transformBackendResponse(response), error: null }
      }

      return { data: response.data as MappedType<R, T>, error: null }
    } catch (error) {
      const requestError: any = axios.isAxiosError(error) ? error : {}
      const responseData = requestError?.response?.data

      if (responseData && typeof responseData === 'object') {
        return Promise.reject({
          data: null,
          error: {
            message: requestError.response.data.message || requestError.response.data.msg || '请求失败',
            status: requestError?.response?.status,
            code: requestError?.code,
            data: responseData
          }
        })
      }

      return Promise.reject({
        data: null,
        error: {
          message: requestError.message || '请求失败',
          status: requestError.response?.status,
          code: requestError.code
        }
      })
    }
  } as FlatRequestInstance

  Object.assign(flatRequest, {
    async get<T = any, R extends ResponseType = 'json'>(url: string, config?: CustomAxiosRequestConfig<R>) {
      return flatRequest<T, R>({ ...config, url, method: 'get' })
    },
    async post<T = any, R extends ResponseType = 'json'>(
      url: string,
      data?: any,
      config?: CustomAxiosRequestConfig<R>
    ) {
      return flatRequest<T, R>({ ...config, url, data, method: 'post' })
    },
    async put<T = any, R extends ResponseType = 'json'>(url: string, data?: any, config?: CustomAxiosRequestConfig<R>) {
      return flatRequest<T, R>({ ...config, url, data, method: 'put' })
    },
    async delete<T = any, R extends ResponseType = 'json'>(url: string, config?: CustomAxiosRequestConfig<R>) {
      return flatRequest<T, R>({ ...config, url, method: 'delete' })
    },
    async delete2<T = any, R extends ResponseType = 'json'>(
      url: string,
      data?: any,
      config?: CustomAxiosRequestConfig<R>
    ) {
      return flatRequest<T, R>({ ...config, url, data, method: 'delete' })
    },
    cancelRequest,
    cancelAllRequest
  })

  return flatRequest
}

export type { CreateAxiosDefaults, AxiosError }
