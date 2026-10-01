/**
 * 文件用途: 升级包表单组合函数（use-ota-package-form）的单元测试。
 * 核心逻辑: 覆盖打开/重置弹窗、载荷构建校验（必填与 additional_info JSON）、保存分流
 *   （新增/编辑）、保存成功后刷新，以及固件文件选择与上传回填。
 * 关键注意事项: savePackage 成功后必须调用 refresh（列表 useListPage.load）再返回 true。
 * 重构建议: 若表单校验改为 rules 驱动，把 buildPayload 相关断言迁移到校验器测试。
 */
import { beforeEach, describe, expect, it, vi } from 'vitest'

const hoisted = vi.hoisted(() => ({
  addOtaPackage: vi.fn(),
  editOtaPackage: vi.fn(),
  uploadFile: vi.fn()
}))

vi.mock('@/service/product/update-package', () => ({
  addOtaPackage: hoisted.addOtaPackage,
  editOtaPackage: hoisted.editOtaPackage
}))

vi.mock('@/service/api/personal-center', () => ({
  uploadFile: hoisted.uploadFile
}))

vi.mock('@/locales', () => ({
  $t: (key: string) => key
}))

import { useOtaPackageForm } from '../use-ota-package-form'

const messageMock = {
  success: vi.fn(),
  warning: vi.fn(),
  error: vi.fn()
}

