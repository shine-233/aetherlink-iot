/**
 * 文件用途: 预注册导入向导组合函数——自动生成/CSV 文件双模式建档与结果回显。
 * 核心逻辑: 校验模式必填项 → (CSV 模式)先经 /file/up 上传取得服务端路径 → 调 addDevice 建档 → 保存结果报告。
 * 关键注意事项: voucher 仅在创建响应中出现一次，页面展示后不落任何持久状态。
 */
import { computed, reactive, ref } from 'vue'
import type { FormInst, FormRules } from 'naive-ui'
import { addDevice, getProductList, uploadImportBatchFile } from '@/service/product/list'
import { $t } from '@/locales'
import type { PreRegisterImportResult } from './types'

export type PreRegisterImportMode = 'auto' | 'file'

export function usePreRegisterImport(options: { onImported: () => void | Promise<void> }) {
  const modalVisible = ref(false)
  const submitting = ref(false)
  const mode = ref<PreRegisterImportMode>('auto')

  const form = reactive({
    product_id: '',
    batch_number: '',
    current_version: '',
    device_count: 1
  })

  const formRef = ref<FormInst & HTMLElement>()
  // 提交前内联校验：必填项就地报错，替代此前"提交后才被后端拒绝"的体验。
  const rules: FormRules = {
    product_id: [{ required: true, message: $t('common.pleaseCheckValue'), trigger: ['blur', 'change'] }],
    batch_number: [
      { required: true, message: $t('common.pleaseCheckValue'), trigger: ['input', 'blur'] },
      { max: 36, message: $t('common.pleaseCheckValue'), trigger: ['input', 'blur'] }
    ],
    device_count: [{ required: true, type: 'number', min: 1, max: 10000, message: $t('common.pleaseCheckValue'), trigger: ['input', 'blur'] }]
  }

  const productOptions = ref<{ label: string; value: string }[]>([])
  const productLoading = ref(false)

  const selectedFile = ref<File | null>(null)
  const uploadedPath = ref('')
  const uploading = ref(false)

  const importResult = ref<PreRegisterImportResult | null>(null)
  const submitError = ref('')

  async function fetchProductOptions(keyword = '') {
    productLoading.value = true
    try {
      const { data, error } = await getProductList({ page: 1, page_size: 100, name: keyword || undefined })
      if (error) return
      const list: any[] = data?.list ?? []
      productOptions.value = list.map((item) => ({ label: item.name, value: item.id }))
    } finally {
      productLoading.value = false
    }
  }

  function openModal() {
    mode.value = 'auto'
    form.product_id = ''
    form.batch_number = ''
    form.current_version = ''
    form.device_count = 1
    selectedFile.value = null
    uploadedPath.value = ''
    importResult.value = null
    submitError.value = ''
    modalVisible.value = true
    fetchProductOptions()
  }

  function selectFile(file: File | null) {
    selectedFile.value = file
    uploadedPath.value = ''
    importResult.value = null
  }

  async function uploadSelectedFile() {
    if (!selectedFile.value) return false
    uploading.value = true
    try {
      const formData = new FormData()
      formData.append('file', selectedFile.value)
      formData.append('type', 'importBatch')
      const { data, error } = await uploadImportBatchFile(formData)
      uploadedPath.value = data?.path ?? ''
      if (error || !uploadedPath.value) {
        window.$message?.error(error ? String(error) : 'upload failed')
        return false
      }
      return true
    } finally {
      uploading.value = false
    }
  }

  const canSubmit = computed(() => {
    if (!form.product_id || !form.batch_number.trim()) return false
    if (mode.value === 'auto') return form.device_count >= 1 && form.device_count <= 10000
    // 已上传或已选择文件均可提交；未上传时由 submitImport 内部先走上传。
    return Boolean(uploadedPath.value || selectedFile.value)
  })

  async function submitImport() {
    if (!canSubmit.value || submitting.value) return false
    try {
      // validate?.() 兼容测试桩：真实 NaiveUI FormInst 必有 validate。
      await formRef.value?.validate?.()
    } catch {
      return false
    }
    submitting.value = true
    submitError.value = ''
    try {
      if (mode.value === 'file' && !uploadedPath.value) {
        const uploaded = await uploadSelectedFile()
        if (!uploaded) return false
      }
      const payload: Record<string, any> = {
        product_id: form.product_id,
        batch_number: form.batch_number.trim(),
        create_type: mode.value === 'auto' ? '1' : '2'
      }
      if (form.current_version.trim()) payload.current_version = form.current_version.trim()
      if (mode.value === 'auto') payload.device_count = form.device_count
      else payload.batch_file = uploadedPath.value

      const { data, error } = await addDevice(payload)
      if (error) {
        submitError.value = typeof error === 'string' ? error : 'import failed'
        return false
      }
      importResult.value = {
        created_count: Number(data?.created_count ?? 0),
        devices: data?.devices ?? [],
        skipped_existing: data?.skipped_existing ?? [],
        skipped_duplicate_rows: data?.skipped_duplicate_rows ?? []
      }
      await options.onImported()
      return true
    } finally {
      submitting.value = false
    }
  }

  return {
    modalVisible,
    submitting,
    uploading,
    uploadSelectedFile,
    mode,
    form,
    formRef,
    rules,
    productOptions,
    productLoading,
    fetchProductOptions,
    selectedFile,
    uploadedPath,
    importResult,
    submitError,
    canSubmit,
    openModal,
    selectFile,
    submitImport
  }
}
