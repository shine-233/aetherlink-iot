<!--
文件用途：提供全局内容容器。
核心逻辑：承载路由页面并协调布局滚动、缓存和内容样式。
关键注意事项：该组件是页面内容的壳层，不应直接包含业务判断。
重构建议：可继续拆分滚动容器和 keep-alive 控制。
-->
<script setup lang="ts">
import { useAppStore } from '@/store/modules/app'
import { useRouteStore } from '@/store/modules/route'

defineOptions({
  name: 'GlobalContent'
})

interface Props {
  /** Show padding for content */
  showPadding?: boolean
}

withDefaults(defineProps<Props>(), {
  showPadding: true
})

const appStore = useAppStore()
const routeStore = useRouteStore()
</script>

<template>
  <RouterView v-slot="{ Component, route }">
    <!-- KeepAlive stays directly under RouterView because route transitions previously caused blank automation pages. -->
    <KeepAlive :include="routeStore.cacheRoutes">
      <Transition name="fade-slide" mode="out-in">
        <component
          :is="Component"
          v-if="appStore.reloadFlag"
          :key="route.path"
          :class="{ 'p-16px': showPadding }"
          class="flex-grow bg-layout transition-300"
        />
      </Transition>
    </KeepAlive>
  </RouterView>
</template>

<style></style>
