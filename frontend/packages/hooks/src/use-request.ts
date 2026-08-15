/**
 * 文件用途：提供带状态管理的请求组合式函数。
 * 核心逻辑：基于 @aetherlink/axios 创建请求实例，并维护 data、error 与 loading 状态。
 * 关键注意事项：请求选项和响应映射会影响 data 类型，错误分支必须由调用方显式处理。
 * 重构建议：可拆分请求实例创建和状态流转逻辑，并补充成功/失败/并发场景测试。
 */
import { ref } from 'vue'
import type { Ref } from 'vue'
import { createFlatRequest } from '@aetherlink/axios'
import type {
  AxiosError,
  CreateAxiosDefaults,
  CustomAxiosRequestConfig,
  MappedType,
  RequestOption,
  ResponseType
} from '@aetherlink/axios'
import useLoading from './use-loading'

export type HookRequestInstanceResponseSuccessData<T = any> = {
  data: Ref<T>
  error: Ref<null>
}

export type HookRequestInstanceResponseFailData<T = any> = {
  data: Ref<null>
  error: Ref<AxiosError<T>>
}

export type HookRequestInstanceResponseData<T = any> = {
  loading: Ref<boolean>
} & (HookRequestInstanceResponseSuccessData<T> | HookRequestInstanceResponseFailData<T>)

export interface HookRequestInstance {
  <T = any, R extends ResponseType = 'json'>(
    config: CustomAxiosRequestConfig
  ): HookRequestInstanceResponseData<MappedType<R, T>>
  cancelRequest: (requestId: string) => void
  cancelAllRequest: () => void
}

/**
 * create a hook requestTs instance
 *
 * @param axiosConfig
 * @param options
 */
export default function createHookRequest<ResponseData = any>(
  axiosConfig?: CreateAxiosDefaults,
  options?: Partial<RequestOption<ResponseData>>
) {
  const request = createFlatRequest<ResponseData>(axiosConfig, options)

  const hookRequest: HookRequestInstance = function hookRequest<T = any, R extends ResponseType = 'json'>(
    config: CustomAxiosRequestConfig
  ) {
    const { loading, startLoading, endLoading } = useLoading()

    const data = ref<MappedType<R, T> | null>(null)
    const error = ref<AxiosError<MappedType<R, T>> | null>(null)

    startLoading()

    request(config).then(res => {
      if (res.data) {
        data.value = res.data
      } else {
        error.value = res.error
      }

      endLoading()
    })

    return {
      loading,
      data,
      error
    }
  } as HookRequestInstance

  hookRequest.cancelRequest = request.cancelRequest
  hookRequest.cancelAllRequest = request.cancelAllRequest

  return hookRequest
}
