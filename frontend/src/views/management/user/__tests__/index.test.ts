/**
 * 文件用途：覆盖 index 在 后台用户管理 场景下的前端行为与契约。
 * 核心逻辑：通过 Vue Test Utils 与 Vitest mock 服务、路由或组件依赖，断言关键渲染、交互和数据流。
 * 关键注意事项：仅作为视图层回归用例，mock 数据需与页面接口契约同步，避免把实现细节当成唯一断言。
 * 重构建议：后续可抽取稳定的工厂数据和挂载工具，减少重复 mock 与选择器耦合。
 */
import { defineComponent, h, ref } from 'vue'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  fetchUserList: vi.fn(),
  delUser: vi.fn(),
  enter: vi.fn(),
  messageSuccess: vi.fn(),
  currentInstanceProxy: {
    getPlatform: () => false
  },
  routeQuery: {} as Record<string, unknown>,
  routerPush: vi.fn()
}))

vi.mock('@/service/api/auth', () => ({
  fetchUserList: hoisted.fetchUserList,
  delUser: hoisted.delUser
}))

// index.vue 通过 useRoute().query.setup 判断租户管理员引导态，
// 未 mock vue-router 时 route 为 undefined，挂载即抛 reading 'query'。
vi.mock('vue-router', () => ({
  useRoute: () => ({ query: hoisted.routeQuery }),
  useRouter: () => ({ push: hoisted.routerPush, replace: vi.fn(), back: vi.fn() })
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

vi.mock('vue', async importOriginal => {
  const actual = await importOriginal<typeof import('vue')>()
  return {
    ...actual,
    getCurrentInstance: () => ({
      proxy: hoisted.currentInstanceProxy
    })
  }
})

vi.mock('@aetherlink/hooks', () => ({
  useBoolean: (init = false) => {
    const bool = ref(init)
    return {
      bool,
      setTrue: vi.fn(() => {
        bool.value = true
      }),
      setFalse: vi.fn(() => {
        bool.value = false
      })
    }
  },
  useLoading: (init = false) => {
    const loading = ref(init)
    return {
      loading,
      startLoading: vi.fn(() => {
        loading.value = true
      }),
      endLoading: vi.fn(() => {
        loading.value = false
      })
    }
  }
}))

vi.mock('@/store/modules/auth', () => ({
  useAuthStore: () => ({
    userInfo: { id: '1', email: 'admin@test.com' },
    isLogin: true,
    enter: hoisted.enter
  })
}))

vi.mock('@/constants/business', () => ({
  userStatusOptions: [
    { label: 'Normal', value: 'N' },
    { label: 'Freeze', value: 'F' }
  ],
  routerSysFlagLabels: {},
  routerTypeLabels: {}
}))

vi.mock('@/assets/data/china-region.json', () => ({
  default: [
    {
      name: '北京市',
      children: [
        {
          name: '北京市',
          children: [{ name: '东城区' }]
        }
      ]
    }
  ]
}))

vi.mock('../components/table-action-modal.vue', () => ({
  default: defineComponent({
    name: 'TableActionModalStub',
    setup() {
      return () => h('div', { class: 'table-action-modal-stub' })
    }
  })
}))

vi.mock('../components/edit-password-modal.vue', () => ({
  default: defineComponent({
    name: 'EditPasswordModalStub',
    setup() {
      return () => h('div', { class: 'edit-password-modal-stub' })
    }
  })
}))

import UserIndex from '../index.vue'

const mountedWrappers: Array<ReturnType<typeof shallowMount>> = []

const mountComponent = () => {
  const wrapper = shallowMount(UserIndex, {
    global: {
      stubs: {
        NCard: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NSpace: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NButton: defineComponent({ emits: ['click'], setup(_, { slots, emit }) { return () => h('button', { onClick: () => emit('click') }, slots.default ? slots.default() : []) } }),
        NInput: defineComponent({ name: 'NInput', props: { value: { default: '' }, placeholder: String }, emits: ['update:value'], setup() { return () => h('div') } }),
        NDataTable: defineComponent({ name: 'NDataTable', props: { data: { type: Array, default: () => [] }, loading: Boolean, pagination: { default: null } }, setup() { return () => h('div') } }),
        NTag: defineComponent({ setup(_, { slots }) { return () => h('span', slots.default ? slots.default() : []) } }),
        NPopconfirm: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NSelect: defineComponent({ name: 'NSelect', props: { value: { default: null }, placeholder: String }, emits: ['update:value'], setup() { return () => h('div') } }),
        NCascader: defineComponent({ name: 'NCascader', props: { value: { default: null }, placeholder: String }, emits: ['update:value'], setup() { return () => h('div') } }),
        NForm: defineComponent({ setup(_, { slots }) { return () => h('form', slots.default ? slots.default() : []) } }),
        NFormItem: defineComponent({ name: 'NFormItem', props: { label: String, path: String }, setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        NEmpty: defineComponent({ setup(_, { slots }) { return () => h('div', slots.default ? slots.default() : []) } }),
        IconIcRoundPlus: true
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

const getSetupState = (wrapper: ReturnType<typeof shallowMount>) => wrapper.vm.$.setupState as Record<string, any>

const mockUser = (overrides: Record<string, any> = {}) => ({
  id: 'u-1',
  email: 'user@test.com',
  name: 'User One',
  phone_number: '+8613800000000',
  created_at: 1718900000,
  status: 'N',
  lastVisitTime: 1718900100,
  remark: 'remark',
  ...overrides
})

describe('management/user/index.vue', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    hoisted.fetchUserList.mockResolvedValue({
      data: { list: [mockUser()], total: 1 }
    })
    hoisted.delUser.mockResolvedValue({ error: null })
    hoisted.enter.mockResolvedValue(undefined)
    ;(globalThis as any).$message = { success: hoisted.messageSuccess }
  })

  afterEach(() => {
    while (mountedWrappers.length > 0) {
      mountedWrappers.pop()?.unmount()
    }
  })

  it('binds the fetched users to the data table on mount', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const table = wrapper.getComponent({ name: 'NDataTable' })

    expect(hoisted.fetchUserList).toHaveBeenCalledWith(
      expect.objectContaining({
        page: 1,
        page_size: 10
      })
    )
    expect(table.props('data')).toEqual([mockUser()])
    expect(table.props('loading')).toBe(false)
    expect(table.props('pagination')).toMatchObject({
      page: 1,
      pageSize: 10,
      itemCount: 1
    })
  })

  it('calls getTableData on init', async () => {
    mountComponent()
    await flushPromises()
    expect(hoisted.fetchUserList).toHaveBeenCalledTimes(1)
  })

  it('populates tableData and pagination on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.tableData).toHaveLength(1)
    expect(state.pagination.itemCount).toBe(1)
    expect(state.loading).toBe(false)
  })

  it('sets showEmpty to true when data list is null', async () => {
    hoisted.fetchUserList.mockResolvedValue({ data: { list: null, total: 0 } })
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.showEmpty).toBe(true)
  })

  it('sets showEmpty to false when data list is not null', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.showEmpty).toBe(false)
  })

  it('customUserStatusOptions maps status options with translations', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const options = state.customUserStatusOptions
    expect(options).toHaveLength(2)
    expect(options[0]).toEqual({ label: 'page.manage.user.status.normal', value: 'N' })
    expect(options[1]).toEqual({ label: 'page.manage.user.status.freeze', value: 'F' })
  })

  it('handleAddTable sets modal type to add and opens modal', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleAddTable()
    expect(state.modalType).toBe('add')
    expect(state.visible).toBe(true)
  })

  it('handleEditTable sets modal type to edit and opens modal with edit data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleEditTable('u-1')
    expect(state.modalType).toBe('edit')
    expect(state.visible).toBe(true)
    expect(state.editData).toEqual(expect.objectContaining({ id: 'u-1' }))
  })

  it('handleEditTable does not set editData when row not found', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleEditTable('non-existent')
    expect(state.modalType).toBe('edit')
    expect(state.visible).toBe(true)
    expect(state.editData).toBeNull()
  })

  it('handleEditPwd opens edit password modal with edit data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleEditPwd('u-1')
    expect(state.editPwdVisible).toBe(true)
    expect(state.editData).toEqual(expect.objectContaining({ id: 'u-1' }))
  })

  it('handleEditPwd does not set editData when row not found', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleEditPwd('non-existent')
    expect(state.editPwdVisible).toBe(true)
    expect(state.editData).toBeNull()
  })

  it('handleEnter calls authStore.enter with rowId', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    await state.handleEnter('u-1')
    expect(hoisted.enter).toHaveBeenCalledWith('u-1')
  })

  it('handleDeleteTable calls delUser and refreshes data on success', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.delUser.mockResolvedValue({ error: null })
    hoisted.fetchUserList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    await state.handleDeleteTable('u-1')
    await flushPromises()
    expect(hoisted.delUser).toHaveBeenCalledWith('u-1')
    expect(hoisted.messageSuccess).toHaveBeenCalledTimes(1)
    expect(hoisted.messageSuccess).toHaveBeenCalledWith('common.deleteSuccess')
    expect(hoisted.fetchUserList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchUserList).toHaveBeenCalledWith(expect.objectContaining({ page: 1, page_size: 10 }))
  })

  it('handleDeleteTable does not refresh when error occurs', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.delUser.mockResolvedValue({ error: 'fail' })
    const state = getSetupState(wrapper)
    await state.handleDeleteTable('u-1')
    expect(hoisted.fetchUserList).toHaveBeenCalledTimes(0)
  })

  it('handleQuery resets page to 1 and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.fetchUserList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.queryParams.page = 5
    state.handleQuery()
    await flushPromises()
    expect(state.queryParams.page).toBe(1)
    expect(hoisted.fetchUserList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchUserList).toHaveBeenCalledWith(expect.objectContaining({ page: 1, page_size: 10 }))
  })

  it('handleReset clears filters and queries', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.fetchUserList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.queryParams.email = 'test@test.com'
    state.queryParams.name = 'test'
    state.queryParams.status = 'N'
    state.queryParams.organization = 'org'
    state.queryParams.timezone = 'Asia/Shanghai'
    state.queryParams.default_language = 'zh-CN'
    state.handleReset()
    await flushPromises()
    expect(state.queryParams.email).toBeNull()
    expect(state.queryParams.name).toBeNull()
    expect(state.queryParams.status).toBeNull()
    expect(state.queryParams.organization).toBeNull()
    expect(state.queryParams.timezone).toBeNull()
    expect(state.queryParams.default_language).toBeNull()
    expect(state.queryParams.page).toBe(1)
    expect(hoisted.fetchUserList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchUserList).toHaveBeenCalledWith(expect.objectContaining({
      email: null,
      name: null,
      status: null,
      organization: null,
      timezone: null,
      default_language: null,
      page: 1,
      page_size: 10
    }))
  })

  it('handleReset clears address fields', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.fetchUserList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.queryParams.address.province = '北京市'
    state.queryParams.address.city = '北京市'
    state.queryParams.address.district = '东城区'
    state.queryParams.address.detailed_address = '某路1号'
    state.queryParams.address.cascaderValue = ['北京市', '北京市', '东城区']
    state.handleReset()
    await flushPromises()
    expect(state.queryParams.address.province).toBeNull()
    expect(state.queryParams.address.city).toBeNull()
    expect(state.queryParams.address.district).toBeNull()
    expect(state.queryParams.address.detailed_address).toBeNull()
    expect(state.queryParams.address.cascaderValue).toBeNull()
  })

  it('uses localized filter labels and placeholders for tenant profile fields', async () => {
    const wrapper = mountComponent()
    await flushPromises()

    const filterItems = wrapper.findAllComponents({ name: 'NFormItem' }).map(item => item.props())
    expect(filterItems).toEqual(expect.arrayContaining([
      expect.objectContaining({ label: 'page.manage.user.organization', path: 'organization' }),
      expect.objectContaining({ label: 'page.manage.user.address', path: 'address.province' }),
      expect.objectContaining({ label: 'page.manage.user.detailedAddress', path: 'address.detailed_address' }),
      expect.objectContaining({ label: 'page.manage.user.timezone', path: 'timezone' })
    ]))

    const inputPlaceholders = wrapper.findAllComponents({ name: 'NInput' }).map(input => input.props('placeholder'))
    expect(inputPlaceholders).toEqual(expect.arrayContaining([
      'page.manage.user.form.organization',
      'page.manage.user.form.detailedAddress'
    ]))

    expect(wrapper.getComponent({ name: 'NCascader' }).props('placeholder')).toBe('page.manage.user.form.address')

    const selectPlaceholders = wrapper.findAllComponents({ name: 'NSelect' }).map(select => select.props('placeholder'))
    expect(selectPlaceholders).toEqual(expect.arrayContaining([
      'page.manage.user.form.timezone',
      'page.manage.user.form.defaultLanguage'
    ]))
  })

  it('handleAddressChange maps cascader value to address fields when length >= 3', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleAddressChange(['北京市', '北京市', '东城区'])
    expect(state.queryParams.address.province).toBe('北京市')
    expect(state.queryParams.address.city).toBe('北京市')
    expect(state.queryParams.address.district).toBe('东城区')
    expect(state.queryParams.address.cascaderValue).toEqual(['北京市', '北京市', '东城区'])
  })

  it('handleAddressChange clears address fields when length < 3', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    state.handleAddressChange(['北京市'])
    expect(state.queryParams.address.province).toBeNull()
    expect(state.queryParams.address.city).toBeNull()
    expect(state.queryParams.address.district).toBeNull()
  })

  it('filterCascader matches label case-insensitively', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.filterCascader('北京', { label: '北京市' })).toBe(true)
    expect(state.filterCascader('shanghai', { label: '北京市' })).toBe(false)
  })

  it('pagination.onChange updates page and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.fetchUserList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.pagination.onChange(3)
    await flushPromises()
    expect(state.queryParams.page).toBe(3)
    expect(state.pagination.page).toBe(3)
    expect(hoisted.fetchUserList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchUserList).toHaveBeenCalledWith(expect.objectContaining({ page: 3, page_size: 10 }))
  })

  it('pagination.onUpdatePageSize resets to page 1 and fetches', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    vi.clearAllMocks()
    hoisted.fetchUserList.mockResolvedValue({ data: { list: [], total: 0 } })
    const state = getSetupState(wrapper)
    state.queryParams.page = 4
    state.pagination.onUpdatePageSize(20)
    await flushPromises()
    expect(state.queryParams.page_size).toBe(20)
    expect(state.queryParams.page).toBe(1)
    expect(state.pagination.page).toBe(1)
    expect(hoisted.fetchUserList).toHaveBeenCalledTimes(1)
    expect(hoisted.fetchUserList).toHaveBeenCalledWith(expect.objectContaining({ page: 1, page_size: 20 }))
  })

  it('convertPwDataToCascader transforms region data', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const result = state.convertPwDataToCascader([
      {
        name: '北京市',
        children: [{ name: '北京市', children: [{ name: '东城区' }] }]
      }
    ])
    expect(result[0].value).toBe('北京市')
    expect(result[0].label).toBe('北京市')
    expect(result[0].children[0].value).toBe('北京市')
    expect(result[0].children[0].children[0].value).toBe('东城区')
  })

  it('getPlatform returns value from proxy', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.getPlatform).toBe(false)
  })

  it('columns are defined', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(Array.isArray(state.columns)).toBe(true)
    expect(state.columns.map((column: any) => column.key)).toEqual([
      'email',
      'name',
      'phone_number',
      'created_at',
      'status',
      'lastVisitTime',
      'remark',
      'actions'
    ])
  })

  it('columns include key columns', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    const columnKeys = state.columns.map((c: any) => c.key)
    expect(columnKeys).toContain('email')
    expect(columnKeys).toContain('name')
    expect(columnKeys).toContain('status')
    expect(columnKeys).toContain('actions')
  })

  it('queryParams has correct initial values', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.queryParams.page).toBe(1)
    expect(state.queryParams.page_size).toBe(10)
    expect(state.queryParams.email).toBeNull()
    expect(state.queryParams.name).toBeNull()
    expect(state.queryParams.status).toBeNull()
    expect(state.queryParams.organization).toBeNull()
    expect(state.queryParams.timezone).toBeNull()
    expect(state.queryParams.default_language).toBeNull()
  })

  it('timezoneOptions contains expected timezones', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.timezoneOptions).toHaveLength(20)
    expect(state.timezoneOptions.some((o: any) => o.value === 'Asia/Shanghai')).toBe(true)
    expect(state.timezoneOptions.some((o: any) => o.value === 'UTC')).toBe(true)
  })

  it('languageOptions contains expected languages', async () => {
    const wrapper = mountComponent()
    await flushPromises()
    const state = getSetupState(wrapper)
    expect(state.languageOptions).toEqual([
      { label: 'Chinese', value: 'zh-CN' },
      { label: 'English', value: 'en-US' },
      { label: 'Francais', value: 'fr-FR' },
      { label: 'Espanol', value: 'es-ES' }
    ])
    expect(state.languageOptions.some((o: any) => o.value === 'zh-CN')).toBe(true)
    expect(state.languageOptions.some((o: any) => o.value === 'en-US')).toBe(true)
  })

  it('provinceCityData is populated from region data', async () => {
    const wrapper = mountComponent()
    const state = getSetupState(wrapper)
    // The component loads the static region module from onMounted. Waiting on
    // the setup loader avoids relying on scheduler timing when all frontend
    // test files are running in parallel on the hosted runner.
    await state.loadProvinceCityData()
    await flushPromises()
    expect(Array.isArray(state.provinceCityData)).toBe(true)
    expect(state.provinceCityData.length).toBeGreaterThan(0)
    expect(state.provinceCityData[0].value).toBe('北京市')
    expect(state.provinceCityData[0].children[0].children[0].value).toBe('东城区')
  })
})
