/**
 * 文件用途: 升级包新增/编辑的表单状态与提交逻辑（页面与 package-form-modal 共用）。
 * 核心逻辑: 表单模型、打开/重置、固件上传路径回填、载荷构建与保存；保存成功后调用
 *   options.refresh 刷新列表。纯 UI 的文件拖拽/原生 input 状态在 package-form-modal 内。
 * 关键注意事项: additional_info 必须是合法 JSON，构建载荷失败只提示不提交。
 * 重构建议: 表单校验加重后可引入 naive-ui Form rules，把校验从 buildPayload 中拆出。
 */
import { reactive, ref } from 'vue'
import { addOtaPackage, editOtaPackage } from '@/service/product/update-package'
import { uploadFile } from '@/service/api/personal-center'
import { $t } from '@/locales'
import type { OtaPackageRecord } from './ota-package-types'

export interface OtaPackageFormModel {
  id: string
  name: string
  version: string
  target_version: string
  device_config_id: string | null
  module: string
  package_type: number
  signature_type: string
  package_url: string
  additional_info: string
  description: string
  remark: string
}

interface UseOtaPackageFormOptions {
  /** 保存成功后的列表刷新（升级包列表的 useListPage.load）。 */
  refresh: () => Promise<unknown>
}

export function useOtaPackageForm(options: UseOtaPackageFormOptions) {
  const saving = ref(false)
  const uploading = ref(false)
  const modalVisible = ref(false)
  const isEditing = ref(false)
  const selectedFile = ref<File | null>(null)

  const form = reactive<OtaPackageFormModel>({
    id: '',
    name: '',
    version: '',
    target_version: '',
    device_config_id: null as string | null,
    module: '',
    package_type: 2,
    signature_type: 'MD5',
    package_url: '',
    additional_info: '{}',
    description: '',
    remark: ''
  })

  const packageTypeOptions = [
    { label: $t('page.product.update-package.diff'), value: 1 },
    { label: $t('page.product.update-package.full'), value: 2 }
  ]

  const signatureOptions = [
    { label: 'MD5', value: 'MD5' },
    { label: 'SHA256', value: 'SHA256' }
  ]

  function resetForm() {
    form.id = ''
    form.name = ''
    form.version = ''
    form.target_version = ''
    form.device_config_id = null
    form.module = ''
    form.package_type = 2
    form.signature_type = 'MD5'
    form.package_url = ''
    form.additional_info = '{}'
    form.description = ''
    form.remark = ''
    selectedFile.value = null
  }

  function openCreateModal() {
    resetForm()
    isEditing.value = false
    modalVisible.value = true
  }

  function openEditModal(row: OtaPackageRecord) {
    resetForm()
    isEditing.value = true
    form.id = row.id
    form.name = row.name || ''
    form.version = row.version || ''
    form.target_version = row.target_version || ''
    form.device_config_id = row.device_config_id || null
    form.module = row.module || ''
    form.package_type = Number(row.package_type || 2)
    form.signature_type = row.signature_type || 'MD5'
    form.package_url = row.package_url || ''
    form.additional_info = row.additional_info || '{}'
    form.description = row.description || ''
    form.remark = row.remark || ''
    modalVisible.value = true
  }

  function selectPackageFile(file?: File | null) {
    selectedFile.value = file || null
  }

  async function uploadSelectedFile() {
    if (!selectedFile.value) {
      window.$message?.warning($t('page.product.update-package.packagePlaceholder'))
      return
    }
    uploading.value = true
    try {
      const payload = new FormData()
      payload.append('file', selectedFile.value)
      payload.append('type', 'upgradePackage')
      const { data, error } = await uploadFile(payload)
      if (!error) {
        const uploadedPath = String(data?.path || '').trim()
        if (!uploadedPath) {
          window.$message?.error($t('custom.management.uploadFailed'))
          return
        }
        form.package_url = uploadedPath
        window.$message?.success($t('common.operationSuccess'))
      }
    } finally {
      uploading.value = false
    }
  }

  function buildPayload() {
    const additionalInfo = form.additional_info.trim() || '{}'
    try {
      JSON.parse(additionalInfo)
    } catch {
      window.$message?.error($t('page.product.update-package.customInfo'))
      return null
    }

    if (!form.name.trim() || !form.version.trim() || !form.device_config_id || !form.package_url.trim()) {
      window.$message?.warning($t('common.saveFailed'))
      return null
    }

    return {
      id: form.id || undefined,
      name: form.name.trim(),
      version: form.version.trim(),
      target_version: form.target_version.trim(),
      device_config_id: form.device_config_id,
      module: form.module.trim(),
      package_type: form.package_type,
      signature_type: form.signature_type,
      package_url: form.package_url.trim(),
      additional_info: additionalInfo,
      description: form.description.trim(),
      remark: form.remark.trim()
    }
  }

  async function savePackage() {
    const payload = buildPayload()
    if (!payload) return false
    saving.value = true
    try {
      const { error } = isEditing.value ? await editOtaPackage(payload) : await addOtaPackage(payload)
      if (!error) {
        window.$message?.success($t('common.saveSuccess'))
        modalVisible.value = false
        await options.refresh()
        return true
      }
      return false
    } finally {
      saving.value = false
    }
  }

  return {
    saving,
    uploading,
    modalVisible,
    isEditing,
    selectedFile,
    form,
    packageTypeOptions,
    signatureOptions,
    resetForm,
    openCreateModal,
    openEditModal,
    selectPackageFile,
    uploadSelectedFile,
    buildPayload,
    savePackage
  }
}
