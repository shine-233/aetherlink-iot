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

  // 默认值必须跟随 Vite 的构建模式，不能硬编码 'dev'。
  //
  // 为什么：'dev' 的 baseURL 是绝对地址 http://127.0.0.1:9999/api/v1。
  // 一旦 `vite build` 没有显式带 VITE_SERVICE_ENV，这个绝对地址就会被打进产物，
  // 造成两个后果：
  //   1. 真实部署指向 localhost，请求全废；
  //   2. 本地预览代理（automation_tests 的 serve_preview_with_api_proxy）下，
  //      浏览器会绕过代理直连 9999 并被 CORS 拦掉 —— 表现为**整站白屏**
  //      （SPA 不挂载，title 停在 index.html 的原始值），极难定位。
  // 2026-09-15 这个坑实际发生了一次：并行工作流重建 dist 时漏了环境变量，
  // 全站白屏被误判成"全局回归"。
  //
  // 跟随模式后：vite build → prod（baseURL 为空串，走同源相对路径，代理与部署都对）；
  //            vite dev  → dev （直连本地后端，开发体验不变）。
  // 需要覆盖时仍可显式设 VITE_SERVICE_ENV=dev|test|prod。
  const fallbackEnvType: App.Service.EnvType = env.PROD ? 'prod' : 'dev';
  const envType = env.VITE_SERVICE_ENV || fallbackEnvType;

  return serviceConfigMap[envType];
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
