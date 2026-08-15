/**
 * 文件用途：根据当前前端运行环境创建服务请求配置。
 * 核心逻辑：为 dev/test/prod 映射 baseURL、其他服务地址和 SSE 入口，默认 dev 指向本地后端。
 * 关键注意事项：这里的默认地址会影响所有请求代理，公开发布前不要写入真实共享环境或私有凭据。
 * 重构建议：建议将环境 URL 校验和 preview-proxy 规则抽成可测试配置函数。
 */
export function createServiceConfig(env: Env.ImportMeta) {
  // Keep the local default stable, but allow verification/development hosts
  // to point at a separately started backend without editing source files.
  const devURL = env.VITE_DEV_API_URL?.trim() || 'http://127.0.0.1:9999/api/v1';
  const testURL = '';
  const prodURL = '';

  const serviceConfigMap: App.Service.ServiceConfigMap = {
    dev: {
      baseURL: devURL,
      otherBaseURL: {
        platform: devURL
      },
      sseEndpoint: '/proxy-default/events'
    },
    test: {
      baseURL: testURL,
      otherBaseURL: {
        platform: testURL
      },
      sseEndpoint: '/api/v1/events'
    },
    prod: {
      baseURL: prodURL,
      otherBaseURL: {
        platform: prodURL
      },
      sseEndpoint: '/api/v1/events'
    }
  };

  const { VITE_SERVICE_ENV = 'dev' } = env;

  return serviceConfigMap[VITE_SERVICE_ENV];
}

/**
 * Get proxy pattern of service url.
 *
 * @param key If not set, the default service proxy is used.
 */
export function createProxyPattern(key?: App.Service.OtherBaseURLKey) {
  if (!key) {
    return '/proxy-default';
  }

  return `/proxy-${key}`;
}

/**
 * Get SSE endpoint URL by current env.
 */
export function getSSEEndpoint(env: Env.ImportMeta) {
  const serviceConfig = createServiceConfig(env);
  return serviceConfig.sseEndpoint;
}
