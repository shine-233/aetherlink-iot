<!--
  文件用途：承载 frontend/src/views/dashboard/workspace/index.vue 对应的页面或局部组件视图。
  核心逻辑：组合模板、响应式状态、路由或局部组件，向用户呈现当前页面所需的主要内容和交互入口。
  关键注意事项：修改可见文案、路由依赖或交互分支时，要同步维护相邻测试和 README 职责说明。
  重构建议：当模板或脚本继续变长时，优先抽出局部组件或组合式函数，再用 focused tests 锁定行为一致性。
-->
<script setup lang="ts">
import { $t } from '@/locales'

const thingsVisEntrypoints = [
  {
    key: 'projects',
    title: $t('custom.dashboardWorkspace.projectsTitle'),
    description: $t('custom.dashboardWorkspace.projectsDesc'),
    path: '/visualization/thingsvis'
  },
  {
    key: 'dashboards',
    title: $t('custom.dashboardWorkspace.dashboardsTitle'),
    description: $t('custom.dashboardWorkspace.dashboardsDesc'),
    path: '/visualization/thingsvis-dashboards'
  },
  {
    key: 'workbench',
    title: $t('custom.dashboardWorkspace.workbenchTitle'),
    description: $t('custom.dashboardWorkspace.workbenchDesc'),
    path: '/dashboard/workbench'
  }
]

const thingsVisCompatEnabled = import.meta.env.VITE_ENABLE_THINGSVIS_COMPAT === 'Y'
const nativeEntrypoint = {
  key: 'native-boards',
  title: $t('custom.nativeBoards.title'),
  description: $t('custom.nativeBoards.subtitle'),
  path: '/visualization/native-boards'
}
const visualizationEntrypoints = thingsVisCompatEnabled
  ? thingsVisEntrypoints
  : [nativeEntrypoint, thingsVisEntrypoints[2]]
const availableVisualizationEntrypoints = visualizationEntrypoints
const workspaceDescription = thingsVisCompatEnabled
  ? $t('custom.dashboardWorkspace.description')
  : $t('custom.nativeBoards.subtitle')
const handoffDescription = thingsVisCompatEnabled
  ? $t('custom.dashboardWorkspace.handoffDesc')
  : $t('custom.nativeBoards.subtitle')
</script>

<template>
  <section class="dashboard-workspace-page">
    <div class="workspace-hero">
      <p class="workspace-eyebrow">{{ $t('custom.dashboardWorkspace.eyebrow') }}</p>
      <h1>{{ $t('custom.dashboardWorkspace.title') }}</h1>
      <p class="workspace-description">{{ workspaceDescription }}</p>
    </div>

    <div class="entry-grid">
      <RouterLink
        v-for="entry in availableVisualizationEntrypoints"
        :key="entry.key"
        class="entry-card"
        :to="entry.path"
      >
        <span class="entry-title">{{ entry.title }}</span>
        <span class="entry-description">{{ entry.description }}</span>
      </RouterLink>
    </div>

    <div class="handoff-note">
      <strong>{{ $t('custom.dashboardWorkspace.handoffTitle') }}</strong>
      <span>{{ handoffDescription }}</span>
    </div>
  </section>
</template>

<style scoped>
.dashboard-workspace-page {
  display: grid;
  gap: 16px;
}

.workspace-hero {
  border: 1px solid rgb(var(--border-color));
  border-radius: 8px;
  background: rgb(var(--card-color));
  padding: 24px;
}

.workspace-eyebrow {
  margin: 0 0 8px;
  color: rgb(var(--primary-color));
  font-size: 13px;
  font-weight: 600;
}

.workspace-hero h1 {
  margin: 0;
  font-size: 24px;
  line-height: 1.3;
}

.workspace-description {
  max-width: 760px;
  margin: 12px 0 0;
  color: rgb(var(--text-color-2));
  line-height: 1.7;
}

.entry-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
}

.entry-card {
  display: grid;
  gap: 8px;
  min-height: 120px;
  border: 1px solid rgb(var(--border-color));
  border-radius: 8px;
  background: rgb(var(--card-color));
  padding: 18px;
  color: inherit;
  text-decoration: none;
  transition:
    border-color 0.2s ease,
    transform 0.2s ease;
}

.entry-card:hover {
  border-color: rgb(var(--primary-color));
  transform: translateY(-1px);
}

.entry-title {
  font-size: 16px;
  font-weight: 700;
}

.entry-description {
  color: rgb(var(--text-color-2));
  line-height: 1.6;
}

.handoff-note {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  border-radius: 8px;
  background: rgb(var(--info-color-suppl));
  padding: 14px 16px;
  color: rgb(var(--text-color-2));
}

.handoff-note strong {
  color: rgb(var(--text-color-1));
}
</style>
