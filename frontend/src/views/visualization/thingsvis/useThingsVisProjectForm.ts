// 文件用途：ThingsVis 项目的新建/编辑弹窗状态与提交逻辑。
// 核心逻辑：按 provider 能力门禁决定是否允许打开弹窗，提交时区分新建与更新两条分支，
// 成功后刷新列表；首设备引导流程下创建成功会直接跳进新项目。
// 关键注意事项：能力门禁（create/update）必须先于弹窗打开，否则用户会看到无法提交的表单；
// 提示文案与失败分支要保持不变。
import { ref } from 'vue'
import { useMessage } from 'naive-ui'
import type { VisualizationProject } from '@/service/visualization-provider/index'
import { $t } from '@/locales'
import type { ThingsVisProjectList } from './useThingsVisProjectList'

export function useThingsVisProjectForm(list: ThingsVisProjectList) {
  const message = useMessage()
  const { projectCapabilities, provider, projects, isFirstDeviceOnboarding, enterProject } = list

  const showModal = ref(false)
  const editingProject = ref<VisualizationProject | null>(null)
  const formData = ref({
    name: '',
    description: ''
  })

  const notifyLoadFailed = () => {
    message.error($t('rdi.thingsvis.loadProjectsFailed'))
  }

  /** Open create modal */
  const openCreateModal = () => {
    if (!projectCapabilities.value.create) {
      message.warning($t('rdi.thingsvis.projectCreationUnsupported'))
      return
    }
    editingProject.value = null
    formData.value = { name: '', description: '' }
    showModal.value = true
  }

  const openFirstDeviceProjectCreateModal = () => {
    if (!projectCapabilities.value.create && projects.value.length > 0) {
      enterProject(projects.value[0].id)
      return
    }
    if (!projectCapabilities.value.create) {
      message.warning($t('rdi.thingsvis.projectCreationUnsupported'))
      return
    }
    editingProject.value = null
    formData.value = {
      name: $t('rdi.thingsvis.firstDeviceProjectName'),
      description: ''
    }
    showModal.value = true
  }

  /** Open edit modal */
  const openEditModal = (project: VisualizationProject) => {
    if (!projectCapabilities.value.update) {
      message.warning($t('rdi.thingsvis.projectUpdateUnsupported'))
      return
    }
    editingProject.value = project
    formData.value = {
      name: project.name,
      description: project.description || ''
    }
    showModal.value = true
  }

  /** Save project */
  const handleSaveProject = async () => {
    if (!formData.value.name.trim()) {
      message.error($t('rdi.thingsvis.projectNamePlaceholder'))
      return
    }

    try {
      if (editingProject.value) {
        const result = await provider.execute((current) =>
          current.updateProject(editingProject.value!.id, {
            name: formData.value.name,
            description: formData.value.description || undefined
          })
        )
        if (result.ok) {
          message.success($t('rdi.thingsvis.updateProjectSuccess'))
          showModal.value = false
          await list.fetchProjects(notifyLoadFailed)
        } else {
          message.error($t('rdi.thingsvis.updateProjectFailed'))
        }
      } else {
        const result = await provider.execute((current) =>
          current.createProject({
            name: formData.value.name,
            description: formData.value.description || undefined
          })
        )
        if (result.ok) {
          message.success($t('rdi.thingsvis.createSuccess'))
          showModal.value = false
          formData.value = { name: '', description: '' }
          await list.fetchProjects(notifyLoadFailed)
          if (isFirstDeviceOnboarding.value && result.data.id) {
            enterProject(result.data.id)
          }
        } else {
          message.error($t('rdi.thingsvis.createFailed'))
        }
      }
    } catch (e) {
      message.error($t('common.operationFailed'))
      console.error(e)
    }
  }

  return {
    showModal,
    editingProject,
    formData,
    openCreateModal,
    openFirstDeviceProjectCreateModal,
    openEditModal,
    handleSaveProject
  }
}
