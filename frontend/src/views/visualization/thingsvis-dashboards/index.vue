<!--
文件用途: 承载ThingsVis 仪表盘相关的可视化页面或业务组件。
核心逻辑: 组织页面状态、接口调用、表单/列表交互和子组件协作，向用户呈现可操作的业务流程。
关键注意事项: 修改时要同步核对路由参数、接口载荷、权限状态和用户可见提示，避免只改前端状态。
重构建议: 可逐步把查询、提交和弹窗状态拆成组合函数，让组件更专注于布局与事件编排。
-->
<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import {
  NButton,
  NAlert,
  NCard,
  NGrid,
  NGridItem,
  NBreadcrumb,
  NBreadcrumbItem,
  NInput,
  NModal,
  NForm,
  NFormItem,
  NEmpty,
  NSpin,
  NSwitch,
  NInputNumber,
  NSelect,
  useMessage
} from 'naive-ui'
import { useRouterPush } from '@/hooks/common/router'
import { $t } from '@/locales'
import { useAuthStore } from '@/store/modules/auth'
import { resolveVisualizationProviderId } from '@/service/visualization-provider/composition'
import { NATIVE_BOARD_PROJECT_ID, NATIVE_BOARD_PROVIDER_ID } from '@/service/visualization-provider/provider-ids'
import { isSysAdminUser } from '@/utils/thingsvis/space'
import ThingsVisDashboardCard from './ThingsVisDashboardCard.vue'
import { useThingsVisDashboardActions } from './useThingsVisDashboardActions'
import { useThingsVisDashboardList } from './useThingsVisDashboardList'
import { useThingsVisDashboardMenuConfig } from './useThingsVisDashboardMenuConfig'

const route = useRoute()
const { routerPushByKey } = useRouterPush()
const message = useMessage()
const authStore = useAuthStore()

// 从路由获取项目ID
const providerId = computed(() => resolveVisualizationProviderId({
  provider: route.query.provider,
  projectId: String(route.query.projectId || '').trim()
}))
const projectId = computed(() => {
  const routeProjectId = String(route.query.projectId || '').trim()
  return routeProjectId || (providerId.value === NATIVE_BOARD_PROVIDER_ID ? NATIVE_BOARD_PROJECT_ID : '')
})
const isNativeProvider = computed(() => providerId.value === NATIVE_BOARD_PROVIDER_ID)
/**
 * The dashboard-menu API is tenant-scoped. Native SYS_ADMIN fallback pages
 * have no tenant context, so they must not probe that API or offer a write
 * path that is guaranteed to be rejected. Tenant users keep the existing
 * menu configuration flow, including when they explicitly select Native.
 */
const dashboardMenuConfigAvailable = computed(
  () => !(isNativeProvider.value && isSysAdminUser(authStore.userInfo))
)
const isFirstDeviceOnboarding = computed(() => route.query.onboarding === 'first-device')

const {
  menuConfigs,
  showMenuModal,
  menuSaving,
  menuForm,
  registerMenuConfigDashboards,
  requestMenuConfig,
  openMenuConfig,
  handleSaveMenuConfig
} = useThingsVisDashboardMenuConfig({
  t: $t,
  message,
  getRouteFullPath: () => route.fullPath,
  providerId,
  dashboardMenuConfigAvailable
})

const {
  loading,
  providerError,
  project,
  allDashboards,
  dashboards,
  searchKeyword,
  loadProject,
  fetchDashboards,
  requestThumbnail,
  getThumbnailUrl
} =
  useThingsVisDashboardList({
    projectId,
    providerId,
    message,
    t: $t,
    registerMenuConfigDashboards
  })

