<!--
文件用途: 承载ThingsVis 总览相关的可视化页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { NAlert, NButton, NCard, NGrid, NGridItem, NInput, NModal, NForm, NFormItem, NEmpty, NSpin, useMessage } from 'naive-ui'
import { useRouterPush } from '@/hooks/common/router'
import {
  getDefaultVisualizationProviderFacade,
  NATIVE_BOARD_PROVIDER_ID,
  type VisualizationProject
} from '@/service/visualization-provider/index'
import { resolveVisualizationProviderId } from '@/service/visualization-provider/composition'
import { deleteDashboardMenuConfig } from '@/service/api/dashboard-menu'
import { refreshAuthRoutes } from '@/utils/router/refresh-auth-routes'
import { clearThingsVisHomeCache } from '@/utils/thingsvis/home-cache'
import { $t } from '@/locales'

const { routerPushByKey } = useRouterPush()
const message = useMessage()
const route = useRoute()
const providerId = computed(() => resolveVisualizationProviderId({
  provider: route.query.provider,
  projectId: route.query.projectId
}))
const provider = getDefaultVisualizationProviderFacade({ providerId: providerId.value })
const providerError = computed(() => provider.selectionError)
const providerBlockedMessage = computed(() => {
  if (providerError.value?.code === 'external-blocked') {
    return $t('rdi.thingsvis.externalProviderDisabledDescription')
  }

  return providerError.value?.message || $t('rdi.thingsvis.loadProjectsFailed')
})
const isNativeProvider = computed(() => providerId.value === NATIVE_BOARD_PROVIDER_ID)
const isFirstDeviceOnboarding = computed(() => route.query.onboarding === 'first-device')
const onboardingDashboardQuery = computed((): Record<string, string> =>
  isFirstDeviceOnboarding.value
    ? { onboarding: 'first-device', ...(isNativeProvider.value ? { provider: 'native' } : {}) }
    : (isNativeProvider.value ? { provider: 'native' } : {})
)

// State
const loading = ref(false)
const deletingId = ref<string | null>(null)
const allProjects = ref<VisualizationProject[]>([])
const showModal = ref(false)
const editingProject = ref<VisualizationProject | null>(null)
const searchKeyword = ref('')
const deleteConfirmModal = ref(false)
const pendingDeleteProject = ref<{ id: string; name: string } | null>(null)
const projects = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase()
  if (!keyword) return allProjects.value

  return allProjects.value.filter(item => item.name.toLowerCase().includes(keyword))
})

// Form state
const formData = ref({
  name: '',
  description: ''
})

/** Fetch project list */
const fetchProjects = async () => {
  loading.value = true
  try {
    if (providerError.value) return

    const result = await provider.execute(current => current.listProjects({ page: 1, limit: 100 }))
    if (result.ok) {
      allProjects.value = result.data.items
    } else {
      message.error($t('rdi.thingsvis.loadProjectsFailed'))
    }
  } finally {
    loading.value = false
  }
}

/** Open create modal */
const openCreateModal = () => {
  editingProject.value = null
  formData.value = { name: '', description: '' }
  showModal.value = true
}

const openFirstDeviceProjectCreateModal = () => {
  editingProject.value = null
  formData.value = {
    name: $t('rdi.thingsvis.firstDeviceProjectName'),
    description: ''
  }
  showModal.value = true
}

