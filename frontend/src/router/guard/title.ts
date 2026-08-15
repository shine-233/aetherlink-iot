/**
 * 文件用途：浏览器标题路由守卫。
 * 核心逻辑：根据路由 meta、登录子路径和系统设置生成页面标题。
 * 关键注意事项：登录页子模块标题来自语言包，系统名称为空时回退到默认应用标题。
 * 重构建议：可将标题解析逻辑抽为纯函数，单独覆盖登录子路由和普通业务路由。
 */
import type { Router } from 'vue-router'
import { useTitle } from '@vueuse/core'
import { $t } from '@/locales'
import { useSysSettingStore } from '@/store/modules/sys-setting'
import { resolveDocumentTitle } from './title-helper'

export function createDocumentTitleGuard(router: Router) {
  router.afterEach(to => {
    const sysSettingStore = useSysSettingStore()
    const appTitle = sysSettingStore.system_name || $t('title')
    const documentTitle = resolveDocumentTitle(to, appTitle, $t)

    useTitle(documentTitle)
  })
}