const {
  deletingId,
  publishingId,
  duplicatingId,
  creatingHomepageDashboard,
  showModal,
  deleteConfirmModal,
  pendingDeleteDashboard,
  formData,
  dashboardTemplateOptions,
  openCreateModal,
  handleCreateDashboard,
  handleCreateHomepageDashboard,
  openDeleteConfirm,
  handleDeleteDashboard,
  handleSetAsHomepage,
  handlePublishDashboard,
  handleDuplicateDashboard,
  copyDashboardViewerLink
} = useThingsVisDashboardActions({
  projectId,
  providerId,
  message,
  t: $t,
  fetchDashboards
})
const hasHomepageDashboard = computed(() => allDashboards.value.some(dashboard => dashboard.home))
const providerBlockedMessage = computed(() => {
  if (providerError?.code === 'external-blocked') {
    return $t('rdi.thingsvis.externalProviderDisabledDescription')
  }

  return providerError?.message || $t('rdi.thingsvis.loadDashboardsFailed')
})
const showFirstDeviceHomepageCta = computed(
  () => isFirstDeviceOnboarding.value && allDashboards.value.length > 0 && !hasHomepageDashboard.value
)

/** 打开编辑器 */
const openEditor = (dashboardId: string) => {
  if (isNativeProvider.value) {
    routerPushByKey('visualization_native-board-editor', {
      query: { id: dashboardId }
    })
    return
  }

  routerPushByKey('visualization_thingsvis-editor', {
    query: {
      id: dashboardId,
      projectId: projectId.value,
      ...(isNativeProvider.value ? { provider: 'native' } : {})
    }
  })
}

/** 返回项目列表 */
const goBackToProjects = () => {
  routerPushByKey('visualization_thingsvis', {
    query: isNativeProvider.value ? { provider: 'native' } : undefined
  })
}

onMounted(async () => {
  if (!projectId.value) {
    await loadProject()
    return
  }

  await Promise.all([loadProject(), fetchDashboards()])
})
</script>