/** Open edit modal */
const openEditModal = (project: VisualizationProject) => {
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
      const result = await provider.execute(current => current.updateProject(editingProject.value!.id, {
        name: formData.value.name,
        description: formData.value.description || undefined
      }))
      if (result.ok) {
        message.success($t('rdi.thingsvis.updateProjectSuccess'))
        showModal.value = false
        await fetchProjects()
      } else {
        message.error($t('rdi.thingsvis.updateProjectFailed'))
      }
    } else {
      const result = await provider.execute(current => current.createProject({
        name: formData.value.name,
        description: formData.value.description || undefined
      }))
      if (result.ok) {
        message.success($t('rdi.thingsvis.createSuccess'))
        showModal.value = false
        formData.value = { name: '', description: '' }
        await fetchProjects()
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

/** Delete project */
const openDeleteConfirm = (id: string, name: string) => {
  const project = allProjects.value.find(item => item.id === id)
  if ((project?.dashboardCount || 0) > 0) {
    message.warning($t('rdi.thingsvis.projectHasDashboardsWarning'))
    return
  }

  pendingDeleteProject.value = { id, name }
  deleteConfirmModal.value = true
}

const handleDeleteProject = async () => {
  if (!pendingDeleteProject.value || deletingId.value) return
  deletingId.value = pendingDeleteProject.value.id
  try {
    const { id } = pendingDeleteProject.value
    const dashboardsResult = await provider.execute(current => current.listDashboards({
      projectId: id,
      page: 1,
      limit: 1000
    }))
    if (!dashboardsResult.ok) {
      message.error($t('rdi.thingsvis.deleteFailed'))
      return
    }
    const dashboardIds = dashboardsResult.data.items.map(dashboard => dashboard.id)

    for (const did of dashboardIds) {
      const { error } = await deleteDashboardMenuConfig(did)
      if (error) {
        message.error($t('rdi.thingsvis.deleteMenuCleanupFailed'))
        return
      }
    }

    const deleteResult = await provider.execute(current => current.deleteProject(id))
    if (deleteResult.ok) {
      const deletedName = pendingDeleteProject.value?.name || ''
      deleteConfirmModal.value = false
      pendingDeleteProject.value = null
      await refreshAuthRoutes(route.fullPath)
      clearThingsVisHomeCache()
      message.success($t('rdi.thingsvis.projectDeleted', { name: deletedName }))
      await fetchProjects()
    } else {
      console.warn(`[handleDeleteProject] Failed to delete project ${id}`)
      message.error($t('rdi.thingsvis.deleteFailed'))
    }
  } catch (e) {
    message.error($t('rdi.thingsvis.deleteFailed'))
    console.error(e)
  } finally {
    deletingId.value = null
  }
}

/** Enter project dashboard list */
const enterProject = (projectId: string) => {
  routerPushByKey('visualization_thingsvis-dashboards', {
    query: {
      projectId,
      ...(isNativeProvider.value ? { provider: 'native' } : {}),
      ...onboardingDashboardQuery.value
    }
  })
}

onMounted(() => {
  fetchProjects()
})
</script>

<template>
  <div class="h-full">
    <NCard>
      <NAlert
        v-if="providerError"
        type="warning"
        class="mb-4"
        data-testid="thingsvis-provider-blocked"
        :data-provider-error="providerError.code"
      >
        <template #header>{{ $t('rdi.thingsvis.externalProviderDisabledTitle') }}</template>
        {{ providerBlockedMessage }}
      </NAlert>

      <!-- Header toolbar -->
      <div class="mb-5 flex items-center justify-between gap-4">
        <div class="flex items-center gap-3">
          <h2 class="text-xl font-bold">{{ $t('rdi.thingsvis.visualProjects') }}</h2>
          <span class="text-gray-400">{{ $t('rdi.thingsvis.projectCount', { count: projects.length }) }}</span>
        </div>

        <div class="flex items-center gap-3">
          <!-- Search box -->
          <NInput
            v-model:value="searchKeyword"
            clearable
            :placeholder="$t('rdi.thingsvis.searchProjectPlaceholder')"
            style="width: 240px"
          >
            <template #prefix>
              <icon-mdi:magnify />
            </template>
          </NInput>

          <!-- Create button -->
          <NButton v-if="!isNativeProvider && !providerError" type="primary" @click="openCreateModal">
            <template #icon>
              <icon-mdi:plus />
            </template>
            {{ $t('rdi.thingsvis.newProject') }}
          </NButton>
        </div>
      </div>

      <div v-if="isFirstDeviceOnboarding && !providerError" class="thingsvis-onboarding-banner">
        <div class="min-w-0">
          <div class="thingsvis-onboarding-banner__eyebrow">
            {{ $t('rdi.thingsvis.firstDeviceDashboardStep') }}
          </div>
          <div class="thingsvis-onboarding-banner__title">
            {{ $t('rdi.thingsvis.firstDeviceDashboardTitle') }}
          </div>
          <div class="thingsvis-onboarding-banner__desc">
            {{ $t('rdi.thingsvis.firstDeviceDashboardDesc') }}
          </div>
        </div>
        <NButton type="primary" @click="projects.length ? enterProject(projects[0].id) : openFirstDeviceProjectCreateModal()">
          <template #icon>
            <icon-mdi:home-plus-outline />
          </template>
          {{
            projects.length
              ? $t('rdi.thingsvis.firstDeviceDashboardContinue')
              : $t('rdi.thingsvis.firstDeviceProjectCreate')
          }}
        </NButton>
      </div>

      <!-- Loading state -->
      <NSpin :show="loading">
        <!-- Empty state -->
        <NEmpty
          v-if="!loading && !providerError && projects.length === 0"
          :description="
            isFirstDeviceOnboarding ? $t('rdi.thingsvis.firstDeviceEmptyProject') : $t('rdi.thingsvis.emptyProject')
          "
          class="py-20"
        >
          <template #icon>
            <icon-mdi:folder-open-outline class="text-50px text-gray-300" />
          </template>
          <template #extra>
            <NButton type="primary" @click="isFirstDeviceOnboarding ? openFirstDeviceProjectCreateModal() : openCreateModal()">
              <template #icon>
                <icon-mdi:plus />
              </template>
              {{
                isFirstDeviceOnboarding
                  ? $t('rdi.thingsvis.firstDeviceProjectCreate')
                  : $t('rdi.thingsvis.newProject')
              }}
            </NButton>
          </template>
        </NEmpty>

        <!-- Project grid -->
        <NGrid v-else-if="!providerError" x-gap="24" y-gap="24" cols="1 s:2 m:3 l:4" responsive="screen">
          <NGridItem v-for="project in projects" :key="project.id">
            <!-- Project card -->
            <div
              class="group relative cursor-pointer overflow-hidden rounded-lg border border-gray-200 bg-white transition-all hover:border-primary hover:shadow-lg"
              @click="enterProject(project.id)"
            >
              <!-- Card content -->
              <div class="p-5">
                <!-- Header: icon and actions -->
                <div class="mb-3 flex items-start justify-between">
                  <div class="flex h-12 w-12 items-center justify-center rounded-lg bg-primary/10">
                    <icon-mdi:folder class="text-24px text-primary" />
                  </div>

                  <!-- Hover actions -->
                  <div v-if="!isNativeProvider" class="flex gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                    <NButton size="small" quaternary circle @click.stop="openEditModal(project)">
                      <template #icon>
                        <icon-mdi:pencil class="text-16px" />
                      </template>
                    </NButton>

                    <NButton size="small" quaternary circle @click.stop="openDeleteConfirm(project.id, project.name)">
                      <template #icon>
                        <icon-mdi:delete class="text-16px" />
                      </template>
                    </NButton>
                  </div>
                </div>

                <!-- Project name -->
                <h3 class="mb-2 truncate text-lg font-semibold">
                  {{ project.name }}
                </h3>

                <!-- Project description -->
                <p class="mb-4 line-clamp-2 h-10 text-sm text-gray-500">
                  {{ project.description || $t('rdi.thingsvis.noDescription') }}
                </p>

                <!-- Footer info -->
                <div class="flex items-center justify-between text-xs text-gray-400">
                  <div class="flex items-center gap-1">
                    <icon-mdi:chart-box-outline />
                    <span>{{ $t('rdi.thingsvis.dashboardCount', { count: project.dashboardCount || 0 }) }}</span>
                  </div>
                  <div class="flex items-center gap-1">
                    <icon-mdi:clock-outline />
                    <span>{{ new Date(project.updatedAt).toLocaleDateString() }}</span>
                  </div>
                </div>
              </div>
            </div>
          </NGridItem>
        </NGrid>
      </NSpin>
    </NCard>

    <!-- Create/edit modal -->
    <NModal
      v-model:show="showModal"
      preset="card"
      :title="editingProject ? $t('rdi.thingsvis.editProject') : $t('rdi.thingsvis.newProject')"
      class="w-500px"
    >
      <NForm :model="formData">
        <NFormItem :label="$t('rdi.thingsvis.projectName')" path="name">
          <NInput
            v-model:value="formData.name"
            :placeholder="$t('rdi.thingsvis.projectNamePlaceholder')"
            maxlength="50"
            show-count
          />
        </NFormItem>

        <NFormItem :label="$t('rdi.thingsvis.projectDescription')">
          <NInput
            v-model:value="formData.description"
            type="textarea"
            :placeholder="$t('rdi.thingsvis.projectDescriptionPlaceholder')"
            :rows="4"
            maxlength="200"
            show-count
          />
        </NFormItem>
      </NForm>

      <template #footer>
        <div class="flex justify-end gap-2">
          <NButton @click="showModal = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" @click="handleSaveProject">
            {{ editingProject ? $t('common.update') : $t('rdi.thingsvis.create') }}
          </NButton>
        </div>
      </template>
    </NModal>

    <!-- Delete project confirm modal -->
    <NModal
      v-model:show="deleteConfirmModal"
      preset="dialog"
      type="warning"
      :title="$t('rdi.thingsvis.confirmDelete')"
      :action-style="{ gap: '8px' }"
    >
      <template #icon>
        <icon-mdi:alert-circle class="text-24px text-orange-400" />
      </template>
      <template #default>
        {{ $t('rdi.thingsvis.deleteProjectConfirm', { name: pendingDeleteProject?.name || '' }) }}
      </template>
      <template #action>
        <NButton :disabled="!!deletingId" @click="deleteConfirmModal = false">{{ $t('common.cancel') }}</NButton>
        <NButton type="error" :loading="!!deletingId" @click="handleDeleteProject">
          {{ $t('rdi.thingsvis.confirmDeleteAction') }}
        </NButton>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.thingsvis-onboarding-banner {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 18px;
  padding: 14px 16px;
  border: 1px solid #bfdbfe;
  border-radius: 8px;
  background: #eff6ff;
}

.thingsvis-onboarding-banner__eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 600;
}

.thingsvis-onboarding-banner__title {
  margin-top: 4px;
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.thingsvis-onboarding-banner__desc {
  margin-top: 4px;
  color: #475569;
  font-size: 13px;
  line-height: 1.5;
}

@media (max-width: 640px) {
  .thingsvis-onboarding-banner {
    flex-direction: column;
  }
}
</style>
