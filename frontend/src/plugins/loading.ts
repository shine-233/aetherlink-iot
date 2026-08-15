/**
 * 文件用途：提供全局加载反馈插件。
 * 核心逻辑：封装 Naive UI loading bar 或应用加载状态，让请求和路由流程能统一展示加载反馈。
 * 关键注意事项：加载状态需要与错误、路由切换和并发请求配合，避免提前结束或一直挂起。
 * 重构建议：可将状态计数和 UI 调用拆分，支持更多加载来源。
 */
// @unocss-include
import { getRgbOfColor } from '@aetherlink/utils'
import { $t } from '@/locales'
import { localStg } from '@/utils/storage'

const defaultRdiLogo = '/rdi/logo.png'

function resolveLoadingTitle() {
  const systemName = String(localStg.get('systemName') || '').trim()
  return systemName || $t('title')
}

export function setupLoading() {
  const app = document.getElementById('app')
  if (!app) return

  // If Vue has already mounted on #app, never overwrite its DOM.
  // Vue 3 attaches a private reference on the mount container.
  if ((app as any).__vue_app__) return

  const themeColor = localStg.get('themeColor') || '#646cff'
  const logoLoading = localStg.get('logoLoading') || ''

  const { r, g, b } = getRgbOfColor(themeColor)

  const primaryColor = `${r} ${g} ${b}`

  const loadingClasses = [
    'left-0 top-0',
    'left-0 bottom-0 animate-delay-500',
    'right-0 top-0 animate-delay-1000',
    'right-0 bottom-0 animate-delay-1500'
  ]

  const loading = document.createElement('div')
  loading.className = 'fixed-center flex-col'
  loading.style.setProperty('--primary-color', primaryColor)

  const logo = document.createElement('img')
  logo.src = logoLoading || defaultRdiLogo
  logo.style.maxWidth = '88px'
  logo.style.height = 'auto'
  loading.appendChild(logo)

  const spinner = document.createElement('div')
  spinner.className = 'w-32px h-32px my-36px'

  const spinnerContent = document.createElement('div')
  spinnerContent.className = 'relative h-full animate-spin'

  loadingClasses.forEach(item => {
    const dot = document.createElement('div')
    dot.className = `absolute w-10px h-10px bg-primary rounded-8px animate-pulse ${item}`
    spinnerContent.appendChild(dot)
  })

  spinner.appendChild(spinnerContent)
  loading.appendChild(spinner)

  const title = document.createElement('h2')
  title.className = 'text-28px text-center font-500 text-#646464'
  title.textContent = resolveLoadingTitle()
  loading.appendChild(title)

  app.replaceChildren(loading)
}
