/*
 * 文件用途：统一处理前端 service 调用结果和适配器返回值。
 * 核心逻辑：根据后端响应 code、message、data 执行成功返回、错误提示和登录失效处理。
 * 关键注意事项：这里是接口错误进入 UI 的统一边界，改动会影响大量请求体验。
 * 重构建议：建议减少 `any`，并把错误码策略配置化。
 */
/** 统一失败和成功的请求结果的数据类型 */
export async function handleServiceResult<T = any>(
  error: App.Service.RequestError | null,
  data: any,
  msg: string = ''
) {
  if (error) {
    const fail: App.Service.FailedResult = {
      error,
      data: null
    }
    return fail
  }
  const success: App.Service.SuccessResult<T> = {
    error: null,
    data
  }
  return {
    ...success,
    msg
  }
}

/** 请求结果的适配器：用于接收适配器函数和请求结果 */
export function adapter<T extends App.Service.ServiceAdapter>(
  adapterFun: T,
  ...args: App.Service.MultiRequestResult<Parameters<T>>
): App.Service.RequestResult<ReturnType<T>> {
  let result: App.Service.RequestResult | undefined

  const hasError = args.some(item => {
    const flag = Boolean(item.error)
    if (flag) {
      result = {
        error: item.error,
        data: null
      }
    }
    return flag
  })

  if (!hasError) {
    const adapterFunArgs = args.map(item => item.data)
    result = {
      error: null,
      data: adapterFun(...adapterFunArgs)
    }
  }

  return result!
}
