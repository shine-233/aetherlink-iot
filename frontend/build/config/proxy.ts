/**
 * 文件用途：构造 Vite 开发代理表，转发平台 API、文件与 ThingsVis 请求。
 * 核心逻辑：以 env.config.ts 的 createServiceConfig/createProxyPattern 为唯一来源推导代理前缀与目标。
 * 关键注意事项：代理键必须与前端固定路径常量一致（见 src/utils/thingsvis/constants.ts 的 THINGSVIS_API_PROXY_PATH）。
 * 重构建议：新增后端服务时优先扩展 otherBaseURL，而不是在此硬编码新前缀。
 */
import type { ProxyOptions } from 'vite'
import { createProxyPattern, createServiceConfig } from '../../env.config'

/** ThingsVis 代理前缀，必须与 src/utils/thingsvis/constants.ts 保持一致 */
const THINGSVIS_API_PROXY_PATH = '/thingsvis-api'

/**
 * 把形如 http://host:port/api/v1 的 baseURL 拆成代理 target 与需要补回的路径前缀。
 *
 * 代理前缀（/proxy-default）在请求发出前替换掉 baseURL，因此 rewrite 必须把
 * baseURL 的 path 部分（/api/v1）加回去，否则后端会收到不带版本前缀的路径。
 */
function splitBaseURL(baseURL: string) {
  const url = new URL(baseURL)
  const basePath = url.pathname.replace(/\/$/, '')

  return { target: url.origin, basePath }
}

/**
 * 创建 Vite 开发代理。
 *
 * @param env 当前 vite 环境变量
 * @param enable 是否启用代理，未显式传入时取 VITE_HTTP_PROXY === 'Y'
 */
export function createViteProxy(env: Env.ImportMeta, enable?: boolean): Record<string, ProxyOptions> | undefined {
  const isEnable = enable ?? env.VITE_HTTP_PROXY === 'Y'

  if (!isEnable) return undefined

  const { baseURL, otherBaseURL } = createServiceConfig(env)
  const proxy: Record<string, ProxyOptions> = {}

  // 默认平台服务：/proxy-default -> ${origin}/api/v1
  if (baseURL) {
    const { target, basePath } = splitBaseURL(baseURL)
    const pattern = createProxyPattern()

    proxy[pattern] = {
      target,
      changeOrigin: true,
      ws: true,
      rewrite: path => path.replace(new RegExp(`^${pattern}`), basePath)
    }

    // 文件下载/上传走后端同源路径，前端直接请求 /files
    proxy['/files'] = {
      target,
      changeOrigin: true
    }
  }

  // 其它命名服务：/proxy-<key> -> 对应 baseURL
  Object.entries(otherBaseURL ?? {}).forEach(([key, url]) => {
    if (!url || url === baseURL) return

    const { target, basePath } = splitBaseURL(url)
    const pattern = createProxyPattern(key as App.Service.OtherBaseURLKey)

    proxy[pattern] = {
      target,
      changeOrigin: true,
      ws: true,
      rewrite: path => path.replace(new RegExp(`^${pattern}`), basePath)
    }
  })

  // ThingsVis 后端：前端固定请求 /thingsvis-api，重写为 ${VITE_THINGSVIS_API_URL}/api/v1
  const thingsvisTarget = env.VITE_THINGSVIS_API_URL

  if (thingsvisTarget) {
    proxy[THINGSVIS_API_PROXY_PATH] = {
      target: thingsvisTarget,
      changeOrigin: true,
      ws: true,
      rewrite: path => path.replace(new RegExp(`^${THINGSVIS_API_PROXY_PATH}`), '/api/v1')
    }
  }

  return proxy
}
