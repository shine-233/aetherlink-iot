/**
 * 文件用途: 实体关系管理页的挂载契约（ROADMAP P1.1）。
 * 核心逻辑: 用真实项目 i18n 与 API 桩渲染页面，断言关键结构存在。
 * 关键注意事项:
 *  1. 这是**挂载冒烟**，不是业务逻辑测试——业务规则由 entity-relation-model.test.ts 覆盖。
 *     这里只回答"页面能不能真的渲染出来"，因为 typecheck 证明不了这一点。
 *  2. 反向关系提示与删除策略说明必须出现在界面上：
 *     前者防止用户以为反向会自动生成，后者防止把"保护生效"误读成"接口坏了"。
 */
import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { h } from 'vue'
import { createPinia } from 'pinia'
import { NAlert, NButton, NCard, NDataTable, NForm, NFormItem, NInput, NMessageProvider, NSelect } from 'naive-ui'
import { createI18n } from 'vue-i18n'
import EntityRelationPage from '../index.vue'

vi.mock('@/service/api', () => ({
  createEntityRelation: vi.fn(async () => ({ data: null, error: null })),
  listEntityRelations: vi.fn(async () => ({ data: { list: [], total: 0 }, error: null })),
  deleteEntityRelation: vi.fn(async () => ({ data: { deleted: 1 }, error: null })),
  deleteEntityRelationsForEntity: vi.fn(async () => ({ data: { deleted: 0 }, error: null }))
}))

const messages = {
  // 项目全局 i18n 默认 locale 为 en，这里按 en 提供文案，
  // 否则 $t 会回退并让断言凭空失败（不是页面没渲染，是语言不匹配）。
  en: {
    common: { add: 'Create', delete: 'Delete', action: 'Action', addSuccess: 'Added', deleteSuccess: 'Deleted' },
    custom: {
      entityRelation: {
        from: 'From',
        to: 'To',
        relationType: 'Relation type',
        metadata: 'Metadata',
        metadataBytes: 'Metadata bytes',
        createTitle: 'Create relation',
        listTitle: 'Relations',
        purgeTitle: 'Delete relations by entity',
        policyProtect: 'Protect (refuse if any relation exists)',
        policyCascade: 'Cascade (delete all relations)',
        entityId: 'Entity ID',
        entityIdRequired: 'Entity ID is required',
        confirmDelete: 'Delete this relation?',
        confirmPurge: 'Delete using the selected policy?',
        reverseExists: 'A reverse relation already exists'
      }
    }
  }
}

function mountPage() {
  const i18n = createI18n({ legacy: false, locale: 'en', messages })
  // 页面在 setup 里调用 useMessage()，缺少 provider 会直接抛错——
  // 这不是被测逻辑的问题，是挂载环境的必要条件。
  return mount(NMessageProvider, {
    slots: { default: () => h(EntityRelationPage) },
    global: {
      plugins: [i18n, createPinia()],
      stubs: {
        NAlert,
        NButton,
        NCard,
        NDataTable,
        NForm,
        NFormItem,
        NInput,
        NSelect
      }
    }
  })
}

describe('entity relation page', () => {
  it('mounts with the three sections', async () => {
    const wrapper = mountPage()
    await wrapper.vm.$nextTick()
    const text = wrapper.text()
    expect(text).toContain('Create relation')
    expect(text).toContain('Relations')
    expect(text).toContain('Delete relations by entity')
  })

  it('spells out the deletion policy consequence rather than a bare button', async () => {
    const wrapper = mountPage()
    await wrapper.vm.$nextTick()
    // protect 是默认策略：界面必须说明"有关系则拒绝"，
    // 否则用户遇到后端拒绝时会以为接口坏了。
    expect(wrapper.text()).toContain('保护')
  })
})
