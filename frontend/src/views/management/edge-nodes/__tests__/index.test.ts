/**
 * 文件用途：覆盖 management/edge-nodes 迁移 useListPage 并拆分模块后的页面契约。
 * 核心逻辑：mock 边缘节点 API，验证列表加载与本地切片分页（/edge/nodes 仅支持 limit）、
 *   心跳后回刷列表，以及注册 / 证书 / 升级三个子组件的请求参数、表单清洗与事件。
 * 关键注意事项：本套件只覆盖前端组件行为，不证明后端注册幂等/证书签发/版本递增语义。
 */
import { defineComponent, h } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import type { VueWrapper } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchEdgeNodes: vi.fn(),
  heartbeatEdgeNode: vi.fn(),
  registerEdgeNode: vi.fn(),
  fetchEdgeNodeCertificate: vi.fn(),
  issueEdgeNodeCertificate: vi.fn(),
  revokeEdgeNodeCertificate: vi.fn(),
  upgradeEdgeNode: vi.fn(),
  rollbackEdgeNode: vi.fn(),
  fetchEdgeNodeUpgradeHistory: vi.fn()
}))

vi.mock('@/service/api', () => ({
  fetchEdgeNodes: hoisted.fetchEdgeNodes,
  heartbeatEdgeNode: hoisted.heartbeatEdgeNode,
  registerEdgeNode: hoisted.registerEdgeNode,
  fetchEdgeNodeCertificate: hoisted.fetchEdgeNodeCertificate,
  issueEdgeNodeCertificate: hoisted.issueEdgeNodeCertificate,
  revokeEdgeNodeCertificate: hoisted.revokeEdgeNodeCertificate,
  upgradeEdgeNode: hoisted.upgradeEdgeNode,
  rollbackEdgeNode: hoisted.rollbackEdgeNode,
  fetchEdgeNodeUpgradeHistory: hoisted.fetchEdgeNodeUpgradeHistory
}))

vi.mock('@/locales', () => ({ $t: (key: string) => key }))

function tagStub(tag = 'div') {
  return defineComponent({
    name: 'TagStub',
    inheritAttrs: false,
    setup(_, { attrs, slots }) {
      return () => h(tag, attrs, slots.default?.())
    }
  })
}

import EdgeNodesPage from '../index.vue'
import NodeCertModal from '../modules/node-cert-modal.vue'
import NodeRegisterModal from '../modules/node-register-modal.vue'
import NodeUpgradeDrawer from '../modules/node-upgrade-drawer.vue'

const nodeFixture = (id: string, overrides: Record<string, unknown> = {}) => ({
  id,
  tenant_id: 'tenant-1',
  version: '1.0.0',
  capabilities: ['modbus'],
  status: 'active',
  last_seen_at: '2026-09-01T00:00:00Z',
  health: 'online',
  ...overrides
})

const certFixture = {
  id: 'cert-1',
  node_id: 'node-1',
  serial_number: 'SERIAL-1',
  fingerprint: 'FP:01',
  common_name: 'node-1',
  certificate: 'CERT-PEM',
  not_before: '2026-01-01T00:00:00Z',
  not_after: '2027-01-01T00:00:00Z',
  status: 'active',
  issued_at: '2026-01-01T00:00:00Z'
}

const historyFixture = {
  id: 'h-1',
  tenant_id: 'tenant-1',
  node_id: 'node-1',
  from_version: '1.0.0',
  target_version: '1.1.0',
  status: 'success',
  operator_id: 'op-1',
  created_at: '2026-09-01T00:00:00Z',
  updated_at: '2026-09-01T00:05:00Z'
}

const mountedWrappers: Array<VueWrapper> = []

