/**
 * 文件用途：构建期替换 index.html 中的 %VITE_APP_TITLE% 占位符并兜底默认标题。
 * 核心逻辑：Vite 只在存在同名环境变量时才替换 HTML 内的 %VAR% 占位符；clean checkout
 *   没有 .env 文件时 %VITE_APP_TITLE% 会字面留在 <title> 里（ROADMAP §1.3-B 记录的
 *   SPA 白屏排查中发现的既知问题）。本插件在 transformIndexHtml 阶段做确定性替换：
 *   有环境变量用环境变量，没有用产品默认值，绝不让占位符进入产物。
 * 关键注意事项：只处理 %VITE_APP_TITLE% 一个变量，不触碰其它占位符；
 *   默认值与 .env.example 的 VITE_APP_TITLE 保持一致（AetherLink IoT）。
 * 重构建议：若未来 HTML 需要更多构建期注入（描述、OG 标签等），扩展为通用占位符表。
 */
import type { PluginOption } from 'vite'

const DEFAULT_APP_TITLE = 'AetherLink IoT'

export function setupHtmlTitlePlugin(viteEnv: Record<string, string | undefined>): PluginOption[] {
  const title = viteEnv.VITE_APP_TITLE || DEFAULT_APP_TITLE

  return [
    {
      name: 'aetherlink-html-title',
      transformIndexHtml(html) {
        return html.replace(/%VITE_APP_TITLE%/g, title)
      }
    }
  ]
}