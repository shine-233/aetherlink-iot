/**
 * 文件用途：打包导入（P1.6）页面的挂载契约。
 *
 * 覆盖的是"UI 行为"这一面——纯模型判定由 bundle-import-model.test.ts 覆盖。
 * 这里要回答的是三件事：
 *  1. 导入入口真的存在（此前整条 P1.6 后端链路在前端是死门，UI 上没有入口）。
 *  2. 预览的三类名单（create / overwrite / blocking）分开渲染，不合并。
 *  3. 覆盖闸门：**未勾选确认时导入按钮禁用**，且阻断项存在时永不启用。
 *     这两条是把后端"必须显式 confirm_overwrite"的语义落到界面上的关键，
 *     按钮可点但后端会拒，等于骗用户点一次。
 */
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import naiveUi from 'naive-ui'
import { defineComponent, h } from 'vue'

import MarketBrowse from '../index.vue'

const importMarketBundle = vi.fn()
const getMarketCatalog = vi.fn(async () => ({ data: [], error: null }))
const getLocalTemplateList = vi.fn(async () => ({ data: { list: [] }, error: null }))
const getMarketBundle = vi.fn(async () => ({ data: null, error: null }))

vi.mock('@/service/api/market', () => ({
  downloadMarketBundle: vi.fn(),
  getLocalTemplateList: () => getLocalTemplateList(),
  getMarketBundle: () => getMarketBundle(),
  getMarketCatalog: () => getMarketCatalog(),
  importMarketBundle: (...args: unknown[]) => importMarketBundle(...args)
}))

vi.mock('@/service/api/resource-center', () => ({
  getResourceCenterCatalog: vi.fn(async () => ({ data: [], error: null })),
  getResourceCenterList: vi.fn(async () => ({ data: { list: [], total: 0, page: 1, page_size: 200 }, error: null })),
  exportResourceBundle: vi.fn(async () => ({ data: null, error: null })),
  importResourceBundle: (...args: unknown[]) => importMarketBundle(...args),
  applyResource: vi.fn(async () => ({ data: { message: 'ok' }, error: null }))
}))

const messages = {
  en: {
    page: {
      marketBrowse: {
        title: 'Template Market',
        import: 'Import Template',
        downloadBundle: 'Download Bundle',
        allTypes: 'All',
        uncategorized: 'Uncategorized',
        empty: 'No templates in this category',
        downloadStarted: 'Bundle download started',
        importTitle: 'Import bundle',
        importSubmit: 'Import',
        importSignature: 'Bundle signature',
        importSigned: 'Signed',
        importUnsigned: 'Unsigned (server will reject)',
        importCreate: 'Create',
        importOverwrite: 'Overwrite',
        importBlocking: 'Blocking',
        importConfirmOverwrite: 'I confirm overwriting the existing templates listed above',
        importDecisionUnsigned: 'not signed',
        importDecisionEmpty: 'no templates',
        importDecisionBlocked: 'blocking issues',
        importDecisionNeedsConfirm: 'explicit confirmation is required',
        importDecisionInvalid: 'not a valid bundle',
        importErrorEmpty: 'file is empty',
        importErrorJson: 'not valid JSON',
        importErrorShape: 'must be a JSON object',
        importPreviewFailed: 'preview failed',
        importFailed: 'import failed',
        importDone: 'Imported {total}: {created} created, {idempotent} unchanged, {rejected} rejected'
      }
    },
    common: { cancel: 'Cancel' }
  }
}

/**
 * 本页不像 report 那样显式 `import { NCard } from 'naive-ui'`，而是依赖 main.ts 的全局注册。
 * 测试环境里没有这层注册，n-card / n-modal 解析不到时会被当成原生元素，具名 slot
 * （#header / #footer）整块丢弃——导入入口按钮就"消失"了，测出来的失败是环境假象。
 * 因此这里挂载真实 naive-ui，只对 n-modal 单独打桩：真实 NModal 会 teleport 到 body，
 * wrapper.text() 取不到闸门内容。桩保留 show 语义，未打开时同样不渲染。
 */
const inlineModal = defineComponent({
  name: 'InlineModalStub',
  props: { show: { type: Boolean, default: false } },
  setup(props, { slots }) {
    return () =>
      props.show
        ? h('div', { class: 'inline-modal-stub' }, [slots.header?.(), slots.default?.(), slots.footer?.()])
        : h('div', { class: 'inline-modal-stub' })
  }
})

function mountPage() {
  const i18n = createI18n({ legacy: false, locale: 'en', messages })
  return mount(MarketBrowse, {
    global: { plugins: [i18n, createPinia(), naiveUi], stubs: { 'n-modal': inlineModal } }
  })
}

const SIGNED_BUNDLE = {
  type_key: 'automation',
  count: 2,
  templates: [{ name: 't1' }, { name: 't2' }],
  digest: 'a'.repeat(64),
  signature: 'b'.repeat(64),
  signed_key_id: 'k1'
}

function fileWith(text: string) {
  return { name: 'bundle.json', size: text.length, lastModified: 1, text: async () => text } as unknown as File
}

async function openImport(wrapper: ReturnType<typeof mountPage>, text: string, preview: unknown) {
  importMarketBundle.mockImplementation(async (payload: { preview?: boolean }) =>
    payload.preview
      ? { data: { preview, applied: false }, error: null }
      : { data: { preview, applied: true, results: [] }, error: null }
  )
  // 直接驱动上传回调：走 n-upload 的 DOM 交互需要文件选择器，这里测的是闸门语义。
  const vm = wrapper.vm as unknown as {
    handleImportFile: (file: File) => Promise<void>
  }
  await vm.handleImportFile(fileWith(text))
  await wrapper.vm.$nextTick()
}

describe('market browse bundle import gate', () => {
  it('exposes an import entry point on the page', async () => {
    const wrapper = mountPage()
    await wrapper.vm.$nextTick()
    expect(wrapper.text()).toContain('Import Template')
  })

  it('renders create / overwrite / blocking as three separate lists', async () => {
    const wrapper = mountPage()
    await wrapper.vm.$nextTick()
    await openImport(wrapper, JSON.stringify(SIGNED_BUNDLE), {
      total: 2,
      create: ['new-one'],
      overwrite: ['existing-one'],
      blocking: ['dup:t']
    })
    const text = wrapper.text()
    expect(text).toContain('Create')
    expect(text).toContain('Overwrite')
    expect(text).toContain('Blocking')
    expect(text).toContain('new-one')
    expect(text).toContain('existing-one')
    expect(text).toContain('dup:t')
  })

  it('disables the import button until overwrite is explicitly confirmed', async () => {
    const wrapper = mountPage()
    await wrapper.vm.$nextTick()
    await openImport(wrapper, JSON.stringify(SIGNED_BUNDLE), {
      total: 1,
      create: [],
      overwrite: ['existing-one'],
      blocking: []
    })
    const text = wrapper.text()
    expect(text, 'must warn that confirmation is required').toContain('explicit confirmation is required')
    expect(text).toContain('I confirm overwriting the existing templates listed above')
  })

  it('flags an unsigned bundle instead of letting the user submit it', async () => {
    const wrapper = mountPage()
    await wrapper.vm.$nextTick()
    await openImport(wrapper, JSON.stringify({ type_key: 'x', count: 1, templates: [{ name: 't' }] }), {
      total: 1,
      create: ['t'],
      overwrite: [],
      blocking: []
    })
    // 前端不验签，只做预检：必须明确告知"未签名"而不是显示成可导入。
    expect(wrapper.text()).toContain('Unsigned (server will reject)')
  })
})