function mountComponent(
  component: typeof EdgeNodesPage | typeof NodeCertModal | typeof NodeRegisterModal | typeof NodeUpgradeDrawer,
  props: Record<string, unknown> = {}
) {
  const wrapper = shallowMount(component as never, {
    props,
    global: {
      stubs: {
        'n-card': tagStub(),
        'n-data-table': tagStub(),
        'n-button': tagStub('button'),
        NodeRegisterModal: tagStub(),
        NodeCertModal: tagStub(),
        NodeUpgradeDrawer: tagStub()
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function getSetupState(wrapper: VueWrapper) {
  return wrapper.vm.$.setupState as unknown as Record<string, any>
}

describe('management/edge-nodes/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchEdgeNodes.mockResolvedValue({
      data: [nodeFixture('node-1'), nodeFixture('node-2')],
      error: null
    })
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('loads the node registry on mount with the 200 limit', async () => {
    const wrapper = mountComponent(EdgeNodesPage)
    await flushPromises()

    expect(hoisted.fetchEdgeNodes).toHaveBeenCalledWith(200)

    const state = getSetupState(wrapper)
    expect(state.nodes).toHaveLength(2)
    expect(state.pagination.itemCount).toBe(2)
  })

  it('slices the fetched registry locally by page/page_size', async () => {
    hoisted.fetchEdgeNodes.mockResolvedValue({
      data: [nodeFixture('node-1'), nodeFixture('node-2'), nodeFixture('node-3')],
      error: null
    })
    const wrapper = mountComponent(EdgeNodesPage)
    await flushPromises()

    const state = getSetupState(wrapper)
    await state.pagination.onUpdatePageSize(2)
    await flushPromises()

    expect(state.nodes.map((row: { id: string }) => row.id)).toEqual(['node-1', 'node-2'])

    await state.pagination.onUpdatePage(2)
    await flushPromises()

    // 后端仍只被以 limit 调用，分页发生在 fetcher 内的本地切片。
    expect(hoisted.fetchEdgeNodes).toHaveBeenLastCalledWith(200)
    expect(state.nodes.map((row: { id: string }) => row.id)).toEqual(['node-3'])
    expect(state.pagination.itemCount).toBe(3)
  })

  it('sends a heartbeat and reloads the list', async () => {
    hoisted.heartbeatEdgeNode.mockResolvedValue({ data: null, error: null })
    const wrapper = mountComponent(EdgeNodesPage)
    await flushPromises()

    const state = getSetupState(wrapper)
    await state.handleHeartbeat('node-1')
    await flushPromises()

    expect(hoisted.heartbeatEdgeNode).toHaveBeenCalledWith('node-1')
    expect(hoisted.fetchEdgeNodes).toHaveBeenCalledTimes(2)
  })
})

describe('management/edge-nodes/modules/node-register-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('registers with trimmed fields, split capabilities and reloads through the parent', async () => {
    hoisted.registerEdgeNode.mockResolvedValue({ data: null, error: null })
    const wrapper = mountComponent(NodeRegisterModal, { show: true })
    await flushPromises()

    const state = getSetupState(wrapper)
    state.registerForm.node_id = ' edge-01 '
    state.registerForm.version = ' 1.0.0 '
    state.registerForm.capabilities = ' modbus , opcua , '
    await state.handleRegister()
    await flushPromises()

    expect(hoisted.registerEdgeNode).toHaveBeenCalledWith({
      node_id: 'edge-01',
      version: '1.0.0',
      capabilities: ['modbus', 'opcua']
    })
    expect(wrapper.emitted('registered')).toHaveLength(1)
    expect(wrapper.emitted('update:show')?.at(-1)).toEqual([false])
    expect(state.registerForm.node_id).toBe('')
  })

  it('omits capabilities when empty and keeps the modal open on failure', async () => {
    hoisted.registerEdgeNode.mockResolvedValueOnce({ data: null, error: new Error('conflict') })
    const wrapper = mountComponent(NodeRegisterModal, { show: true })
    await flushPromises()

    const state = getSetupState(wrapper)
    state.registerForm.node_id = 'edge-02'
    state.registerForm.version = '2.0.0'
    state.registerForm.capabilities = '  ,  '
    await state.handleRegister()
    await flushPromises()

    expect(hoisted.registerEdgeNode.mock.calls[0][0]).not.toHaveProperty('capabilities')
    expect(wrapper.emitted('registered')).toBeUndefined()
    expect(wrapper.emitted('update:show')).toBeUndefined()
  })
})