<template>
  <div class="h-full">
    <NCard>
      <!-- 面包屑导航 -->
      <NBreadcrumb class="mb-4">
        <NBreadcrumbItem class="cursor-pointer" @click="goBackToProjects">
          <div class="flex items-center gap-1">
            <icon-mdi:chevron-left />
            {{ $t('rdi.thingsvis.visualProjects') }}
          </div>
        </NBreadcrumbItem>
        <NBreadcrumbItem>{{ project?.name || $t('rdi.thingsvis.loading') }}</NBreadcrumbItem>
      </NBreadcrumb>

      <!-- 头部工具栏 -->
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

      <template v-else>
      <div class="mb-5 flex items-center justify-between gap-4">
        <div class="flex items-center gap-3">
          <h2 class="text-xl font-bold">{{ project?.name }}</h2>
          <span class="text-gray-400">
            {{
              $t('rdi.thingsvis.dashboardCount', {
                count: searchKeyword ? `${dashboards.length} / ${allDashboards.length}` : dashboards.length
              })
            }}
          </span>
        </div>

        <div class="flex items-center gap-3">
          <!-- 搜索框 -->
          <NInput
            v-model:value="searchKeyword"
            clearable
            :placeholder="$t('rdi.thingsvis.searchDashboardPlaceholder')"
            style="width: 240px"
          >
            <template #prefix>
              <icon-mdi:magnify />
            </template>
          </NInput>

          <!-- 新建按钮 -->
          <NButton type="primary" @click="openCreateModal">
            <template #icon>
              <icon-mdi:plus />
            </template>
            {{ $t('rdi.thingsvis.newDashboard') }}
          </NButton>
        </div>
      </div>

      <!-- 项目描述 -->
      <div v-if="project?.description" class="mb-4 text-sm text-gray-500">
        {{ project.description }}
      </div>

      <div v-if="showFirstDeviceHomepageCta" class="thingsvis-onboarding-dashboard">
        <div class="min-w-0">
          <div class="thingsvis-onboarding-dashboard__eyebrow">
            {{ $t('rdi.thingsvis.firstDeviceDashboardStep') }}
          </div>
          <div class="thingsvis-onboarding-dashboard__title">
            {{ $t('rdi.thingsvis.firstDeviceHomepageMissingTitle') }}
          </div>
          <div class="thingsvis-onboarding-dashboard__desc">
            {{ $t('rdi.thingsvis.firstDeviceHomepageMissingDesc') }}
          </div>
        </div>
        <NButton type="primary" :loading="creatingHomepageDashboard" @click="handleCreateHomepageDashboard">
          <template #icon>
            <icon-mdi:home-plus-outline />
          </template>
          {{ $t('rdi.thingsvis.createHomepageDashboard') }}
        </NButton>
      </div>

      <!-- 加载状态 -->
      <NSpin :show="loading">
        <!-- 空状态 -->
        <NEmpty
          v-if="!loading && dashboards.length === 0"
          :description="
            allDashboards.length === 0 ? $t('rdi.thingsvis.emptyDashboard') : $t('rdi.thingsvis.noMatchedDashboard')
          "
          class="py-20"
        >
          <template #icon>
            <icon-mdi:chart-box-outline class="text-50px text-gray-300" />
          </template>
          <template v-if="allDashboards.length === 0" #extra>
            <div class="thingsvis-empty-action">
              <div class="thingsvis-empty-action__hint">
                {{ $t('rdi.thingsvis.emptyDashboardHomepageHint') }}
              </div>
              <NButton type="primary" :loading="creatingHomepageDashboard" @click="handleCreateHomepageDashboard">
                <template #icon>
                  <icon-mdi:home-plus-outline />
                </template>
                {{ $t('rdi.thingsvis.createHomepageDashboard') }}
              </NButton>
            </div>
          </template>
        </NEmpty>

        <!-- Dashboard 网格 -->
        <NGrid
          v-else
          x-gap="24"
          y-gap="24"
          cols="1 s:2 m:3 l:4"
          responsive="screen"
          data-testid="thingsvis-dashboard-list"
        >
          <NGridItem v-for="dashboard in dashboards" :key="dashboard.id">
            <ThingsVisDashboardCard
              :dashboard="dashboard"
              :menu-config="menuConfigs[dashboard.id]"
              :menu-config-loaded="!dashboardMenuConfigAvailable || dashboard.id in menuConfigs"
              :thumbnail-url="getThumbnailUrl(dashboard.thumbnail)"
              :publishing="publishingId === dashboard.id"
              :duplicating="duplicatingId === dashboard.id"
              @edit="openEditor"
              @menu="openMenuConfig"
              @set-home="handleSetAsHomepage"
              @publish="handlePublishDashboard"
              @duplicate="handleDuplicateDashboard"
              @copy-link="copyDashboardViewerLink"
              @request-thumbnail="requestThumbnail"
              @request-menu-config="requestMenuConfig"
              @delete="openDeleteConfirm"
            />
          </NGridItem>
        </NGrid>
      </NSpin>
      </template>
    </NCard>

    <!-- 新建弹窗 -->
    <NModal v-model:show="showModal" preset="card" :title="$t('rdi.thingsvis.createDashboard')" class="w-500px">
      <NForm :model="formData">
        <NFormItem :label="$t('rdi.thingsvis.dashboardName')" path="name">
          <NInput
            v-model:value="formData.name"
            :placeholder="$t('rdi.thingsvis.dashboardNamePlaceholder')"
            maxlength="50"
            show-count
          />
        </NFormItem>

        <NFormItem :label="$t('rdi.thingsvis.template')">
          <NSelect v-model:value="formData.template" :options="dashboardTemplateOptions" />
        </NFormItem>

        <NFormItem :label="$t('rdi.thingsvis.canvasMode')">
          <div class="flex gap-2">
            <NButton
              :type="formData.canvasMode === 'fixed' ? 'primary' : 'default'"
              @click="formData.canvasMode = 'fixed'"
            >
              {{ $t('rdi.thingsvis.fixedSize') }}
            </NButton>
            <NButton
              :type="formData.canvasMode === 'grid' ? 'primary' : 'default'"
              @click="formData.canvasMode = 'grid'"
            >
              {{ $t('rdi.thingsvis.gridLayout') }}
            </NButton>
            <NButton
              :type="formData.canvasMode === 'infinite' ? 'primary' : 'default'"
              @click="formData.canvasMode = 'infinite'"
            >
              {{ $t('rdi.thingsvis.infiniteCanvas') }}
            </NButton>
          </div>
        </NFormItem>

        <NFormItem v-if="formData.canvasMode === 'fixed'" :label="$t('rdi.thingsvis.canvasSize')">
          <div class="flex items-center gap-2">
            <NInputNumber
              v-model:value="formData.canvasWidth"
              :placeholder="$t('rdi.thingsvis.width')"
              style="width: 120px"
              :show-button="false"
            />
            <span>×</span>
            <NInputNumber
              v-model:value="formData.canvasHeight"
              :placeholder="$t('rdi.thingsvis.height')"
              style="width: 120px"
              :show-button="false"
            />
            <span class="text-sm text-gray-400">px</span>
          </div>
          <div class="mt-2 text-xs text-gray-400">{{ $t('rdi.thingsvis.commonSizes') }}</div>
        </NFormItem>
      </NForm>

      <template #footer>
        <div class="flex justify-end gap-2">
          <NButton @click="showModal = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" @click="handleCreateDashboard">{{ $t('rdi.thingsvis.create') }}</NButton>
        </div>
      </template>
    </NModal>

    <!-- 系统菜单配置 -->
    <NModal v-model:show="showMenuModal" preset="card" :title="$t('rdi.thingsvis.systemMenuConfig')" class="w-500px">
      <NForm :model="menuForm">
        <NFormItem :label="$t('rdi.thingsvis.dashboard')">
          <NInput :value="menuForm.dashboardName" disabled />
        </NFormItem>

        <NFormItem :label="$t('rdi.thingsvis.setAsSystemMenu')">
          <NSwitch
            v-model:value="menuForm.enabled"
            @update:value="
              (value) => {
                if (value && !menuForm.menuName) menuForm.menuName = menuForm.dashboardName
              }
            "
          />
        </NFormItem>

        <NFormItem v-if="menuForm.enabled" :label="$t('rdi.thingsvis.menuName')">
          <NInput
            v-model:value="menuForm.menuName"
            :placeholder="$t('rdi.thingsvis.menuNamePlaceholder')"
            maxlength="50"
            show-count
          />
        </NFormItem>

        <NFormItem v-if="menuForm.enabled" :label="$t('rdi.thingsvis.menuSort')">
          <NInputNumber
            v-model:value="menuForm.menuSort"
            :min="1"
            :precision="0"
            :placeholder="$t('rdi.thingsvis.sortValue')"
            style="width: 160px"
          />
        </NFormItem>
      </NForm>

      <template #footer>
        <div class="flex justify-end gap-2">
          <NButton @click="showMenuModal = false">{{ $t('common.cancel') }}</NButton>
          <NButton type="primary" :loading="menuSaving" @click="handleSaveMenuConfig">
            {{ $t('rdi.thingsvis.saveMenu') }}
          </NButton>
        </div>
      </template>
    </NModal>

    <!-- 删除仪表盘确认弹窗 -->
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
        {{ $t('rdi.thingsvis.deleteConfirm', { name: pendingDeleteDashboard?.name || '' }) }}
      </template>
      <template #action>
        <NButton :disabled="!!deletingId" @click="deleteConfirmModal = false">{{ $t('common.cancel') }}</NButton>
        <NButton type="error" :loading="!!deletingId" @click="handleDeleteDashboard">
          {{ $t('rdi.thingsvis.confirmDeleteAction') }}
        </NButton>
      </template>
    </NModal>
  </div>
</template>

<style scoped>
.thingsvis-empty-action {
  display: grid;
  justify-items: center;
  gap: 12px;
  max-width: 420px;
}

.thingsvis-empty-action__hint {
  color: var(--text-color-3);
  font-size: 13px;
  line-height: 1.6;
  text-align: center;
}

.thingsvis-onboarding-dashboard {
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

.thingsvis-onboarding-dashboard__eyebrow {
  color: #2563eb;
  font-size: 12px;
  font-weight: 600;
}

.thingsvis-onboarding-dashboard__title {
  margin-top: 4px;
  color: #0f172a;
  font-size: 15px;
  font-weight: 700;
}

.thingsvis-onboarding-dashboard__desc {
  margin-top: 4px;
  color: #475569;
  font-size: 13px;
  line-height: 1.5;
}

@media (max-width: 640px) {
  .thingsvis-onboarding-dashboard {
    flex-direction: column;
  }
}
</style>
