/**
 * 文件用途：初始化 Iconify 图标能力。
 * 核心逻辑：注册或配置 Iconify 相关资源，让全局图标组件能够解析远程或本地图标名称。
 * 关键注意事项：图标前缀和本地图标注册需与 SvgIcon、icon selector 保持一致。
 * 重构建议：可将图标集合注册与组件安装拆分，便于按需加载。
 */
import { addAPIProvider, disableCache } from '@iconify/vue'

/** Set up the iconify offline */
export function setupIconifyOffline() {
  const { VITE_ICONIFY_URL } = import.meta.env

  if (VITE_ICONIFY_URL) {
    addAPIProvider('', { resources: [VITE_ICONIFY_URL] })

    disableCache('all')
  }
}
