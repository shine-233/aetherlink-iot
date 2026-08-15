/**
 * 文件用途：提供 axios 请求封装的共享辅助函数。
 * 核心逻辑：读取请求 Content-Type，并判断 HTTP 状态与 AxiosResponse 是否成功。
 * 关键注意事项：成功状态范围会影响重试、错误处理和上层响应分支。
 * 重构建议：可把状态码策略抽成可注入配置，便于不同后端协议复用。
 */
import type { AxiosHeaderValue, AxiosResponse, InternalAxiosRequestConfig } from 'axios'

export function getContentType(config: InternalAxiosRequestConfig) {
  const contentType: AxiosHeaderValue = config.headers?.['Content-Type'] || 'application/json'

  return contentType
}

/**
 * check if http status is success
 *
 * @param status
 */
export function isHttpSuccess(status: number) {
  const isSuccessCode = status >= 200 && status < 300
  return isSuccessCode || status === 304
}

/**
 * is response json
 *
 * @param response axios response
 */
export function isResponseJson(response: AxiosResponse) {
  const { responseType } = response.config

  return responseType === 'json' || responseType === undefined
}
