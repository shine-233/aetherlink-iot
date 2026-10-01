/**
 * 文件用途：按需加载 naive-ui 暗色主题。
 * 核心逻辑：darkTheme 会拖入全部组件的暗色样式变量（构建实测 ~37KB），亮色用户永远用不到；
 *   改为暗色模式首次开启时动态 import，结果缓存在 shallowRef 中供 NConfigProvider 使用。
 * 关键注意事项：启动时若已是暗色模式，main.ts 会在 mount 前 await ensureNaiveDarkTheme()，避免亮色闪屏。
 */
import { shallowRef } from 'vue'
import type { GlobalTheme } from 'naive-ui'

export const naiveDarkThemeRef = shallowRef<GlobalTheme | null>(null)

let loading: Promise<GlobalTheme | null> | null = null

export function ensureNaiveDarkTheme(): Promise<GlobalTheme | null> {
  if (naiveDarkThemeRef.value) return Promise.resolve(naiveDarkThemeRef.value)
  if (!loading) {
    loading = import('./chunk')
      .then(({ darkTheme }) => {
        naiveDarkThemeRef.value = darkTheme
        return darkTheme
      })
      .catch((error) => {
        // 加载失败时允许下次切换重试；界面保持亮色主题而不是报错白屏。
        loading = null
        console.warn('[theme] Failed to load naive-ui dark theme:', error)
        return null
      })
  }
  return loading
}
