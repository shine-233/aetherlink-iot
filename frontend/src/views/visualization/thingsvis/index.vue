<!--
文件用途: 承载ThingsVis 总览相关的可视化页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { onMounted } from 'vue'
import { useMessage } from 'naive-ui'
import { NATIVE_BOARD_PROJECT_ID } from '@/service/visualization-provider/index'
import { $t } from '@/locales'
import { useThingsVisProjectList } from './useThingsVisProjectList'
import { useThingsVisProjectForm } from './useThingsVisProjectForm'
import { useThingsVisProjectDelete } from './useThingsVisProjectDelete'

const message = useMessage()

const projectList = useThingsVisProjectList()
const {
  provider,
  providerError,
  projectCapabilities,
  providerBlockedMessage,
  isNativeProvider,
  isFirstDeviceOnboarding,
  onboardingDashboardQuery,
  loading,
  allProjects,
  searchKeyword,
  projects,
  fetchProjects,
  enterProject
} = projectList

const {
  showModal,
  editingProject,
  formData,
  openCreateModal,
  openFirstDeviceProjectCreateModal,
  openEditModal,
  handleSaveProject
} = useThingsVisProjectForm(projectList)

const { deletingId, deleteConfirmModal, pendingDeleteProject, openDeleteConfirm, handleDeleteProject } =
  useThingsVisProjectDelete(projectList)

const notifyLoadFailed = () => {
  message.error($t('rdi.thingsvis.loadProjectsFailed'))
}

onMounted(() => {
  fetchProjects(notifyLoadFailed)
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
          <NButton v-if="projectCapabilities.create && !providerError" type="primary" @click="openCreateModal">
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
        <NButton
          v-if="projects.length || projectCapabilities.create"
          type="primary"
          data-testid="first-device-project-action"
          @click="projects.length ? enterProject(projects[0].id) : openFirstDeviceProjectCreateModal()"
        >
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
          <template v-if="projectCapabilities.create" #extra>
            <NButton
              type="primary"
              @click="isFirstDeviceOnboarding ? openFirstDeviceProjectCreateModal() : openCreateModal()"
            >
              <template #icon>
                <icon-mdi:plus />
              </template>
              {{
                isFirstDeviceOnboarding ? $t('rdi.thingsvis.firstDeviceProjectCreate') : $t('rdi.thingsvis.newProject')
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

                  <!-- Hover actions：内置项目不可改名/删除（provider 层 fail closed），不渲染入口 -->
                  <div
                    v-if="
                      (projectCapabilities.update || projectCapabilities.delete) &&
                      project.id !== NATIVE_BOARD_PROJECT_ID
                    "
                    class="flex gap-1 opacity-0 transition-opacity group-hover:opacity-100"
                  >
                    <NButton
                      v-if="projectCapabilities.update"
                      size="small"
                      quaternary
                      circle
                      @click.stop="openEditModal(project)"
                    >
                      <template #icon>
                        <icon-mdi:pencil class="text-16px" />
                      </template>
                    </NButton>

                    <NButton
                      v-if="projectCapabilities.delete"
                      size="small"
                      quaternary
                      circle
                      @click.stop="openDeleteConfirm(project.id, project.name)"
                    >
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
      v-if="projectCapabilities.create || projectCapabilities.update"
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
      v-if="projectCapabilities.delete"
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
