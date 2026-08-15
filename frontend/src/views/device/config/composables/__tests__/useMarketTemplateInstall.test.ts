import { describe, expect, it, vi } from 'vitest'
import {
  buildAlreadyInstalledContent,
  buildInstallSuccessContent,
  buildPluginInstallFailureContent,
  classifyInstallError,
  formatMissingPlugins,
  useMarketTemplateInstall
} from '../useMarketTemplateInstall'

const t = (key: string) => key

describe('useMarketTemplateInstall', () => {
  it('classifies duplicate, plugin, auth, and unknown install errors in one helper', () => {
    expect(classifyInstallError('模板已存在')).toBe('duplicate')
    expect(classifyInstallError('already installed')).toBe('duplicate')
    expect(classifyInstallError('duplicate template')).toBe('duplicate')
    expect(classifyInstallError('missing plugin')).toBe('plugin')
    expect(classifyInstallError('缺少插件')).toBe('plugin')
    expect(classifyInstallError('unauthorized token')).toBe('auth')
    expect(classifyInstallError('请重新登录')).toBe('auth')
    expect(classifyInstallError('network failed')).toBe('unknown')
  })

  it('formats missing plugin requirements for a single result dialog', () => {
    expect(
      formatMissingPlugins(
        [
          { plugin_name: 'mqtt', min_version: '1.2.0', required: true },
          { plugin_name: 'opcua', required: false }
        ],
        t
      )
    ).toBe('mqtt (>=1.2.0) [market.pluginRequired]\nopcua [market.pluginOptional]')
  })

  it('builds install dialog content without hard-coded page copy inside the flow', () => {
    expect(buildInstallSuccessContent('Pump Controller', [], t)).toBe(
      'Pump Controller\nmarket.installSuccessNextStep'
    )
    expect(
      buildInstallSuccessContent(
        'Pump Controller',
        [{ plugin_name: 'mqtt', min_version: '1.2.0', required: true }],
        t
      )
    ).toContain('market.missingPluginsMessage\nmqtt (>=1.2.0) [market.pluginRequired]\nmarket.contactAdmin')
    expect(buildAlreadyInstalledContent('Pump Controller', t)).toBe(
      'Pump Controller\nmarket.alreadyInstalledNextStep'
    )
    expect(buildPluginInstallFailureContent('missing plugin', t)).toBe(
      'missing plugin\n\nmarket.pluginFailureHint'
    )
  })

  it('opens login instead of silently returning when token is missing', async () => {
    const openLoginModal = vi.fn()
    const warning = vi.fn()
    const install = useMarketTemplateInstall({
      isLoggedIn: () => true,
      getToken: () => '',
      clearToken: vi.fn(),
      openLoginModal,
      resolveTemplateName: () => '',
      installTemplate: vi.fn(),
      onInstalled: vi.fn(),
      t,
      message: {
        warning
      },
      dialog: {}
    })

    await install.doInstall('tpl-1')

    expect(install.pendingInstallId.value).toBe('tpl-1')
    expect(warning).toHaveBeenCalledWith('market.tokenExpired')
    expect(openLoginModal).toHaveBeenCalledTimes(1)
  })

  it('does not submit the same template twice while an install is already in flight', async () => {
    let resolveInstall: (value: { error: null; data: {} }) => void = () => {}
    const installTemplate = vi.fn(
      () =>
        new Promise<{ error: null; data: {} }>((resolve) => {
          resolveInstall = resolve
        })
    )
    const install = useMarketTemplateInstall({
      isLoggedIn: () => true,
      getToken: () => 'token-1',
      clearToken: vi.fn(),
      openLoginModal: vi.fn(),
      resolveTemplateName: () => '',
      installTemplate,
      onInstalled: vi.fn(),
      t,
      message: {},
      dialog: {}
    })

    const firstInstall = install.doInstall('tpl-1')
    await install.doInstall('tpl-1')

    expect(install.isInstalling('tpl-1')).toBe(true)
    expect(installTemplate).toHaveBeenCalledTimes(1)

    resolveInstall({ error: null, data: {} })
    await firstInstall
    expect(install.isInstalling('tpl-1')).toBe(false)
  })

  it('clears token and reopens login when the backend returns an auth-shaped error message', async () => {
    const clearToken = vi.fn()
    const openLoginModal = vi.fn()
    const error = vi.fn()
    const install = useMarketTemplateInstall({
      isLoggedIn: () => true,
      getToken: () => 'token-1',
      clearToken,
      openLoginModal,
      resolveTemplateName: () => '',
      installTemplate: vi.fn().mockResolvedValue({ error: { msg: 'unauthorized token' }, data: {} }),
      onInstalled: vi.fn(),
      t,
      message: {
        error
      },
      dialog: {}
    })

    await install.doInstall('tpl-1')

    expect(clearToken).toHaveBeenCalledTimes(1)
    expect(error).toHaveBeenCalledWith('market.tokenExpired')
    expect(openLoginModal).toHaveBeenCalledTimes(1)
    expect(install.pendingInstallId.value).toBe('tpl-1')
  })
})
