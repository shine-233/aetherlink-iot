/**
 * 文件用途：沉淀 告警通知组管理 的局部工具方法与共享状态。
 * 核心逻辑：集中处理页面内选项转换、接口辅助逻辑或告警字段映射，供同目录组件复用。
 * 关键注意事项：仅服务当前视图上下文，若被跨页面引用需先评估是否迁移到公共工具层。
 * 重构建议：后续可补充纯函数测试并减少可变的模块级状态，提升并发渲染安全性。
 */
import { reactive, ref } from 'vue'
import { debounce } from 'lodash-es'
import { getUserList } from '@/service/api/notification'

const loading = ref(false)
const memberOptionsHasMore = ref(true)
let memberSearchRequestSeq = 0

const pagination = reactive({
  page: 1,
  pageSize: 10,
  name: ''
})

export const initMemberData = { name: '', notificationType: [] }
export const memberTypeData = ref<any>([initMemberData])

export const handleDeleteMember = (index: number) => {
  memberTypeData.value.splice(index, 1)
}

export const handleUpdateMember = (updateIndex: number, data: { name: string; notificationType: string[] }) => {
  const filterData = memberTypeData.value.map((item, index) => {
    if (index === updateIndex) {
      return {
        name: data.name,
        notificationType: data.notificationType
      }
    }
    return item
  })
  memberTypeData.value = [...filterData]
}

export const notificationTypeOptions = ref<{ label: string; value: string }[]>([])
export const memberOptionsLoading = loading

const loadMemberOptions = async (query = '', append = false) => {
  const normalizedQuery = query.trim()

  if (append && (!memberOptionsHasMore.value || loading.value)) return

  const requestSeq = ++memberSearchRequestSeq

  loading.value = true
  if (!append) {
    pagination.name = normalizedQuery
    pagination.page = 1
    memberOptionsHasMore.value = true
    notificationTypeOptions.value = []
  } else if (normalizedQuery !== pagination.name) {
    pagination.name = normalizedQuery
    pagination.page = 1
    memberOptionsHasMore.value = true
    notificationTypeOptions.value = []
  }

  try {
    const res = await getUserList({
      page: pagination.page,
      page_size: pagination.pageSize,
      name: pagination.name
    })
    if (requestSeq !== memberSearchRequestSeq) return

    if (res?.data) {
      const userList = res.data?.list || []
      const total = Number(res.data?.total)
      const formatList = userList.map(item => {
        return {
          label: item.name,
          value: item.user_id
        }
      })
      notificationTypeOptions.value = [...notificationTypeOptions.value, ...formatList]
      memberOptionsHasMore.value = Number.isFinite(total)
        ? notificationTypeOptions.value.length < total
        : userList.length >= pagination.pageSize
    } else {
      memberOptionsHasMore.value = false
    }
  } finally {
    if (requestSeq === memberSearchRequestSeq) {
      loading.value = false
    }
  }
}

const debouncedMemberSearch = debounce((query?: string) => {
  void loadMemberOptions(query || '')
}, 300)

export const handleSearch = (query?: string) => {
  debouncedMemberSearch(query)
}

export const handleScroll = e => {
  const currentTarget = e.currentTarget as HTMLElement
  if (
    currentTarget.scrollTop + currentTarget.offsetHeight >= currentTarget.scrollHeight &&
    !loading.value &&
    memberOptionsHasMore.value
  ) {
    pagination.page += 1
    debouncedMemberSearch.cancel()
    void loadMemberOptions(pagination.name, true)
  }
}

export const getCurrentName = (index: number) => {
  return memberTypeData.value[index].name
}
