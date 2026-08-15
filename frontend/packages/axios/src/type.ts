/**
 * 文件用途：定义 axios 请求封装对外和内部共享类型。
 * 核心逻辑：描述内容类型、请求选项、响应映射、错误处理器和请求实例能力。
 * 关键注意事项：这些类型是调用方契约，字段收窄或改名会造成包级破坏性变更。
 * 重构建议：后续可按配置、响应、错误和实例能力拆分类型文件。
 */
import type { AxiosError, AxiosInstance, AxiosRequestConfig, AxiosResponse, InternalAxiosRequestConfig } from 'axios'

export type ContentType =
  | 'text/html'
  | 'text/plain'
  | 'multipart/form-data'
  | 'application/json'
  | 'application/x-www-form-urlencoded'
  | 'application/octet-stream'

export interface RequestOption<ResponseData = any> {
  onRequest: (config: InternalAxiosRequestConfig) => InternalAxiosRequestConfig | Promise<InternalAxiosRequestConfig>
  isBackendSuccess: (response: AxiosResponse<ResponseData>) => boolean
  onBackendFail: (
    response: AxiosResponse<ResponseData>,
    instance: AxiosInstance
  ) => Promise<AxiosResponse> | Promise<void>
  transformBackendResponse(response: AxiosResponse<ResponseData>): any | Promise<any>
  onError: (
    error: AxiosError<ResponseData>,
    instance: AxiosInstance
  ) => AxiosResponse<ResponseData> | void | Promise<AxiosResponse<ResponseData> | void>
}

interface ResponseMap {
  blob: Blob
  text: string
  arrayBuffer: ArrayBuffer
  arraybuffer: ArrayBuffer
  stream: ReadableStream<Uint8Array>
  document: Document
}

export type ResponseType = keyof ResponseMap | 'json'

export type MappedType<R extends ResponseType, JsonType = any> = R extends keyof ResponseMap ? ResponseMap[R] : JsonType

export type CustomAxiosRequestConfig<R extends ResponseType = 'json'> = Omit<AxiosRequestConfig, 'responseType'> & {
  responseType?: R
  /**
   * 内部标记：该请求是否已重试过，用于防止 401 重试死循环。
   * 由请求拦截器设置，业务代码一般不需要显式传 true。
   */
  _retry?: boolean
  /**
   * 是否静默处理错误（不弹全局 message）。
   * 适用于后台轮询、预加载等不希望打扰用户的请求。
   */
  silentError?: boolean
  /**
   * 是否返回完整响应体（包含 code/message/data），而非仅返回 data 字段。
   * 适用于需要读取业务错误码或额外元信息的场景。
   */
  needMessage?: boolean
}

export interface RequestInstance {
  <T = any, R extends ResponseType = 'json'>(config: CustomAxiosRequestConfig<R>): Promise<MappedType<R, T>>

  get<T = any, R extends ResponseType = 'json'>(
    url: string,
    config?: CustomAxiosRequestConfig<R>
  ): Promise<MappedType<R, T>>

  post<T = any, R extends ResponseType = 'json'>(
    url: string,
    data?: any,
    config?: CustomAxiosRequestConfig<R>
  ): Promise<MappedType<R, T>>

  put<T = any, R extends ResponseType = 'json'>(
    url: string,
    data?: any,
    config?: CustomAxiosRequestConfig<R>
  ): Promise<MappedType<R, T>>

  delete<T = any, R extends ResponseType = 'json'>(
    url: string,
    config?: CustomAxiosRequestConfig<R>
  ): Promise<MappedType<R, T>>

  delete2<T = any, R extends ResponseType = 'json'>(
    url: string,
    data?: any,
    config?: CustomAxiosRequestConfig<R>
  ): Promise<MappedType<R, T>>

  cancelRequest: (requestId: string) => void
  cancelAllRequest: () => void
}

export type FlatResponseSuccessData<T = any> = {
  data: T
  error: null
}

export type FlatResponseFailData<T = any> = {
  data: null
  error: AxiosError<T>
}

export type FlatResponseData<T = any> = FlatResponseSuccessData<T> | FlatResponseFailData<T>

export interface FlatRequestInstance {
  <T = any, R extends ResponseType = 'json'>(
    config: CustomAxiosRequestConfig<R>
  ): Promise<FlatResponseData<MappedType<R, T>>>

  get<T = any, R extends ResponseType = 'json'>(
    url: string,
    config?: CustomAxiosRequestConfig<R>
  ): Promise<FlatResponseData<MappedType<R, T>>>

  post<T = any, R extends ResponseType = 'json'>(
    url: string,
    data?: any,
    config?: CustomAxiosRequestConfig<R>
  ): Promise<FlatResponseData<MappedType<R, T>>>

  put<T = any, R extends ResponseType = 'json'>(
    url: string,
    data?: any,
    config?: CustomAxiosRequestConfig<R>
  ): Promise<FlatResponseData<MappedType<R, T>>>

  delete<T = any, R extends ResponseType = 'json'>(
    url: string,
    config?: CustomAxiosRequestConfig<R>
  ): Promise<FlatResponseData<MappedType<R, T>>>

  delete2<T = any, R extends ResponseType = 'json'>(
    url: string,
    data?: any,
    config?: CustomAxiosRequestConfig<R>
  ): Promise<FlatResponseData<MappedType<R, T>>>

  cancelRequest: (requestId: string) => void
  cancelAllRequest: () => void
}