describe('management/edge-nodes/modules/node-cert-modal.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches the certificate when opened for a node', async () => {
    hoisted.fetchEdgeNodeCertificate.mockResolvedValue({ data: certFixture, error: null })
    const wrapper = mountComponent(NodeCertModal, { show: false, node: nodeFixture('node-1') })
    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(hoisted.fetchEdgeNodeCertificate).toHaveBeenCalledWith('node-1')
    expect(getSetupState(wrapper).certInfo?.serial_number).toBe('SERIAL-1')
  })

  it('issues a certificate with the chosen validity and shows the one-time private key', async () => {
    hoisted.fetchEdgeNodeCertificate.mockResolvedValue({ data: null, error: null })
    hoisted.issueEdgeNodeCertificate.mockResolvedValue({ data: { ...certFixture, private_key: 'PK-PEM' }, error: null })
    const wrapper = mountComponent(NodeCertModal, { show: false, node: nodeFixture('node-1') })
    await wrapper.setProps({ show: true })
    await flushPromises()

    const state = getSetupState(wrapper)
    state.certValidityDays = 30
    await state.handleIssueCert()
    await flushPromises()

    expect(hoisted.issueEdgeNodeCertificate).toHaveBeenCalledWith('node-1', 30)
    expect(state.newlyIssuedKey).toBe('PK-PEM')
  })

  it('revokes the certificate and clears the one-time key', async () => {
    hoisted.issueEdgeNodeCertificate.mockResolvedValue({ data: { ...certFixture, private_key: 'PK-PEM' }, error: null })
    hoisted.revokeEdgeNodeCertificate.mockResolvedValue({ data: { message: 'ok' }, error: null })
    const wrapper = mountComponent(NodeCertModal, { show: false, node: nodeFixture('node-1') })
    await wrapper.setProps({ show: true })
    await flushPromises()

    const state = getSetupState(wrapper)
    await state.handleIssueCert()
    await flushPromises()

    await state.handleRevokeCert()
    await flushPromises()

    expect(hoisted.revokeEdgeNodeCertificate).toHaveBeenCalledWith('node-1')
    expect(state.certInfo).toBeNull()
    expect(state.newlyIssuedKey).toBe('')
  })
})

describe('management/edge-nodes/modules/node-upgrade-drawer.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchEdgeNodeUpgradeHistory.mockResolvedValue({ data: [historyFixture], error: null })
  })

  it('loads the upgrade history when opened for a node', async () => {
    const wrapper = mountComponent(NodeUpgradeDrawer, { show: false, node: nodeFixture('node-1') })
    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(hoisted.fetchEdgeNodeUpgradeHistory).toHaveBeenCalledWith('node-1', 50)
    expect(getSetupState(wrapper).upgradeHistory).toHaveLength(1)
  })

  it('skips the upgrade call without a target version, then submits trimmed payloads and refreshes', async () => {
    hoisted.upgradeEdgeNode.mockResolvedValue({
      data: {
        history_id: 'h-2',
        node_id: 'node-1',
        from_version: '1.0.0',
        target_version: '1.1.0',
        status: 'dispatched',
        message: 'ok'
      },
      error: null
    })
    const wrapper = mountComponent(NodeUpgradeDrawer, { show: false, node: nodeFixture('node-1') })
    await wrapper.setProps({ show: true })
    await flushPromises()

    const state = getSetupState(wrapper)
    await state.handleUpgrade()
    expect(hoisted.upgradeEdgeNode).not.toHaveBeenCalled()

    state.upgradeForm.target_version = ' 1.1.0 '
    state.upgradeForm.package_url = ' https://pkg '
    state.upgradeForm.checksum = ' sha256:abc '
    state.upgradeForm.description = ' nightly '
    await state.handleUpgrade()
    await flushPromises()

    expect(hoisted.upgradeEdgeNode).toHaveBeenCalledWith('node-1', {
      target_version: '1.1.0',
      package_url: 'https://pkg',
      checksum: 'sha256:abc',
      description: 'nightly'
    })
    expect(wrapper.emitted('updated')).toHaveLength(1)
    expect(hoisted.fetchEdgeNodeUpgradeHistory).toHaveBeenCalledTimes(2)
    expect(state.upgradeForm.target_version).toBe('')
  })

  it('rolls back to a history entry and refreshes history plus the node list', async () => {
    hoisted.rollbackEdgeNode.mockResolvedValue({
      data: { history_id: 'h-1', node_id: 'node-1', rolled_to_version: '1.0.0', status: 'dispatched', message: 'ok' },
      error: null
    })
    const wrapper = mountComponent(NodeUpgradeDrawer, { show: false, node: nodeFixture('node-1') })
    await wrapper.setProps({ show: true })
    await flushPromises()

    const state = getSetupState(wrapper)
    await state.handleRollback('h-1')
    await flushPromises()

    expect(hoisted.rollbackEdgeNode).toHaveBeenCalledWith('node-1', 'h-1')
    expect(wrapper.emitted('updated')).toHaveLength(1)
    expect(hoisted.fetchEdgeNodeUpgradeHistory).toHaveBeenCalledTimes(2)
  })
})
