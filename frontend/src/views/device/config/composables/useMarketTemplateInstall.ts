import { ref } from 'vue'

export type MissingMarketPlugin = {
  plugin_name?: string
  min_version?: string
  required?: boolean
}

export type MarketInstallErrorType = 'duplicate' | 'plugin' | 'auth' | 'unknown'

type Translate = (key: string) => string

type MarketInstallMessageApi = {
  success?: (message: string) => void
  warning?: (message: string) => void
  error?: (message: string) => void
}

type MarketInstallDialogApi = {
  success?: (options: { title: string; content: string; positiveText: string }) => void
  warning?: (options: { title: string; content: string; positiveText: string }) => void
}

type MarketInstallResponse = {
  error?: {
    msg?: string
  } | null
  data?: {
    missing_plugins?: MissingMarketPlugin[]
  } | null
}

type UseMarketTemplateInstallOptions = {
  isLoggedIn: () => boolean
  getToken: () => string | null
  clearToken: () => void
  openLoginModal: () => void
  resolveTemplateName: (id: string) => string
  installTemplate: (payload: { market_template_id: string; market_token: string }) => Promise<MarketInstallResponse>
  onInstalled: () => void
  t: Translate
  message: MarketInstallMessageApi
  dialog: MarketInstallDialogApi
}

export const classifyInstallError = (message: string): MarketInstallErrorType => {
  const normalized = message.toLowerCase()
  if (
    message.includes('已存在') ||
    message.includes('已安装') ||
    normalized.includes('duplicate') ||
    normalized.includes('already installed') ||
    normalized.includes('already exists')
  ) {
    return 'duplicate'
  }
  if (
    message.includes('插件') ||
    normalized.includes('plugin') ||
    normalized.includes('protocol dependency') ||
    normalized.includes('missing protocol')
  ) {
    return 'plugin'
  }
  if (
    message.includes('登录') ||
    message.includes('认证') ||
    normalized.includes('unauthorized') ||
    normalized.includes('token') ||
    normalized.includes('login')
  ) {
    return 'auth'
  }
  return 'unknown'
}

export const formatMissingPlugins = (plugins: MissingMarketPlugin[], t: Translate) =>
  plugins
    .map((plugin) => {
      const version = plugin.min_version ? ` (>=${plugin.min_version})` : ''
      const required = plugin.required ? t('market.pluginRequired') : t('market.pluginOptional')
      return `${plugin.plugin_name || '--'}${version} [${required}]`
    })
    .join('\n')

export const buildInstallSuccessContent = (
  templateName: string,
  missingPlugins: MissingMarketPlugin[],
  t: Translate
) => {
  const templateLine = templateName ? `${templateName}\n` : ''
  const missingPluginText = missingPlugins.length
    ? `\n\n${t('market.missingPluginsMessage')}\n${formatMissingPlugins(missingPlugins, t)}\n${t('market.contactAdmin')}`
    : ''

  return `${templateLine}${t('market.installSuccessNextStep')}${missingPluginText}`
}

export const buildAlreadyInstalledContent = (templateName: string, t: Translate) => {
  const templateLine = templateName ? `${templateName}\n` : ''
  return `${templateLine}${t('market.alreadyInstalledNextStep')}`
}

export const buildPluginInstallFailureContent = (message: string, t: Translate) =>
  `${message || t('market.installFailed')}\n\n${t('market.pluginFailureHint')}`

export const useMarketTemplateInstall = (options: UseMarketTemplateInstallOptions) => {
  const pendingInstallId = ref('')
  const installingIds = ref(new Set<string>())

  const templateName = (id: string) => options.resolveTemplateName(id)
  const isInstalling = (id: string) => installingIds.value.has(String(id))

  const showInstallSuccess = (id: string, missingPlugins: MissingMarketPlugin[]) => {
    options.dialog.success?.({
      title: options.t('market.installSuccess'),
      content: buildInstallSuccessContent(templateName(id), missingPlugins, options.t),
      positiveText: options.t('common.confirm')
    })
    options.message.success?.(options.t('market.installSuccess'))
  }

  const showAlreadyInstalled = (id: string) => {
    options.dialog.warning?.({
      title: options.t('market.alreadyInstalled'),
      content: buildAlreadyInstalledContent(templateName(id), options.t),
      positiveText: options.t('common.confirm')
    })
    options.message.warning?.(options.t('market.alreadyInstalled'))
  }

  const showPluginInstallFailure = (message: string) => {
    options.dialog.warning?.({
      title: options.t('market.installFailed'),
      content: buildPluginInstallFailureContent(message, options.t),
      positiveText: options.t('common.confirm')
    })
  }

  const openLoginForInstall = (id: string) => {
    pendingInstallId.value = id
    options.openLoginModal()
  }

  const doInstall = async (id: string) => {
    const installId = String(id)
    if (isInstalling(installId)) return

    const token = options.getToken()
    if (!token) {
      options.message.warning?.(options.t('market.tokenExpired'))
      openLoginForInstall(id)
      return
    }

    installingIds.value = new Set([...installingIds.value, installId])
    try {
      const res = await options.installTemplate({
        market_template_id: id,
        market_token: token
      })
      if (!res.error) {
        const missingPlugins = Array.isArray(res.data?.missing_plugins) ? res.data.missing_plugins : []
        showInstallSuccess(id, missingPlugins)
        options.onInstalled()
        return
      }

      const msg = res.error?.msg || ''
      const errorType = classifyInstallError(msg)
      if (errorType === 'duplicate') {
        showAlreadyInstalled(id)
        options.onInstalled()
      } else if (errorType === 'plugin') {
        showPluginInstallFailure(msg)
      } else if (errorType === 'auth') {
        options.clearToken()
        options.message.error?.(options.t('market.tokenExpired'))
        openLoginForInstall(id)
      } else {
        options.message.error?.(`${options.t('market.installFailed')}: ${msg}`)
      }
    } catch (e: any) {
      if (e?.response?.status === 401) {
        options.clearToken()
        options.message.error?.(options.t('market.tokenExpired'))
        openLoginForInstall(id)
        return
      }
      options.message.error?.(`${options.t('market.installFailed')}: ${e?.message || ''}`)
    } finally {
      const nextInstallingIds = new Set(installingIds.value)
      nextInstallingIds.delete(installId)
      installingIds.value = nextInstallingIds
    }
  }

  const handleInstall = async (id: string) => {
    if (!options.isLoggedIn()) {
      openLoginForInstall(id)
      return
    }
    await doInstall(id)
  }

  const onMarketLoginSuccess = async () => {
    if (!pendingInstallId.value) return

    const installId = pendingInstallId.value
    pendingInstallId.value = ''
    await doInstall(installId)
  }

  return {
    pendingInstallId,
    installingIds,
    isInstalling,
    handleInstall,
    doInstall,
    onMarketLoginSuccess
  }
}
