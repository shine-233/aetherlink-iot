/**
 * 文件用途：定义地图 SDK 相关常量和加载配置。
 * 核心逻辑：集中维护地图脚本、密钥占位、版本和依赖标识。
 * 关键注意事项：地图供应商参数通常受环境配置影响，避免硬编码生产密钥。
 * 重构建议：可抽象为 MapProviderConfig，支持多供应商和按环境注入。
 */
/**
 * 文件：地图 SDK 地址常量。
 * 作用：维护百度、高德、腾讯地图 SDK 的前端加载地址。
 * 依赖：依赖外部地图服务的公开脚本地址和对应 key。
 * 维护：替换 key 或 SDK 版本前确认额度、域名白名单和地图页面兼容性。
 */

/** 根据部署环境中的 key 构造百度地图 SDK 地址；未配置时禁用外部脚本加载。 */
export function buildBaiduMapSdkUrl(key?: string): string {
  const normalizedKey = key?.trim()
  return normalizedKey
    ? `https://api.map.baidu.com/getscript?v=3.0&ak=${encodeURIComponent(normalizedKey)}&services=&t=20210201100830&s=1`
    : ''
}

export const BAIDU_MAP_SDK_URL = buildBaiduMapSdkUrl(import.meta.env.VITE_BAIDU_MAP_KEY)

/** 根据部署环境中的 key 构造高德地图 SDK 地址；未配置时禁用外部脚本加载。 */
export function buildAmapSdkUrl(key?: string): string {
  const normalizedKey = key?.trim()
  return normalizedKey ? `https://webapi.amap.com/maps?v=2.0&key=${encodeURIComponent(normalizedKey)}` : ''
}

export const AMAP_SDK_URL = buildAmapSdkUrl(import.meta.env.VITE_AMAP_KEY)

/** 根据部署环境中的 key 构造腾讯地图 SDK 地址；未配置时禁用外部脚本加载。 */
export function buildTencentMapSdkUrl(key?: string): string {
  const normalizedKey = key?.trim()
  return normalizedKey ? `https://map.qq.com/api/gljs?v=1.exp&key=${encodeURIComponent(normalizedKey)}` : ''
}

export const TENCENT_MAP_SDK_URL = buildTencentMapSdkUrl(import.meta.env.VITE_TENCENT_MAP_KEY)
