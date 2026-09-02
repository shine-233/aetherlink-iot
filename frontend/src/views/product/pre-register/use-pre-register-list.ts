/**
 * 文件用途: 预注册设备列表查询组合函数——筛选、分页与远程数据加载。
 * 核心逻辑: 维护查询参数与分页状态，调用 getPreProductList 拉取租户内预注册清单。
 * 关键注意事项: 后端契约 page/page_size 必填，product/batch/activate 为可选过滤。
 */
import { computed, reactive, ref } from 'vue'
import { getPreProductList } from '@/service/product/list'

export function usePreRegisterList() {
  const loading = ref(false)
  const tableData = ref<any[]>([])
  const queryParams = reactive({
    product_id: '',
    batch_number: '',
    activate_flag: null as string | null
  })

  async function fetchList(page = 1, pageSize = 10) {
    loading.value = true
    try {
      const params: Record<string, any> = { page, page_size: pageSize }
      if (queryParams.product_id) params.product_id = queryParams.product_id
      if (queryParams.batch_number.trim()) params.batch_number = queryParams.batch_number.trim()
      if (queryParams.activate_flag) params.activate_flag = queryParams.activate_flag
      const { data, error } = await getPreProductList(params)
      if (error) return
      tableData.value = data?.list ?? []
      pagination.itemCount = Number(data?.total ?? 0)
      pagination.page = page
      pagination.pageSize = pageSize
    } finally {
      loading.value = false
    }
  }

  const pagination = reactive({
    page: 1,
    pageSize: 10,
    itemCount: 0,
    showSizePicker: true,
    pageSizes: [10, 20, 50],
    onChange: (page: number) => {
      fetchList(page, pagination.pageSize)
    },
    onUpdatePageSize: (pageSize: number) => {
      fetchList(1, pageSize)
    }
  })

  function resetQuery() {
    queryParams.product_id = ''
    queryParams.batch_number = ''
    queryParams.activate_flag = null
    fetchList()
  }

  const hasActiveFilters = computed(() =>
    Boolean(queryParams.product_id || queryParams.batch_number.trim() || queryParams.activate_flag)
  )

  return {
    loading,
    tableData,
    queryParams,
    pagination,
    hasActiveFilters,
    fetchList,
    resetQuery
  }
}
