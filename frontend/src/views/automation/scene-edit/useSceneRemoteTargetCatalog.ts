/**
 * 文件用途：场景动作「激活场景(20) / 触发告警(30)」分支的远程目录（场景列表、告警配置列表）。
 * 核心逻辑：按名称远程搜索；ensure* 仅在列表为空时首次加载，避免每次展开下拉都请求。
 * 关键注意事项：两类目录共用一个 loading；请求序号保证慢的旧搜索不会覆盖新结果。
 */
import { ref } from 'vue'
import { warningMessageList } from '@/service/api/alarm'
import { sceneGet } from '@/service/api/automation'
import type { SelectOption } from './useSceneActionTargetCatalog'

const PAGE_SIZE = 10

type ListFetcher = (query: { page: number; page_size: number; name: string }) => Promise<{
  data?: { list?: SelectOption[] } | null
}>

export function useSceneRemoteTargetCatalog() {
  const remoteSelectLoading = ref(false)
  let pending = 0

  const createCatalog = (fetcher: ListFetcher) => {
    const list = ref<SelectOption[]>([])
    let sequence = 0
    const search = async (name: string) => {
      const seq = ++sequence
      pending += 1
      remoteSelectLoading.value = true
      try {
        const res = await fetcher({ page: 1, page_size: PAGE_SIZE, name: name || '' })
        if (seq === sequence) list.value = res.data?.list || []
      } finally {
        pending -= 1
        remoteSelectLoading.value = pending > 0
      }
    }
    const ensureLoaded = () => {
      if (list.value.length > 0) return
      void search('')
    }
    return { list, search, ensureLoaded }
  }

  const scenes = createCatalog(sceneGet as unknown as ListFetcher)
  const alarms = createCatalog(warningMessageList as unknown as ListFetcher)

  return {
    remoteSelectLoading,
    sceneList: scenes.list,
    getSceneList: scenes.search,
    ensureSceneListLoaded: scenes.ensureLoaded,
    alarmList: alarms.list,
    getAlarmList: alarms.search,
    ensureAlarmListLoaded: alarms.ensureLoaded
  }
}