describe('useOtaPackageForm', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    Object.defineProperty(window, '$message', {
      configurable: true,
      value: messageMock
    })
    hoisted.addOtaPackage.mockResolvedValue({ error: null })
    hoisted.editOtaPackage.mockResolvedValue({ error: null })
    hoisted.uploadFile.mockResolvedValue({ data: { path: './upgradePackage/pkg.bin' }, error: null })
  })

  it('openCreateModal resets the form and shows the modal', () => {
    const { form, modalVisible, isEditing, openEditModal, openCreateModal } = useOtaPackageForm({ refresh: vi.fn() })

    openEditModal({ id: 'pkg-1', name: 'Pkg 1' })
    openCreateModal()

    expect(modalVisible.value).toBe(true)
    expect(isEditing.value).toBe(false)
    expect(form.id).toBe('')
    expect(form.name).toBe('')
    expect(form.package_type).toBe(2)
    expect(form.signature_type).toBe('MD5')
  })

  it('openEditModal fills the form from a row', () => {
    const { form, modalVisible, isEditing, openEditModal } = useOtaPackageForm({ refresh: vi.fn() })

    openEditModal({
      id: '1',
      name: 'Pkg1',
      version: '1.0',
      target_version: '2.0',
      device_config_id: 'dc1',
      module: 'mod',
      package_type: 1,
      signature_type: 'SHA256',
      package_url: '/pkg.bin',
      additional_info: '{}',
      description: 'desc',
      remark: ''
    })

    expect(modalVisible.value).toBe(true)
    expect(isEditing.value).toBe(true)
    expect(form.id).toBe('1')
    expect(form.name).toBe('Pkg1')
    expect(form.package_type).toBe(1)
    expect(form.device_config_id).toBe('dc1')
  })

  it('rejects payload with empty required fields', () => {
    const { form, buildPayload } = useOtaPackageForm({ refresh: vi.fn() })
    form.name = ''
    form.version = ''
    form.device_config_id = null
    form.package_url = ''
    form.additional_info = '{}'

    expect(buildPayload()).toBeNull()
    expect(messageMock.warning).toHaveBeenCalledWith('common.saveFailed')
  })

  it('rejects invalid additional_info JSON before saving', () => {
    const { form, buildPayload } = useOtaPackageForm({ refresh: vi.fn() })
    form.name = 'Pkg 1'
    form.version = '1.0.0'
    form.device_config_id = 'cfg-1'
    form.package_url = '/files/pkg.bin'
    form.additional_info = '{bad json'

    expect(buildPayload()).toBeNull()
    expect(messageMock.error).toHaveBeenCalledWith('page.product.update-package.customInfo')
  })

  it('trims fields and builds a create payload', () => {
    const { form, buildPayload } = useOtaPackageForm({ refresh: vi.fn() })
    form.name = '  Pkg 1  '
    form.version = ' 1.0.0 '
    form.target_version = ' 2.0.0 '
    form.device_config_id = 'cfg-1'
    form.module = ' firmware '
    form.package_type = 1
    form.signature_type = 'SHA256'
    form.package_url = ' /files/pkg.bin '
    form.additional_info = ''
    form.description = ' release '
    form.remark = ' remark '

    expect(buildPayload()).toEqual({
      id: undefined,
      name: 'Pkg 1',
      version: '1.0.0',
      target_version: '2.0.0',
      device_config_id: 'cfg-1',
      module: 'firmware',
      package_type: 1,
      signature_type: 'SHA256',
      package_url: '/files/pkg.bin',
      additional_info: '{}',
      description: 'release',
      remark: 'remark'
    })
  })

  it('adds a package, refreshes the list and closes the modal on success', async () => {
    const refresh = vi.fn().mockResolvedValue(undefined)
    const { form, modalVisible, savePackage } = useOtaPackageForm({ refresh })
    form.name = 'Pkg 1'
    form.version = '1.0.0'
    form.device_config_id = 'cfg-1'
    form.package_url = '/files/pkg.bin'

    const saved = await savePackage()

    expect(saved).toBe(true)
    expect(hoisted.addOtaPackage).toHaveBeenCalledTimes(1)
    expect(hoisted.addOtaPackage).toHaveBeenCalledWith(
      expect.objectContaining({ name: 'Pkg 1', device_config_id: 'cfg-1' })
    )
    expect(refresh).toHaveBeenCalledTimes(1)
    expect(modalVisible.value).toBe(false)
  })

  it('edits instead of adding when the edit modal is active', async () => {
    const refresh = vi.fn().mockResolvedValue(undefined)
    const { openEditModal, savePackage } = useOtaPackageForm({ refresh })
    openEditModal({
      id: 'pkg-1',
      name: 'Pkg 1',
      device_config_id: 'cfg-1',
      package_url: '/files/pkg.bin',
      version: '1.0'
    })

    const saved = await savePackage()

    expect(saved).toBe(true)
    expect(hoisted.editOtaPackage).toHaveBeenCalledTimes(1)
    expect(hoisted.editOtaPackage).toHaveBeenCalledWith(expect.objectContaining({ id: 'pkg-1' }))
    expect(hoisted.addOtaPackage).toHaveBeenCalledTimes(0)
    expect(refresh).toHaveBeenCalledTimes(1)
  })

  it('does not refresh when saving fails', async () => {
    hoisted.addOtaPackage.mockResolvedValueOnce({ error: 'boom' })
    const refresh = vi.fn().mockResolvedValue(undefined)
    const { form, savePackage } = useOtaPackageForm({ refresh })
    form.name = 'Pkg 1'
    form.version = '1.0.0'
    form.device_config_id = 'cfg-1'
    form.package_url = '/files/pkg.bin'

    expect(await savePackage()).toBe(false)
    expect(refresh).not.toHaveBeenCalled()
  })

  it('uploads the selected firmware file and stores the returned path', async () => {
    const { selectedFile, selectPackageFile, form, uploadSelectedFile } = useOtaPackageForm({ refresh: vi.fn() })
    selectPackageFile(new File(['abc'], 'pkg.bin'))
    expect(selectedFile.value?.name).toBe('pkg.bin')

    await uploadSelectedFile()

    expect(hoisted.uploadFile).toHaveBeenCalledTimes(1)
    const formData = hoisted.uploadFile.mock.calls[0][0] as FormData
    expect(formData.get('type')).toBe('upgradePackage')
    expect(form.package_url).toBe('./upgradePackage/pkg.bin')
    expect(messageMock.success).toHaveBeenCalledWith('common.operationSuccess')
  })

  it('warns and skips upload when no firmware file is selected', async () => {
    const { uploadSelectedFile } = useOtaPackageForm({ refresh: vi.fn() })

    await uploadSelectedFile()

    expect(messageMock.warning).toHaveBeenCalledWith('page.product.update-package.packagePlaceholder')
    expect(hoisted.uploadFile).toHaveBeenCalledTimes(0)
  })

  it('reports an error when the upload response has no path', async () => {
    hoisted.uploadFile.mockResolvedValueOnce({ data: { path: '' }, error: null })
    const { form, selectPackageFile, uploadSelectedFile } = useOtaPackageForm({ refresh: vi.fn() })
    selectPackageFile(new File(['abc'], 'pkg.bin'))

    await uploadSelectedFile()

    expect(messageMock.error).toHaveBeenCalledWith('custom.management.uploadFailed')
    expect(form.package_url).toBe('')
  })
})
